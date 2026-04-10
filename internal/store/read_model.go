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
with candidates as (
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
        coalesce(window_stats.txns_window, 0)::bigint as txns_window,
        coalesce(window_stats.buys_window, 0)::bigint as buys_window,
        coalesce(window_stats.sells_window, 0)::bigint as sells_window,
        coalesce(window_stats.volume_window, 0)::double precision as volume_window
    from tokens t
    left join token_metadata_current m
        on m.mint = t.mint
    left join token_markets am
        on am.market_id = t.active_market_id
    left join lateral (
        select
            count(*)::bigint as txns_window,
            count(*) filter (where tt.side = 'buy')::bigint as buys_window,
            count(*) filter (where tt.side = 'sell')::bigint as sells_window,
            coalesce(sum(
                scale_amount_numeric(
                    tt.quote_amount_raw,
                    resolved_quote_decimals(tt.quote_mint, m.quote_mint, m.quote_decimals)
                )
            ), 0)::double precision as volume_window
        from token_trade_events tt
        where tt.mint = t.mint
          and tt.event_time > now() - $2::interval
    ) window_stats on true
    order by
        case when $3 = 'hot' then coalesce(window_stats.txns_window, 0) end desc nulls last,
        case when $3 = 'hot' then coalesce(window_stats.volume_window, 0) end desc nulls last,
        case when $3 = 'hot' then greatest(t.first_seen_at, coalesce(t.migrated_at, t.first_seen_at)) end desc nulls last,
        case when $3 = 'new' then greatest(t.first_seen_at, coalesce(t.migrated_at, t.first_seen_at)) end desc nulls last,
        t.first_seen_at desc
    limit $1
),
enriched as (
    select
        c.*,
        latest.latest_price,
        latest.latest_event_unix_ts,
        coalesce(anchor_before.anchor_price, anchor_inside.anchor_price) as anchor_price,
        case
            when c.total_supply_raw is not null and latest.latest_price is not null then (
                scale_amount_numeric(c.total_supply_raw, c.decimals)::double precision
                * latest.latest_price
            )
            else null
        end as market_cap_quote
    from candidates c
    left join lateral (
        select
            extract(epoch from tt.event_time)::bigint as latest_event_unix_ts,
            trade_price_quote(
                tt.quote_amount_raw,
                tt.token_amount_raw,
                c.decimals,
                resolved_quote_decimals(tt.quote_mint, c.quote_mint, c.quote_decimals)
            )::double precision as latest_price
        from token_trade_events tt
        where tt.mint = c.mint
        order by tt.event_time desc, tt.slot desc, tt.insert_seq desc
        limit 1
    ) latest on true
    left join lateral (
        select
            trade_price_quote(
                tt.quote_amount_raw,
                tt.token_amount_raw,
                c.decimals,
                resolved_quote_decimals(tt.quote_mint, c.quote_mint, c.quote_decimals)
            )::double precision as anchor_price
        from token_trade_events tt
        where tt.mint = c.mint
          and tt.event_time <= now() - $2::interval
        order by tt.event_time desc, tt.slot desc, tt.insert_seq desc
        limit 1
    ) anchor_before on true
    left join lateral (
        select
            trade_price_quote(
                tt.quote_amount_raw,
                tt.token_amount_raw,
                c.decimals,
                resolved_quote_decimals(tt.quote_mint, c.quote_mint, c.quote_decimals)
            )::double precision as anchor_price
        from token_trade_events tt
        where tt.mint = c.mint
          and tt.event_time > now() - $2::interval
        order by tt.event_time asc, tt.slot asc, tt.insert_seq asc
        limit 1
    ) anchor_inside on true
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
    e.volume_window,
    e.txns_window,
    e.buys_window,
    e.sells_window,
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
    case when $3 = 'hot' then e.txns_window end desc nulls last,
    case when $3 = 'hot' then e.volume_window end desc nulls last,
    case when $3 = 'new' then e.active_since_unix end desc nulls last,
    e.first_seen_at_unix desc
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
    latest.latest_price
from tokens t
left join token_metadata_current m
    on m.mint = t.mint
left join lateral (
    select
        trade_price_quote(
            tt.quote_amount_raw,
            tt.token_amount_raw,
            m.decimals,
            resolved_quote_decimals(tt.quote_mint, m.quote_mint, m.quote_decimals)
        )::double precision as latest_price
    from token_trade_events tt
    where tt.mint = t.mint
    order by tt.event_time desc, tt.slot desc, tt.insert_seq desc
    limit 1
) latest on true
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
	Price5mAgo      *float64
	Price1hAgo      *float64
	Price4hAgo      *float64
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

type TradeMetricPoint struct {
	EventUnixTS int64
	Slot        int64
	InsertSeq   int64
	Side        string
	Price       float64
	Volume      float64
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
	startedAt := unixToTimestampSeconds(market.StartedAt)
	var endedAt *float64
	if market.EndedAt != nil {
		value := unixToTimestampSeconds(*market.EndedAt)
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
	_, err := s.db.Pool.Exec(ctx, closeTokenMarketSQL, marketID, unixToTimestampSeconds(endedAt))
	if err != nil {
		return fmt.Errorf("close token market: %w", err)
	}

	return nil
}

func (s *ReadModelStore) InsertTradeEvent(ctx context.Context, trade TradeEventRecord) error {
	_, err := s.db.Pool.Exec(
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

	_, err := s.db.Pool.Exec(
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
	row := s.db.Pool.QueryRow(ctx, loadTokenTradeSummarySQL, mint)

	var summary TradeSummaryRecord
	err := row.Scan(
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load trade summary by mint=%s: %w", mint, err)
	}

	return &summary, nil
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
