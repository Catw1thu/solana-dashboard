package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/ingest"
	"solana-dashboard-go/internal/query"
	"solana-dashboard-go/internal/realtime"
	storepkg "solana-dashboard-go/internal/store"
	"testing"
)

type mockStore struct {
	inserted bool
	err      error
}

type mockEventQuery struct {
	events []events.Envelope
	detail query.TokenDetail
	err    error
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

func (m *mockEventQuery) ListTradesByMint(ctx context.Context, mint string, limit int) ([]storepkg.TradeRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.detail.RecentTrades) <= limit {
		return m.detail.RecentTrades, nil
	}
	return m.detail.RecentTrades[:limit], nil
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
		"instruction_path":{
			"source":"outer",
			"outer_index":1,
			"inner_index":null
		},
		"event_source":"logs",
		"event_unix_ts":1770000000,
		"refs":{
			"mint":"mint_1",
			"pool":null,
			"bonding_curve":"curve_1",
			"user":"user_1",
			"creator":"creator_1",
			"base_mint":null,
			"quote_mint":null,
			"lp_mint":null
		},
		"payload":{
			"side":"buy",
			"ix_name":"buy",
			"mint":"mint_1",
			"user":"user_1",
			"bonding_curve":"curve_1",
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
			"instruction_args":{
				"amount":"1000",
				"max_sol_cost":"2000",
				"min_sol_output":null,
				"spendable_sol_in":null,
				"min_tokens_out":null
			}
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

func TestIngestEventRejectsInvalidJSON(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: false, err: nil}
	service := ingest.NewService(hub, store)
	defer service.Close()
	handler := NewHandler(service, nil)

	req := httptest.NewRequest(http.MethodPost, "/internal/events", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.IngestEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestListTokenEventsReturnsMintFeed(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	query := &mockEventQuery{
		events: []events.Envelope{
			{
				EventID:     "event_1",
				Protocol:    "pumpfun",
				EventType:   "create",
				TxSignature: "sig_1",
				Slot:        11,
			},
			{
				EventID:     "event_2",
				Protocol:    "pumpfun",
				EventType:   "trade",
				TxSignature: "sig_2",
				Slot:        10,
			},
		},
	}
	handler := NewHandler(service, query)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tokens/{mint}/events", handler.ListTokenEvents)

	req := httptest.NewRequest(http.MethodGet, "/tokens/mint_1/events?limit=2", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response struct {
		Mint   string            `json:"mint"`
		Count  int               `json:"count"`
		Events []events.Envelope `json:"events"`
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
	if len(response.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(response.Events))
	}
}

func TestGetTokenDetailReturnsMintCentricPayload(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	queryMock := &mockEventQuery{
		detail: query.TokenDetail{
			Mint: "mint_1",
			CreateEvent: &query.TokenCreateSummary{
				EventID:     "create_1",
				Protocol:    "pumpfun",
				EventType:   "create",
				EventUnixTS: 1770000000,
				Name:        "Token One",
				Symbol:      "ONE",
				URI:         "https://example.com/one.json",
			},
			Markets: []storepkg.MarketRecord{
				{MarketID: "pool_1", Mint: "mint_1", Protocol: "pumpamm", MarketType: "pumpamm_pool", StartedAt: 1770000100},
			},
			RecentTrades: []storepkg.TradeRecord{
				{EventID: "trade_1", Mint: "mint_1", Side: "buy", TokenAmount: "100", QuoteAmount: "2"},
			},
			RecentEvents: []events.Envelope{
				{EventID: "event_1", EventType: "trade", Protocol: "pumpfun"},
			},
		},
	}
	handler := NewHandler(service, queryMock)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tokens/{mint}", handler.GetTokenDetail)

	req := httptest.NewRequest(http.MethodGet, "/tokens/mint_1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response query.TokenDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Mint != "mint_1" {
		t.Fatalf("expected mint_1, got %s", response.Mint)
	}
	if response.CreateEvent == nil || response.CreateEvent.Symbol != "ONE" {
		t.Fatalf("expected create event summary with symbol ONE, got %#v", response.CreateEvent)
	}
	if len(response.Markets) != 1 {
		t.Fatalf("expected 1 market, got %d", len(response.Markets))
	}
}

func TestGetTokenDetailReturnsNotFound(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	handler := NewHandler(service, &mockEventQuery{err: query.ErrTokenNotFound})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tokens/{mint}", handler.GetTokenDetail)

	req := httptest.NewRequest(http.MethodGet, "/tokens/missing_mint", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestListTokenTradesReturnsRecentTrades(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	handler := NewHandler(service, &mockEventQuery{
		detail: query.TokenDetail{
			RecentTrades: []storepkg.TradeRecord{
				{EventID: "trade_2", Mint: "mint_1", Side: "sell", TokenAmount: "50", QuoteAmount: "1"},
				{EventID: "trade_1", Mint: "mint_1", Side: "buy", TokenAmount: "100", QuoteAmount: "2"},
			},
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
		Mint   string                 `json:"mint"`
		Count  int                    `json:"count"`
		Trades []storepkg.TradeRecord `json:"trades"`
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
	if len(response.Trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(response.Trades))
	}
}
