package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"solana-dashboard-go/internal/broadcaster"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/ingest"
	"solana-dashboard-go/internal/query"
	"strconv"
)

const (
	defaultEventListLimit    = 100
	maxEventListLimit        = 500
	defaultActivityListLimit = 100
	maxActivityListLimit     = 500
	defaultCandleListLimit   = 300
	maxCandleListLimit       = 2000
)

type eventQuery interface {
	ListTokens(ctx context.Context, opts query.TokenListOptions) ([]query.TokenListItem, error)
	SearchTokens(ctx context.Context, rawQuery string, limit int) ([]query.TokenSearchItem, error)
	ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error)
	ListTradesByMint(ctx context.Context, mint string, limit int) ([]query.TokenTrade, error)
	ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int, beforeTime *int64) ([]query.TokenCandle, error)
	ListActivityByMint(ctx context.Context, mint string, limit int) ([]query.TokenActivity, error)
	ListActivityPageByMint(ctx context.Context, mint string, limit int, cursor *query.TokenActivityCursor) (*query.TokenActivityPage, error)
	GetTokenDetail(ctx context.Context, mint string) (query.TokenDetail, error)
	BuildRealtimeStatsPayload(ctx context.Context, mint string, nowTs int64) (*broadcaster.TokenStatsPayload, error)
}

type Handler struct {
	service    *ingest.Service
	eventQuery eventQuery
}

func NewHandler(service *ingest.Service, eventQuery eventQuery) *Handler {
	return &Handler{
		service:    service,
		eventQuery: eventQuery,
	}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	setJSONHeaders(w)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func (h *Handler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var event events.Envelope
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&event); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if event.EventID == "" || event.Protocol == "" || event.EventType == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	log.Printf(
		"event_id=%s protocol=%s type=%s slot=%d sig=%s",
		event.EventID, event.Protocol, event.EventType, event.Slot, event.TxSignature,
	)

	if err := h.service.HandleEvent(r.Context(), event); err != nil {
		log.Printf("failed to handle event_id=%s: %v", event.EventID, err)
		http.Error(w, "failed to handle event", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) ListTokenEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.eventQuery == nil {
		http.Error(w, "events query not configured", http.StatusInternalServerError)
		return
	}

	mint := r.PathValue("mint")
	if mint == "" {
		http.Error(w, "missing mint", http.StatusBadRequest)
		return
	}

	limit, err := parseListLimit(r, defaultEventListLimit, maxEventListLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	eventList, err := h.eventQuery.ListServiceEventsByMint(r.Context(), mint, limit)
	if err != nil {
		log.Printf("failed to list events for mint=%s: %v", mint, err)
		http.Error(w, "failed to list events", http.StatusInternalServerError)
		return
	}

	response := struct {
		Mint   string            `json:"mint"`
		Count  int               `json:"count"`
		Events []events.Envelope `json:"events"`
	}{
		Mint:   mint,
		Count:  len(eventList),
		Events: eventList,
	}

	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode token events response for mint=%s: %v", mint, err)
	}
}

func (h *Handler) ListTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.eventQuery == nil {
		http.Error(w, "token query not configured", http.StatusInternalServerError)
		return
	}

	limit, err := parseListLimit(r, defaultEventListLimit, maxEventListLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	view := r.URL.Query().Get("view")
	window := r.URL.Query().Get("window")

	items, err := h.eventQuery.ListTokens(r.Context(), query.TokenListOptions{
		Limit:  limit,
		View:   view,
		Window: window,
	})
	if err != nil {
		if errors.Is(err, query.ErrInvalidTokenListWindow) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("failed to list tokens: %v", err)
		http.Error(w, "failed to list tokens", http.StatusInternalServerError)
		return
	}

	response := struct {
		Count  int                   `json:"count"`
		View   string                `json:"view,omitempty"`
		Window string                `json:"window,omitempty"`
		Tokens []query.TokenListItem `json:"tokens"`
	}{
		Count:  len(items),
		View:   view,
		Window: window,
		Tokens: items,
	}

	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode token list response: %v", err)
	}
}

func (h *Handler) SearchTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.eventQuery == nil {
		http.Error(w, "token query not configured", http.StatusInternalServerError)
		return
	}

	limit, err := parseListLimit(r, 8, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	searchQuery := r.URL.Query().Get("q")
	items, err := h.eventQuery.SearchTokens(r.Context(), searchQuery, limit)
	if err != nil {
		log.Printf("failed to search tokens: %v", err)
		http.Error(w, "failed to search tokens", http.StatusInternalServerError)
		return
	}

	response := struct {
		Query  string                  `json:"query"`
		Count  int                     `json:"count"`
		Tokens []query.TokenSearchItem `json:"tokens"`
	}{
		Query:  searchQuery,
		Count:  len(items),
		Tokens: items,
	}

	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode token search response: %v", err)
	}
}

func (h *Handler) GetTokenDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.eventQuery == nil {
		http.Error(w, "token query not configured", http.StatusInternalServerError)
		return
	}

	mint := r.PathValue("mint")
	if mint == "" {
		http.Error(w, "missing mint", http.StatusBadRequest)
		return
	}

	detail, err := h.eventQuery.GetTokenDetail(r.Context(), mint)
	if err != nil {
		if errors.Is(err, query.ErrTokenNotFound) {
			http.Error(w, "token not found", http.StatusNotFound)
			return
		}
		log.Printf("failed to get token detail for mint=%s: %v", mint, err)
		http.Error(w, "failed to get token detail", http.StatusInternalServerError)
		return
	}

	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(detail); err != nil {
		log.Printf("failed to encode token detail response for mint=%s: %v", mint, err)
	}
}

