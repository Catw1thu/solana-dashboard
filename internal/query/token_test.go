package query

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/store"
)

type mockTokenEventReader struct {
	events      []events.Envelope
	createEvent *events.Envelope
	err         error
}

func (m *mockTokenEventReader) ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.events) <= limit {
		return m.events, nil
	}
	return m.events[:limit], nil
}

func (m *mockTokenEventReader) FindLatestCreateEventByMint(ctx context.Context, mint string) (*events.Envelope, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.createEvent, nil
}

type mockTokenReadModel struct {
	snapshots  []store.TokenSnapshotRecord
	snapshot   *store.TokenSnapshotRecord
	markets    []store.TokenMarketRecord
	trades     []store.TradeEventRecord
	activities []store.ActivityEventRecord
	summary    *store.TradeSummaryRecord
	candles    []store.TokenCandleRecord
	err        error
}

func (m *mockTokenReadModel) ListTokenSnapshots(ctx context.Context, limit int) ([]store.TokenSnapshotRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.snapshots) <= limit {
		return m.snapshots, nil
	}
	return m.snapshots[:limit], nil
}

func (m *mockTokenReadModel) FindTokenSnapshotByMint(ctx context.Context, mint string) (*store.TokenSnapshotRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.snapshot != nil {
		return m.snapshot, nil
	}
	for _, item := range m.snapshots {
		if item.Mint == mint {
			found := item
			return &found, nil
		}
	}
	return nil, nil
}

func (m *mockTokenReadModel) ListTokenMarketsByMint(ctx context.Context, mint string, limit int) ([]store.TokenMarketRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.markets) <= limit {
		return m.markets, nil
	}
	return m.markets[:limit], nil
}

func (m *mockTokenReadModel) ListTradeEventsByMint(ctx context.Context, mint string, limit int) ([]store.TradeEventRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.trades) <= limit {
		return m.trades, nil
	}
	return m.trades[:limit], nil
}

func (m *mockTokenReadModel) ListActivityEventsByMint(ctx context.Context, mint string, limit int) ([]store.ActivityEventRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.activities) <= limit {
		return m.activities, nil
	}
	return m.activities[:limit], nil
}

func (m *mockTokenReadModel) LoadTradeSummaryByMint(ctx context.Context, mint string) (*store.TradeSummaryRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.summary, nil
}

func (m *mockTokenReadModel) ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int) ([]store.TokenCandleRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.candles) <= limit {
		return m.candles, nil
	}
	return m.candles[:limit], nil
}

func TestListTokensBuildsNewTokenList(t *testing.T) {
	mint := "mint_1"
	service := NewTokenService(nil, &mockTokenReadModel{
		snapshots: []store.TokenSnapshotRecord{
			{
				Mint:         mint,
				Name:         stringPtr("Token One"),
				Symbol:       stringPtr("ONE"),
				FirstSeenAt:  1770000001,
				CurrentStage: "pool",
			},
		},
		summary: &store.TradeSummaryRecord{
			LatestPrice:     floatPtr(0.02),
			LatestEventUnix: int64Ptr(1770000200),
			Txns24h:         12,
			Buys24h:         7,
			Sells24h:        5,
			Volume24h:       42,
			BuyVolume24h:    25,
			SellVolume24h:   17,
			Makers24h:       8,
			Buyers24h:       5,
			Sellers24h:      4,
		},
	})

	items, err := service.ListTokens(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListTokens returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Mint != mint {
		t.Fatalf("expected mint=%s, got %s", mint, items[0].Mint)
	}
	if items[0].LatestPrice == nil || *items[0].LatestPrice != 0.02 {
		t.Fatalf("expected latest price 0.02, got %#v", items[0].LatestPrice)
	}
	if items[0].Stats24h == nil || items[0].Stats24h.Txns != 12 {
		t.Fatalf("expected stats_24h, got %#v", items[0].Stats24h)
	}
}

func TestGetTokenDetailReturnsNotFoundWhenMintHasNoData(t *testing.T) {
	service := NewTokenService(nil, &mockTokenReadModel{})

	_, err := service.GetTokenDetail(context.Background(), "missing_mint")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestGetTokenDetailBuildsCreateSummary(t *testing.T) {
	createEvent := events.Envelope{
		EventID:     "create_event_1",
		Protocol:    "pumpfun",
		EventType:   "create",
		EventUnixTS: 1770000000,
		Payload: json.RawMessage(`{
			"ix_name":"create",
			"mint":"mint_1",
			"bonding_curve":"curve_1",
			"user":"user_1",
			"creator":"creator_1",
			"name":"Token One",
			"symbol":"ONE",
			"uri":"https://example.com/one.json",
			"token_program":"token_program_1",
			"virtual_token_reserves":"1000",
			"virtual_sol_reserves":"2000",
			"real_token_reserves":"3000",
			"token_total_supply":"4000",
			"is_mayhem_mode":false,
			"is_cashback_enabled":false
		}`),
	}

	service := NewTokenService(
		&mockTokenEventReader{createEvent: &createEvent},
		&mockTokenReadModel{
			snapshot: &store.TokenSnapshotRecord{
				Mint:         "mint_1",
				Name:         stringPtr("Token One"),
				Symbol:       stringPtr("ONE"),
				FirstSeenAt:  1770000000,
				CurrentStage: "bonding_curve",
			},
		},
	)

	detail, err := service.GetTokenDetail(context.Background(), "mint_1")
	if err != nil {
		t.Fatalf("GetTokenDetail returned error: %v", err)
	}
	if detail.CreateEvent == nil || detail.CreateEvent.Symbol != "ONE" {
		t.Fatalf("expected create summary with symbol ONE, got %#v", detail.CreateEvent)
	}
}

func TestFormatAmountAppliesDecimals(t *testing.T) {
	decimals := int32(6)
	if got := formatAmount("123450000", &decimals); got != "123.45" {
		t.Fatalf("expected 123.45, got %s", got)
	}
}

func stringPtr(v string) *string {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}
