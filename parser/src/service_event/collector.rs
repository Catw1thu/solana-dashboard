use super::{
    model::ServiceEventEnvelope,
    pumpamm::{
        build_pumpamm_create_pool_service_event, build_pumpamm_liquidity_service_event,
        build_pumpamm_swap_service_event,
    },
    pumpfun::{
        build_pumpfun_create_service_event, build_pumpfun_migrate_service_event,
        build_pumpfun_trade_service_event,
    },
};
use crate::{
    transaction_view::TransactionView,
    unified_parser::{ParsedEvent, parse_view},
};

pub fn collect_service_events(view: &TransactionView) -> Vec<ServiceEventEnvelope> {
    parse_view(view)
        .into_iter()
        .map(|event| match event {
            ParsedEvent::PumpfunTrade(trade) => build_pumpfun_trade_service_event(view, &trade),
            ParsedEvent::PumpfunCreate(create) => build_pumpfun_create_service_event(view, &create),
            ParsedEvent::PumpfunMigrate(migrate) => {
                build_pumpfun_migrate_service_event(view, &migrate)
            }
            ParsedEvent::PumpAmmSwap(swap) => build_pumpamm_swap_service_event(view, &swap),
            ParsedEvent::PumpAmmCreatePool(create_pool) => {
                build_pumpamm_create_pool_service_event(view, &create_pool)
            }
            ParsedEvent::PumpAmmLiquidity(liquidity) => {
                build_pumpamm_liquidity_service_event(view, &liquidity)
            }
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::collect_service_events;
    use crate::transaction_view::TransactionView;

    fn load_fixture(file_name: &str) -> TransactionView {
        let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("samples")
            .join("tests")
            .join("views")
            .join(file_name);
        let content = std::fs::read_to_string(path).unwrap();
        serde_json::from_str(&content).unwrap()
    }

    #[test]
    fn collects_all_service_events_for_migration_fixture() {
        let view = load_fixture(
            "409576108-3zCwTozsNVfMaSftorXKLbbdVAmNPaPy3oZXN5ch6eMBdYdKfoB9GAgsiwhAFq786wnYoP9Lv64XjC8LbaKnbijZ.json",
        );

        let service_events = collect_service_events(&view);
        assert_eq!(service_events.len(), 2);
        let first = serde_json::to_string(&service_events[0]).unwrap();
        let second = serde_json::to_string(&service_events[1]).unwrap();
        assert!(first.contains("\"event_type\":\"migrate\""));
        assert!(second.contains("\"event_type\":\"create_pool\""));
        assert!(service_events.iter().any(|event| {
            serde_json::to_string(event)
                .unwrap()
                .contains("\"event_type\":\"migrate\"")
        }));
        assert!(service_events.iter().any(|event| {
            serde_json::to_string(event)
                .unwrap()
                .contains("\"event_type\":\"create_pool\"")
        }));
    }

    #[test]
    fn collects_all_service_events_for_withdraw_fixture() {
        let view = load_fixture(
            "409590250-yoENqqk48Fq9LJa8wS8RJdtTgYLsSuFXaFJDbbAsD7G1mnGgYgRJDEeyMWFmKwsJzN9GjoFfd5PTceDKawW65pw.json",
        );

        let service_events = collect_service_events(&view);
        assert_eq!(service_events.len(), 1);
        let json = serde_json::to_string(&service_events[0]).unwrap();
        assert!(json.contains("\"event_type\":\"withdraw\""));
        assert!(json.contains("\"protocol\":\"pumpamm\""));
        assert!(json.contains("\"pool\""));
    }
}
