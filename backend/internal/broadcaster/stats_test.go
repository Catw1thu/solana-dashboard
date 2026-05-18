package broadcaster

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/realtime"
	"solana-dashboard-go/internal/store"
)

type mockTradeMetricsStore struct {
	tradeMetrics  map[string][]store.TradeMetricPoint
	recentMetrics map[string][]store.TradeMetricPoint
	longMetrics   map[string]store.LongWindowMetricsRecord
	latestSeeds   map[string]store.LatestTradeSeedRecord
	candidates    []string
	upserts       []store.TokenMetricsCurrentRecord
}

func (m *mockTradeMetricsStore) ListTradeMetricsForStatsByMint(ctx context.Context, mint string) ([]store.TradeMetricPoint, error) {
	return m.tradeMetrics[mint], nil
}

func (m *mockTradeMetricsStore) ListRecentTradeMetricsByMint(ctx context.Context, mint string, sinceUnix int64) ([]store.TradeMetricPoint, error) {
	return m.recentMetrics[mint], nil
}

func (m *mockTradeMetricsStore) ListLongWindowMetricsByMints(ctx context.Context, mints []string, nowTs int64) (map[string]store.LongWindowMetricsRecord, error) {
	result := make(map[string]store.LongWindowMetricsRecord, len(mints))
	for _, mint := range mints {
		if record, ok := m.longMetrics[mint]; ok {
			result[mint] = record
		}
	}
	return result, nil
}

func (m *mockTradeMetricsStore) ListMetricBackfillCandidates(ctx context.Context, limit int) ([]string, error) {
	if len(m.candidates) <= limit {
		return m.candidates, nil
	}
	return m.candidates[:limit], nil
}

func (m *mockTradeMetricsStore) ListLatestTradeSeedsByMints(ctx context.Context, mints []string) (map[string]store.LatestTradeSeedRecord, error) {
	result := make(map[string]store.LatestTradeSeedRecord, len(mints))
	for _, mint := range mints {
		if record, ok := m.latestSeeds[mint]; ok {
			result[mint] = record
		}
	}
	return result, nil
}

func (m *mockTradeMetricsStore) UpsertTokenMetricsCurrent(ctx context.Context, records []store.TokenMetricsCurrentRecord) error {
	m.upserts = append(m.upserts, records...)
	return nil
}

func (m *mockTradeMetricsStore) LoadTokenMetricsCurrentByMint(ctx context.Context, mint string) (*store.TokenMetricsCurrentRecord, error) {
	for _, record := range m.upserts {
		if record.Mint == mint {
			copy := record
			return &copy, nil
		}
	}
	return nil, nil
}

func TestBuildPayloadFromTradeMetricsFallsBackToEarliestTradeForYoungToken(t *testing.T) {
	nowTs := int64(1_700_300_000)
	payload, ok := BuildPayloadFromTradeMetrics("mint_1", []store.TradeMetricPoint{
		{
			EventUnixTS: nowTs - 2400,
			Side:        "buy",
			Price:       0.0000003,
			Volume:      0.1,
		},
		{
			EventUnixTS: nowTs - 10,
			Side:        "buy",
			Price:       0.00001,
			Volume:      0.2,
		},
	}, nowTs)
	if !ok {
		t.Fatal("expected payload to be built")
	}
	if payload.P1h != 0.0000003 || payload.P4h != 0.0000003 || payload.P24h != 0.0000003 {
		t.Fatalf("expected seeded anchors from DB metrics, got p1h=%f p4h=%f p24h=%f", payload.P1h, payload.P4h, payload.P24h)
	}
}

func TestEnsureWindowSeededLoadsRecentTrades(t *testing.T) {
	mint := "mint_1"
	nowTs := int64(1_700_400_000)

	store := &mockTradeMetricsStore{
		recentMetrics: map[string][]store.TradeMetricPoint{
			mint: {
				{
					EventUnixTS: nowTs - 120,
					Side:        "buy",
					Price:       1.2,
					Volume:      3,
				},
				{
					EventUnixTS: nowTs - 15,
					Side:        "sell",
					Price:       1.3,
					Volume:      1,
				},
			},
		},
	}

	b := &Broadcaster{
		hub:          realtime.NewHub(),
		metricsStore: store,
		windows:      make(map[string]*TokenWindowState),
	}

	b.ensureWindowSeeded(mint, nowTs)
	window := b.getWindow(mint)
	window.mu.Lock()
	defer window.mu.Unlock()

	if !window.Seeded {
		t.Fatal("expected window to be marked seeded")
	}
	if window.LastPrice != 1.3 {
		t.Fatalf("expected last seeded price 1.3, got %f", window.LastPrice)
	}
}

func TestHandleTradeAcceptsPumpAmmSwap(t *testing.T) {
	hub := realtime.NewHub()
	b := &Broadcaster{
		hub:     hub,
		windows: make(map[string]*TokenWindowState),
	}

	mint := "Token111111111111111111111111111111111111"
	baseMint := wrappedSolMint

	payload := events.PumpAmmSwapPayload{
		Side:           "sell",
		BaseMint:       baseMint,
		QuoteMint:      mint,
		BaseAmountIn:   stringPtr("1000000000"),
		QuoteAmountOut: stringPtr("500000000"),
	}
	bytes, _ := json.Marshal(payload)

	env := events.Envelope{
		Protocol:    "pumpamm",
		EventType:   "swap",
		Payload:     json.RawMessage(bytes),
		EventUnixTS: time.Now().Unix(),
		Refs: events.EventRefs{
			Mint: stringPtr(mint),
		},
	}

	b.handleTrade(&env)

	window := b.getWindow(mint)
	window.mu.Lock()
	defer window.mu.Unlock()
	if window.LastPrice <= 0 {
		t.Fatalf("expected positive price, got %f", window.LastPrice)
	}
}

