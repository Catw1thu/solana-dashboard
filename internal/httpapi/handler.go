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
		http.Error(w, "failed to handle event", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
