create index if not exists service_events_refs_mint_slot_idx
    on service_events ((refs->>'mint'), slot desc, tx_index desc, outer_index desc, inner_index desc)
    where refs->>'mint' is not null;

create index if not exists service_events_refs_pool_slot_idx
    on service_events ((refs->>'pool'), slot desc, tx_index desc, outer_index desc, inner_index desc)
    where refs->>'pool' is not null;
