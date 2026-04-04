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

type mockTokenMarketReader struct {
	markets []store.MarketRecord
	err     error
}

func (m *mockTokenMarketReader) ListMarketsByMint(ctx context.Context, mint string, limit int) ([]store.MarketRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.markets) <= limit {
		return m.markets, nil
	}
	return m.markets[:limit], nil
}

type mockTokenTradeReader struct {
	trades []store.TradeRecord
	err    error
}

func (m *mockTokenTradeReader) ListTradesByMint(ctx context.Context, mint string, limit int) ([]store.TradeRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.trades) <= limit {
		return m.trades, nil
	}
	return m.trades[:limit], nil
}

func TestGetTokenDetailBuildsMintCentricResponse(t *testing.T) {
	mint := "mint_1"
	creator := "creator_1"
	bondingCurve := "curve_1"
	createEvent := events.Envelope{
		EventID:     "create_event_1",
		Protocol:    "pumpfun",
		EventType:   "create",
		EventUnixTS: 1770000000,
		Refs: events.EventRefs{
			Creator:      &creator,
			BondingCurve: &bondingCurve,
		},
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
		&mockTokenEventReader{
			createEvent: &createEvent,
			events: []events.Envelope{
				{EventID: "event_2", EventType: "migrate", Protocol: "pumpfun"},
				{EventID: "event_1", EventType: "trade", Protocol: "pumpfun"},
			},
		},
		&mockTokenMarketReader{
			markets: []store.MarketRecord{
				{MarketID: "pool_1", Mint: mint, Protocol: "pumpamm", MarketType: "pumpamm_pool", StartedAt: 1770000100},
				{MarketID: "curve_1", Mint: mint, Protocol: "pumpfun", MarketType: "pumpfun_curve", StartedAt: 1770000000, EndedAt: int64Ptr(1770000100)},
			},
		},
		&mockTokenTradeReader{
			trades: []store.TradeRecord{
				{EventID: "trade_1", Mint: mint, MarketID: "pool_1", Protocol: "pumpamm", Side: "buy", TokenAmount: "100", QuoteAmount: "2"},
			},
		},
	)

	detail, err := service.GetTokenDetail(context.Background(), mint)
	if err != nil {
		t.Fatalf("GetTokenDetail returned error: %v", err)
	}
	if detail.Mint != mint {
		t.Fatalf("expected mint=%s, got %s", mint, detail.Mint)
	}
	if detail.CreateEvent == nil {
		t.Fatal("expected create event summary")
	}
	if detail.CreateEvent.Name != "Token One" {
		t.Fatalf("expected name Token One, got %s", detail.CreateEvent.Name)
	}
	if detail.ActiveMarket == nil || detail.ActiveMarket.MarketID != "pool_1" {
		t.Fatalf("expected active market pool_1, got %#v", detail.ActiveMarket)
	}
	if len(detail.Markets) != 2 {
		t.Fatalf("expected 2 markets, got %d", len(detail.Markets))
	}
	if len(detail.RecentTrades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(detail.RecentTrades))
	}
	if len(detail.RecentEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(detail.RecentEvents))
	}
}

func TestGetTokenDetailReturnsNotFoundWhenMintHasNoData(t *testing.T) {
	service := NewTokenService(
		&mockTokenEventReader{},
		&mockTokenMarketReader{},
		&mockTokenTradeReader{},
	)

	_, err := service.GetTokenDetail(context.Background(), "missing_mint")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
