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
	"solana-dashboard-go/internal/query"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type WSMessage struct {
	Action string `json:"action"` // "subscribe", "unsubscribe"
	Topic  string `json:"topic"`
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	outCh := make(chan events.Envelope, 256)

	var subsMu sync.Mutex
	subs := make(map[string]chan events.Envelope)

	defer func() {
		subsMu.Lock()
		defer subsMu.Unlock()
		for _, ch := range subs {
			h.service.Unsubscribe(ch)
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
					topicCh := h.service.Subscribe(msg.Topic, 64)
					subs[msg.Topic] = topicCh

					if snapshot, err := h.tokenStatSnapshot(msg.Topic); err == nil && snapshot != nil {
						select {
						case outCh <- *snapshot:
						default:
						}
					}

					go func(tc chan events.Envelope) {
						for {
							select {
							case <-ctx.Done():
								return
							case ev, ok := <-tc:
								if !ok {
									return
								}
								select {
								case outCh <- ev:
								default:
								}
							}
						}
					}(topicCh)
				}
				subsMu.Unlock()
				continue
			}

			if msg.Action == "unsubscribe" && msg.Topic != "" {
				subsMu.Lock()
				if ch, exists := subs[msg.Topic]; exists {
					delete(subs, msg.Topic)
					h.service.Unsubscribe(ch)
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
		case ev := <-outCh:
			if err := wsjson.Write(ctx, conn, ev); err != nil {
				return
			}
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
