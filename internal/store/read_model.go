package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"solana-dashboard-go/internal/db"

	"github.com/jackc/pgx/v5"
)

const (
	SolMint     = "So11111111111111111111111111111111111111112"
	SolDecimals = int32(9)
)

const upsertTokenSQL = `
insert into tokens (
    mint,
    creator,
    bonding_curve,
    token_program,
    create_event_id,
    first_seen_at,
    current_stage,
    active_market_id,
    active_market_type,
    migrated_at,
    updated_at
) values (
    $1, $2, $3, $4, $5, to_timestamp($6), $7, $8, $9, to_timestamp($10), now()
)
on conflict (mint) do update set
    creator = coalesce(excluded.creator, tokens.creator),
    bonding_curve = coalesce(excluded.bonding_curve, tokens.bonding_curve),
    token_program = coalesce(excluded.token_program, tokens.token_program),
    create_event_id = coalesce(tokens.create_event_id, excluded.create_event_id),
    first_seen_at = least(tokens.first_seen_at, excluded.first_seen_at),
    current_stage = excluded.current_stage,
    active_market_id = coalesce(excluded.active_market_id, tokens.active_market_id),
    active_market_type = coalesce(excluded.active_market_type, tokens.active_market_type),
    migrated_at = coalesce(excluded.migrated_at, tokens.migrated_at),
    updated_at = now()
`

const upsertTokenMetadataSQL = `
insert into token_metadata_current (
    mint,
    name,
    symbol,
    uri,
    decimals,
    total_supply_raw,
    quote_mint,
    quote_decimals,
    creator,
    bonding_curve,
    token_program,
    is_mayhem_mode,
    is_cashback_enabled,
    source_event_id,
    updated_at
) values (
    $1, $2, $3, $4, $5, $6::numeric, $7, $8, $9, $10, $11, $12, $13, $14, now()
)
on conflict (mint) do update set
    name = coalesce(excluded.name, token_metadata_current.name),
    symbol = coalesce(excluded.symbol, token_metadata_current.symbol),
    uri = coalesce(excluded.uri, token_metadata_current.uri),
    decimals = coalesce(excluded.decimals, token_metadata_current.decimals),
    total_supply_raw = coalesce(excluded.total_supply_raw, token_metadata_current.total_supply_raw),
    quote_mint = coalesce(excluded.quote_mint, token_metadata_current.quote_mint),
    quote_decimals = coalesce(excluded.quote_decimals, token_metadata_current.quote_decimals),
    creator = coalesce(excluded.creator, token_metadata_current.creator),
    bonding_curve = coalesce(excluded.bonding_curve, token_metadata_current.bonding_curve),
    token_program = coalesce(excluded.token_program, token_metadata_current.token_program),
    is_mayhem_mode = coalesce(excluded.is_mayhem_mode, token_metadata_current.is_mayhem_mode),
    is_cashback_enabled = coalesce(excluded.is_cashback_enabled, token_metadata_current.is_cashback_enabled),
    source_event_id = coalesce(excluded.source_event_id, token_metadata_current.source_event_id),
    updated_at = now()
`

const upsertTokenMarketSQL = `
insert into token_markets (
    market_id,
    mint,
    protocol,
    market_type,
    bonding_curve,
    pool,
    base_mint,
    quote_mint,
    base_mint_decimals,
    quote_mint_decimals,
    lp_mint,
    started_at,
    ended_at,
    create_event_id,
    updated_at
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, to_timestamp($12), to_timestamp($13), $14, now()
)
on conflict (market_id) do update set
    mint = excluded.mint,
    protocol = excluded.protocol,
    market_type = excluded.market_type,
    bonding_curve = coalesce(excluded.bonding_curve, token_markets.bonding_curve),
    pool = coalesce(excluded.pool, token_markets.pool),
    base_mint = coalesce(excluded.base_mint, token_markets.base_mint),
    quote_mint = coalesce(excluded.quote_mint, token_markets.quote_mint),
    base_mint_decimals = coalesce(excluded.base_mint_decimals, token_markets.base_mint_decimals),
    quote_mint_decimals = coalesce(excluded.quote_mint_decimals, token_markets.quote_mint_decimals),
    lp_mint = coalesce(excluded.lp_mint, token_markets.lp_mint),
    started_at = least(token_markets.started_at, excluded.started_at),
    ended_at = coalesce(excluded.ended_at, token_markets.ended_at),
    updated_at = now()
`

