create extension if not exists timescaledb cascade;

create table if not exists trade_ticks (
    event_time timestamptz not null,
    event_id text not null,
    mint text not null,
    market_id text not null,
    market_type text not null,
    protocol text not null,
    side text not null,
    user_address text not null,
    quote_mint text not null,
    price numeric not null,
    token_amount numeric not null,
    quote_amount numeric not null,
    tx_signature text not null,
    slot bigint not null,
    created_at timestamptz not null default now(),
    primary key (event_time, event_id)
);

select create_hypertable('trade_ticks', 'event_time', if_not_exists => true, migrate_data => true);

create index if not exists trade_ticks_mint_event_time_idx
    on trade_ticks (mint, event_time desc);

create index if not exists trade_ticks_mint_side_event_time_idx
    on trade_ticks (mint, side, event_time desc);

create index if not exists trade_ticks_market_event_time_idx
    on trade_ticks (market_id, event_time desc);
