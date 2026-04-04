package query

import (
	"context"
	"errors"
	"fmt"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/store"
)

const (
	defaultDetailMarketLimit = 10
	defaultDetailTradeLimit  = 20
	defaultDetailEventLimit  = 20
)

var ErrTokenNotFound = errors.New("token not found")

type tokenEventReader interface {
	ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error)
	FindLatestCreateEventByMint(ctx context.Context, mint string) (*events.Envelope, error)
}

type tokenMarketReader interface {
	ListMarketsByMint(ctx context.Context, mint string, limit int) ([]store.MarketRecord, error)
}

type tokenTradeReader interface {
	ListTradesByMint(ctx context.Context, mint string, limit int) ([]store.TradeRecord, error)
}

type TokenCreateSummary struct {
	EventID      string  `json:"event_id"`
	Protocol     string  `json:"protocol"`
	EventType    string  `json:"event_type"`
	EventUnixTS  int64   `json:"event_unix_ts"`
	Creator      *string `json:"creator,omitempty"`
	BondingCurve *string `json:"bonding_curve,omitempty"`
	Name         string  `json:"name"`
	Symbol       string  `json:"symbol"`
	URI          string  `json:"uri"`
}

type TokenDetail struct {
	Mint         string               `json:"mint"`
	CreateEvent  *TokenCreateSummary  `json:"create_event,omitempty"`
	ActiveMarket *store.MarketRecord  `json:"active_market,omitempty"`
	Markets      []store.MarketRecord `json:"markets"`
	RecentTrades []store.TradeRecord  `json:"recent_trades"`
	RecentEvents []events.Envelope    `json:"recent_events"`
}

type TokenService struct {
	events  tokenEventReader
	markets tokenMarketReader
	trades  tokenTradeReader
}

func NewTokenService(events tokenEventReader, markets tokenMarketReader, trades tokenTradeReader) *TokenService {
	return &TokenService{
		events:  events,
		markets: markets,
		trades:  trades,
	}
}

func (s *TokenService) ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error) {
	return s.events.ListServiceEventsByMint(ctx, mint, limit)
}

func (s *TokenService) ListTradesByMint(ctx context.Context, mint string, limit int) ([]store.TradeRecord, error) {
	return s.trades.ListTradesByMint(ctx, mint, limit)
}

func (s *TokenService) GetTokenDetail(ctx context.Context, mint string) (TokenDetail, error) {
	createEvent, err := s.events.FindLatestCreateEventByMint(ctx, mint)
	if err != nil {
		return TokenDetail{}, fmt.Errorf("load create event: %w", err)
	}

	markets, err := s.markets.ListMarketsByMint(ctx, mint, defaultDetailMarketLimit)
	if err != nil {
		return TokenDetail{}, fmt.Errorf("load markets: %w", err)
	}

	trades, err := s.trades.ListTradesByMint(ctx, mint, defaultDetailTradeLimit)
	if err != nil {
		return TokenDetail{}, fmt.Errorf("load trades: %w", err)
	}

	recentEvents, err := s.events.ListServiceEventsByMint(ctx, mint, defaultDetailEventLimit)
	if err != nil {
		return TokenDetail{}, fmt.Errorf("load recent events: %w", err)
	}

	if createEvent == nil && len(markets) == 0 && len(trades) == 0 && len(recentEvents) == 0 {
		return TokenDetail{}, ErrTokenNotFound
	}

	detail := TokenDetail{
		Mint:         mint,
		Markets:      markets,
		RecentTrades: trades,
		RecentEvents: recentEvents,
	}

	if createEvent != nil {
		summary, err := buildCreateSummary(*createEvent)
		if err != nil {
			return TokenDetail{}, err
		}
		detail.CreateEvent = summary
	}

	if active := selectActiveMarket(markets); active != nil {
		detail.ActiveMarket = active
	}

	return detail, nil
}

func buildCreateSummary(event events.Envelope) (*TokenCreateSummary, error) {
	payload, err := events.DecodePayload(event)
	if err != nil {
		return nil, fmt.Errorf("decode create payload for event %s: %w", event.EventID, err)
	}

	switch value := payload.(type) {
	case events.PumpfunCreatePayload:
		return &TokenCreateSummary{
			EventID:      event.EventID,
			Protocol:     event.Protocol,
			EventType:    event.EventType,
			EventUnixTS:  event.EventUnixTS,
			Creator:      event.Refs.Creator,
			BondingCurve: event.Refs.BondingCurve,
			Name:         value.Name,
			Symbol:       value.Symbol,
			URI:          value.URI,
		}, nil
	default:
		return &TokenCreateSummary{
			EventID:      event.EventID,
			Protocol:     event.Protocol,
			EventType:    event.EventType,
			EventUnixTS:  event.EventUnixTS,
			Creator:      event.Refs.Creator,
			BondingCurve: event.Refs.BondingCurve,
		}, nil
	}
}

func selectActiveMarket(markets []store.MarketRecord) *store.MarketRecord {
	for _, market := range markets {
		if market.EndedAt == nil {
			selected := market
			return &selected
		}
	}
	if len(markets) == 0 {
		return nil
	}

	selected := markets[0]
	return &selected
}
