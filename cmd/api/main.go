package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"solana-dashboard-go/internal/config"
	"solana-dashboard-go/internal/db"
	"solana-dashboard-go/internal/httpapi"
	"solana-dashboard-go/internal/ingest"
	"solana-dashboard-go/internal/realtime"
	"solana-dashboard-go/internal/store"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	ctx := context.Background()
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	hub := realtime.NewHub()
	serviceEventStore := store.NewServiceEventStore(database)
	service := ingest.NewService(hub, serviceEventStore)
	handler := httpapi.NewHandler(service)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Healthz)
	mux.HandleFunc("/internal/events", handler.IngestEvent)
	mux.HandleFunc("/ws", handler.ServeWS)

	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on %s", server.Addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("failed to start server: %v", err)
	}
}
