package store

import (
	"context"
	"database/sql"
	"fmt"
	"solana-dashboard-go/internal/db"
)

const insertTradeTickSQL = `
insert into trade_ticks (
    event_time,
    event_id,
    mint,
    market_id,
    market_type,
    protocol,
    side,
    user_address,
    quote_mint,
    price,
    token_amount,
    quote_amount,
    tx_signature,
    slot
) values (
    to_timestamp($1),
    $2, $3, $4, $5, $6, $7, $8, $9,
    $10::numeric, $11::numeric, $12::numeric,
    $13, $14
)
on conflict (event_time, event_id) do nothing
`

const listCandlesByMintSQL = `
with ranked as (
    select
        time_bucket($2::interval, event_time) as bucket,
        event_time,
        price,
        quote_amount,
        row_number() over (partition by time_bucket($2::interval, event_time) order by event_time asc, event_id asc) as rn_open,
        row_number() over (partition by time_bucket($2::interval, event_time) order by event_time desc, event_id desc) as rn_close
    from trade_ticks
    where mint = $1
),
aggregated as (
    select
        bucket,
        max(price) filter (where rn_open = 1)::double precision as open,
        max(price)::double precision as high,
        min(price)::double precision as low,
        max(price) filter (where rn_close = 1)::double precision as close,
        coalesce(sum(quote_amount), 0)::double precision as volume
    from ranked
    group by bucket
),
limited as (
    select
        extract(epoch from bucket)::bigint as time_unix,
        open,
        high,
        low,
        close,
        volume
    from aggregated
    order by bucket desc
    limit $3
)
select
    time_unix,
    open,
    high,
    low,
    close,
    volume
from limited
order by time_unix asc
`

const loadTradeTickSummaryByMintSQL = `
with latest as (
    select event_time, price, quote_mint
    from trade_ticks
    where mint = $1
    order by event_time desc, event_id desc
    limit 1
),
prices as (
    select
        l.quote_mint,
        l.price::double precision as latest_price,
        extract(epoch from l.event_time)::bigint as latest_event_unix_ts,
        (
            select t.price::double precision
            from trade_ticks t
            where t.mint = $1 and t.event_time <= l.event_time - interval '5 minutes'
            order by t.event_time desc, t.event_id desc
            limit 1
        ) as price_5m_ago,
        (
            select t.price::double precision
            from trade_ticks t
            where t.mint = $1 and t.event_time <= l.event_time - interval '1 hour'
            order by t.event_time desc, t.event_id desc
            limit 1
        ) as price_1h_ago,
        (
            select t.price::double precision
            from trade_ticks t
            where t.mint = $1 and t.event_time <= l.event_time - interval '6 hours'
            order by t.event_time desc, t.event_id desc
            limit 1
        ) as price_6h_ago,
        (
            select t.price::double precision
            from trade_ticks t
            where t.mint = $1 and t.event_time <= l.event_time - interval '24 hours'
            order by t.event_time desc, t.event_id desc
            limit 1
        ) as price_24h_ago
    from latest l
),
stats_24h as (
    select
        count(*)::bigint as txns_24h,
        count(*) filter (where side = 'buy')::bigint as buys_24h,
        count(*) filter (where side = 'sell')::bigint as sells_24h,
        coalesce(sum(quote_amount), 0)::double precision as volume_24h,
        coalesce(sum(quote_amount) filter (where side = 'buy'), 0)::double precision as buy_volume_24h,
        coalesce(sum(quote_amount) filter (where side = 'sell'), 0)::double precision as sell_volume_24h,
        count(distinct user_address)::bigint as makers_24h,
        count(distinct user_address) filter (where side = 'buy')::bigint as buyers_24h,
        count(distinct user_address) filter (where side = 'sell')::bigint as sellers_24h
    from trade_ticks t
    join latest l on true
    where t.mint = $1
      and t.event_time > l.event_time - interval '24 hours'
)
select
    p.quote_mint,
    p.latest_price,
    p.latest_event_unix_ts,
    p.price_5m_ago,
    p.price_1h_ago,
    p.price_6h_ago,
    p.price_24h_ago,
    s.txns_24h,
    s.buys_24h,
    s.sells_24h,
    s.volume_24h,
    s.buy_volume_24h,
    s.sell_volume_24h,
    s.makers_24h,
    s.buyers_24h,
    s.sellers_24h
from prices p
cross join stats_24h s
`

