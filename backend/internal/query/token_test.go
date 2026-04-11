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
	events       []events.Envelope
	createEvent  *events.Envelope
	migrateEvent *events.Envelope
	err          error
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

func (m *mockTokenEventReader) ListServiceEventsByMintAfterLogID(ctx context.Context, mint string, afterLogID int64, limit int) ([]events.Envelope, error) {
	if m.err != nil {
		return nil, m.err
	}
	filtered := make([]events.Envelope, 0, len(m.events))
	for _, event := range m.events {
		if event.LogID > afterLogID {
			filtered = append(filtered, event)
		}
	}
	if len(filtered) <= limit {
		return filtered, nil
	}
	return filtered[:limit], nil
}

func (m *mockTokenEventReader) FindLatestCreateEventByMint(ctx context.Context, mint string) (*events.Envelope, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.createEvent, nil
}

func (m *mockTokenEventReader) FindLatestMigrateEventByMint(ctx context.Context, mint string) (*events.Envelope, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.migrateEvent, nil
}

func (m *mockTokenEventReader) LoadProjectionCheckpoint(ctx context.Context, projectorName string) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return 123, nil
}

type mockTokenReadModel struct {
	boardRows      []store.TokenBoardRecord
	searchRows     []store.TokenSearchRecord
	snapshots      []store.TokenSnapshotRecord
	snapshot       *store.TokenSnapshotRecord
	markets        []store.TokenMarketRecord
	trades         []store.TradeEventRecord
	tradeMetrics   []store.TradeMetricPoint
	activities     []store.ActivityEventRecord
	summary        *store.TradeSummaryRecord
	currentMetrics *store.TokenMetricsCurrentRecord
	candles        []store.TokenCandleRecord
	err            error
}

func (m *mockTokenReadModel) ListTokenBoardRows(ctx context.Context, query store.TokenBoardQuery) ([]store.TokenBoardRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.boardRows) <= query.Limit {
		return m.boardRows, nil
	}
	return m.boardRows[:query.Limit], nil
}

func (m *mockTokenReadModel) SearchTokenSnapshots(ctx context.Context, query string, limit int) ([]store.TokenSearchRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.searchRows) <= limit {
		return m.searchRows, nil
	}
	return m.searchRows[:limit], nil
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

