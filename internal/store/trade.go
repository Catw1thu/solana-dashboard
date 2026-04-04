package store

import (
	"context"
	"fmt"
	"solana-dashboard-go/internal/db"
)

const insertTradeSQL = `
insert into trades (
    event_id,
    mint,
    market_id,
    market_type,
    protocol,
    side,
    ix_name,
    user_address,
    bonding_curve,
    pool,
    quote_mint,
    token_amount,
    quote_amount,
    tx_signature,
    slot,
    event_time,
    raw_event_source
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::numeric, $13::numeric, $14, $15, to_timestamp($16), $17
)
on conflict (event_id) do nothing
`

type TradeRecord struct {
	EventID        string  `json:"event_id"`
	Mint           string  `json:"mint"`
	MarketID       string  `json:"market_id"`
	MarketType     string  `json:"market_type"`
	Protocol       string  `json:"protocol"`
	Side           string  `json:"side"`
	IxName         string  `json:"ix_name"`
	UserAddress    string  `json:"user_address"`
	BondingCurve   *string `json:"bonding_curve,omitempty"`
	Pool           *string `json:"pool,omitempty"`
	QuoteMint      string  `json:"quote_mint"`
	TokenAmount    string  `json:"token_amount"`
	QuoteAmount    string  `json:"quote_amount"`
	TxSignature    string  `json:"tx_signature"`
	Slot           uint64  `json:"slot"`
	EventUnixTS    int64   `json:"event_unix_ts"`
	RawEventSource string  `json:"raw_event_source"`
}

type TradeStore struct {
	db *db.DB
}

const listTradesByMintSQL = `
select
    event_id,
    mint,
    market_id,
    market_type,
    protocol,
    side,
    ix_name,
    user_address,
    bonding_curve,
    pool,
    quote_mint,
    token_amount::text,
    quote_amount::text,
    tx_signature,
    slot,
    extract(epoch from event_time)::bigint as event_unix_ts,
    raw_event_source
from trades
where mint = $1
order by event_time desc, created_at desc
limit $2
`

func NewTradeStore(database *db.DB) *TradeStore {
	return &TradeStore{db: database}
}

func (s *TradeStore) InsertTrade(ctx context.Context, trade TradeRecord) error {
	_, err := s.db.Pool.Exec(
		ctx,
		insertTradeSQL,
		trade.EventID,
		trade.Mint,
		trade.MarketID,
		trade.MarketType,
		trade.Protocol,
		trade.Side,
		trade.IxName,
		trade.UserAddress,
		trade.BondingCurve,
		trade.Pool,
		trade.QuoteMint,
		trade.TokenAmount,
		trade.QuoteAmount,
		trade.TxSignature,
		int64(trade.Slot),
		float64(trade.EventUnixTS),
		trade.RawEventSource,
	)
	if err != nil {
		return fmt.Errorf("insert trade: %w", err)
	}

	return nil
}

func (s *TradeStore) ListTradesByMint(ctx context.Context, mint string, limit int) ([]TradeRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTradesByMintSQL, mint, limit)
	if err != nil {
		return nil, fmt.Errorf("list trades by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	trades := make([]TradeRecord, 0, limit)
	for rows.Next() {
		var (
			record TradeRecord
			slot   int64
		)
		if err := rows.Scan(
			&record.EventID,
			&record.Mint,
			&record.MarketID,
			&record.MarketType,
			&record.Protocol,
			&record.Side,
			&record.IxName,
			&record.UserAddress,
			&record.BondingCurve,
			&record.Pool,
			&record.QuoteMint,
			&record.TokenAmount,
			&record.QuoteAmount,
			&record.TxSignature,
			&slot,
			&record.EventUnixTS,
			&record.RawEventSource,
		); err != nil {
			return nil, fmt.Errorf("scan trade row: %w", err)
		}
		record.Slot = uint64(slot)
		trades = append(trades, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trade rows: %w", err)
	}

	return trades, nil
}
