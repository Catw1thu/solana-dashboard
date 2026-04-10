package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"solana-dashboard-go/internal/broadcaster"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/ingest"
	"solana-dashboard-go/internal/query"
	"solana-dashboard-go/internal/realtime"
)

type mockStore struct {
	inserted bool
	logID    int64
	err      error
}

type mockEventQuery struct {
	events               []events.Envelope
	detail               query.TokenDetail
	list                 []query.TokenListItem
	search               []query.TokenSearchItem
	trades               []query.TokenTrade
	candles              []query.TokenCandle
	activity             []query.TokenActivity
	lastCandleBeforeTime *int64
	err                  error
}

func (m *mockStore) InsertServiceEvent(ctx context.Context, event *events.Envelope) (bool, int64, error) {
	return m.inserted, m.logID, m.err
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

func (m *mockEventQuery) ListServiceEventsByMintAfterLogID(ctx context.Context, mint string, afterLogID int64, limit int) ([]events.Envelope, error) {
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

func (m *mockEventQuery) ListTokens(ctx context.Context, opts query.TokenListOptions) ([]query.TokenListItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.list) <= opts.Limit {
		return m.list, nil
	}
	return m.list[:opts.Limit], nil
}

func (m *mockEventQuery) SearchTokens(ctx context.Context, rawQuery string, limit int) ([]query.TokenSearchItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.search) <= limit {
		return m.search, nil
	}
	return m.search[:limit], nil
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

func (m *mockEventQuery) ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int, beforeTime *int64) ([]query.TokenCandle, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.lastCandleBeforeTime = beforeTime
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

func (m *mockEventQuery) ListActivityPageByMint(ctx context.Context, mint string, limit int, cursor *query.TokenActivityCursor) (*query.TokenActivityPage, error) {
	if m.err != nil {
		return nil, m.err
	}

	start := 0
	if cursor != nil {
		for idx, item := range m.activity {
			if item.EventUnixTS == cursor.EventUnixTS && item.Slot == cursor.Slot {
				start = idx + 1
				break
			}
		}
	}

	if start > len(m.activity) {
		start = len(m.activity)
	}

	end := start + limit
	if end > len(m.activity) {
		end = len(m.activity)
	}

	var nextCursor *query.TokenActivityCursor
	hasMore := end < len(m.activity)
	if hasMore && end > start {
		last := m.activity[end-1]
		nextCursor = &query.TokenActivityCursor{
			EventUnixTS: last.EventUnixTS,
			Slot:        last.Slot,
			InsertSeq:   int64(end),
		}
	}

	return &query.TokenActivityPage{
		Activity:   m.activity[start:end],
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func (m *mockEventQuery) GetTokenDetail(ctx context.Context, mint string) (query.TokenDetail, error) {
	if m.err != nil {
		return query.TokenDetail{}, m.err
	}
	return m.detail, nil
}

func (m *mockEventQuery) BuildRealtimeStatsPayload(ctx context.Context, mint string, nowTs int64) (*broadcaster.TokenStatsPayload, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, nil
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

func TestListTokensReturnsWindowBuySellCounts(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	handler := NewHandler(service, &mockEventQuery{
		list: []query.TokenListItem{
			{
				Mint:         "mint_1",
				WindowTxns:   12,
				WindowBuys:   7,
				WindowSells:  5,
				WindowVolume: 42.5,
			},
		},
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tokens", handler.ListTokens)

	req := httptest.NewRequest(http.MethodGet, "/tokens?limit=1&view=hot&window=5m", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response struct {
		Tokens []query.TokenListItem `json:"tokens"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(response.Tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(response.Tokens))
	}
	if response.Tokens[0].WindowBuys != 7 || response.Tokens[0].WindowSells != 5 {
		t.Fatalf("expected buy/sell counts to round-trip, got %+v", response.Tokens[0])
	}
}

func TestListTokenCandlesPassesBeforeTime(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	eventQuery := &mockEventQuery{
		candles: []query.TokenCandle{{Time: 1700000000, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}},
	}
	handler := NewHandler(service, eventQuery)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tokens/{mint}/candles", handler.ListTokenCandles)

	req := httptest.NewRequest(http.MethodGet, "/tokens/mint_1/candles?resolution=1m&limit=1&before_time=1700000000", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if eventQuery.lastCandleBeforeTime == nil || *eventQuery.lastCandleBeforeTime != 1700000000 {
		t.Fatalf("expected before_time to round-trip, got %#v", eventQuery.lastCandleBeforeTime)
	}
}

func TestSearchTokensReturnsMatches(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	handler := NewHandler(service, &mockEventQuery{
		search: []query.TokenSearchItem{
			{
				Mint:     "mint_1",
				Name:     stringPtrHandler("Moon Cat"),
				Symbol:   stringPtrHandler("MOON"),
				ImageURI: stringPtrHandler("https://example.com/moon.png"),
			},
		},
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search/tokens", handler.SearchTokens)

	req := httptest.NewRequest(http.MethodGet, "/search/tokens?q=moon&limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response struct {
		Count  int                     `json:"count"`
		Tokens []query.TokenSearchItem `json:"tokens"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal search response: %v", err)
	}
	if response.Count != 1 || len(response.Tokens) != 1 {
		t.Fatalf("expected one search result, got %#v", response)
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

func TestListTokenActivityReturnsCursorPage(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := ingest.NewService(hub, store)
	defer service.Close()

	handler := NewHandler(service, &mockEventQuery{
		activity: []query.TokenActivity{
			{EventID: "event_3", Mint: "mint_1", EventUnixTS: 300, Slot: 3},
			{EventID: "event_2", Mint: "mint_1", EventUnixTS: 200, Slot: 2},
			{EventID: "event_1", Mint: "mint_1", EventUnixTS: 100, Slot: 1},
		},
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tokens/{mint}/activity", handler.ListTokenActivity)

	req := httptest.NewRequest(http.MethodGet, "/tokens/mint_1/activity?limit=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response struct {
		Count      int                        `json:"count"`
		HasMore    bool                       `json:"has_more"`
		NextCursor *query.TokenActivityCursor `json:"next_cursor"`
		Activity   []query.TokenActivity      `json:"activity"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Count != 2 || !response.HasMore || response.NextCursor == nil {
		t.Fatalf("expected cursor page metadata, got %#v", response)
	}

	req = httptest.NewRequest(
		http.MethodGet,
		"/tokens/mint_1/activity?limit=2&before_time=200&before_slot=2&before_seq=2",
		nil,
	)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d on second page, got %d", http.StatusOK, rec.Code)
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal second page response: %v", err)
	}
	if response.Count != 1 || response.HasMore {
		t.Fatalf("expected final page with one item, got %#v", response)
	}
}

func stringPtrHandler(value string) *string {
	return &value
}
