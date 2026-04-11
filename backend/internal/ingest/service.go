package ingest

import (
	"context"
	"fmt"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/observability"
	"solana-dashboard-go/internal/realtime"
	"time"
)

const wrappedSolMint = "So11111111111111111111111111111111111111112"

type eventStore interface {
	InsertServiceEvent(ctx context.Context, event *events.Envelope) (bool, int64, error)
}

type Service struct {
	hub   *realtime.Hub
	store eventStore
}

func NewService(hub *realtime.Hub, store eventStore) *Service {
	return &Service{
		hub:   hub,
		store: store,
	}
}

func (s *Service) HandleEvent(ctx context.Context, event events.Envelope) error {
	decoded, err := events.DecodeEnvelope(event)
	if err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	return s.HandleDecodedEvent(ctx, decoded)
}

func (s *Service) HandleDecodedEvent(ctx context.Context, decoded events.DecodedEnvelope) error {
	start := time.Now()
	payload := decoded.Payload

	switch payload.(type) {
	case events.PumpfunTradePayload:
	case events.PumpfunCreatePayload:
	case events.PumpfunMigratePayload:
	case events.PumpAmmSwapPayload:
	case events.PumpAmmCreatePoolPayload:
	case events.PumpAmmLiquidityPayload:
	default:
		observability.Default().IncCounter("ingest_errors_total", 1)
		return fmt.Errorf("unsupported payload type=%T", payload)
	}

	event, err := decoded.EnvelopeWithPayload()
	if err != nil {
		observability.Default().IncCounter("ingest_errors_total", 1)
		return fmt.Errorf("encode payload: %w", err)
	}

	inserted, logID, err := s.store.InsertServiceEvent(ctx, &event)
	if err != nil {
		observability.Default().IncCounter("ingest_errors_total", 1)
		return fmt.Errorf("insert service event: %w", err)
	}
	if inserted {
		observability.Default().IncCounter("ingest_events_total", 1)
		observability.Default().SetGauge("ingest_last_log_id", logID)
		observability.Default().SetGauge("ingest_last_event_unix", event.EventUnixTS)
		observability.Default().ObserveDuration("ingest_handle_latency_ms", time.Since(start))
		event.LogID = logID
		// Publish to a specific token topic if we can determine the mint
		if mint := extractMint(event.Refs, payload); mint != "" {
			s.hub.Publish("token:"+mint, event)
		}
		// Always publish to global for broad listeners
		s.hub.Publish("global", event)
	}
	return nil
}

func extractMint(refs events.EventRefs, payload interface{}) string {
	if refs.Mint != nil && *refs.Mint != "" {
		return *refs.Mint
	}

	switch p := payload.(type) {
	case events.PumpfunTradePayload:
		return p.Mint
	case events.PumpfunCreatePayload:
		return p.Mint
	case events.PumpfunMigratePayload:
		return p.Mint
	case events.PumpAmmSwapPayload:
		return resolveNonSolMint(p.BaseMint, p.QuoteMint)
	case events.PumpAmmCreatePoolPayload:
		return resolveNonSolMint(p.BaseMint, p.QuoteMint)
	case events.PumpAmmLiquidityPayload:
		return resolveNonSolMint(p.BaseMint, p.QuoteMint)
	}
	return ""
}

func resolveNonSolMint(baseMint string, quoteMint string) string {
	switch {
	case baseMint == wrappedSolMint && quoteMint != "":
		return quoteMint
	case quoteMint == wrappedSolMint && baseMint != "":
		return baseMint
	case baseMint != "":
		return baseMint
	default:
		return quoteMint
	}
}

func (s *Service) Subscribe(topic string, buffer int) *realtime.Subscription {
	return s.hub.Subscribe(topic, buffer)
}

func (s *Service) Unsubscribe(sub *realtime.Subscription) {
	s.hub.Unsubscribe(sub)
}

func (s *Service) SubscribeCount() int {
	if s == nil || s.hub == nil {
		return 0
	}
	return s.hub.TotalSubscribers()
}

func (s *Service) Close() {}