const closeTokenMarketSQL = `
update token_markets
set
    ended_at = to_timestamp($2),
    updated_at = now()
where market_id = $1
`

const insertTokenTradeEventSQL = `
insert into token_trade_events (
    event_time,
    event_id,
    mint,
    market_id,
    market_type,
    protocol,
    side,
    ix_name,
    user_address,
    quote_mint,
    token_amount_raw,
    quote_amount_raw,
    tx_signature,
    slot,
    raw_event_source
) values (
    to_timestamp($1),
    $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11::numeric, $12::numeric,
    $13, $14, $15
)
on conflict (event_time, event_id) do nothing
`

const insertTokenActivityEventSQL = `
insert into token_activity_events (
    event_time,
    event_id,
    mint,
    protocol,
    event_type,
    activity_type,
    market_id,
    market_type,
    user_address,
    side,
    quote_mint,
    token_amount_raw,
    quote_amount_raw,
    tx_signature,
    slot,
    raw_event_source,
    details
) values (
    to_timestamp($1),
    $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
    $12::numeric, $13::numeric,
    $14, $15, $16, $17
)
on conflict (event_time, event_id) do nothing
`

const listTokenSnapshotsSQL = `
select
    t.mint,
    m.name,
    m.symbol,
    m.uri,
    m.decimals,
    m.total_supply_raw::text,
    m.quote_mint,
    m.quote_decimals,
    coalesce(m.creator, t.creator),
    coalesce(m.bonding_curve, t.bonding_curve),
    coalesce(m.token_program, t.token_program),
    extract(epoch from t.first_seen_at)::bigint as first_seen_at_unix,
    t.current_stage,
    t.active_market_id,
    t.active_market_type,
    extract(epoch from t.migrated_at)::bigint as migrated_at_unix,
    t.create_event_id,
    m.is_mayhem_mode,
    m.is_cashback_enabled
from tokens t
left join token_metadata_current m
    on m.mint = t.mint
order by t.first_seen_at desc
limit $1
`

const findTokenSnapshotByMintSQL = `
select
    t.mint,
    m.name,
    m.symbol,
    m.uri,
    m.decimals,
    m.total_supply_raw::text,
    m.quote_mint,
    m.quote_decimals,
    coalesce(m.creator, t.creator),
    coalesce(m.bonding_curve, t.bonding_curve),
    coalesce(m.token_program, t.token_program),
    extract(epoch from t.first_seen_at)::bigint as first_seen_at_unix,
    t.current_stage,
    t.active_market_id,
    t.active_market_type,
    extract(epoch from t.migrated_at)::bigint as migrated_at_unix,
    t.create_event_id,
    m.is_mayhem_mode,
    m.is_cashback_enabled
from tokens t
left join token_metadata_current m
    on m.mint = t.mint
where t.mint = $1
limit 1
`

const listTokenMarketsByMintSQL = `
select
    market_id,
    mint,
    protocol,
    market_type,
    bonding_curve,
    pool,
    base_mint,
    quote_mint,
    base_mint_decimals,
    quote_mint_decimals,
    lp_mint,
    extract(epoch from started_at)::bigint as started_at_unix,
    extract(epoch from ended_at)::bigint as ended_at_unix,
    create_event_id
from token_markets
where mint = $1
order by started_at desc
limit $2
`

