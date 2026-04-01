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
    pumpamm::{extract_liquidity_actions, extract_pool_creations, extract_swaps},
    pumpfun::{extract_creates, extract_migrations, extract_trades},
    transaction_view::TransactionView,
};

pub fn collect_service_events(view: &TransactionView) -> Vec<ServiceEventEnvelope> {
    let mut service_events = Vec::new();

    for trade in extract_trades(view) {
        service_events.push(build_pumpfun_trade_service_event(view, &trade));
    }

    for create in extract_creates(view) {
        service_events.push(build_pumpfun_create_service_event(view, &create));
    }

    for migration in extract_migrations(view) {
        service_events.push(build_pumpfun_migrate_service_event(view, &migration));
    }

    for swap in extract_swaps(view) {
        service_events.push(build_pumpamm_swap_service_event(view, &swap));
    }

    for creation in extract_pool_creations(view) {
        service_events.push(build_pumpamm_create_pool_service_event(view, &creation));
    }

    for action in extract_liquidity_actions(view) {
        service_events.push(build_pumpamm_liquidity_service_event(view, &action));
    }

    service_events
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
