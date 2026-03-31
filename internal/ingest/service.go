package ingest

import (
	"context"
	"fmt"
	"log"
	"solana-dashboard-go/internal/events"
)

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) HandleEvent(ctx context.Context, env events.Envelope) error {
	payload, err := events.DecodePayload(env)
	if err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	switch p := payload.(type) {
	case events.PumpfunTradePayload:
		log.Printf("[ingest] pumpfun trade mint=%s user=%s side=%s", p.Mint, p.User, p.Side)
		return nil
	case events.PumpAmmSwapPayload:
		log.Printf("[ingest] pumpamm swap pool=%s user=%s side=%s", p.Pool, p.User, p.Side)
		return nil
	default:
		return fmt.Errorf("unsupported payload type=%T", p)
	}
}