const listTokenTradeEventsByMintSQL = `
select
    event_id,
    mint,
    market_id,
    market_type,
    protocol,
    side,
    ix_name,
    user_address,
    quote_mint,
    token_amount_raw::text,
    quote_amount_raw::text,
    tx_signature,
    slot,
    extract(epoch from event_time)::bigint as event_unix_ts,
    raw_event_source
from token_trade_events
where mint = $1
order by event_time desc, event_id desc
limit $2
`

const listTokenActivityEventsByMintSQL = `
select
    event_id,
    mint,
    protocol,
    event_type,
    activity_type,
    market_id,
    market_type,
    user_address,
    side,
    quote_mint,
    token_amount_raw::text,
    quote_amount_raw::text,
    tx_signature,
    slot,
    extract(epoch from event_time)::bigint as event_unix_ts,
    raw_event_source,
    details
from token_activity_events
where mint = $1
order by event_time desc, event_id desc
limit $2
`

const loadLatestTokenTradeByMintSQL = `
select
    event_id,
    mint,
    market_id,
    market_type,
    protocol,
    side,
    ix_name,
    user_address,
    quote_mint,
    token_amount_raw::text,
    quote_amount_raw::text,
    tx_signature,
    slot,
    extract(epoch from event_time)::bigint as event_unix_ts,
    raw_event_source
from token_trade_events
where mint = $1
order by event_time desc, event_id desc
limit 1
`

