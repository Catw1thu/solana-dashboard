create table if not exists trades (
    event_id text primary key,
    mint text not null,
    market_id text not null,
    market_type text not null,
    protocol text not null,
    side text not null,
    ix_name text not null,
    user_address text not null,
    bonding_curve text null,
    pool text null,
    quote_mint text not null,
    token_amount numeric not null,
    quote_amount numeric not null,
    tx_signature text not null,
    slot bigint not null,
    event_time timestamptz not null,
    raw_event_source text not null,
    created_at timestamptz not null default now()
);

create index if not exists trades_mint_event_time_idx
    on trades (mint, event_time desc);

create index if not exists trades_market_event_time_idx
    on trades (market_id, event_time desc);

create index if not exists trades_tx_signature_idx
    on trades (tx_signature);
