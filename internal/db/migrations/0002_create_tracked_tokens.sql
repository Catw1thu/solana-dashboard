create table if not exists tracked_tokens (
    mint text primary key,
    creator text null,
    bonding_curve text null,
    token_program text null,
    create_event_id text not null,
    accepted_at timestamptz not null,
    current_stage text not null,
    current_market_type text not null,
    current_market_id text null,
    migrated_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists tracked_tokens_current_stage_idx
    on tracked_tokens (current_stage);

create index if not exists tracked_tokens_current_market_type_idx
    on tracked_tokens (current_market_type);
