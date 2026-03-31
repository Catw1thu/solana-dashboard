package main

import (
	"errors"
	"log"
	"net/http"
	"solana-dashboard-go/internal/httpapi"
	"solana-dashboard-go/internal/ingest"
	"time"
)

func main() {
	service := ingest.NewService()
	handler := httpapi.NewHandler(service)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Healthz)
	mux.HandleFunc("/internal/events", handler.IngestEvent)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on %s", server.Addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("failed to start server: %v", err)
	}
}
