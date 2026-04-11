create sequence if not exists token_trade_events_insert_seq_seq;

alter table token_trade_events
add column if not exists insert_seq bigint;

alter table token_trade_events
alter column insert_seq
set default nextval(
    'token_trade_events_insert_seq_seq'
);

update token_trade_events
set
    insert_seq = nextval(
        'token_trade_events_insert_seq_seq'
    )
where
    insert_seq is null;

alter table token_trade_events
alter column insert_seq
set not null;

alter sequence token_trade_events_insert_seq_seq owned by token_trade_events.insert_seq;

create index if not exists token_trade_events_mint_event_time_slot_seq_idx on token_trade_events (
    mint,
    event_time desc,
    slot desc,
    insert_seq desc
);