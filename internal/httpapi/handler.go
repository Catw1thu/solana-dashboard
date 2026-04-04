package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/ingest"
	"solana-dashboard-go/internal/query"
	"solana-dashboard-go/internal/store"
	"strconv"
)

const (
	defaultEventListLimit = 100
	maxEventListLimit     = 500
)

type eventQuery interface {
	ListTokens(ctx context.Context, limit int) ([]query.TokenListItem, error)
	ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error)
	ListTradesByMint(ctx context.Context, mint string, limit int) ([]store.TradeRecord, error)
	GetTokenDetail(ctx context.Context, mint string) (query.TokenDetail, error)
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

	w.Header().Set("Content-Type", "application/json")
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

	w.Header().Set("Content-Type", "application/json")
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

	items, err := h.eventQuery.ListTokens(r.Context(), limit)
	if err != nil {
		log.Printf("failed to list tokens: %v", err)
		http.Error(w, "failed to list tokens", http.StatusInternalServerError)
		return
	}

	response := struct {
		Count  int                   `json:"count"`
		Tokens []query.TokenListItem `json:"tokens"`
	}{
		Count:  len(items),
		Tokens: items,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode token list response: %v", err)
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

	w.Header().Set("Content-Type", "application/json")
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
		Mint   string              `json:"mint"`
		Count  int                 `json:"count"`
		Trades []store.TradeRecord `json:"trades"`
	}{
		Mint:   mint,
		Count:  len(tradeList),
		Trades: tradeList,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode token trades response for mint=%s: %v", mint, err)
	}
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