type TradeTickRecord struct {
	EventID     string
	Mint        string
	MarketID    string
	MarketType  string
	Protocol    string
	Side        string
	UserAddress string
	QuoteMint   string
	Price       string
	TokenAmount string
	QuoteAmount string
	TxSignature string
	Slot        uint64
	EventUnixTS int64
}

type CandleRecord struct {
	TimeUnix int64   `json:"time"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   float64 `json:"volume"`
}

type TradeTickSummary struct {
	QuoteMint        string
	LatestPrice      float64
	LatestEventUnix  int64
	Price5mAgo       *float64
	Price1hAgo       *float64
	Price6hAgo       *float64
	Price24hAgo      *float64
	Txns24h          int64
	Buys24h          int64
	Sells24h         int64
	Volume24h        float64
	BuyVolume24h     float64
	SellVolume24h    float64
	Makers24h        int64
	Buyers24h        int64
	Sellers24h       int64
}

type TradeTickStore struct {
	db *db.DB
}

func NewTradeTickStore(database *db.DB) *TradeTickStore {
	return &TradeTickStore{db: database}
}

func (s *TradeTickStore) InsertTradeTick(ctx context.Context, tick TradeTickRecord) error {
	_, err := s.db.Pool.Exec(
		ctx,
		insertTradeTickSQL,
		float64(tick.EventUnixTS),
		tick.EventID,
		tick.Mint,
		tick.MarketID,
		tick.MarketType,
		tick.Protocol,
		tick.Side,
		tick.UserAddress,
		tick.QuoteMint,
		tick.Price,
		tick.TokenAmount,
		tick.QuoteAmount,
		tick.TxSignature,
		int64(tick.Slot),
	)
	if err != nil {
		return fmt.Errorf("insert trade tick: %w", err)
	}

	return nil
}

func (s *TradeTickStore) ListCandlesByMint(
	ctx context.Context,
	mint string,
	resolution string,
	limit int,
) ([]CandleRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listCandlesByMintSQL, mint, resolution, limit)
	if err != nil {
		return nil, fmt.Errorf("list candles by mint=%s resolution=%s: %w", mint, resolution, err)
	}
	defer rows.Close()

	candles := make([]CandleRecord, 0, limit)
	for rows.Next() {
		var candle CandleRecord
		if err := rows.Scan(
			&candle.TimeUnix,
			&candle.Open,
			&candle.High,
			&candle.Low,
			&candle.Close,
			&candle.Volume,
		); err != nil {
			return nil, fmt.Errorf("scan candle row: %w", err)
		}
		candles = append(candles, candle)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candle rows: %w", err)
	}

	return candles, nil
}

func (s *TradeTickStore) LoadSummaryByMint(ctx context.Context, mint string) (*TradeTickSummary, error) {
	var (
		summary          TradeTickSummary
		price5mAgo       sql.NullFloat64
		price1hAgo       sql.NullFloat64
		price6hAgo       sql.NullFloat64
		price24hAgo      sql.NullFloat64
		latestEventUnix  sql.NullInt64
		quoteMint        sql.NullString
	)

	err := s.db.Pool.QueryRow(ctx, loadTradeTickSummaryByMintSQL, mint).Scan(
		&quoteMint,
		&summary.LatestPrice,
		&latestEventUnix,
		&price5mAgo,
		&price1hAgo,
		&price6hAgo,
		&price24hAgo,
		&summary.Txns24h,
		&summary.Buys24h,
		&summary.Sells24h,
		&summary.Volume24h,
		&summary.BuyVolume24h,
		&summary.SellVolume24h,
		&summary.Makers24h,
		&summary.Buyers24h,
		&summary.Sellers24h,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load trade tick summary by mint=%s: %w", mint, err)
	}

	if quoteMint.Valid {
		summary.QuoteMint = quoteMint.String
	}
	if latestEventUnix.Valid {
		summary.LatestEventUnix = latestEventUnix.Int64
	}
	summary.Price5mAgo = nullableFloat(price5mAgo)
	summary.Price1hAgo = nullableFloat(price1hAgo)
	summary.Price6hAgo = nullableFloat(price6hAgo)
	summary.Price24hAgo = nullableFloat(price24hAgo)

	return &summary, nil
}

func nullableFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}