func (h *Handler) ListTokenTrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.eventQuery == nil {
		http.Error(w, "token query not configured", http.StatusInternalServerError)
		return
	}

	mint := r.PathValue("mint")
	if mint == "" {
		http.Error(w, "missing mint", http.StatusBadRequest)
		return
	}

	limit, err := parseListLimit(r, defaultEventListLimit, maxEventListLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tradeList, err := h.eventQuery.ListTradesByMint(r.Context(), mint, limit)
	if err != nil {
		log.Printf("failed to list trades for mint=%s: %v", mint, err)
		http.Error(w, "failed to list trades", http.StatusInternalServerError)
		return
	}

	response := struct {
		Mint   string             `json:"mint"`
		Count  int                `json:"count"`
		Trades []query.TokenTrade `json:"trades"`
	}{
		Mint:   mint,
		Count:  len(tradeList),
		Trades: tradeList,
	}

	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode token trades response for mint=%s: %v", mint, err)
	}
}

func (h *Handler) ListTokenCandles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.eventQuery == nil {
		http.Error(w, "token query not configured", http.StatusInternalServerError)
		return
	}

	mint := r.PathValue("mint")
	if mint == "" {
		http.Error(w, "missing mint", http.StatusBadRequest)
		return
	}

	limit, err := parseListLimit(r, defaultCandleListLimit, maxCandleListLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resolution := r.URL.Query().Get("resolution")
	if resolution == "" {
		resolution = "1m"
	}

	beforeTime, err := parseBeforeUnixTime(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	candles, err := h.eventQuery.ListCandlesByMint(r.Context(), mint, resolution, limit, beforeTime)
	if err != nil {
		if errors.Is(err, query.ErrInvalidCandleResolution) {
			http.Error(w, "invalid resolution", http.StatusBadRequest)
			return
		}
		log.Printf("failed to list candles for mint=%s resolution=%s: %v", mint, resolution, err)
		http.Error(w, "failed to list candles", http.StatusInternalServerError)
		return
	}

	response := struct {
		Mint       string              `json:"mint"`
		Resolution string              `json:"resolution"`
		Count      int                 `json:"count"`
		Candles    []query.TokenCandle `json:"candles"`
	}{
		Mint:       mint,
		Resolution: resolution,
		Count:      len(candles),
		Candles:    candles,
	}

	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode token candles response for mint=%s: %v", mint, err)
	}
}

func (h *Handler) ListTokenActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.eventQuery == nil {
		http.Error(w, "token query not configured", http.StatusInternalServerError)
		return
	}

	mint := r.PathValue("mint")
	if mint == "" {
		http.Error(w, "missing mint", http.StatusBadRequest)
		return
	}

	limit, err := parseListLimit(r, defaultActivityListLimit, maxActivityListLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cursor, err := parseActivityCursor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page, err := h.eventQuery.ListActivityPageByMint(r.Context(), mint, limit, cursor)
	if err != nil {
		log.Printf("failed to list activity for mint=%s: %v", mint, err)
		http.Error(w, "failed to list activity", http.StatusInternalServerError)
		return
	}

	response := struct {
		Mint       string                     `json:"mint"`
		Count      int                        `json:"count"`
		HasMore    bool                       `json:"has_more"`
		NextCursor *query.TokenActivityCursor `json:"next_cursor,omitempty"`
		Activity   []query.TokenActivity      `json:"activity"`
	}{
		Mint:       mint,
		Count:      len(page.Activity),
		HasMore:    page.HasMore,
		NextCursor: page.NextCursor,
		Activity:   page.Activity,
	}

	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode token activity response for mint=%s: %v", mint, err)
	}
}

func parseActivityCursor(r *http.Request) (*query.TokenActivityCursor, error) {
	rawTime := r.URL.Query().Get("before_time")
	rawSlot := r.URL.Query().Get("before_slot")
	rawSeq := r.URL.Query().Get("before_seq")

	if rawTime == "" && rawSlot == "" && rawSeq == "" {
		return nil, nil
	}
	if rawTime == "" || rawSlot == "" || rawSeq == "" {
		return nil, fmt.Errorf("invalid activity cursor")
	}

	eventUnixTS, err := strconv.ParseInt(rawTime, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid activity cursor")
	}
	slot, err := strconv.ParseUint(rawSlot, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid activity cursor")
	}
	insertSeq, err := strconv.ParseInt(rawSeq, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid activity cursor")
	}

	return &query.TokenActivityCursor{
		EventUnixTS: eventUnixTS,
		Slot:        slot,
		InsertSeq:   insertSeq,
	}, nil
}

func parseBeforeUnixTime(r *http.Request) (*int64, error) {
	raw := r.URL.Query().Get("before_time")
	if raw == "" {
		return nil, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid before_time")
	}

	return &value, nil
}

func parseListLimit(r *http.Request, defaultLimit int, maxLimit int) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultLimit, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf("invalid limit")
	}
	if limit > maxLimit {
		return maxLimit, nil
	}

	return limit, nil
}

func setJSONHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}
