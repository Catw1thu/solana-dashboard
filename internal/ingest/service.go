package ingest

import (
	"context"
	"fmt"
	"log"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/realtime"
)

type Service struct {
	hub *realtime.Hub
}

func NewService(hub *realtime.Hub) *Service {
	return &Service{
		hub: hub,
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
		s.hub.Publish(event)
		return nil
	case events.PumpAmmSwapPayload:
		log.Printf("[ingest] pumpamm swap pool=%s user=%s side=%s", p.Pool, p.User, p.Side)
		s.hub.Publish(event)
		return nil
	default:
		return fmt.Errorf("unsupported payload type=%T", p)
	}
}

func (s *Service) Subscribe(buffer int) chan events.Envelope {
	return s.hub.Subscribe(buffer)
}

func (s *Service) Unsubscribe(ch chan events.Envelope) {
	s.hub.Unsubscribe(ch)
}
