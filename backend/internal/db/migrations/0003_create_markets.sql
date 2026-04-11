create table if not exists markets (
    market_id text primary key,
    mint text not null,
    protocol text not null,
    market_type text not null,
    bonding_curve text null,
    pool text null,
    base_mint text null,
    quote_mint text null,
    lp_mint text null,
    started_at timestamptz not null,
    ended_at timestamptz null,
    create_event_id text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists markets_mint_started_at_idx
    on markets (mint, started_at desc);

create unique index if not exists markets_bonding_curve_uidx
    on markets (bonding_curve)
    where bonding_curve is not null;

create unique index if not exists markets_pool_uidx
    on markets (pool)
    where pool is not null;
