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
	"solana-dashboard-go/internal/jetstream"
	"solana-dashboard-go/internal/projector"
	"solana-dashboard-go/internal/query"
	"solana-dashboard-go/internal/realtime"
	"solana-dashboard-go/internal/store"
	"time"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

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
	readModelStore := store.NewReadModelStore(database)
	eventProjector := projector.New(readModelStore)
	service := ingest.NewService(hub, serviceEventStore)
	tokenQueries := query.NewTokenService(serviceEventStore, readModelStore)
	handler := httpapi.NewHandler(service, tokenQueries)
	projectionRunner := projector.NewRunner("token_read_model", serviceEventStore, eventProjector)

	go func() {
		if err := projectionRunner.Run(ctx); err != nil {
			log.Fatalf("failed to run projector: %v", err)
		}
	}()

	if cfg.NATSURL != "" {
		consumer := jetstream.NewConsumer(cfg.NATSURL, service)
		go func() {
			if err := consumer.Run(ctx); err != nil {
				log.Fatalf("failed to run jetstream consumer: %v", err)
			}
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Healthz)
	mux.HandleFunc("/internal/events", handler.IngestEvent)
	mux.HandleFunc("GET /tokens", handler.ListTokens)
	mux.HandleFunc("GET /tokens/{mint}", handler.GetTokenDetail)
	mux.HandleFunc("GET /tokens/{mint}/events", handler.ListTokenEvents)
	mux.HandleFunc("GET /tokens/{mint}/timeline", handler.ListTokenTimeline)
	mux.HandleFunc("GET /tokens/{mint}/candles", handler.ListTokenCandles)
	mux.HandleFunc("GET /tokens/{mint}/activity", handler.ListTokenActivity)
	mux.HandleFunc("GET /tokens/{mint}/trades", handler.ListTokenTrades)
	mux.HandleFunc("/ws", handler.ServeWS)

	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           corsMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on %s", server.Addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("failed to start server: %v", err)
	}
}
