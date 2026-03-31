package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/ingest"
)

type Handler struct {
	service *ingest.Service
}

func NewHandler(service *ingest.Service) *Handler {
	return &Handler{
		service: service,
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

	if err := h.service.HandleEvent(r.Context(), env); err != nil {
		http.Error(w, "failed to handle event", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
