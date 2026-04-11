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
    m.image_uri,
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

const listTokenBoardRowsSQL = `
with enriched as (
    select
        t.mint,
        m.name,
        m.symbol,
        m.uri,
        m.image_uri,
        m.decimals,
        m.total_supply_raw,
        m.quote_mint,
        m.quote_decimals,
        coalesce(m.creator, t.creator) as creator,
        coalesce(m.bonding_curve, t.bonding_curve) as bonding_curve,
        coalesce(m.token_program, t.token_program) as token_program,
        extract(epoch from t.first_seen_at)::bigint as first_seen_at_unix,
        extract(epoch from greatest(t.first_seen_at, coalesce(t.migrated_at, t.first_seen_at)))::bigint as active_since_unix,
        t.current_stage,
        t.active_market_id,
        t.active_market_type,
        extract(epoch from t.migrated_at)::bigint as migrated_at_unix,
        am.base_mint as active_base_mint,
        am.quote_mint as active_quote_mint,
        am.base_mint_decimals as active_base_mint_decimals,
        am.quote_mint_decimals as active_quote_mint_decimals,
        tm.latest_price,
        extract(epoch from tm.latest_trade_at)::bigint as latest_event_unix_ts,
        case
            when $2 = '1m' then tm.anchor_price_1m
            when $2 = '5m' then tm.anchor_price_5m
            when $2 = '1h' then tm.anchor_price_1h
            when $2 = '4h' then tm.anchor_price_4h
            else tm.anchor_price_24h
        end as anchor_price,
        case
            when $2 = '1m' then tm.volume_1m
            when $2 = '5m' then tm.volume_5m
            when $2 = '1h' then tm.volume_1h
            when $2 = '4h' then tm.volume_4h
            else tm.volume_24h
        end as volume_window,
        case
            when $2 = '1m' then tm.txns_1m
            when $2 = '5m' then tm.txns_5m
            when $2 = '1h' then tm.txns_1h
            when $2 = '4h' then tm.txns_4h
            else tm.txns_24h
        end as txns_window,
        case
            when $2 = '1m' then tm.buys_1m
            when $2 = '5m' then tm.buys_5m
            when $2 = '1h' then tm.buys_1h
            when $2 = '4h' then tm.buys_4h
            else tm.buys_24h
        end as buys_window,
        case
            when $2 = '1m' then tm.sells_1m
            when $2 = '5m' then tm.sells_5m
            when $2 = '1h' then tm.sells_1h
            when $2 = '4h' then tm.sells_4h
            else tm.sells_24h
        end as sells_window,
        case
            when m.total_supply_raw is not null and tm.latest_price is not null then (
                scale_amount_numeric(m.total_supply_raw, m.decimals)::double precision
                * tm.latest_price
            )
            else null
        end as market_cap_quote
    from tokens t
    left join token_metadata_current m
        on m.mint = t.mint
    left join token_markets am
        on am.market_id = t.active_market_id
    left join token_metrics_current tm
        on tm.mint = t.mint
)
select
    e.mint,
    e.name,
    e.symbol,
    e.uri,
    e.image_uri,
    e.creator,
    e.bonding_curve,
    e.token_program,
    e.decimals,
    e.quote_mint,
    e.quote_decimals,
    e.first_seen_at_unix,
    e.active_since_unix,
    e.current_stage,
    e.active_market_id,
    e.active_market_type,
    e.migrated_at_unix,
    e.latest_price,
    e.latest_event_unix_ts,
    case
        when e.latest_price is null or e.anchor_price is null or e.anchor_price = 0 then null
        else ((e.latest_price - e.anchor_price) / e.anchor_price) * 100
    end as price_change,
    coalesce(e.volume_window, 0)::double precision as volume_window,
    coalesce(e.txns_window, 0)::bigint as txns_window,
    coalesce(e.buys_window, 0)::bigint as buys_window,
    coalesce(e.sells_window, 0)::bigint as sells_window,
    reserve_state.liquidity_quote,
    e.market_cap_quote
from enriched e
left join lateral (
    select
        case
            when se.protocol = 'pumpfun' then (
                coalesce(
                    scale_amount_numeric(nullif(se.payload->>'real_sol_reserves', '')::numeric, 9),
                    scale_amount_numeric(nullif(se.payload->>'virtual_sol_reserves', '')::numeric, 9),
                    0
                )::double precision
                + (
                    case
                        when e.latest_price is null then 0
                        else coalesce(
                            scale_amount_numeric(
                                coalesce(
                                    nullif(se.payload->>'real_token_reserves', ''),
                                    nullif(se.payload->>'virtual_token_reserves', '')
                                )::numeric,
                                coalesce(e.decimals, 6)
                            ),
                            0
                        )::double precision * e.latest_price
                    end
                )
            )
            when se.protocol = 'pumpamm' then (
                coalesce(
                    scale_amount_numeric(
                        (
                            case
                                when coalesce(nullif(se.payload->>'base_mint', ''), e.active_base_mint) = e.mint then
                                    coalesce(
                                        nullif(se.payload->>'pool_quote_token_reserves', ''),
                                        nullif(se.payload->>'quote_amount_in', '')
                                    )
                                when coalesce(nullif(se.payload->>'quote_mint', ''), e.active_quote_mint) = e.mint then
                                    coalesce(
                                        nullif(se.payload->>'pool_base_token_reserves', ''),
                                        nullif(se.payload->>'base_amount_in', '')
                                    )
                            end
                        )::numeric,
                        case
                            when coalesce(nullif(se.payload->>'base_mint', ''), e.active_base_mint) = e.mint then
                                coalesce(
                                    e.active_quote_mint_decimals,
                                    case
                                        when coalesce(nullif(se.payload->>'quote_mint', ''), e.active_quote_mint) = 'So11111111111111111111111111111111111111112'
                                            then 9
                                        else e.quote_decimals
                                    end
                                )
                            when coalesce(nullif(se.payload->>'quote_mint', ''), e.active_quote_mint) = e.mint then
                                coalesce(
                                    e.active_base_mint_decimals,
                                    case
                                        when coalesce(nullif(se.payload->>'base_mint', ''), e.active_base_mint) = 'So11111111111111111111111111111111111111112'
                                            then 9
                                        else e.quote_decimals
                                    end
                                )
                        end
                    ),
                    0
                )::double precision
                + (
                    case
                        when e.latest_price is null then 0
                        else coalesce(
                            scale_amount_numeric(
                                (
                                    case
                                        when coalesce(nullif(se.payload->>'base_mint', ''), e.active_base_mint) = e.mint then
                                            coalesce(
                                                nullif(se.payload->>'pool_base_token_reserves', ''),
                                                nullif(se.payload->>'base_amount_in', '')
                                            )
                                        when coalesce(nullif(se.payload->>'quote_mint', ''), e.active_quote_mint) = e.mint then
                                            coalesce(
                                                nullif(se.payload->>'pool_quote_token_reserves', ''),
                                                nullif(se.payload->>'quote_amount_in', '')
                                            )
                                    end
                                )::numeric,
                                coalesce(e.decimals, e.active_base_mint_decimals, 6)
                            ),
                            0
                        )::double precision * e.latest_price
                    end
                )
            )
            else null
        end as liquidity_quote
    from service_events se
    where se.refs->>'mint' = e.mint
      and (
          (se.protocol = 'pumpfun' and se.event_type in ('trade', 'create'))
          or (se.protocol = 'pumpamm' and se.event_type in ('swap', 'create_pool'))
      )
    order by se.slot desc, se.tx_index desc, se.outer_index desc, se.inner_index desc nulls last
    limit 1
) reserve_state on true
order by
    case when $3 = 'hot' then coalesce(e.txns_window, 0) end desc nulls last,
    case when $3 = 'hot' then coalesce(e.volume_window, 0) end desc nulls last,
    case when $3 = 'new' then e.active_since_unix end desc nulls last,
    e.first_seen_at_unix desc
limit $1
`

