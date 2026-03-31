package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"solana-dashboard-go/internal/events"
)

type Handler struct {
}

func NewHandler() *Handler {
	return &Handler{}
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

	var env events.Envelope
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&env); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if env.EventID == "" || env.Protocol == "" || env.EventType == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	log.Printf(
		"event_id=%s protocol=%s type=%s slot=%d sig=%s",
		env.EventID, env.Protocol, env.EventType, env.Slot, env.TxSignature,
	)

	payload, err := events.DecodePayload(env)
	if err != nil {
		http.Error(w, "failed to decode payload", http.StatusBadRequest)
		return
	}
	switch p := payload.(type) {
	case events.PumpfunTradePayload:
		log.Printf("[pumpfun trade] mint=%s user=%s side=%s", p.Mint, p.User, p.Side)
	case events.PumpAmmSwapPayload:
		log.Printf("[pumpamm swap] pool=%s user=%s side=%s", p.Pool, p.User, p.Side)
	default:
		log.Printf("decoded unsupported payload type %T", p)
	}

	w.WriteHeader(http.StatusAccepted)
}
