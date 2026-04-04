alter table service_events
    add column if not exists log_id bigserial;

create unique index if not exists service_events_log_id_uidx
    on service_events (log_id);

create table if not exists projection_checkpoints (
    projector_name text primary key,
    last_log_id bigint not null,
    updated_at timestamptz not null default now()
);
