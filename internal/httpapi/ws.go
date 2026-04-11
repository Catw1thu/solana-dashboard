package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"solana-dashboard-go/internal/broadcaster"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/observability"
	"solana-dashboard-go/internal/query"
	"solana-dashboard-go/internal/realtime"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var errReplayBackpressure = errors.New("ws replay backpressure")

type WSMessage struct {
	Action     string `json:"action"` // "subscribe", "unsubscribe"
	Topic      string `json:"topic"`
	SinceLogID *int64 `json:"since_log_id,omitempty"`
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	observability.Default().AddGauge("ws_connections", 1)
	defer observability.Default().AddGauge("ws_connections", -1)

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	closeOnce := sync.Once{}
	closeConn := func(status websocket.StatusCode, reason string) {
		closeOnce.Do(func() {
			_ = conn.Close(status, reason)
			cancel()
		})
	}

	outCh := make(chan events.Envelope, 256)

	var subsMu sync.Mutex
	subs := make(map[string]*realtime.Subscription)

	defer func() {
		subsMu.Lock()
		defer subsMu.Unlock()
		observability.Default().AddGauge("ws_topic_subscriptions", -int64(len(subs)))
		for _, sub := range subs {
			h.service.Unsubscribe(sub)
		}
		if h.service != nil {
			observability.Default().SetGauge("ws_hub_subscribers", int64(h.service.SubscribeCount()))
		}
	}()

	// Read loop
	go func() {
		defer cancel()
		for {
			var msg WSMessage
			if err := wsjson.Read(ctx, conn, &msg); err != nil {
				return
			}

			if msg.Action == "subscribe" && msg.Topic != "" {
				subsMu.Lock()
				if _, exists := subs[msg.Topic]; !exists {
					topicSub := h.service.Subscribe(msg.Topic, 64)
					subs[msg.Topic] = topicSub
					observability.Default().AddGauge("ws_topic_subscriptions", 1)
					if h.service != nil {
						observability.Default().SetGauge("ws_hub_subscribers", int64(h.service.SubscribeCount()))
					}

					if snapshot, err := h.tokenStatSnapshot(msg.Topic); err == nil && snapshot != nil {
						select {
						case outCh <- *snapshot:
						default:
						}
					}

					go func(topic string, sub *realtime.Subscription, sinceLogID *int64) {
						var (
							replayReady = sinceLogID == nil || *sinceLogID <= 0 || !strings.HasPrefix(topic, "token:")
							replayMax   int64
							buffered    []events.Envelope
							replayDone  = make(chan int64, 1)
						)

						if !replayReady {
							replayMax = *sinceLogID
							go func(afterLogID int64) {
								observability.Default().IncCounter("ws_replay_requests_total", 1)
								replayedMax, err := h.replayTopic(ctx, topic, afterLogID, outCh)
								if err != nil {
									status := websocket.StatusInternalError
									reason := "replay_failed"
									if errors.Is(err, errReplayBackpressure) {
										status = websocket.StatusPolicyViolation
										reason = "replay_overflow"
										observability.Default().IncCounter("ws_replay_overflow_total", 1)
									}
									closeConn(status, reason)
									return
								}
								replayDone <- replayedMax
							}(replayMax)
						}

						for {
							select {
							case <-ctx.Done():
								return
							case _, ok := <-sub.Overflow:
								if !ok {
									return
								}
								observability.Default().IncCounter("ws_subscriber_overflow_total", 1)
								closeConn(websocket.StatusPolicyViolation, "subscriber_overflow")
								return
							case replayedMax := <-replayDone:
								replayMax = replayedMax
								replayReady = true
								for _, ev := range buffered {
									if ev.LogID <= replayMax {
										continue
									}
									if !enqueueEnvelope(ctx, outCh, ev) {
										observability.Default().IncCounter("ws_writer_overflow_total", 1)
										closeConn(websocket.StatusPolicyViolation, "writer_overflow")
										return
									}
								}
								buffered = nil
							case ev, ok := <-sub.Events:
								if !ok {
									return
								}
								if !replayReady {
									buffered = append(buffered, ev)
									continue
								}
								if !enqueueEnvelope(ctx, outCh, ev) {
									closeConn(websocket.StatusPolicyViolation, "writer_overflow")
									return
								}
							}
						}
					}(msg.Topic, topicSub, msg.SinceLogID)
				}
				subsMu.Unlock()
				continue
			}

			if msg.Action == "unsubscribe" && msg.Topic != "" {
				subsMu.Lock()
				if sub, exists := subs[msg.Topic]; exists {
					delete(subs, msg.Topic)
					h.service.Unsubscribe(sub)
					observability.Default().AddGauge("ws_topic_subscriptions", -1)
					if h.service != nil {
						observability.Default().SetGauge("ws_hub_subscribers", int64(h.service.SubscribeCount()))
					}
				}
				subsMu.Unlock()
			}
		}
	}()

	// Write loop
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-outCh:
			if !ok {
				return
			}
			if err := wsjson.Write(ctx, conn, ev); err != nil {
				return
			}
		}
	}
}

