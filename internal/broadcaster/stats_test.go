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
	metrics map[string][]store.TradeMetricPoint
}

func (m *mockTradeMetricsStore) ListTradeMetricsForStatsByMint(ctx context.Context, mint string) ([]store.TradeMetricPoint, error) {
	return m.metrics[mint], nil
}

func TestSweepExpiry(t *testing.T) {
	hub := realtime.NewHub()
	b := &Broadcaster{
		nc:      nil,
		hub:     hub,
		windows: make(map[string]*TokenSlidingWindow),
	}

	mint := "Token111111111111111111111111111111111111"

	// Create mock trade exactly 4 minutes and 58 seconds ago
	nowTs := time.Now().Unix()
	pastTs := nowTs - 298

	p := events.PumpfunTradePayload{
		Mint:        mint,
		SolAmount:   "1000000000",
		TokenAmount: "1000000000",
		Side:        "buy",
	}
	bytes, _ := json.Marshal(p)

	env := events.Envelope{
		Protocol:    "pumpfun",
		EventType:   "trade",
		Payload:     json.RawMessage(bytes),
		EventUnixTS: pastTs,
	}

	b.handleTrade(&env)

	w := b.getWindow(mint)
	if len(w.ShortTrades) != 1 {
		t.Fatalf("expected 1 short trade, got %d", len(w.ShortTrades))
	}

	// Verify sweeping immediately does not remove it
	b.Sweep()
	w = b.getWindow(mint)
	if len(w.ShortTrades) != 1 {
		t.Fatalf("expected 1 short trade after immediate sweep, got %d", len(w.ShortTrades))
	}

	// Wait 3 seconds so it ages out to strictly greater than 5 minutes old
	time.Sleep(3 * time.Second)

	// Sweep again
	b.Sweep()
	w = b.getWindow(mint)
	if len(w.ShortTrades) != 1 {
		t.Fatalf("expected 1 retained anchor trade after 5m expiry, got %d", len(w.ShortTrades))
	}
	if w.ShortTrades[0].Timestamp != pastTs {
		t.Fatalf("expected retained anchor trade timestamp %d, got %d", pastTs, w.ShortTrades[0].Timestamp)
	}

	t.Log("Successfully verified 5m short trades retain one pre-window anchor trade")
}