const findTokenSnapshotByMintSQL = `
select
    t.mint,
    m.name,
    m.symbol,
    m.uri,
    m.image_uri,
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

const searchTokenSnapshotsSQL = `
select
    t.mint,
    m.name,
    m.symbol,
    m.image_uri,
    tm.latest_price
from tokens t
left join token_metadata_current m
    on m.mint = t.mint
left join token_metrics_current tm
    on tm.mint = t.mint
where (
    t.mint ilike $1
    or coalesce(m.name, '') ilike $1
    or coalesce(m.symbol, '') ilike $1
)
order by
    case
        when lower(t.mint) = lower($2) then 0
        when lower(coalesce(m.symbol, '')) = lower($2) then 1
        when lower(coalesce(m.name, '')) = lower($2) then 2
        when lower(t.mint) like lower($2) || '%' then 3
        when lower(coalesce(m.symbol, '')) like lower($2) || '%' then 4
        when lower(coalesce(m.name, '')) like lower($2) || '%' then 5
        else 6
    end,
    t.first_seen_at desc
limit $3
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
order by event_time desc, slot desc, insert_seq desc
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
    details,
    insert_seq
from token_activity_events
where mint = $1
order by event_time desc, slot desc, insert_seq desc
limit $2
`

const listTokenActivityEventsPageByMintSQL = `
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
    details,
    insert_seq
from token_activity_events
where mint = $1
  and (
      $3::bigint is null
      or (extract(epoch from event_time)::bigint, slot, insert_seq) < ($3::bigint, $4::bigint, $5::bigint)
  )
order by event_time desc, slot desc, insert_seq desc
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
order by event_time desc, slot desc, insert_seq desc
limit 1
`

const loadTokenTradeSummaryCurrentSQL = `
select
    m.quote_mint,
    tm.latest_price,
    extract(epoch from tm.latest_trade_at)::bigint as latest_event_unix_ts,
    tm.anchor_price_5m,
    tm.anchor_price_1h,
    tm.anchor_price_4h,
    tm.anchor_price_24h,
    tm.txns_24h,
    tm.buys_24h,
    tm.sells_24h,
    tm.volume_24h,
    tm.buy_volume_24h,
    tm.sell_volume_24h,
    0::bigint as makers_24h,
    0::bigint as buyers_24h,
    0::bigint as sellers_24h
from token_metrics_current tm
left join token_metadata_current m
    on m.mint = tm.mint
where tm.mint = $1
`

const loadTokenMetricsCurrentByMintSQL = `
select
    mint,
    latest_price,
    extract(epoch from latest_trade_at)::bigint as latest_event_unix_ts,
    anchor_price_1m,
    anchor_price_5m,
    anchor_price_1h,
    anchor_price_4h,
    anchor_price_24h,
    txns_1m,
    txns_5m,
    txns_1h,
    txns_4h,
    txns_24h,
    buys_1m,
    buys_5m,
    buys_1h,
    buys_4h,
    buys_24h,
    sells_1m,
    sells_5m,
    sells_1h,
    sells_4h,
    sells_24h,
    volume_1m,
    volume_5m,
    volume_1h,
    volume_4h,
    volume_24h,
    buy_volume_1m,
    buy_volume_5m,
    buy_volume_1h,
    buy_volume_4h,
    buy_volume_24h,
    sell_volume_1m,
    sell_volume_5m,
    sell_volume_1h,
    sell_volume_4h,
    sell_volume_24h,
    source_log_id
