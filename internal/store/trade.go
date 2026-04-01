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
	EventID        string
	Mint           string
	MarketID       string
	MarketType     string
	Protocol       string
	Side           string
	IxName         string
	UserAddress    string
	BondingCurve   *string
	Pool           *string
	QuoteMint      string
	TokenAmount    string
	QuoteAmount    string
	TxSignature    string
	Slot           uint64
	EventUnixTS    int64
	RawEventSource string
}

type TradeStore struct {
	db *db.DB
}

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
