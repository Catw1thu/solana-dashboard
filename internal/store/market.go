package store

import (
	"context"
	"fmt"
	"solana-dashboard-go/internal/db"
)

const upsertMarketSQL = `
insert into markets (
    market_id,
    mint,
    protocol,
    market_type,
    bonding_curve,
    pool,
    base_mint,
    quote_mint,
    lp_mint,
    started_at,
    ended_at,
    create_event_id,
    updated_at
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, to_timestamp($10), to_timestamp($11), $12, now()
)
on conflict (market_id) do update set
    mint = excluded.mint,
    protocol = excluded.protocol,
    market_type = excluded.market_type,
    bonding_curve = coalesce(excluded.bonding_curve, markets.bonding_curve),
    pool = coalesce(excluded.pool, markets.pool),
    base_mint = coalesce(excluded.base_mint, markets.base_mint),
    quote_mint = coalesce(excluded.quote_mint, markets.quote_mint),
    lp_mint = coalesce(excluded.lp_mint, markets.lp_mint),
    started_at = least(markets.started_at, excluded.started_at),
    ended_at = coalesce(excluded.ended_at, markets.ended_at),
    updated_at = now()
`

const closeMarketSQL = `
update markets
set
    ended_at = to_timestamp($2),
    updated_at = now()
where market_id = $1
`

type MarketRecord struct {
	MarketID      string
	Mint          string
	Protocol      string
	MarketType    string
	BondingCurve  *string
	Pool          *string
	BaseMint      *string
	QuoteMint     *string
	LPMint        *string
	StartedAt     int64
	EndedAt       *int64
	CreateEventID string
}

type MarketStore struct {
	db *db.DB
}

func NewMarketStore(database *db.DB) *MarketStore {
	return &MarketStore{db: database}
}

func (s *MarketStore) UpsertMarket(ctx context.Context, market MarketRecord) error {
	startedAt := float64(market.StartedAt)
	var endedAt *float64
	if market.EndedAt != nil {
		value := float64(*market.EndedAt)
		endedAt = &value
	}

	_, err := s.db.Pool.Exec(
		ctx,
		upsertMarketSQL,
		market.MarketID,
		market.Mint,
		market.Protocol,
		market.MarketType,
		market.BondingCurve,
		market.Pool,
		market.BaseMint,
		market.QuoteMint,
		market.LPMint,
		startedAt,
		endedAt,
		market.CreateEventID,
	)
	if err != nil {
		return fmt.Errorf("upsert market: %w", err)
	}

	return nil
}

func (s *MarketStore) CloseMarket(ctx context.Context, marketID string, endedAt int64) error {
	_, err := s.db.Pool.Exec(ctx, closeMarketSQL, marketID, float64(endedAt))
	if err != nil {
		return fmt.Errorf("close market: %w", err)
	}

	return nil
}
