create extension if not exists timescaledb cascade;

drop table if exists token_timeline_events cascade;
drop table if exists trade_ticks cascade;
drop table if exists trades cascade;
drop table if exists markets cascade;
drop table if exists tracked_tokens cascade;
drop table if exists projection_checkpoints cascade;

create table if not exists projection_offsets (
    projector_name text primary key,
    last_log_id bigint not null,
    updated_at timestamptz not null default now()
);

create table if not exists tokens (
    mint text primary key,
    creator text,
    bonding_curve text,
    token_program text,
    create_event_id text,
    first_seen_at timestamptz not null,
    current_stage text not null,
    active_market_id text,
    active_market_type text,
    migrated_at timestamptz,
    updated_at timestamptz not null default now()
);

create index if not exists tokens_first_seen_at_idx
    on tokens (first_seen_at desc);

create table if not exists token_metadata_current (
    mint text primary key,
    name text,
    symbol text,
    uri text,
    decimals integer,
    total_supply_raw numeric,
    quote_mint text,
    quote_decimals integer,
    creator text,
    bonding_curve text,
    token_program text,
    is_mayhem_mode boolean,
    is_cashback_enabled boolean,
    source_event_id text,
    updated_at timestamptz not null default now()
);

create index if not exists token_metadata_current_symbol_idx
    on token_metadata_current (symbol);

create table if not exists token_markets (
    market_id text primary key,
    mint text not null,
    protocol text not null,
    market_type text not null,
    bonding_curve text,
    pool text,
    base_mint text,
    quote_mint text,
    base_mint_decimals integer,
    quote_mint_decimals integer,
    lp_mint text,
    started_at timestamptz not null,
    ended_at timestamptz,
    create_event_id text not null,
    updated_at timestamptz not null default now()
);

create index if not exists token_markets_mint_started_at_idx
    on token_markets (mint, started_at desc);

create index if not exists token_markets_active_mint_idx
    on token_markets (mint, ended_at, started_at desc);

create table if not exists token_trade_events (
    event_time timestamptz not null,
    event_id text not null,
    mint text not null,
    market_id text not null,
    market_type text not null,
    protocol text not null,
    side text not null,
    ix_name text not null,
    user_address text not null,
    quote_mint text not null,
    token_amount_raw numeric not null,
    quote_amount_raw numeric not null,
    tx_signature text not null,
    slot bigint not null,
    raw_event_source text not null,
    primary key (event_time, event_id)
);

select create_hypertable(
    'token_trade_events',
    'event_time',
    if_not_exists => true,
    migrate_data => true
);

create index if not exists token_trade_events_mint_event_time_idx
    on token_trade_events (mint, event_time desc);

create index if not exists token_trade_events_mint_side_event_time_idx
    on token_trade_events (mint, side, event_time desc);

create index if not exists token_trade_events_market_event_time_idx
    on token_trade_events (market_id, event_time desc);

create table if not exists token_activity_events (
    event_time timestamptz not null,
    event_id text not null,
    mint text not null,
    protocol text not null,
    event_type text not null,
    activity_type text not null,
    market_id text,
    market_type text,
    user_address text,
    side text,
    quote_mint text,
    token_amount_raw numeric,
    quote_amount_raw numeric,
    tx_signature text not null,
    slot bigint not null,
    raw_event_source text not null,
    details jsonb not null default '{}'::jsonb,
    primary key (event_time, event_id)
);

select create_hypertable(
    'token_activity_events',
    'event_time',
    if_not_exists => true,
    migrate_data => true
);

create index if not exists token_activity_events_mint_event_time_idx
    on token_activity_events (mint, event_time desc);

create index if not exists token_activity_events_mint_type_event_time_idx
    on token_activity_events (mint, activity_type, event_time desc);

create or replace function resolved_quote_decimals(
    trade_quote_mint text,
    metadata_quote_mint text,
    metadata_quote_decimals integer
) returns integer
language sql
immutable
as $$
    select case
        when metadata_quote_mint is not null and trade_quote_mint = metadata_quote_mint
            then metadata_quote_decimals
        when trade_quote_mint = 'So11111111111111111111111111111111111111112'
            then 9
        else metadata_quote_decimals
    end
$$;

create or replace function scale_amount_numeric(
    raw_amount numeric,
    decimals integer
) returns numeric
language sql
immutable
as $$
    select case
        when raw_amount is null then null
        when decimals is null then raw_amount
        else raw_amount / power(10::numeric, decimals)
    end
$$;

create or replace function trade_price_quote(
    quote_amount_raw numeric,
    token_amount_raw numeric,
    token_decimals integer,
    quote_decimals integer
) returns double precision
language sql
immutable
as $$
    select case
        when token_amount_raw is null or token_amount_raw = 0 then null
        when token_decimals is not null and quote_decimals is not null then (
            (
                scale_amount_numeric(quote_amount_raw, quote_decimals)
                / nullif(scale_amount_numeric(token_amount_raw, token_decimals), 0)
            )::double precision
        )
        else (quote_amount_raw / nullif(token_amount_raw, 0))::double precision
    end
$$;