const loadTokenTradeSummarySQL = `
with meta as (
    select
        decimals,
        quote_mint as metadata_quote_mint,
        quote_decimals as metadata_quote_decimals
    from token_metadata_current
    where mint = $1
),
latest as (
    select
        event_time,
        event_id,
        quote_mint,
        token_amount_raw,
        quote_amount_raw
    from token_trade_events
    where mint = $1
    order by event_time desc, event_id desc
    limit 1
),
prices as (
    select
        l.quote_mint,
        extract(epoch from l.event_time)::bigint as latest_event_unix_ts,
        trade_price_quote(
            l.quote_amount_raw,
            l.token_amount_raw,
            m.decimals,
            resolved_quote_decimals(l.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
        ) as latest_price,
        (
            select trade_price_quote(
                t.quote_amount_raw,
                t.token_amount_raw,
                m.decimals,
                resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
            )
            from token_trade_events t
            where t.mint = $1
              and t.event_time <= l.event_time - interval '5 minutes'
            order by t.event_time desc, t.event_id desc
            limit 1
        ) as price_5m_ago,
        (
            select trade_price_quote(
                t.quote_amount_raw,
                t.token_amount_raw,
                m.decimals,
                resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
            )
            from token_trade_events t
            where t.mint = $1
              and t.event_time <= l.event_time - interval '1 hour'
            order by t.event_time desc, t.event_id desc
            limit 1
        ) as price_1h_ago,
        (
            select trade_price_quote(
                t.quote_amount_raw,
                t.token_amount_raw,
                m.decimals,
                resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
            )
            from token_trade_events t
            where t.mint = $1
              and t.event_time <= l.event_time - interval '6 hours'
            order by t.event_time desc, t.event_id desc
            limit 1
        ) as price_6h_ago,
        (
            select trade_price_quote(
                t.quote_amount_raw,
                t.token_amount_raw,
                m.decimals,
                resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
            )
            from token_trade_events t
            where t.mint = $1
              and t.event_time <= l.event_time - interval '24 hours'
            order by t.event_time desc, t.event_id desc
            limit 1
        ) as price_24h_ago
    from latest l
    left join meta m on true
),
stats_24h as (
    select
        count(*)::bigint as txns_24h,
        count(*) filter (where side = 'buy')::bigint as buys_24h,
        count(*) filter (where side = 'sell')::bigint as sells_24h,
        coalesce(sum(
            scale_amount_numeric(
                t.quote_amount_raw,
                resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
            )
        ), 0)::double precision as volume_24h,
        coalesce(sum(
            scale_amount_numeric(
                t.quote_amount_raw,
                resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
            )
        ) filter (where side = 'buy'), 0)::double precision as buy_volume_24h,
        coalesce(sum(
            scale_amount_numeric(
                t.quote_amount_raw,
                resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
            )
        ) filter (where side = 'sell'), 0)::double precision as sell_volume_24h,
        count(distinct user_address)::bigint as makers_24h,
        count(distinct user_address) filter (where side = 'buy')::bigint as buyers_24h,
        count(distinct user_address) filter (where side = 'sell')::bigint as sellers_24h
    from token_trade_events t
    join latest l on true
    left join meta m on true
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

const listTokenCandlesByMintSQL = `
with meta as (
    select
        decimals,
        quote_mint as metadata_quote_mint,
        quote_decimals as metadata_quote_decimals
    from token_metadata_current
    where mint = $1
),
enriched as (
    select
        time_bucket($2::interval, t.event_time) as bucket,
        t.event_time,
        t.event_id,
        trade_price_quote(
            t.quote_amount_raw,
            t.token_amount_raw,
            m.decimals,
            resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
        ) as price,
        scale_amount_numeric(
            t.quote_amount_raw,
            resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
        ) as volume_quote
    from token_trade_events t
    left join meta m on true
    where t.mint = $1
),
ranked as (
    select
        bucket,
        event_time,
        event_id,
        price,
        volume_quote,
        row_number() over (partition by bucket order by event_time asc, event_id asc) as rn_open,
        row_number() over (partition by bucket order by event_time desc, event_id desc) as rn_close
    from enriched
),
aggregated as (
    select
        bucket,
        max(price) filter (where rn_open = 1)::double precision as open,
        max(price)::double precision as high,
        min(price)::double precision as low,
        max(price) filter (where rn_close = 1)::double precision as close,
        coalesce(sum(volume_quote), 0)::double precision as volume
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

type ReadModelStore struct {
	db *db.DB
}

type TokenRecord struct {
	Mint             string  `json:"mint"`
	Creator          *string `json:"creator,omitempty"`
	BondingCurve     *string `json:"bonding_curve,omitempty"`
	TokenProgram     *string `json:"token_program,omitempty"`
	CreateEventID    *string `json:"create_event_id,omitempty"`
	FirstSeenAt      int64   `json:"first_seen_at"`
	CurrentStage     string  `json:"current_stage"`
	ActiveMarketID   *string `json:"active_market_id,omitempty"`
	ActiveMarketType *string `json:"active_market_type,omitempty"`
	MigratedAt       *int64  `json:"migrated_at,omitempty"`
}

type TokenSnapshotRecord struct {
	Mint              string  `json:"mint"`
	Name              *string `json:"name,omitempty"`
	Symbol            *string `json:"symbol,omitempty"`
	URI               *string `json:"uri,omitempty"`
	Decimals          *int32  `json:"decimals,omitempty"`
	TotalSupplyRaw    *string `json:"total_supply_raw,omitempty"`
	QuoteMint         *string `json:"quote_mint,omitempty"`
	QuoteDecimals     *int32  `json:"quote_decimals,omitempty"`
	Creator           *string `json:"creator,omitempty"`
	BondingCurve      *string `json:"bonding_curve,omitempty"`
	TokenProgram      *string `json:"token_program,omitempty"`
	FirstSeenAt       int64   `json:"first_seen_at"`
	CurrentStage      string  `json:"current_stage"`
	ActiveMarketID    *string `json:"active_market_id,omitempty"`
	ActiveMarketType  *string `json:"active_market_type,omitempty"`
	MigratedAt        *int64  `json:"migrated_at,omitempty"`
	CreateEventID     *string `json:"create_event_id,omitempty"`
	IsMayhemMode      *bool   `json:"is_mayhem_mode,omitempty"`
	IsCashbackEnabled *bool   `json:"is_cashback_enabled,omitempty"`
}

type TokenMetadataRecord struct {
	Mint              string  `json:"mint"`
	Name              *string `json:"name,omitempty"`
	Symbol            *string `json:"symbol,omitempty"`
	URI               *string `json:"uri,omitempty"`
	Decimals          *int32  `json:"decimals,omitempty"`
	TotalSupplyRaw    *string `json:"total_supply_raw,omitempty"`
	QuoteMint         *string `json:"quote_mint,omitempty"`
	QuoteDecimals     *int32  `json:"quote_decimals,omitempty"`
	Creator           *string `json:"creator,omitempty"`
	BondingCurve      *string `json:"bonding_curve,omitempty"`
	TokenProgram      *string `json:"token_program,omitempty"`
	IsMayhemMode      *bool   `json:"is_mayhem_mode,omitempty"`
	IsCashbackEnabled *bool   `json:"is_cashback_enabled,omitempty"`
	SourceEventID     *string `json:"source_event_id,omitempty"`
}

type TokenMarketRecord struct {
	MarketID          string  `json:"market_id"`
	Mint              string  `json:"mint"`
	Protocol          string  `json:"protocol"`
	MarketType        string  `json:"market_type"`
	BondingCurve      *string `json:"bonding_curve,omitempty"`
	Pool              *string `json:"pool,omitempty"`
	BaseMint          *string `json:"base_mint,omitempty"`
	QuoteMint         *string `json:"quote_mint,omitempty"`
	BaseMintDecimals  *int32  `json:"base_mint_decimals,omitempty"`
	QuoteMintDecimals *int32  `json:"quote_mint_decimals,omitempty"`
	LPMint            *string `json:"lp_mint,omitempty"`
	StartedAt         int64   `json:"started_at"`
	EndedAt           *int64  `json:"ended_at,omitempty"`
	CreateEventID     string  `json:"create_event_id"`
}

type TradeEventRecord struct {
	EventID        string `json:"event_id"`
	Mint           string `json:"mint"`
	MarketID       string `json:"market_id"`
	MarketType     string `json:"market_type"`
	Protocol       string `json:"protocol"`
	Side           string `json:"side"`
	IxName         string `json:"ix_name"`
	UserAddress    string `json:"user_address"`
	QuoteMint      string `json:"quote_mint"`
	TokenAmountRaw string `json:"token_amount_raw"`
	QuoteAmountRaw string `json:"quote_amount_raw"`
	TxSignature    string `json:"tx_signature"`
	Slot           uint64 `json:"slot"`
	EventUnixTS    int64  `json:"event_unix_ts"`
	RawEventSource string `json:"raw_event_source"`
}

type ActivityEventRecord struct {
	EventID        string          `json:"event_id"`
	Mint           string          `json:"mint"`
	Protocol       string          `json:"protocol"`
	EventType      string          `json:"event_type"`
	ActivityType   string          `json:"activity_type"`
	MarketID       *string         `json:"market_id,omitempty"`
	MarketType     *string         `json:"market_type,omitempty"`
	UserAddress    *string         `json:"user_address,omitempty"`
	Side           *string         `json:"side,omitempty"`
	QuoteMint      *string         `json:"quote_mint,omitempty"`
	TokenAmountRaw *string         `json:"token_amount_raw,omitempty"`
	QuoteAmountRaw *string         `json:"quote_amount_raw,omitempty"`
	TxSignature    string          `json:"tx_signature"`
	Slot           uint64          `json:"slot"`
	EventUnixTS    int64           `json:"event_unix_ts"`
	RawEventSource string          `json:"raw_event_source"`
	Details        json.RawMessage `json:"details"`
}

type TradeSummaryRecord struct {
	QuoteMint       string
	LatestPrice     *float64
	LatestEventUnix *int64
	Price5mAgo      *float64
	Price1hAgo      *float64
	Price6hAgo      *float64
	Price24hAgo     *float64
	Txns24h         int64
	Buys24h         int64
	Sells24h        int64
	Volume24h       float64
	BuyVolume24h    float64
	SellVolume24h   float64
	Makers24h       int64
	Buyers24h       int64
	Sellers24h      int64
}

type TokenCandleRecord struct {
	TimeUnix int64   `json:"time"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   float64 `json:"volume"`
}

func NewReadModelStore(database *db.DB) *ReadModelStore {
	return &ReadModelStore{db: database}
}

func (s *ReadModelStore) UpsertToken(ctx context.Context, record TokenRecord) error {
	firstSeenAt := float64(record.FirstSeenAt)
	var migratedAt *float64
	if record.MigratedAt != nil {
		value := float64(*record.MigratedAt)
		migratedAt = &value
	}

	_, err := s.db.Pool.Exec(
		ctx,
		upsertTokenSQL,
		record.Mint,
		record.Creator,
		record.BondingCurve,
		record.TokenProgram,
		record.CreateEventID,
		firstSeenAt,
		record.CurrentStage,
		record.ActiveMarketID,
		record.ActiveMarketType,
		migratedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert token: %w", err)
	}

	return nil
}

func (s *ReadModelStore) UpsertTokenMetadata(ctx context.Context, record TokenMetadataRecord) error {
	_, err := s.db.Pool.Exec(
		ctx,
		upsertTokenMetadataSQL,
		record.Mint,
		record.Name,
		record.Symbol,
		record.URI,
		record.Decimals,
		record.TotalSupplyRaw,
		record.QuoteMint,
		record.QuoteDecimals,
		record.Creator,
		record.BondingCurve,
		record.TokenProgram,
		record.IsMayhemMode,
		record.IsCashbackEnabled,
		record.SourceEventID,
	)
	if err != nil {
		return fmt.Errorf("upsert token metadata: %w", err)
	}

	return nil
}

func (s *ReadModelStore) UpsertTokenMarket(ctx context.Context, market TokenMarketRecord) error {
	startedAt := float64(market.StartedAt)
	var endedAt *float64
	if market.EndedAt != nil {
		value := float64(*market.EndedAt)
		endedAt = &value
	}

	_, err := s.db.Pool.Exec(
		ctx,
		upsertTokenMarketSQL,
		market.MarketID,
		market.Mint,
		market.Protocol,
		market.MarketType,
		market.BondingCurve,
		market.Pool,
		market.BaseMint,
		market.QuoteMint,
		market.BaseMintDecimals,
		market.QuoteMintDecimals,
		market.LPMint,
		startedAt,
		endedAt,
		market.CreateEventID,
	)
	if err != nil {
		return fmt.Errorf("upsert token market: %w", err)
	}

	return nil
}

func (s *ReadModelStore) CloseTokenMarket(ctx context.Context, marketID string, endedAt int64) error {
	_, err := s.db.Pool.Exec(ctx, closeTokenMarketSQL, marketID, float64(endedAt))
	if err != nil {
		return fmt.Errorf("close token market: %w", err)
	}

	return nil
}

func (s *ReadModelStore) InsertTradeEvent(ctx context.Context, trade TradeEventRecord) error {
	_, err := s.db.Pool.Exec(
		ctx,
		insertTokenTradeEventSQL,
		float64(trade.EventUnixTS),
		trade.EventID,
		trade.Mint,
		trade.MarketID,
		trade.MarketType,
		trade.Protocol,
		trade.Side,
		trade.IxName,
		trade.UserAddress,
		trade.QuoteMint,
		trade.TokenAmountRaw,
		trade.QuoteAmountRaw,
		trade.TxSignature,
		int64(trade.Slot),
		trade.RawEventSource,
	)
	if err != nil {
		return fmt.Errorf("insert trade event: %w", err)
	}

	return nil
}

func (s *ReadModelStore) InsertActivityEvent(ctx context.Context, activity ActivityEventRecord) error {
	details := activity.Details
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}

	_, err := s.db.Pool.Exec(
		ctx,
		insertTokenActivityEventSQL,
		float64(activity.EventUnixTS),
		activity.EventID,
		activity.Mint,
		activity.Protocol,
		activity.EventType,
		activity.ActivityType,
		activity.MarketID,
		activity.MarketType,
		activity.UserAddress,
		activity.Side,
		activity.QuoteMint,
		activity.TokenAmountRaw,
		activity.QuoteAmountRaw,
		activity.TxSignature,
		int64(activity.Slot),
		activity.RawEventSource,
		[]byte(details),
	)
	if err != nil {
		return fmt.Errorf("insert activity event: %w", err)
	}

	return nil
}

func (s *ReadModelStore) ListTokenSnapshots(ctx context.Context, limit int) ([]TokenSnapshotRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTokenSnapshotsSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("list token snapshots: %w", err)
	}
	defer rows.Close()

	items := make([]TokenSnapshotRecord, 0, limit)
	for rows.Next() {
		record, err := scanTokenSnapshot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate token snapshots: %w", err)
	}

	return items, nil
}

func (s *ReadModelStore) FindTokenSnapshotByMint(ctx context.Context, mint string) (*TokenSnapshotRecord, error) {
	rows, err := s.db.Pool.Query(ctx, findTokenSnapshotByMintSQL, mint)
	if err != nil {
		return nil, fmt.Errorf("find token snapshot by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate token snapshot by mint=%s: %w", mint, err)
		}
		return nil, nil
	}

	record, err := scanTokenSnapshot(rows)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (s *ReadModelStore) ListTokenMarketsByMint(ctx context.Context, mint string, limit int) ([]TokenMarketRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTokenMarketsByMintSQL, mint, limit)
	if err != nil {
		return nil, fmt.Errorf("list token markets by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	items := make([]TokenMarketRecord, 0, limit)
	for rows.Next() {
		record, err := scanTokenMarket(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate token markets by mint=%s: %w", mint, err)
	}

	return items, nil
}

func (s *ReadModelStore) ListTradeEventsByMint(ctx context.Context, mint string, limit int) ([]TradeEventRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTokenTradeEventsByMintSQL, mint, limit)
	if err != nil {
		return nil, fmt.Errorf("list trade events by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	items := make([]TradeEventRecord, 0, limit)
	for rows.Next() {
		record, err := scanTradeEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trade events by mint=%s: %w", mint, err)
	}

	return items, nil
}

func (s *ReadModelStore) LoadLatestTradeByMint(ctx context.Context, mint string) (*TradeEventRecord, error) {
	rows, err := s.db.Pool.Query(ctx, loadLatestTokenTradeByMintSQL, mint)
	if err != nil {
		return nil, fmt.Errorf("load latest trade by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate latest trade by mint=%s: %w", mint, err)
		}
		return nil, nil
	}

	record, err := scanTradeEvent(rows)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (s *ReadModelStore) ListActivityEventsByMint(ctx context.Context, mint string, limit int) ([]ActivityEventRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTokenActivityEventsByMintSQL, mint, limit)
	if err != nil {
		return nil, fmt.Errorf("list activity events by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	items := make([]ActivityEventRecord, 0, limit)
	for rows.Next() {
		record, err := scanActivityEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity events by mint=%s: %w", mint, err)
	}

	return items, nil
}

func (s *ReadModelStore) LoadTradeSummaryByMint(ctx context.Context, mint string) (*TradeSummaryRecord, error) {
	row := s.db.Pool.QueryRow(ctx, loadTokenTradeSummarySQL, mint)

	var summary TradeSummaryRecord
	err := row.Scan(
		&summary.QuoteMint,
		&summary.LatestPrice,
		&summary.LatestEventUnix,
		&summary.Price5mAgo,
		&summary.Price1hAgo,
		&summary.Price6hAgo,
		&summary.Price24hAgo,
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load trade summary by mint=%s: %w", mint, err)
	}

	return &summary, nil
}

func (s *ReadModelStore) ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int) ([]TokenCandleRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTokenCandlesByMintSQL, mint, resolution, limit)
	if err != nil {
		return nil, fmt.Errorf("list candles by mint=%s resolution=%s: %w", mint, resolution, err)
	}
	defer rows.Close()

	candles := make([]TokenCandleRecord, 0, limit)
	for rows.Next() {
		var candle TokenCandleRecord
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

type tokenSnapshotScanner interface {
	Scan(dest ...any) error
}

func scanTokenSnapshot(scanner tokenSnapshotScanner) (TokenSnapshotRecord, error) {
	var record TokenSnapshotRecord
	if err := scanner.Scan(
		&record.Mint,
		&record.Name,
		&record.Symbol,
		&record.URI,
		&record.Decimals,
		&record.TotalSupplyRaw,
		&record.QuoteMint,
		&record.QuoteDecimals,
		&record.Creator,
		&record.BondingCurve,
		&record.TokenProgram,
		&record.FirstSeenAt,
		&record.CurrentStage,
		&record.ActiveMarketID,
		&record.ActiveMarketType,
		&record.MigratedAt,
		&record.CreateEventID,
		&record.IsMayhemMode,
		&record.IsCashbackEnabled,
	); err != nil {
		return TokenSnapshotRecord{}, fmt.Errorf("scan token snapshot: %w", err)
	}

	return record, nil
}

type tokenMarketScanner interface {
	Scan(dest ...any) error
}

func scanTokenMarket(scanner tokenMarketScanner) (TokenMarketRecord, error) {
	var record TokenMarketRecord
	if err := scanner.Scan(
		&record.MarketID,
		&record.Mint,
		&record.Protocol,
		&record.MarketType,
		&record.BondingCurve,
		&record.Pool,
		&record.BaseMint,
		&record.QuoteMint,
		&record.BaseMintDecimals,
		&record.QuoteMintDecimals,
		&record.LPMint,
		&record.StartedAt,
		&record.EndedAt,
		&record.CreateEventID,
	); err != nil {
		return TokenMarketRecord{}, fmt.Errorf("scan token market: %w", err)
	}

	return record, nil
}

type tradeEventScanner interface {
	Scan(dest ...any) error
}

func scanTradeEvent(scanner tradeEventScanner) (TradeEventRecord, error) {
	var (
		record TradeEventRecord
		slot   int64
	)
	if err := scanner.Scan(
		&record.EventID,
		&record.Mint,
		&record.MarketID,
		&record.MarketType,
		&record.Protocol,
		&record.Side,
		&record.IxName,
		&record.UserAddress,
		&record.QuoteMint,
		&record.TokenAmountRaw,
		&record.QuoteAmountRaw,
		&record.TxSignature,
		&slot,
		&record.EventUnixTS,
		&record.RawEventSource,
	); err != nil {
		return TradeEventRecord{}, fmt.Errorf("scan trade event: %w", err)
	}
	record.Slot = uint64(slot)

	return record, nil
}

type activityEventScanner interface {
	Scan(dest ...any) error
}

func scanActivityEvent(scanner activityEventScanner) (ActivityEventRecord, error) {
	var (
		record ActivityEventRecord
		slot   int64
	)
	if err := scanner.Scan(
		&record.EventID,
		&record.Mint,
		&record.Protocol,
		&record.EventType,
		&record.ActivityType,
		&record.MarketID,
		&record.MarketType,
		&record.UserAddress,
		&record.Side,
		&record.QuoteMint,
		&record.TokenAmountRaw,
		&record.QuoteAmountRaw,
		&record.TxSignature,
		&slot,
		&record.EventUnixTS,
		&record.RawEventSource,
		&record.Details,
	); err != nil {
		return ActivityEventRecord{}, fmt.Errorf("scan activity event: %w", err)
	}
	record.Slot = uint64(slot)

	return record, nil
}
