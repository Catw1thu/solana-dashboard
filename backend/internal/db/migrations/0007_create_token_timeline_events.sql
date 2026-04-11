create table if not exists token_timeline_events (
    event_id text primary key,
    mint text not null,
    protocol text not null,
    event_type text not null,
    timeline_type text not null,
    market_id text null,
    market_type text null,
    user_address text null,
    side text null,
    quote_mint text null,
    token_amount numeric null,
    quote_amount numeric null,
    tx_signature text not null,
    slot bigint not null,
    event_time timestamptz not null,
    raw_event_source text not null,
    details jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create index if not exists token_timeline_events_mint_event_time_idx
    on token_timeline_events (mint, event_time desc);

create index if not exists token_timeline_events_market_event_time_idx
    on token_timeline_events (market_id, event_time desc)
    where market_id is not null;
