package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/ingest"
	"solana-dashboard-go/internal/query"
	"solana-dashboard-go/internal/realtime"
)

type mockStore struct {
	inserted bool
	err      error
}

type mockEventQuery struct {
	events   []events.Envelope
	detail   query.TokenDetail
	list     []query.TokenListItem
	timeline []query.TokenActivity
	trades   []query.TokenTrade
	candles  []query.TokenCandle
	activity []query.TokenActivity
	err      error
}

func (m *mockStore) InsertServiceEvent(ctx context.Context, event *events.Envelope) (bool, error) {
	return m.inserted, m.err
}

func (m *mockEventQuery) ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.events) <= limit {
		return m.events, nil
	}
	return m.events[:limit], nil
}

func (m *mockEventQuery) ListTokens(ctx context.Context, limit int) ([]query.TokenListItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.list) <= limit {
		return m.list, nil
	}
	return m.list[:limit], nil
}

func (m *mockEventQuery) ListTimelineByMint(ctx context.Context, mint string, limit int) ([]query.TokenActivity, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.timeline) <= limit {
		return m.timeline, nil
	}
	return m.timeline[:limit], nil
}

func (m *mockEventQuery) ListTradesByMint(ctx context.Context, mint string, limit int) ([]query.TokenTrade, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.trades) <= limit {
		return m.trades, nil
	}
	return m.trades[:limit], nil
}

func (m *mockEventQuery) ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int) ([]query.TokenCandle, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.candles) <= limit {
		return m.candles, nil
	}
	return m.candles[:limit], nil
}

func (m *mockEventQuery) ListActivityByMint(ctx context.Context, mint string, limit int) ([]query.TokenActivity, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.activity) <= limit {
		return m.activity, nil
	}
	return m.activity[:limit], nil
}

func (m *mockEventQuery) GetTokenDetail(ctx context.Context, mint string) (query.TokenDetail, error) {
	if m.err != nil {
		return query.TokenDetail{}, m.err
	}
	return m.detail, nil
}

func TestIngestEventAcceptsValidPumpfunTrade(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()
	handler := NewHandler(service, nil)

	body := []byte(`{
		"schema_version":1,
		"event_id":"solana:pumpfun:trade:testsig:outer:1",
		"chain":"solana",
		"protocol":"pumpfun",
		"event_type":"trade",
		"commitment":"processed",
		"slot":1,
		"tx_signature":"testsig",
		"tx_index":0,
		"instruction_path":{"source":"outer","outer_index":1,"inner_index":null},
		"event_source":"logs",
		"event_unix_ts":1770000000,
		"refs":{"mint":"mint_1","bonding_curve":"curve_1","user":"user_1","creator":"creator_1"},
		"payload":{
			"side":"buy",
			"ix_name":"buy",
			"mint":"mint_1",
			"user":"user_1",
			"bonding_curve":"curve_1",
			"associated_bonding_curve":"associated_curve_1",
			"creator":"creator_1",
			"creator_vault":"vault_1",
			"token_program":"token_program_1",
			"sol_amount":"100",
			"token_amount":"200",
			"fee":"1",
			"creator_fee":"2",
			"virtual_sol_reserves":"300",
			"virtual_token_reserves":"400",
			"real_sol_reserves":"500",
			"real_token_reserves":"600",
			"track_volume":true,
			"mayhem_mode":false,
			"cashback":"0",
			"instruction_args":{"amount":"1000","max_sol_cost":"2000"}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/internal/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.IngestEvent(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestListTokenTradesReturnsRecentTrades(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	handler := NewHandler(service, &mockEventQuery{
		trades: []query.TokenTrade{
			{EventID: "trade_2", Mint: "mint_1", Side: "sell", TokenAmount: "50", QuoteAmount: "1"},
			{EventID: "trade_1", Mint: "mint_1", Side: "buy", TokenAmount: "100", QuoteAmount: "2"},
		},
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tokens/{mint}/trades", handler.ListTokenTrades)

	req := httptest.NewRequest(http.MethodGet, "/tokens/mint_1/trades?limit=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response struct {
		Mint   string             `json:"mint"`
		Count  int                `json:"count"`
		Trades []query.TokenTrade `json:"trades"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Mint != "mint_1" {
		t.Fatalf("expected mint_1, got %s", response.Mint)
	}
	if response.Count != 2 {
		t.Fatalf("expected count=2, got %d", response.Count)
	}
}

func TestListTokenTimelineReturnsActivityRows(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	handler := NewHandler(service, &mockEventQuery{
		timeline: []query.TokenActivity{
			{EventID: "activity_2", Mint: "mint_1", ActivityType: "migrate"},
			{EventID: "activity_1", Mint: "mint_1", ActivityType: "trade"},
		},
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tokens/{mint}/timeline", handler.ListTokenTimeline)

	req := httptest.NewRequest(http.MethodGet, "/tokens/mint_1/timeline?limit=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response struct {
		Mint     string                `json:"mint"`
		Count    int                   `json:"count"`
		Timeline []query.TokenActivity `json:"timeline"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Count != 2 {
		t.Fatalf("expected count=2, got %d", response.Count)
	}
}
