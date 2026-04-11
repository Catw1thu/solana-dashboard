create or replace function raw_trade_price(
    quote_amount_raw numeric,
    token_amount_raw numeric
) returns double precision
language sql
immutable
as $$
    select case
        when token_amount_raw is null or token_amount_raw = 0 then null
        else (quote_amount_raw / token_amount_raw)::double precision
    end
$$;

create or replace function normalize_raw_trade_price(
    raw_price double precision,
    token_decimals integer,
    quote_decimals integer
) returns double precision
language sql
immutable
as $$
    select case
        when raw_price is null then null
        when token_decimals is null or quote_decimals is null then raw_price
        else (
            raw_price
            * power(10::numeric, token_decimals - quote_decimals)
        )::double precision
    end
$$;

drop materialized view if exists token_candles_1d cascade;
drop materialized view if exists token_candles_4h cascade;
drop materialized view if exists token_candles_1h cascade;
drop materialized view if exists token_candles_15m cascade;
drop materialized view if exists token_candles_5m cascade;
drop materialized view if exists token_candles_1m cascade;

create materialized view token_candles_1m
with (
    timescaledb.continuous,
    timescaledb.create_group_indexes = false
) as
select
    time_bucket(interval '1 minute', event_time) as bucket,
    mint,
    first(raw_trade_price(quote_amount_raw, token_amount_raw), insert_seq) as open_raw_price,
    max(raw_trade_price(quote_amount_raw, token_amount_raw)) as high_raw_price,
    min(raw_trade_price(quote_amount_raw, token_amount_raw)) as low_raw_price,
    last(raw_trade_price(quote_amount_raw, token_amount_raw), insert_seq) as close_raw_price,
    min(quote_mint) as quote_mint_sample,
    sum(quote_amount_raw)::numeric as volume_quote_raw
from token_trade_events
where token_amount_raw <> 0
group by 1, 2;

alter materialized view token_candles_1m
set (timescaledb.materialized_only = false);

create index token_candles_1m_mint_bucket_idx
    on token_candles_1m (mint, bucket desc);

select add_continuous_aggregate_policy(
    'token_candles_1m',
    start_offset => interval '365 days',
    end_offset => interval '1 minute',
    schedule_interval => interval '1 minute'
);

create materialized view token_candles_5m
with (
    timescaledb.continuous,
    timescaledb.create_group_indexes = false
) as
select
    time_bucket(interval '5 minutes', bucket) as bucket,
    mint,
    first(open_raw_price, bucket) as open_raw_price,
    max(high_raw_price) as high_raw_price,
    min(low_raw_price) as low_raw_price,
    last(close_raw_price, bucket) as close_raw_price,
    min(quote_mint_sample) as quote_mint_sample,
    sum(volume_quote_raw)::numeric as volume_quote_raw
from token_candles_1m
group by 1, 2;

alter materialized view token_candles_5m
set (timescaledb.materialized_only = false);

create index token_candles_5m_mint_bucket_idx
    on token_candles_5m (mint, bucket desc);

select add_continuous_aggregate_policy(
    'token_candles_5m',
    start_offset => interval '365 days',
    end_offset => interval '5 minutes',
    schedule_interval => interval '5 minutes'
);

create materialized view token_candles_15m
with (
    timescaledb.continuous,
    timescaledb.create_group_indexes = false
) as
select
    time_bucket(interval '15 minutes', bucket) as bucket,
    mint,
    first(open_raw_price, bucket) as open_raw_price,
    max(high_raw_price) as high_raw_price,
    min(low_raw_price) as low_raw_price,
    last(close_raw_price, bucket) as close_raw_price,
    min(quote_mint_sample) as quote_mint_sample,
    sum(volume_quote_raw)::numeric as volume_quote_raw
from token_candles_5m
group by 1, 2;

alter materialized view token_candles_15m
set (timescaledb.materialized_only = false);

create index token_candles_15m_mint_bucket_idx
    on token_candles_15m (mint, bucket desc);

select add_continuous_aggregate_policy(
    'token_candles_15m',
    start_offset => interval '365 days',
    end_offset => interval '15 minutes',
    schedule_interval => interval '15 minutes'
);

create materialized view token_candles_1h
with (
    timescaledb.continuous,
    timescaledb.create_group_indexes = false
) as
select
    time_bucket(interval '1 hour', bucket) as bucket,
    mint,
    first(open_raw_price, bucket) as open_raw_price,
    max(high_raw_price) as high_raw_price,
    min(low_raw_price) as low_raw_price,
    last(close_raw_price, bucket) as close_raw_price,
    min(quote_mint_sample) as quote_mint_sample,
    sum(volume_quote_raw)::numeric as volume_quote_raw
from token_candles_15m
group by 1, 2;

alter materialized view token_candles_1h
set (timescaledb.materialized_only = false);

create index token_candles_1h_mint_bucket_idx
    on token_candles_1h (mint, bucket desc);

select add_continuous_aggregate_policy(
    'token_candles_1h',
    start_offset => interval '365 days',
    end_offset => interval '1 hour',
    schedule_interval => interval '1 hour'
);

create materialized view token_candles_4h
with (
    timescaledb.continuous,
    timescaledb.create_group_indexes = false
) as
select
    time_bucket(interval '4 hours', bucket) as bucket,
    mint,
    first(open_raw_price, bucket) as open_raw_price,
    max(high_raw_price) as high_raw_price,
    min(low_raw_price) as low_raw_price,
    last(close_raw_price, bucket) as close_raw_price,
    min(quote_mint_sample) as quote_mint_sample,
    sum(volume_quote_raw)::numeric as volume_quote_raw
from token_candles_1h
group by 1, 2;

alter materialized view token_candles_4h
set (timescaledb.materialized_only = false);

create index token_candles_4h_mint_bucket_idx
    on token_candles_4h (mint, bucket desc);

select add_continuous_aggregate_policy(
    'token_candles_4h',
    start_offset => interval '365 days',
    end_offset => interval '4 hours',
    schedule_interval => interval '4 hours'
);

create materialized view token_candles_1d
with (
    timescaledb.continuous,
    timescaledb.create_group_indexes = false
) as
select
    time_bucket(interval '1 day', bucket) as bucket,
    mint,
    first(open_raw_price, bucket) as open_raw_price,
    max(high_raw_price) as high_raw_price,
    min(low_raw_price) as low_raw_price,
    last(close_raw_price, bucket) as close_raw_price,
    min(quote_mint_sample) as quote_mint_sample,
    sum(volume_quote_raw)::numeric as volume_quote_raw
from token_candles_4h
group by 1, 2;

alter materialized view token_candles_1d
set (timescaledb.materialized_only = false);

create index token_candles_1d_mint_bucket_idx
    on token_candles_1d (mint, bucket desc);

select add_continuous_aggregate_policy(
    'token_candles_1d',
    start_offset => interval '365 days',
    end_offset => interval '1 day',
    schedule_interval => interval '1 day'
);