func (m *mockTokenReadModel) ListActivityEventsPageByMint(ctx context.Context, mint string, limit int, cursor *store.ActivityEventCursor) (*store.ActivityEventPage, error) {
	if m.err != nil {
		return nil, m.err
	}

	start := 0
	if cursor != nil {
		for idx, item := range m.activities {
			if item.EventUnixTS == cursor.EventUnixTS && item.Slot == cursor.Slot && item.InsertSeq == cursor.InsertSeq {
				start = idx + 1
				break
			}
		}
	}
	if start > len(m.activities) {
		start = len(m.activities)
	}

	end := start + limit
	if end > len(m.activities) {
		end = len(m.activities)
	}

	var nextCursor *store.ActivityEventCursor
	hasMore := end < len(m.activities)
	if hasMore && end > start {
		last := m.activities[end-1]
		nextCursor = &store.ActivityEventCursor{
			EventUnixTS: last.EventUnixTS,
			Slot:        last.Slot,
			InsertSeq:   last.InsertSeq,
		}
	}

	return &store.ActivityEventPage{
		Items:      m.activities[start:end],
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func (m *mockTokenReadModel) LoadTradeSummaryByMint(ctx context.Context, mint string) (*store.TradeSummaryRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.summary, nil
}

func (m *mockTokenReadModel) ListTradeMetricsForStatsByMint(ctx context.Context, mint string) ([]store.TradeMetricPoint, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tradeMetrics, nil
}

func (m *mockTokenReadModel) LoadTokenMetricsCurrentByMint(ctx context.Context, mint string) (*store.TokenMetricsCurrentRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.currentMetrics, nil
}

func (m *mockTokenReadModel) ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int, beforeTime *int64) ([]store.TokenCandleRecord, error) {
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
		boardRows: []store.TokenBoardRecord{
			{
				Mint:              mint,
				Name:              stringPtr("Token One"),
				Symbol:            stringPtr("ONE"),
				FirstSeenAt:       1770000001,
				ActiveSince:       1770000010,
				CurrentStage:      "pool",
				LatestPrice:       floatPtr(0.02),
				LatestEventUnixTS: int64Ptr(1770000200),
				PriceChange:       floatPtr(12.5),
				WindowVolume:      42,
				WindowTxns:        12,
				LiquidityQuote:    floatPtr(55),
				MarketCapQuote:    floatPtr(88),
			},
		},
	})

	items, err := service.ListTokens(context.Background(), TokenListOptions{
		Limit:  10,
		View:   "hot",
		Window: "24h",
	})
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
	if items[0].WindowTxns != 12 || items[0].WindowVolume != 42 {
		t.Fatalf("expected window metrics, got txns=%d volume=%f", items[0].WindowTxns, items[0].WindowVolume)
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

func TestGetTokenDetailIncludesLatestMigrateEvent(t *testing.T) {
	migrateEvent := events.Envelope{
		EventID:     "migrate_event_1",
		Protocol:    "pumpfun",
		EventType:   "migrate",
		EventUnixTS: 1770001200,
		Payload: json.RawMessage(`{
			"mint":"mint_1",
			"sol_amount":"2500000000",
			"mint_amount":"5000000000000"
		}`),
	}

	service := NewTokenService(
		&mockTokenEventReader{migrateEvent: &migrateEvent},
		&mockTokenReadModel{
			snapshot: &store.TokenSnapshotRecord{
				Mint:         "mint_1",
				FirstSeenAt:  1770000000,
				CurrentStage: "pool",
				MigratedAt:   int64Ptr(1770001200),
			},
		},
	)

	detail, err := service.GetTokenDetail(context.Background(), "mint_1")
	if err != nil {
		t.Fatalf("GetTokenDetail returned error: %v", err)
	}
	if detail.MigrateEvent == nil || detail.MigrateEvent.EventID != "migrate_event_1" {
		t.Fatalf("expected migrate event to be present, got %#v", detail.MigrateEvent)
	}
}

func TestBuildRealtimeStatsPayloadUsesDatabaseMetrics(t *testing.T) {
	nowTs := int64(1_700_300_000)
	service := NewTokenService(nil, &mockTokenReadModel{
		snapshot: &store.TokenSnapshotRecord{
			Mint:         "mint_1",
			FirstSeenAt:  nowTs - 2400,
			CurrentStage: "pool",
		},
		tradeMetrics: []store.TradeMetricPoint{
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
		},
	})

	payload, err := service.BuildRealtimeStatsPayload(context.Background(), "mint_1", nowTs)
	if err != nil {
		t.Fatalf("BuildRealtimeStatsPayload returned error: %v", err)
	}
	if payload == nil {
		t.Fatal("expected non-nil realtime stats payload")
	}
	if payload.P1h != 0.0000003 || payload.P4h != 0.0000003 || payload.P24h != 0.0000003 {
		t.Fatalf("expected seeded anchors from DB metrics, got p1h=%f p4h=%f p24h=%f", payload.P1h, payload.P4h, payload.P24h)
	}
}

func TestSearchTokensReturnsGlobalMatches(t *testing.T) {
	service := NewTokenService(nil, &mockTokenReadModel{
		searchRows: []store.TokenSearchRecord{
			{
				Mint:        "mint_1",
				Name:        stringPtr("Moon Cat"),
				Symbol:      stringPtr("MOON"),
				LatestPrice: floatPtr(0.12),
			},
		},
	})

	items, err := service.SearchTokens(context.Background(), "moon", 5)
	if err != nil {
		t.Fatalf("SearchTokens returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Mint != "mint_1" {
		t.Fatalf("expected mint_1, got %s", items[0].Mint)
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
