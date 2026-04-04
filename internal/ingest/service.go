package ingest

import (
	"context"
	"fmt"
	"log"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/realtime"
)

type eventStore interface {
	InsertServiceEvent(ctx context.Context, event *events.Envelope) (bool, error)
}

type eventProjector interface {
	Project(ctx context.Context, event *events.Envelope, payload any) error
}

type Service struct {
	hub       *realtime.Hub
	store     eventStore
	projector eventProjector
}

func NewService(hub *realtime.Hub, store eventStore, projector eventProjector) *Service {
	return &Service{
		hub:       hub,
		store:     store,
		projector: projector,
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

	switch p := payload.(type) {
	case events.PumpfunTradePayload:
		log.Printf("[ingest] pumpfun trade mint=%s user=%s side=%s", p.Mint, p.User, p.Side)
	case events.PumpfunCreatePayload:
		log.Printf("[ingest] pumpfun create mint=%s creator=%s symbol=%s", p.Mint, p.Creator, p.Symbol)
	case events.PumpfunMigratePayload:
		log.Printf("[ingest] pumpfun migrate mint=%s pool=%s", p.Mint, p.Pool)
	case events.PumpAmmSwapPayload:
		log.Printf("[ingest] pumpamm swap pool=%s user=%s side=%s", p.Pool, p.User, p.Side)
	case events.PumpAmmCreatePoolPayload:
		log.Printf("[ingest] pumpamm create_pool pool=%s creator=%s", p.Pool, p.Creator)
	case events.PumpAmmLiquidityPayload:
		log.Printf("[ingest] pumpamm liquidity action=%s pool=%s user=%s", p.Action, p.Pool, p.User)
	default:
		return fmt.Errorf("unsupported payload type=%T", p)
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
		if s.projector != nil {
			if err := s.projector.Project(ctx, &event, payload); err != nil {
				return fmt.Errorf("project event: %w", err)
			}
		}
		s.hub.Publish(event)
	}
	return nil
}

func (s *Service) Subscribe(buffer int) chan events.Envelope {
	return s.hub.Subscribe(buffer)
}

func (s *Service) Unsubscribe(ch chan events.Envelope) {
	s.hub.Unsubscribe(ch)
}
