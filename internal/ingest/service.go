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
	payload, err := events.DecodePayload(event)
	if err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	switch p := payload.(type) {
	case events.PumpfunTradePayload:
		log.Printf("[ingest] pumpfun trade mint=%s user=%s side=%s", p.Mint, p.User, p.Side)
	case events.PumpAmmSwapPayload:
		log.Printf("[ingest] pumpamm swap pool=%s user=%s side=%s", p.Pool, p.User, p.Side)
	default:
		return fmt.Errorf("unsupported payload type=%T", p)
	}

	inserted, err := s.store.InsertServiceEvent(ctx, &event)
	if err != nil {
		return fmt.Errorf("insert service event: %w", err)
	}
	if inserted {
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
