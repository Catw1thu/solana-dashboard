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
	MarketID      string  `json:"market_id"`
	Mint          string  `json:"mint"`
	Protocol      string  `json:"protocol"`
	MarketType    string  `json:"market_type"`
	BondingCurve  *string `json:"bonding_curve,omitempty"`
	Pool          *string `json:"pool,omitempty"`
	BaseMint      *string `json:"base_mint,omitempty"`
	QuoteMint     *string `json:"quote_mint,omitempty"`
	LPMint        *string `json:"lp_mint,omitempty"`
	StartedAt     int64   `json:"started_at"`
	EndedAt       *int64  `json:"ended_at,omitempty"`
	CreateEventID string  `json:"create_event_id"`
}

type MarketStore struct {
	db *db.DB
}

const listMarketsByMintSQL = `
select
    market_id,
    mint,
    protocol,
    market_type,
    bonding_curve,
    pool,
    base_mint,
    quote_mint,
    lp_mint,
    extract(epoch from started_at)::bigint as started_at_unix,
    extract(epoch from ended_at)::bigint as ended_at_unix,
    create_event_id
from markets
where mint = $1
order by started_at desc
limit $2
`

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

func (s *MarketStore) ListMarketsByMint(ctx context.Context, mint string, limit int) ([]MarketRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listMarketsByMintSQL, mint, limit)
	if err != nil {
		return nil, fmt.Errorf("list markets by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	markets := make([]MarketRecord, 0, limit)
	for rows.Next() {
		var (
			record  MarketRecord
			endedAt *int64
		)
		if err := rows.Scan(
			&record.MarketID,
			&record.Mint,
			&record.Protocol,
			&record.MarketType,
			&record.BondingCurve,
			&record.Pool,
			&record.BaseMint,
			&record.QuoteMint,
			&record.LPMint,
			&record.StartedAt,
			&endedAt,
			&record.CreateEventID,
		); err != nil {
			return nil, fmt.Errorf("scan market row: %w", err)
		}
		record.EndedAt = endedAt
		markets = append(markets, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate market rows: %w", err)
	}

	return markets, nil
}
