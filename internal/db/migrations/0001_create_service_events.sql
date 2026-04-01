create table if not exists service_events (
    event_id text primary key,
    schema_version integer not null,
    chain text not null,
    protocol text not null,
    event_type text not null,
    commitment text not null,
    slot bigint not null,
    tx_signature text not null,
    tx_index bigint not null,
    instruction_source text not null,
    outer_index integer not null,
    inner_index integer null,
    event_source text not null,
    event_unix_ts bigint not null,
    refs jsonb not null,
    payload jsonb not null,
    created_at timestamptz not null default now()
);