from token_metrics_current
where mint = $1
`

const loadTokenTradeSummaryFallbackSQL = `
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
		slot,
		insert_seq,
        quote_mint,
        token_amount_raw,
        quote_amount_raw
    from token_trade_events
    where mint = $1
	order by event_time desc, slot desc, insert_seq desc
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
						order by t.event_time desc, t.slot desc, t.insert_seq desc
            limit 1
        ) as price_5m_anchor,
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
						order by t.event_time desc, t.slot desc, t.insert_seq desc
            limit 1
        ) as price_1h_anchor,
        (
            select trade_price_quote(
                t.quote_amount_raw,
                t.token_amount_raw,
                m.decimals,
                resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
            )
            from token_trade_events t
            where t.mint = $1
              and t.event_time <= l.event_time - interval '4 hours'
						order by t.event_time desc, t.slot desc, t.insert_seq desc
            limit 1
        ) as price_4h_anchor,
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
						order by t.event_time desc, t.slot desc, t.insert_seq desc
            limit 1
        ) as price_24h_anchor
    from latest l
    left join meta m on true
),
anchors as (
    select
        quote_mint,
        latest_event_unix_ts,
        latest_price,
        coalesce(
            price_5m_anchor,
            (
                select trade_price_quote(
                    t.quote_amount_raw,
                    t.token_amount_raw,
                    m.decimals,
                    resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
                )
                from token_trade_events t
                where t.mint = $1
                  and t.event_time >= to_timestamp(latest_event_unix_ts) - interval '5 minutes'
                order by t.event_time asc, t.slot asc, t.insert_seq asc
                limit 1
            )
        ) as price_5m_ago,
        coalesce(
            price_1h_anchor,
            (
                select trade_price_quote(
                    t.quote_amount_raw,
                    t.token_amount_raw,
                    m.decimals,
                    resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
                )
                from token_trade_events t
                where t.mint = $1
                  and t.event_time >= to_timestamp(latest_event_unix_ts) - interval '1 hour'
                order by t.event_time asc, t.slot asc, t.insert_seq asc
                limit 1
            )
        ) as price_1h_ago,
        coalesce(
            price_4h_anchor,
            (
                select trade_price_quote(
                    t.quote_amount_raw,
                    t.token_amount_raw,
                    m.decimals,
                    resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
                )
                from token_trade_events t
                where t.mint = $1
                  and t.event_time >= to_timestamp(latest_event_unix_ts) - interval '4 hours'
                order by t.event_time asc, t.slot asc, t.insert_seq asc
                limit 1
            )
        ) as price_4h_ago,
        coalesce(
            price_24h_anchor,
            (
                select trade_price_quote(
                    t.quote_amount_raw,
                    t.token_amount_raw,
                    m.decimals,
                    resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
                )
                from token_trade_events t
                where t.mint = $1
                  and t.event_time >= to_timestamp(latest_event_unix_ts) - interval '24 hours'
                order by t.event_time asc, t.slot asc, t.insert_seq asc
                limit 1
            )
        ) as price_24h_ago
    from prices
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
    p.price_4h_ago,
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
from anchors p
cross join stats_24h s
`

const listTradeMetricsForStatsByMintSQL = `
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
        slot,
        insert_seq
    from token_trade_events
    where mint = $1
    order by event_time desc, slot desc, insert_seq desc
    limit 1
),
anchor as (
    select
        extract(epoch from t.event_time)::bigint as event_unix_ts,
        t.slot::bigint as slot,
        t.insert_seq::bigint as insert_seq,
        t.side,
        trade_price_quote(
            t.quote_amount_raw,
            t.token_amount_raw,
            m.decimals,
            resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as price,
        scale_amount_numeric(
            t.quote_amount_raw,
            resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as volume_quote
    from token_trade_events t
    join latest l on true
    left join meta m on true
    where t.mint = $1
      and (t.event_time, t.slot, t.insert_seq) <= (
          l.event_time - interval '24 hours',
          9223372036854775807::bigint,
          9223372036854775807::bigint
      )
    order by t.event_time desc, t.slot desc, t.insert_seq desc
    limit 1
),
windowed as (
    select
        extract(epoch from t.event_time)::bigint as event_unix_ts,
        t.slot::bigint as slot,
        t.insert_seq::bigint as insert_seq,
        t.side,
        trade_price_quote(
            t.quote_amount_raw,
            t.token_amount_raw,
            m.decimals,
            resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as price,
        scale_amount_numeric(
            t.quote_amount_raw,
            resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as volume_quote
    from token_trade_events t
    join latest l on true
    left join meta m on true
    where t.mint = $1
      and t.event_time > l.event_time - interval '24 hours'
)
select
    event_unix_ts,
    slot,
    insert_seq,
    side,
    price,
    volume_quote
from (
    select * from anchor
    union all
    select * from windowed
) trades
order by event_unix_ts asc, slot asc, insert_seq asc
`

const listRecentTradeMetricsByMintSQL = `
with meta as (
    select
        decimals,
        quote_mint as metadata_quote_mint,
        quote_decimals as metadata_quote_decimals
    from token_metadata_current
    where mint = $1
)
select
    extract(epoch from t.event_time)::bigint as event_unix_ts,
    t.slot::bigint as slot,
    t.insert_seq::bigint as insert_seq,
    t.side,
    trade_price_quote(
        t.quote_amount_raw,
        t.token_amount_raw,
        m.decimals,
        resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
    )::double precision as price,
    scale_amount_numeric(
        t.quote_amount_raw,
        resolved_quote_decimals(t.quote_mint, m.metadata_quote_mint, m.metadata_quote_decimals)
    )::double precision as volume_quote
from token_trade_events t
left join meta m on true
where t.mint = $1
  and t.event_time >= to_timestamp($2)
order by t.event_time asc, t.slot asc, t.insert_seq asc
`

const listLongWindowMetricsByMintsSQL = `
with requested as (
    select unnest($1::text[]) as mint
),
meta as (
    select
        r.mint,
        m.decimals,
        m.quote_mint as metadata_quote_mint,
        m.quote_decimals as metadata_quote_decimals
    from requested r
    left join token_metadata_current m
        on m.mint = r.mint
),
minute_metrics as (
    select
        tm.mint,
        tm.bucket,
        normalize_raw_trade_price(
            tm.open_raw_price,
            meta.decimals,
            resolved_quote_decimals(tm.quote_mint_sample, meta.metadata_quote_mint, meta.metadata_quote_decimals)
        )::double precision as open_price,
        tm.txns,
        tm.buys,
        tm.sells,
        scale_amount_numeric(
            tm.volume_quote_raw,
            resolved_quote_decimals(tm.quote_mint_sample, meta.metadata_quote_mint, meta.metadata_quote_decimals)
        )::double precision as volume,
        scale_amount_numeric(
            tm.buy_volume_quote_raw,
            resolved_quote_decimals(tm.quote_mint_sample, meta.metadata_quote_mint, meta.metadata_quote_decimals)
        )::double precision as buy_volume,
        scale_amount_numeric(
            tm.sell_volume_quote_raw,
            resolved_quote_decimals(tm.quote_mint_sample, meta.metadata_quote_mint, meta.metadata_quote_decimals)
        )::double precision as sell_volume
    from token_trade_metrics_1m tm
    join meta
        on meta.mint = tm.mint
    where tm.mint = any($1)
      and tm.bucket >= to_timestamp($2) - interval '24 hours'
      and tm.bucket < to_timestamp($2) - interval '5 minutes'
)
select
    r.mint,
    first(mm.open_price, mm.bucket) filter (where mm.bucket >= to_timestamp($2) - interval '1 hour') as anchor_price_1h,
    first(mm.open_price, mm.bucket) filter (where mm.bucket >= to_timestamp($2) - interval '4 hours') as anchor_price_4h,
    first(mm.open_price, mm.bucket) filter (where mm.bucket >= to_timestamp($2) - interval '24 hours') as anchor_price_24h,
    coalesce(sum(mm.txns) filter (where mm.bucket >= to_timestamp($2) - interval '1 hour'), 0)::bigint as txns_1h,
    coalesce(sum(mm.txns) filter (where mm.bucket >= to_timestamp($2) - interval '4 hours'), 0)::bigint as txns_4h,
    coalesce(sum(mm.txns) filter (where mm.bucket >= to_timestamp($2) - interval '24 hours'), 0)::bigint as txns_24h,
    coalesce(sum(mm.buys) filter (where mm.bucket >= to_timestamp($2) - interval '1 hour'), 0)::bigint as buys_1h,
    coalesce(sum(mm.buys) filter (where mm.bucket >= to_timestamp($2) - interval '4 hours'), 0)::bigint as buys_4h,
    coalesce(sum(mm.buys) filter (where mm.bucket >= to_timestamp($2) - interval '24 hours'), 0)::bigint as buys_24h,
    coalesce(sum(mm.sells) filter (where mm.bucket >= to_timestamp($2) - interval '1 hour'), 0)::bigint as sells_1h,
    coalesce(sum(mm.sells) filter (where mm.bucket >= to_timestamp($2) - interval '4 hours'), 0)::bigint as sells_4h,
    coalesce(sum(mm.sells) filter (where mm.bucket >= to_timestamp($2) - interval '24 hours'), 0)::bigint as sells_24h,
    coalesce(sum(mm.volume) filter (where mm.bucket >= to_timestamp($2) - interval '1 hour'), 0)::double precision as volume_1h,
    coalesce(sum(mm.volume) filter (where mm.bucket >= to_timestamp($2) - interval '4 hours'), 0)::double precision as volume_4h,
    coalesce(sum(mm.volume) filter (where mm.bucket >= to_timestamp($2) - interval '24 hours'), 0)::double precision as volume_24h,
    coalesce(sum(mm.buy_volume) filter (where mm.bucket >= to_timestamp($2) - interval '1 hour'), 0)::double precision as buy_volume_1h,
    coalesce(sum(mm.buy_volume) filter (where mm.bucket >= to_timestamp($2) - interval '4 hours'), 0)::double precision as buy_volume_4h,
    coalesce(sum(mm.buy_volume) filter (where mm.bucket >= to_timestamp($2) - interval '24 hours'), 0)::double precision as buy_volume_24h,
    coalesce(sum(mm.sell_volume) filter (where mm.bucket >= to_timestamp($2) - interval '1 hour'), 0)::double precision as sell_volume_1h,
    coalesce(sum(mm.sell_volume) filter (where mm.bucket >= to_timestamp($2) - interval '4 hours'), 0)::double precision as sell_volume_4h,
    coalesce(sum(mm.sell_volume) filter (where mm.bucket >= to_timestamp($2) - interval '24 hours'), 0)::double precision as sell_volume_24h
from requested r
left join minute_metrics mm
    on mm.mint = r.mint
group by r.mint
`

const upsertTokenMetricsCurrentSQL = `
insert into token_metrics_current (
    mint,
    latest_price,
    latest_trade_at,
    anchor_price_1m,
    anchor_price_5m,
    anchor_price_1h,
    anchor_price_4h,
    anchor_price_24h,
    txns_1m,
    txns_5m,
    txns_1h,
    txns_4h,
    txns_24h,
    buys_1m,
    buys_5m,
    buys_1h,
    buys_4h,
    buys_24h,
    sells_1m,
    sells_5m,
    sells_1h,
    sells_4h,
    sells_24h,
    volume_1m,
    volume_5m,
    volume_1h,
    volume_4h,
    volume_24h,
    buy_volume_1m,
    buy_volume_5m,
    buy_volume_1h,
    buy_volume_4h,
    buy_volume_24h,
    sell_volume_1m,
    sell_volume_5m,
    sell_volume_1h,
    sell_volume_4h,
    sell_volume_24h,
    source_log_id,
    updated_at
) values (
    $1,
    $2,
    to_timestamp($3),
    $4, $5, $6, $7, $8,
    $9, $10, $11, $12, $13,
    $14, $15, $16, $17, $18,
    $19, $20, $21, $22, $23,
    $24, $25, $26, $27, $28,
    $29, $30, $31, $32, $33,
    $34, $35, $36, $37, $38,
    $39,
    now()
)
on conflict (mint) do update set
    latest_price = excluded.latest_price,
    latest_trade_at = excluded.latest_trade_at,
    anchor_price_1m = excluded.anchor_price_1m,
    anchor_price_5m = excluded.anchor_price_5m,
    anchor_price_1h = excluded.anchor_price_1h,
    anchor_price_4h = excluded.anchor_price_4h,
    anchor_price_24h = excluded.anchor_price_24h,
    txns_1m = excluded.txns_1m,
    txns_5m = excluded.txns_5m,
    txns_1h = excluded.txns_1h,
    txns_4h = excluded.txns_4h,
    txns_24h = excluded.txns_24h,
    buys_1m = excluded.buys_1m,
    buys_5m = excluded.buys_5m,
    buys_1h = excluded.buys_1h,
    buys_4h = excluded.buys_4h,
    buys_24h = excluded.buys_24h,
    sells_1m = excluded.sells_1m,
    sells_5m = excluded.sells_5m,
    sells_1h = excluded.sells_1h,
    sells_4h = excluded.sells_4h,
    sells_24h = excluded.sells_24h,
    volume_1m = excluded.volume_1m,
    volume_5m = excluded.volume_5m,
    volume_1h = excluded.volume_1h,
    volume_4h = excluded.volume_4h,
    volume_24h = excluded.volume_24h,
    buy_volume_1m = excluded.buy_volume_1m,
    buy_volume_5m = excluded.buy_volume_5m,
    buy_volume_1h = excluded.buy_volume_1h,
    buy_volume_4h = excluded.buy_volume_4h,
    buy_volume_24h = excluded.buy_volume_24h,
    sell_volume_1m = excluded.sell_volume_1m,
    sell_volume_5m = excluded.sell_volume_5m,
    sell_volume_1h = excluded.sell_volume_1h,
    sell_volume_4h = excluded.sell_volume_4h,
    sell_volume_24h = excluded.sell_volume_24h,
    source_log_id = excluded.source_log_id,
	updated_at = now()
`

const listMetricBackfillCandidatesSQL = `
with latest_trade as (
    select
        mint,
        max(event_time) as latest_trade_at
    from token_trade_events
    group by mint
)
select lt.mint
from latest_trade lt
left join token_metrics_current tm
    on tm.mint = lt.mint
where tm.mint is null
order by lt.latest_trade_at desc
limit $1
`

const listLatestTradeSeedsByMintsSQL = `
with requested as (
    select unnest($1::text[]) as mint
),
meta as (
    select
        r.mint,
        m.decimals,
        m.quote_mint as metadata_quote_mint,
        m.quote_decimals as metadata_quote_decimals
    from requested r
    left join token_metadata_current m
        on m.mint = r.mint
),
latest as (
    select distinct on (t.mint)
        t.mint,
        extract(epoch from t.event_time)::bigint as latest_event_unix_ts,
        trade_price_quote(
            t.quote_amount_raw,
            t.token_amount_raw,
            meta.decimals,
            resolved_quote_decimals(t.quote_mint, meta.metadata_quote_mint, meta.metadata_quote_decimals)
        )::double precision as latest_price
    from token_trade_events t
    join meta
        on meta.mint = t.mint
    where t.mint = any($1)
    order by t.mint, t.event_time desc, t.slot desc, t.insert_seq desc
)
select
    r.mint,
    latest.latest_price,
    latest.latest_event_unix_ts
from requested r
left join latest
    on latest.mint = r.mint
`

const listTokenCandlesByMintSQLTemplate = `
with meta as (
    select
        decimals,
        quote_mint as metadata_quote_mint,
        quote_decimals as metadata_quote_decimals
    from token_metadata_current
    where mint = $1
),
limited as (
    select
        extract(epoch from c.bucket)::bigint as time_unix,
        normalize_raw_trade_price(
            c.open_raw_price,
            m.decimals,
            resolved_quote_decimals(c.quote_mint_sample, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as open,
        normalize_raw_trade_price(
            c.high_raw_price,
            m.decimals,
            resolved_quote_decimals(c.quote_mint_sample, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as high,
        normalize_raw_trade_price(
            c.low_raw_price,
            m.decimals,
            resolved_quote_decimals(c.quote_mint_sample, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as low,
        normalize_raw_trade_price(
            c.close_raw_price,
            m.decimals,
            resolved_quote_decimals(c.quote_mint_sample, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as close,
        scale_amount_numeric(
            c.volume_quote_raw,
            resolved_quote_decimals(c.quote_mint_sample, m.metadata_quote_mint, m.metadata_quote_decimals)
        )::double precision as volume,
        false as is_gapfill
    from %s c
    left join meta m on true
    where c.mint = $1
      and ($2::bigint is null or c.bucket < to_timestamp($2))
    order by c.bucket desc
    limit $3
)
select
    time_unix,
    open,
    high,
    low,
    close,
    volume,
    is_gapfill
from limited
order by time_unix asc
`

var candleAggregateViews = map[string]string{
	"1 minute":   "token_candles_1m",
	"5 minutes":  "token_candles_5m",
	"15 minutes": "token_candles_15m",
	"1 hour":     "token_candles_1h",
	"4 hours":    "token_candles_4h",
	"1 day":      "token_candles_1d",
}

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
	ImageURI          *string `json:"image_uri,omitempty"`
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

type TokenBoardQuery struct {
	Limit    int
	View     string
	Interval string
}

type TokenBoardRecord struct {
	Mint              string   `json:"mint"`
	Name              *string  `json:"name,omitempty"`
	Symbol            *string  `json:"symbol,omitempty"`
	URI               *string  `json:"uri,omitempty"`
	ImageURI          *string  `json:"image_uri,omitempty"`
	Creator           *string  `json:"creator,omitempty"`
	BondingCurve      *string  `json:"bonding_curve,omitempty"`
	TokenProgram      *string  `json:"token_program,omitempty"`
	Decimals          *int32   `json:"decimals,omitempty"`
	QuoteMint         *string  `json:"quote_mint,omitempty"`
	QuoteDecimals     *int32   `json:"quote_decimals,omitempty"`
	FirstSeenAt       int64    `json:"first_seen_at"`
	ActiveSince       int64    `json:"active_since"`
	CurrentStage      string   `json:"current_stage"`
	ActiveMarketID    *string  `json:"active_market_id,omitempty"`
	ActiveMarketType  *string  `json:"active_market_type,omitempty"`
	MigratedAt        *int64   `json:"migrated_at,omitempty"`
	LatestPrice       *float64 `json:"latest_price,omitempty"`
	LatestEventUnixTS *int64   `json:"latest_event_unix_ts,omitempty"`
	PriceChange       *float64 `json:"price_change,omitempty"`
	WindowVolume      float64  `json:"window_volume"`
	WindowTxns        int64    `json:"window_txns"`
	WindowBuys        int64    `json:"window_buys"`
	WindowSells       int64    `json:"window_sells"`
	LiquidityQuote    *float64 `json:"liquidity_quote,omitempty"`
	MarketCapQuote    *float64 `json:"market_cap_quote,omitempty"`
}

type TokenSearchRecord struct {
	Mint        string   `json:"mint"`
	Name        *string  `json:"name,omitempty"`
	Symbol      *string  `json:"symbol,omitempty"`
	ImageURI    *string  `json:"image_uri,omitempty"`
	LatestPrice *float64 `json:"latest_price,omitempty"`
}

type TokenMetadataRecord struct {
	Mint              string  `json:"mint"`
	Name              *string `json:"name,omitempty"`
	Symbol            *string `json:"symbol,omitempty"`
	URI               *string `json:"uri,omitempty"`
	ImageURI          *string `json:"image_uri,omitempty"`
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
	InsertSeq      int64           `json:"-"`
}

type ActivityEventCursor struct {
	EventUnixTS int64
	Slot        uint64
	InsertSeq   int64
}

type ActivityEventPage struct {
	Items      []ActivityEventRecord
	HasMore    bool
	NextCursor *ActivityEventCursor
}

type TradeSummaryRecord struct {
	QuoteMint       string
	LatestPrice     *float64
	LatestEventUnix *int64
	Price1mAgo      *float64
	Price5mAgo      *float64
	Price1hAgo      *float64
	Price4hAgo      *float64
	Price24hAgo     *float64
	Txns1m          int64
	Buys1m          int64
	Sells1m         int64
	Volume1m        float64
	BuyVolume1m     float64
	SellVolume1m    float64
	Txns5m          int64
	Buys5m          int64
	Sells5m         int64
	Volume5m        float64
	BuyVolume5m     float64
	SellVolume5m    float64
	Txns1h          int64
	Buys1h          int64
	Sells1h         int64
	Volume1h        float64
	BuyVolume1h     float64
	SellVolume1h    float64
	Txns4h          int64
	Buys4h          int64
	Sells4h         int64
	Volume4h        float64
	BuyVolume4h     float64
	SellVolume4h    float64
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

type TradeMetricPoint struct {
	EventUnixTS int64
	Slot        int64
	InsertSeq   int64
	Side        string
	Price       float64
	Volume      float64
}

type LongWindowMetricsRecord struct {
	Mint           string
	AnchorPrice1h  *float64
	AnchorPrice4h  *float64
	AnchorPrice24h *float64
	Txns1h         int64
	Txns4h         int64
	Txns24h        int64
	Buys1h         int64
	Buys4h         int64
	Buys24h        int64
	Sells1h        int64
	Sells4h        int64
	Sells24h       int64
	Volume1h       float64
	Volume4h       float64
	Volume24h      float64
	BuyVolume1h    float64
	BuyVolume4h    float64
	BuyVolume24h   float64
	SellVolume1h   float64
	SellVolume4h   float64
	SellVolume24h  float64
}

type LatestTradeSeedRecord struct {
	Mint            string
	LatestPrice     *float64
	LatestEventUnix *int64
}

type TokenMetricsCurrentRecord struct {
	Mint            string
	LatestPrice     *float64
	LatestEventUnix *int64
	AnchorPrice1m   *float64
	AnchorPrice5m   *float64
	AnchorPrice1h   *float64
	AnchorPrice4h   *float64
	AnchorPrice24h  *float64
	Txns1m          int64
	Txns5m          int64
	Txns1h          int64
	Txns4h          int64
	Txns24h         int64
	Buys1m          int64
	Buys5m          int64
	Buys1h          int64
	Buys4h          int64
	Buys24h         int64
	Sells1m         int64
	Sells5m         int64
	Sells1h         int64
	Sells4h         int64
	Sells24h        int64
	Volume1m        float64
	Volume5m        float64
	Volume1h        float64
	Volume4h        float64
	Volume24h       float64
	BuyVolume1m     float64
	BuyVolume5m     float64
	BuyVolume1h     float64
	BuyVolume4h     float64
	BuyVolume24h    float64
	SellVolume1m    float64
	SellVolume5m    float64
	SellVolume1h    float64
	SellVolume4h    float64
	SellVolume24h   float64
	SourceLogID     int64
}

type TokenCandleRecord struct {
	TimeUnix  int64   `json:"time"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	IsGapFill bool    `json:"is_gapfill"`
}

func NewReadModelStore(database *db.DB) *ReadModelStore {
	return &ReadModelStore{db: database}
}

func candleAggregateView(resolution string) (string, error) {
	viewName, ok := candleAggregateViews[resolution]
	if !ok {
		return "", fmt.Errorf("unsupported candle resolution %q", resolution)
	}
	return viewName, nil
}

func unixToTimestampSeconds(ts int64) float64 {
	abs := ts
	if abs < 0 {
		abs = -abs
	}

	switch {
	case abs >= 1_000_000_000_000_000_000:
		return float64(ts) / 1_000_000_000
	case abs >= 1_000_000_000_000_000:
		return float64(ts) / 1_000_000
	case abs >= 1_000_000_000_000:
		return float64(ts) / 1_000
	default:
		return float64(ts)
	}
}

func (s *ReadModelStore) UpsertToken(ctx context.Context, record TokenRecord) error {
	firstSeenAt := unixToTimestampSeconds(record.FirstSeenAt)
	var migratedAt *float64
	if record.MigratedAt != nil {
		value := unixToTimestampSeconds(*record.MigratedAt)
		migratedAt = &value
	}

	exec := executorFromContext(ctx, s.db.Pool)
	_, err := exec.Exec(
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
	exec := executorFromContext(ctx, s.db.Pool)
	_, err := exec.Exec(
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
	startedAt := unixToTimestampSeconds(market.StartedAt)
	var endedAt *float64
	if market.EndedAt != nil {
		value := unixToTimestampSeconds(*market.EndedAt)
		endedAt = &value
	}

	exec := executorFromContext(ctx, s.db.Pool)
	_, err := exec.Exec(
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
	exec := executorFromContext(ctx, s.db.Pool)
	_, err := exec.Exec(ctx, closeTokenMarketSQL, marketID, unixToTimestampSeconds(endedAt))
	if err != nil {
		return fmt.Errorf("close token market: %w", err)
	}

	return nil
}

func (s *ReadModelStore) InsertTradeEvent(ctx context.Context, trade TradeEventRecord) error {
	exec := executorFromContext(ctx, s.db.Pool)
	_, err := exec.Exec(
		ctx,
		insertTokenTradeEventSQL,
		unixToTimestampSeconds(trade.EventUnixTS),
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

	exec := executorFromContext(ctx, s.db.Pool)
	_, err := exec.Exec(
		ctx,
		insertTokenActivityEventSQL,
		unixToTimestampSeconds(activity.EventUnixTS),
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

func (s *ReadModelStore) SearchTokenSnapshots(ctx context.Context, query string, limit int) ([]TokenSearchRecord, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.Pool.Query(ctx, searchTokenSnapshotsSQL, pattern, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search token snapshots: %w", err)
	}
	defer rows.Close()

	items := make([]TokenSearchRecord, 0, limit)
	for rows.Next() {
		var record TokenSearchRecord
		if err := rows.Scan(
			&record.Mint,
			&record.Name,
			&record.Symbol,
			&record.ImageURI,
			&record.LatestPrice,
		); err != nil {
			return nil, fmt.Errorf("scan token search row: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate token search rows: %w", err)
	}

	return items, nil
}

func (s *ReadModelStore) ListTokenBoardRows(ctx context.Context, query TokenBoardQuery) ([]TokenBoardRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTokenBoardRowsSQL, query.Limit, query.Interval, query.View)
	if err != nil {
		return nil, fmt.Errorf("list token board rows: %w", err)
	}
	defer rows.Close()

	items := make([]TokenBoardRecord, 0, query.Limit)
	for rows.Next() {
		record, err := scanTokenBoard(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate token board rows: %w", err)
	}

	return items, nil
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
	page, err := s.ListActivityEventsPageByMint(ctx, mint, limit, nil)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (s *ReadModelStore) ListActivityEventsPageByMint(ctx context.Context, mint string, limit int, cursor *ActivityEventCursor) (*ActivityEventPage, error) {
	fetchLimit := limit + 1
	var beforeTime *int64
	var beforeSlot *int64
	var beforeSeq *int64
	if cursor != nil {
		beforeTime = &cursor.EventUnixTS
		slot := int64(cursor.Slot)
		beforeSlot = &slot
		beforeSeq = &cursor.InsertSeq
	}

	rows, err := s.db.Pool.Query(ctx, listTokenActivityEventsPageByMintSQL, mint, fetchLimit, beforeTime, beforeSlot, beforeSeq)
	if err != nil {
		return nil, fmt.Errorf("list activity event page by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	items := make([]ActivityEventRecord, 0, fetchLimit)
	for rows.Next() {
		record, err := scanActivityEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity event page by mint=%s: %w", mint, err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	var nextCursor *ActivityEventCursor
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		nextCursor = &ActivityEventCursor{
			EventUnixTS: last.EventUnixTS,
			Slot:        last.Slot,
			InsertSeq:   last.InsertSeq,
		}
	}

	return &ActivityEventPage{
		Items:      items,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func (s *ReadModelStore) LoadTradeSummaryByMint(ctx context.Context, mint string) (*TradeSummaryRecord, error) {
	row := s.db.Pool.QueryRow(ctx, loadTokenTradeSummaryCurrentSQL, mint)

	summary, err := scanTradeSummaryRow(row)
	if err == nil {
		return summary, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("load trade summary current by mint=%s: %w", mint, err)
	}

	row = s.db.Pool.QueryRow(ctx, loadTokenTradeSummaryFallbackSQL, mint)
	summary, err = scanTradeSummaryRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load trade summary fallback by mint=%s: %w", mint, err)
	}

	return summary, nil
}

func (s *ReadModelStore) ListTradeMetricsForStatsByMint(ctx context.Context, mint string) ([]TradeMetricPoint, error) {
	rows, err := s.db.Pool.Query(ctx, listTradeMetricsForStatsByMintSQL, mint)
	if err != nil {
		return nil, fmt.Errorf("list trade metrics for stats by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	metrics := make([]TradeMetricPoint, 0, 256)
	for rows.Next() {
		var point TradeMetricPoint
		if err := rows.Scan(
			&point.EventUnixTS,
			&point.Slot,
			&point.InsertSeq,
			&point.Side,
			&point.Price,
			&point.Volume,
		); err != nil {
			return nil, fmt.Errorf("scan trade metrics row: %w", err)
		}
		metrics = append(metrics, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trade metrics rows: %w", err)
	}

	return metrics, nil
}

func (s *ReadModelStore) ListRecentTradeMetricsByMint(ctx context.Context, mint string, sinceUnix int64) ([]TradeMetricPoint, error) {
	rows, err := s.db.Pool.Query(ctx, listRecentTradeMetricsByMintSQL, mint, sinceUnix)
	if err != nil {
		return nil, fmt.Errorf("list recent trade metrics by mint=%s since=%d: %w", mint, sinceUnix, err)
	}
	defer rows.Close()

	metrics := make([]TradeMetricPoint, 0, 128)
	for rows.Next() {
		var point TradeMetricPoint
		if err := rows.Scan(
			&point.EventUnixTS,
			&point.Slot,
			&point.InsertSeq,
			&point.Side,
			&point.Price,
			&point.Volume,
		); err != nil {
			return nil, fmt.Errorf("scan recent trade metrics row: %w", err)
		}
		metrics = append(metrics, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent trade metrics rows: %w", err)
	}

	return metrics, nil
}

func (s *ReadModelStore) ListLongWindowMetricsByMints(ctx context.Context, mints []string, nowTs int64) (map[string]LongWindowMetricsRecord, error) {
	if len(mints) == 0 {
		return map[string]LongWindowMetricsRecord{}, nil
	}

	rows, err := s.db.Pool.Query(ctx, listLongWindowMetricsByMintsSQL, mints, nowTs)
	if err != nil {
		return nil, fmt.Errorf("list long window metrics by mints: %w", err)
	}
	defer rows.Close()

	items := make(map[string]LongWindowMetricsRecord, len(mints))
	for rows.Next() {
		var record LongWindowMetricsRecord
		if err := rows.Scan(
			&record.Mint,
			&record.AnchorPrice1h,
			&record.AnchorPrice4h,
			&record.AnchorPrice24h,
			&record.Txns1h,
			&record.Txns4h,
			&record.Txns24h,
			&record.Buys1h,
			&record.Buys4h,
			&record.Buys24h,
			&record.Sells1h,
			&record.Sells4h,
			&record.Sells24h,
			&record.Volume1h,
			&record.Volume4h,
			&record.Volume24h,
			&record.BuyVolume1h,
			&record.BuyVolume4h,
			&record.BuyVolume24h,
			&record.SellVolume1h,
			&record.SellVolume4h,
			&record.SellVolume24h,
		); err != nil {
			return nil, fmt.Errorf("scan long window metrics row: %w", err)
		}
		items[record.Mint] = record
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate long window metrics rows: %w", err)
	}

	return items, nil
}

func (s *ReadModelStore) UpsertTokenMetricsCurrent(ctx context.Context, records []TokenMetricsCurrentRecord) error {
	if len(records) == 0 {
		return nil
	}

	exec := executorFromContext(ctx, s.db.Pool)
	batch := &pgx.Batch{}
	for _, record := range records {
		var latestEventUnix int64
		if record.LatestEventUnix != nil {
			latestEventUnix = *record.LatestEventUnix
		}
		batch.Queue(
			upsertTokenMetricsCurrentSQL,
			record.Mint,
			record.LatestPrice,
			latestEventUnix,
			record.AnchorPrice1m,
			record.AnchorPrice5m,
			record.AnchorPrice1h,
			record.AnchorPrice4h,
			record.AnchorPrice24h,
			record.Txns1m,
			record.Txns5m,
			record.Txns1h,
			record.Txns4h,
			record.Txns24h,
			record.Buys1m,
			record.Buys5m,
			record.Buys1h,
			record.Buys4h,
			record.Buys24h,
			record.Sells1m,
			record.Sells5m,
			record.Sells1h,
			record.Sells4h,
			record.Sells24h,
			record.Volume1m,
			record.Volume5m,
			record.Volume1h,
			record.Volume4h,
			record.Volume24h,
			record.BuyVolume1m,
			record.BuyVolume5m,
			record.BuyVolume1h,
			record.BuyVolume4h,
			record.BuyVolume24h,
			record.SellVolume1m,
			record.SellVolume5m,
			record.SellVolume1h,
			record.SellVolume4h,
			record.SellVolume24h,
			record.SourceLogID,
		)
	}

	results := exec.SendBatch(ctx, batch)
	defer results.Close()
	for _, record := range records {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("upsert token metrics current mint=%s: %w", record.Mint, err)
		}
	}
	return nil
}

func (s *ReadModelStore) LoadTokenMetricsCurrentByMint(ctx context.Context, mint string) (*TokenMetricsCurrentRecord, error) {
	row := s.db.Pool.QueryRow(ctx, loadTokenMetricsCurrentByMintSQL, mint)

	record, err := scanTokenMetricsCurrentRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load token metrics current by mint=%s: %w", mint, err)
	}

	return record, nil
}

func (s *ReadModelStore) InsertTradeEventsBatch(ctx context.Context, trades []TradeEventRecord) error {
	if len(trades) == 0 {
		return nil
	}

	exec := executorFromContext(ctx, s.db.Pool)
	batch := &pgx.Batch{}
	for _, trade := range trades {
		batch.Queue(
			insertTokenTradeEventSQL,
			unixToTimestampSeconds(trade.EventUnixTS),
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
	}

	results := exec.SendBatch(ctx, batch)
	defer results.Close()
	for _, trade := range trades {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert trade event mint=%s event_id=%s: %w", trade.Mint, trade.EventID, err)
		}
	}
	return nil
}

func (s *ReadModelStore) InsertActivityEventsBatch(ctx context.Context, activities []ActivityEventRecord) error {
	if len(activities) == 0 {
		return nil
	}

	exec := executorFromContext(ctx, s.db.Pool)
	batch := &pgx.Batch{}
	for _, activity := range activities {
		details := activity.Details
		if len(details) == 0 {
			details = json.RawMessage(`{}`)
		}
		batch.Queue(
			insertTokenActivityEventSQL,
			unixToTimestampSeconds(activity.EventUnixTS),
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
	}

	results := exec.SendBatch(ctx, batch)
	defer results.Close()
	for _, activity := range activities {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert activity event mint=%s event_id=%s: %w", activity.Mint, activity.EventID, err)
		}
	}
	return nil
}

func (s *ReadModelStore) ListMetricBackfillCandidates(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.Pool.Query(ctx, listMetricBackfillCandidatesSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("list metric backfill candidates: %w", err)
	}
	defer rows.Close()

	mints := make([]string, 0, limit)
	for rows.Next() {
		var mint string
		if err := rows.Scan(&mint); err != nil {
			return nil, fmt.Errorf("scan metric backfill candidate: %w", err)
		}
		mints = append(mints, mint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metric backfill candidates: %w", err)
	}
	return mints, nil
}

func (s *ReadModelStore) ListLatestTradeSeedsByMints(ctx context.Context, mints []string) (map[string]LatestTradeSeedRecord, error) {
	if len(mints) == 0 {
		return map[string]LatestTradeSeedRecord{}, nil
	}

	rows, err := s.db.Pool.Query(ctx, listLatestTradeSeedsByMintsSQL, mints)
	if err != nil {
		return nil, fmt.Errorf("list latest trade seeds by mints: %w", err)
	}
	defer rows.Close()

	items := make(map[string]LatestTradeSeedRecord, len(mints))
	for rows.Next() {
		var record LatestTradeSeedRecord
		if err := rows.Scan(&record.Mint, &record.LatestPrice, &record.LatestEventUnix); err != nil {
			return nil, fmt.Errorf("scan latest trade seed row: %w", err)
		}
		items[record.Mint] = record
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest trade seed rows: %w", err)
	}
	return items, nil
}

func (s *ReadModelStore) ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int, beforeTime *int64) ([]TokenCandleRecord, error) {
	viewName, err := candleAggregateView(resolution)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(listTokenCandlesByMintSQLTemplate, viewName)

	var beforeParam any
	if beforeTime != nil {
		beforeParam = *beforeTime
	}

	rows, err := s.db.Pool.Query(ctx, query, mint, beforeParam, limit)
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
			&candle.IsGapFill,
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

func scanTradeSummaryRow(scanner tokenSnapshotScanner) (*TradeSummaryRecord, error) {
	var summary TradeSummaryRecord
	if err := scanner.Scan(
		&summary.QuoteMint,
		&summary.LatestPrice,
		&summary.LatestEventUnix,
		&summary.Price5mAgo,
		&summary.Price1hAgo,
		&summary.Price4hAgo,
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
	); err != nil {
		return nil, err
	}

	return &summary, nil
}

func scanTokenMetricsCurrentRow(scanner tokenSnapshotScanner) (*TokenMetricsCurrentRecord, error) {
	var record TokenMetricsCurrentRecord
	if err := scanner.Scan(
		&record.Mint,
		&record.LatestPrice,
		&record.LatestEventUnix,
		&record.AnchorPrice1m,
		&record.AnchorPrice5m,
		&record.AnchorPrice1h,
		&record.AnchorPrice4h,
		&record.AnchorPrice24h,
		&record.Txns1m,
		&record.Txns5m,
		&record.Txns1h,
		&record.Txns4h,
		&record.Txns24h,
		&record.Buys1m,
		&record.Buys5m,
		&record.Buys1h,
		&record.Buys4h,
		&record.Buys24h,
		&record.Sells1m,
		&record.Sells5m,
		&record.Sells1h,
		&record.Sells4h,
		&record.Sells24h,
		&record.Volume1m,
		&record.Volume5m,
		&record.Volume1h,
		&record.Volume4h,
		&record.Volume24h,
		&record.BuyVolume1m,
		&record.BuyVolume5m,
		&record.BuyVolume1h,
		&record.BuyVolume4h,
		&record.BuyVolume24h,
		&record.SellVolume1m,
		&record.SellVolume5m,
		&record.SellVolume1h,
		&record.SellVolume4h,
		&record.SellVolume24h,
		&record.SourceLogID,
	); err != nil {
		return nil, err
	}

	return &record, nil
}

func scanTokenSnapshot(scanner tokenSnapshotScanner) (TokenSnapshotRecord, error) {
	var record TokenSnapshotRecord
	if err := scanner.Scan(
		&record.Mint,
		&record.Name,
		&record.Symbol,
		&record.URI,
		&record.ImageURI,
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

func scanTokenBoard(scanner tokenSnapshotScanner) (TokenBoardRecord, error) {
	var record TokenBoardRecord
	if err := scanner.Scan(
		&record.Mint,
		&record.Name,
		&record.Symbol,
		&record.URI,
		&record.ImageURI,
		&record.Creator,
		&record.BondingCurve,
		&record.TokenProgram,
		&record.Decimals,
		&record.QuoteMint,
		&record.QuoteDecimals,
		&record.FirstSeenAt,
		&record.ActiveSince,
		&record.CurrentStage,
		&record.ActiveMarketID,
		&record.ActiveMarketType,
		&record.MigratedAt,
		&record.LatestPrice,
		&record.LatestEventUnixTS,
		&record.PriceChange,
		&record.WindowVolume,
		&record.WindowTxns,
		&record.WindowBuys,
		&record.WindowSells,
		&record.LiquidityQuote,
		&record.MarketCapQuote,
	); err != nil {
		return TokenBoardRecord{}, fmt.Errorf("scan token board row: %w", err)
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
		&record.InsertSeq,
	); err != nil {
		return ActivityEventRecord{}, fmt.Errorf("scan activity event: %w", err)
	}
	record.Slot = uint64(slot)

	return record, nil
}
