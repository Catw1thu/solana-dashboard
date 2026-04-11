create materialized view token_trade_metrics_1m
with (
    timescaledb.continuous,
    timescaledb.create_group_indexes = false
) as
select
    time_bucket(interval '1 minute', event_time) as bucket,
    mint,
    first(raw_trade_price(quote_amount_raw, token_amount_raw), insert_seq) as open_raw_price,
    last(raw_trade_price(quote_amount_raw, token_amount_raw), insert_seq) as close_raw_price,
    min(quote_mint) as quote_mint_sample,
    count(*)::bigint as txns,
    count(*) filter (where side = 'buy')::bigint as buys,
    count(*) filter (where side = 'sell')::bigint as sells,
    coalesce(sum(quote_amount_raw), 0)::numeric as volume_quote_raw,
    coalesce(sum(quote_amount_raw) filter (where side = 'buy'), 0)::numeric as buy_volume_quote_raw,
    coalesce(sum(quote_amount_raw) filter (where side = 'sell'), 0)::numeric as sell_volume_quote_raw
from token_trade_events
where token_amount_raw <> 0
group by 1, 2;

alter materialized view token_trade_metrics_1m
set (timescaledb.materialized_only = false);

create index token_trade_metrics_1m_mint_bucket_idx
    on token_trade_metrics_1m (mint, bucket desc);

select add_continuous_aggregate_policy(
    'token_trade_metrics_1m',
    start_offset => interval '365 days',
    end_offset => interval '1 minute',
    schedule_interval => interval '1 minute'
);

create table token_metrics_current (
    mint text primary key references tokens(mint) on delete cascade,
    latest_price double precision,
    latest_trade_at timestamptz,
    anchor_price_1m double precision,
    anchor_price_5m double precision,
    anchor_price_1h double precision,
    anchor_price_4h double precision,
    anchor_price_24h double precision,
    txns_1m bigint not null default 0,
    txns_5m bigint not null default 0,
    txns_1h bigint not null default 0,
    txns_4h bigint not null default 0,
    txns_24h bigint not null default 0,
    buys_1m bigint not null default 0,
    buys_5m bigint not null default 0,
    buys_1h bigint not null default 0,
    buys_4h bigint not null default 0,
    buys_24h bigint not null default 0,
    sells_1m bigint not null default 0,
    sells_5m bigint not null default 0,
    sells_1h bigint not null default 0,
    sells_4h bigint not null default 0,
    sells_24h bigint not null default 0,
    volume_1m double precision not null default 0,
    volume_5m double precision not null default 0,
    volume_1h double precision not null default 0,
    volume_4h double precision not null default 0,
    volume_24h double precision not null default 0,
    buy_volume_1m double precision not null default 0,
    buy_volume_5m double precision not null default 0,
    buy_volume_1h double precision not null default 0,
    buy_volume_4h double precision not null default 0,
    buy_volume_24h double precision not null default 0,
    sell_volume_1m double precision not null default 0,
    sell_volume_5m double precision not null default 0,
    sell_volume_1h double precision not null default 0,
    sell_volume_4h double precision not null default 0,
    sell_volume_24h double precision not null default 0,
    source_log_id bigint not null default 0,
    updated_at timestamptz not null default now()
);

create index token_metrics_current_latest_trade_idx
    on token_metrics_current (latest_trade_at desc nulls last);