func TestHandleTradeAcceptsPumpAmmSwap(t *testing.T) {
	hub := realtime.NewHub()
	b := &Broadcaster{
		hub:     hub,
		windows: make(map[string]*TokenSlidingWindow),
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
	if len(window.ShortTrades) != 1 {
		t.Fatalf("expected 1 short trade, got %d", len(window.ShortTrades))
	}
	if !window.ShortTrades[0].IsBuy {
		t.Fatalf("expected trade to be normalized as buy for tracked token")
	}
	if window.ShortTrades[0].Price <= 0 {
		t.Fatalf("expected positive price, got %f", window.ShortTrades[0].Price)
	}
}

func TestRunPublishesTokenStatsFromGlobalHub(t *testing.T) {
	hub := realtime.NewHub()
	b := &Broadcaster{
		hub:     hub,
		windows: make(map[string]*TokenSlidingWindow),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- b.Run(ctx)
	}()

	mint := "Token111111111111111111111111111111111111"
	tokenSub := hub.Subscribe("token:"+mint, 4)
	defer hub.Unsubscribe(tokenSub)

	payload := events.PumpfunTradePayload{
		Mint:        mint,
		SolAmount:   "1000000000",
		TokenAmount: "1000000",
		Side:        "buy",
	}
	bytes, _ := json.Marshal(payload)
	time.Sleep(10 * time.Millisecond)
	hub.Publish("global", events.Envelope{
		Protocol:    "pumpfun",
		EventType:   "trade",
		Payload:     json.RawMessage(bytes),
		EventUnixTS: time.Now().Unix(),
	})

	timeout := time.After(2 * time.Second)
	for {
		select {
		case err := <-done:
			t.Fatalf("broadcaster exited early: %v", err)
		case event := <-tokenSub.Events:
			if event.EventType == "token_stat" {
				return
			}
		case <-timeout:
			t.Fatal("timed out waiting for token_stat event")
		}
	}
}

func TestGeneratePayloadFallsBackToEarliestTradeForYoungToken(t *testing.T) {
	b := &Broadcaster{
		hub:     realtime.NewHub(),
		windows: make(map[string]*TokenSlidingWindow),
	}

	mint := "YoungToken11111111111111111111111111111111"
	startTs := int64(1_700_000_000)

	b.recordTrade(mint, true, 0.1, 0.0000002801, startTs)
	b.recordTrade(mint, true, 0.2, 0.0000040, startTs+600)

	window := b.getWindow(mint)
	window.mu.Lock()
	payload := b.generatePayload(mint, window, startTs+600)
	window.mu.Unlock()

	if payload.P24h != 0.0000002801 {
		t.Fatalf("expected earliest trade to anchor 24h change, got %f", payload.P24h)
	}

	change := ((payload.Price - payload.P24h) / payload.P24h) * 100
	if change < 1000 {
		t.Fatalf("expected >=1000%% gain for young token, got %.2f%%", change)
	}
}

func TestGeneratePayloadUsesExactBucketVolumes(t *testing.T) {
	b := &Broadcaster{
		hub:     realtime.NewHub(),
		windows: make(map[string]*TokenSlidingWindow),
	}

	mint := "VolumeToken111111111111111111111111111111"
	startTs := int64(1_700_100_000)

	b.recordTrade(mint, true, 12, 1.0, startTs)
	b.recordTrade(mint, false, 3, 1.2, startTs+120)

	window := b.getWindow(mint)
	window.mu.Lock()
	payload := b.generatePayload(mint, window, startTs+180)
	window.mu.Unlock()

	if payload.BV1h != 12 {
		t.Fatalf("expected exact 1h buy volume 12, got %f", payload.BV1h)
	}
	if payload.SV1h != 3 {
		t.Fatalf("expected exact 1h sell volume 3, got %f", payload.SV1h)
	}
	if payload.BV4h != 12 || payload.SV4h != 3 {
		t.Fatalf("expected exact 4h buy/sell volume 12/3, got %f/%f", payload.BV4h, payload.SV4h)
	}
}

func TestEnsureWindowSeededRestoresLongWindowAnchorFromDatabase(t *testing.T) {
	nowTs := int64(1_700_200_000)
	mint := "RestartedToken11111111111111111111111111111"

	metricsStore := &mockTradeMetricsStore{
		metrics: map[string][]store.TradeMetricPoint{
			mint: {
				{
					EventUnixTS: nowTs - 2400,
					Side:        "buy",
					Price:       0.0000003,
					Volume:      0.1,
				},
				{
					EventUnixTS: nowTs - 30,
					Side:        "buy",
					Price:       0.00001,
					Volume:      0.2,
				},
			},
		},
	}

	b := &Broadcaster{
		hub:          realtime.NewHub(),
		metricsStore: metricsStore,
		windows:      make(map[string]*TokenSlidingWindow),
	}

	b.ensureWindowSeeded(mint, nowTs)

	window := b.getWindow(mint)
	window.mu.Lock()
	payload := b.generatePayload(mint, window, nowTs)
	window.mu.Unlock()

	if payload.P1h != 0.0000003 {
		t.Fatalf("expected seeded 1h anchor 0.0000003, got %f", payload.P1h)
	}
	if payload.P4h != 0.0000003 || payload.P24h != 0.0000003 {
		t.Fatalf("expected seeded 4h/24h anchors 0.0000003, got %f/%f", payload.P4h, payload.P24h)
	}

	change := ((payload.Price - payload.P1h) / payload.P1h) * 100
	if change < 3000 {
		t.Fatalf("expected >=3000%% gain after DB seed restore, got %.2f%%", change)
	}
}

func stringPtr(v string) *string {
	return &v
}
