package ingest

import (
	"context"
	"fmt"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/realtime"
)

const wrappedSolMint = "So11111111111111111111111111111111111111112"

type eventStore interface {
	InsertServiceEvent(ctx context.Context, event *events.Envelope) (bool, error)
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
	payload := decoded.Payload

	switch payload.(type) {
	case events.PumpfunTradePayload:
	case events.PumpfunCreatePayload:
	case events.PumpfunMigratePayload:
	case events.PumpAmmSwapPayload:
	case events.PumpAmmCreatePoolPayload:
	case events.PumpAmmLiquidityPayload:
	default:
		return fmt.Errorf("unsupported payload type=%T", payload)
	}

	event, err := decoded.EnvelopeWithPayload()
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	inserted, err := s.store.InsertServiceEvent(ctx, &event)
	if err != nil {
		return fmt.Errorf("insert service event: %w", err)
	}
	if inserted {
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

func (s *Service) Subscribe(topic string, buffer int) chan events.Envelope {
	return s.hub.Subscribe(topic, buffer)
}

func (s *Service) Unsubscribe(ch chan events.Envelope) {
	s.hub.Unsubscribe(ch)
}

func (s *Service) Close() {}