func enqueueEnvelope(ctx context.Context, outCh chan<- events.Envelope, ev events.Envelope) bool {
	select {
	case <-ctx.Done():
		return false
	case outCh <- ev:
		return true
	default:
		return false
	}
}

func (h *Handler) replayTopic(ctx context.Context, topic string, afterLogID int64, outCh chan<- events.Envelope) (int64, error) {
	if h.eventQuery == nil || !strings.HasPrefix(topic, "token:") {
		return afterLogID, nil
	}

	mint := strings.TrimPrefix(topic, "token:")
	if mint == "" {
		return afterLogID, nil
	}

	lastLogID := afterLogID
	const batchSize = 512
	for {
		eventsList, err := h.eventQuery.ListServiceEventsByMintAfterLogID(ctx, mint, lastLogID, batchSize)
		if err != nil {
			return lastLogID, err
		}
		if len(eventsList) == 0 {
			return lastLogID, nil
		}

		for _, event := range eventsList {
			if event.LogID > lastLogID {
				lastLogID = event.LogID
			}
			if !enqueueEnvelope(ctx, outCh, event) {
				return lastLogID, errReplayBackpressure
			}
			observability.Default().IncCounter("ws_replayed_events_total", 1)
		}

		if len(eventsList) < batchSize {
			return lastLogID, nil
		}
	}
}

func (h *Handler) tokenStatSnapshot(topic string) (*events.Envelope, error) {
	if h.eventQuery == nil || !strings.HasPrefix(topic, "token:") {
		return nil, nil
	}

	mint := strings.TrimPrefix(topic, "token:")
	if mint == "" {
		return nil, nil
	}

	nowTs := time.Now().Unix()
	if payload, err := h.eventQuery.BuildRealtimeStatsPayload(context.Background(), mint, nowTs); err == nil && payload != nil {
		bytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		return &events.Envelope{
			SchemaVersion: 1,
			EventID:       "realtime:token_stat:" + mint,
			Protocol:      "realtime",
			EventType:     "token_stat",
			EventUnixTS:   payload.Timestamp,
			Refs: events.EventRefs{
				Mint: &mint,
			},
			Payload: bytes,
		}, nil
	} else if err != nil && !errors.Is(err, query.ErrTokenNotFound) {
		return nil, err
	}

	detail, err := h.eventQuery.GetTokenDetail(context.Background(), mint)
	if err != nil {
		if errors.Is(err, query.ErrTokenNotFound) {
			return nil, nil
		}
		return nil, err
	}

	latestPrice := 0.0
	if detail.MarketMetrics != nil && detail.MarketMetrics.LatestPrice != nil {
		latestPrice = *detail.MarketMetrics.LatestPrice
	}
	if latestPrice <= 0 {
		return nil, nil
	}

	timestamp := nowTs
	if detail.MarketMetrics != nil && detail.MarketMetrics.LatestEventUnix != nil {
		timestamp = *detail.MarketMetrics.LatestEventUnix
	}

	stats := detail.Stats24h
	payload := broadcaster.TokenStatsPayload{
		Mint:      mint,
		Price:     latestPrice,
		Timestamp: timestamp,
		P1m:       latestPrice,
		P5m:       anchorPrice(latestPrice, detail.PriceChanges, func(v *query.TokenPriceChanges) *float64 { return v.M5 }),
		P1h:       anchorPrice(latestPrice, detail.PriceChanges, func(v *query.TokenPriceChanges) *float64 { return v.H1 }),
		P4h:       anchorPrice(latestPrice, detail.PriceChanges, func(v *query.TokenPriceChanges) *float64 { return v.H4 }),
		P24h:      anchorPrice(latestPrice, detail.PriceChanges, func(v *query.TokenPriceChanges) *float64 { return v.H24 }),
	}
	if stats != nil {
		payload.B24h = int(stats.Buys)
		payload.S24h = int(stats.Sells)
		payload.BV24h = stats.BuyVolume
		payload.SV24h = stats.SellVolume
		payload.V24h = stats.Volume
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &events.Envelope{
		SchemaVersion: 1,
		EventID:       "realtime:token_stat:" + mint,
		Protocol:      "realtime",
		EventType:     "token_stat",
		EventUnixTS:   timestamp,
		Refs: events.EventRefs{
			Mint: &mint,
		},
		Payload: bytes,
	}, nil
}

func anchorPrice(latest float64, changes *query.TokenPriceChanges, getter func(*query.TokenPriceChanges) *float64) float64 {
	if latest <= 0 || changes == nil {
		return latest
	}

	change := getter(changes)
	if change == nil {
		return latest
	}

	divisor := 1 + (*change / 100)
	if divisor <= 0 {
		return latest
	}

	return latest / divisor
}