func TestSweepPublishesOnlySubscribedTokenStats(t *testing.T) {
	hub := realtime.NewHub()
	mintSubscribed := "mint_sub"
	mintUnsubscribed := "mint_unsub"
	nowTs := time.Now().Unix()

	store := &mockTradeMetricsStore{
		longMetrics: map[string]store.LongWindowMetricsRecord{
			mintSubscribed:   {Mint: mintSubscribed},
			mintUnsubscribed: {Mint: mintUnsubscribed},
		},
	}

	b := &Broadcaster{
		hub:          hub,
		metricsStore: store,
		windows:      make(map[string]*TokenWindowState),
	}

	b.recordTrade(mintSubscribed, true, 2, 1.1, nowTs-5, 11)
	b.recordTrade(mintUnsubscribed, true, 2, 1.2, nowTs-5, 12)

	sub := hub.Subscribe("token:"+mintSubscribed, 4)
	defer hub.Unsubscribe(sub)

	b.Sweep()

	select {
	case event := <-sub.Events:
		if event.EventType != "token_stat" {
			t.Fatalf("expected token_stat, got %s", event.EventType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for subscribed token_stat")
	}

	if len(store.upserts) != 2 {
		t.Fatalf("expected 2 current metric upserts, got %d", len(store.upserts))
	}
}

func TestBackfillCurrentMetricsSeedsMissingRows(t *testing.T) {
	nowTs := time.Now().Unix()
	store := &mockTradeMetricsStore{
		candidates: []string{"mint_1"},
		longMetrics: map[string]store.LongWindowMetricsRecord{
			"mint_1": {Mint: "mint_1"},
		},
		latestSeeds: map[string]store.LatestTradeSeedRecord{
			"mint_1": {
				Mint:            "mint_1",
				LatestPrice:     floatPtr(1.25),
				LatestEventUnix: int64Ptr(nowTs - 10),
			},
		},
		recentMetrics: map[string][]store.TradeMetricPoint{
			"mint_1": {
				{
					EventUnixTS: nowTs - 20,
					Side:        "buy",
					Price:       1.2,
					Volume:      2,
				},
			},
		},
	}

	b := &Broadcaster{
		hub:          realtime.NewHub(),
		metricsStore: store,
		windows:      make(map[string]*TokenWindowState),
	}

	if err := b.BackfillCurrentMetrics(context.Background(), 10); err != nil {
		t.Fatalf("BackfillCurrentMetrics returned error: %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upserted current metrics row, got %d", len(store.upserts))
	}
	if store.upserts[0].Mint != "mint_1" {
		t.Fatalf("expected mint_1 backfill record, got %s", store.upserts[0].Mint)
	}
}

func TestRunResubscribesGlobalHubAfterOverflow(t *testing.T) {
	hub := realtime.NewHub()
	mint := "mint_resub"
	nowTs := time.Now().Unix()
	store := &mockTradeMetricsStore{
		candidates: []string{mint},
		longMetrics: map[string]store.LongWindowMetricsRecord{
			mint: {Mint: mint},
		},
		latestSeeds: map[string]store.LatestTradeSeedRecord{
			mint: {
				Mint:            mint,
				LatestPrice:     floatPtr(1),
				LatestEventUnix: int64Ptr(nowTs),
			},
		},
	}
	b := &Broadcaster{
		hub:             hub,
		metricsStore:    store,
		windows:         make(map[string]*TokenWindowState),
		globalHubBuffer: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- b.Run(ctx)
	}()

	waitForSubscriber(t, hub, "global")

	first := makePumpfunTradeEnvelope(mint, "event_first", nowTs)
	second := makePumpfunTradeEnvelope(mint, "event_second", nowTs+1)
	hub.Publish("global", first)
	hub.Publish("global", second)

	waitForSubscriber(t, hub, "global")

	third := makePumpfunTradeEnvelope(mint, "event_third", nowTs+2)
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for trade after resubscribe")
		case <-ticker.C:
			hub.Publish("global", third)
			window := b.getWindow(mint)
			window.mu.Lock()
			lastLogID := window.LastLogID
			window.mu.Unlock()
			if lastLogID == third.LogID {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("Run returned error: %v", err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("timed out waiting for broadcaster shutdown")
				}
				return
			}
		}
	}
}

func waitForSubscriber(t *testing.T, hub *realtime.Hub, topic string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for subscriber on topic %s", topic)
		case <-ticker.C:
			if hub.HasSubscribers(topic) {
				return
			}
		}
	}
}

func makePumpfunTradeEnvelope(mint string, eventID string, eventUnixTS int64) events.Envelope {
	payload := events.PumpfunTradePayload{
		Mint:         mint,
		Side:         "buy",
		SolAmount:    "1000000000",
		TokenAmount:  "1000000",
		BondingCurve: "curve",
	}
	bytes, _ := json.Marshal(payload)
	return events.Envelope{
		LogID:       eventUnixTS,
		EventID:     eventID,
		Protocol:    "pumpfun",
		EventType:   "trade",
		EventUnixTS: eventUnixTS,
		Payload:     json.RawMessage(bytes),
	}
}

func stringPtr(v string) *string {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
