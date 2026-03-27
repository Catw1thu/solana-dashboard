use super::{
    logs::extract_trade_events_from_logs,
    outer::parse_outer_instruction,
    types::{MergedTrade, OuterInstruction, TradeEvent},
};
use crate::types::RawTxView;

pub fn extract_merged_trades(view: &RawTxView) -> Vec<MergedTrade> {
    let mut pending_events = extract_trade_events_from_logs(&view.log_messages);
    let mut merged_trades = Vec::new();

    for (outer_instruction_index, instruction) in view.outer_instructions.iter().enumerate() {
        let Some(outer) = parse_outer_instruction(instruction) else {
            continue;
        };

        if outer.ix_name().is_none() {
            continue;
        }

        let Some(event_index) = pending_events
            .iter()
            .position(|event| event_matches_outer(&outer, event))
        else {
            continue;
        };

        let event = pending_events.remove(event_index);
        merged_trades.push(build_merged_trade(outer_instruction_index, outer, event));
    }

    merged_trades
}

fn event_matches_outer(outer: &OuterInstruction, event: &TradeEvent) -> bool {
    let Some(expected_ix_name) = outer.ix_name() else {
        return false;
    };
    let Some(accounts) = outer.accounts() else {
        return false;
    };

    let expected_is_buy = matches!(
        outer,
        OuterInstruction::Buy(_) | OuterInstruction::BuyExactSolIn(_)
    );

    event.ix_name == expected_ix_name
        && event.is_buy == expected_is_buy
        && event.mint == accounts.mint
        && event.user == accounts.user
}

fn build_merged_trade(
    outer_instruction_index: usize,
    outer: OuterInstruction,
    event: TradeEvent,
) -> MergedTrade {
    let accounts = outer
        .accounts()
        .expect("trade instructions must always carry trade accounts");
    let side = outer
        .side()
        .expect("trade instructions must always map to a side");

    MergedTrade {
        outer_instruction_index,
        side,
        mint: event.mint.clone(),
        user: event.user.clone(),
        bonding_curve: accounts.bonding_curve.clone(),
        associated_bonding_curve: accounts.associated_bonding_curve.clone(),
        creator_vault: accounts.creator_vault.clone(),
        token_program: accounts.token_program.clone(),
        sol_amount: event.sol_amount,
        token_amount: event.token_amount,
        is_buy: event.is_buy,
        track_volume: event.track_volume,
        timestamp: event.timestamp,
        ix_name: event.ix_name.clone(),
        virtual_sol_reserves: event.virtual_sol_reserves,
        virtual_token_reserves: event.virtual_token_reserves,
        real_sol_reserves: event.real_sol_reserves,
        real_token_reserves: event.real_token_reserves,
        fee_recipient: event.fee_recipient.clone(),
        fee_basis_points: event.fee_basis_points,
        fee: event.fee,
        creator: event.creator.clone(),
        creator_fee_basis_points: event.creator_fee_basis_points,
        creator_fee: event.creator_fee,
        current_sol_volume: event.current_sol_volume,
        cashback_fee_basis_points: event.cashback_fee_basis_points,
        cashback: event.cashback,
        outer,
        event,
    }
}

#[cfg(test)]
mod tests {
    use super::extract_merged_trades;
    use crate::types::RawTxView;

    fn load_fixture(file_name: &str) -> RawTxView {
        let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("samples")
            .join("normalized")
            .join(file_name);
        let content = std::fs::read_to_string(path).unwrap();
        serde_json::from_str(&content).unwrap()
    }

    #[test]
    fn pumpfun_direct_buy_merge() {
        let view = load_fixture(
            "408960897-5c6muSpp3Poda2NHHau5AdANHmy9p6sPZJqWCSfZrs3KCU7s1RYcR3rtUkysFopqPxVVMwCqD7kDxtf4N9VyNeqk.json",
        );
        let trades = extract_merged_trades(&view);

        assert!(!trades.is_empty());

        let trade = &trades[0];

        assert!(trade.sol_amount > 0);
        assert!(trade.token_amount > 0);
        assert_eq!(trade.mint, trade.event.mint);
        assert_eq!(trade.user, trade.event.user);
        assert_eq!(trade.ix_name, trade.event.ix_name);
        assert_eq!(trade.is_buy, trade.event.is_buy);
        assert_eq!(trades.len(), 1);
        assert!(trade.is_buy);
    }

    #[test]
    fn pumpfun_direct_sell_merge() {
        let view = load_fixture(
            "408960897-3pJc2Qcj2Zr1xHESpjzfoaRveDRc22czs6yahyaGuYbY4rZUbojHPnPaThAUduxAQNrmoyVGggjdXAR1zGUETihm.json",
        );
        let trades = extract_merged_trades(&view);

        assert!(!trades.is_empty());

        let trade = &trades[0];

        assert!(trade.sol_amount > 0);
        assert!(trade.token_amount > 0);
        assert_eq!(trade.mint, trade.event.mint);
        assert_eq!(trade.user, trade.event.user);
        assert_eq!(trade.ix_name, trade.event.ix_name);
        assert_eq!(trade.is_buy, trade.event.is_buy);
        assert_eq!(trades.len(), 1);
        assert!(!trade.is_buy);
    }

    #[test]
    fn pumpfun_direct_buy_exact_sol_in_merge() {
        let view = load_fixture(
            "408960898-uSsCNYLqCgsvWNdgRaaPBCN8jz47NDA5fwPtXH9MrZYzRMr9JJe4EFaV2onKy4X6hzBEUsjubZ5WEb7gRR7zM7Q.json",
        );
        let trades = extract_merged_trades(&view);

        assert!(!trades.is_empty());

        let trade = &trades[0];

        assert!(trade.sol_amount > 0);
        assert!(trade.token_amount > 0);
        assert_eq!(trade.mint, trade.event.mint);
        assert_eq!(trade.user, trade.event.user);
        assert_eq!(trade.ix_name, trade.event.ix_name);
        assert_eq!(trade.is_buy, trade.event.is_buy);
        assert_eq!(trades.len(), 1);
        assert!(trade.is_buy);
    }

    #[test]
    fn pumpfun_aggregator_wrapped_trade_currently_returns_zero_trades() {
        let view = load_fixture(
            "408960896-3NHdGpk2tq6t8pmD6HYZGxKpDbNT5maTrHNpqxwioA1NQoQRRNYC4WLYKemz8t1WRiG9PXfSXrTAMu5kfFbbxtQs.json",
        );
        let trades = extract_merged_trades(&view);

        assert!(trades.is_empty());
    }
}
