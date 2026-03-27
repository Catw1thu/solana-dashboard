use super::{
    events::extract_trade_events,
    invocation::extract_invocations,
    model::{ParsedTrade, PumpfunInstruction, PumpfunInvocation, TradeEvent},
};
use crate::transaction_view::TransactionView;

pub fn extract_trades(view: &TransactionView) -> Vec<ParsedTrade> {
    let mut pending_events = extract_trade_events(&view.log_messages);
    let invocations = extract_invocations(view);
    let mut trades = Vec::new();

    for invocation in invocations {
        let Some(event_index) = pending_events
            .iter()
            .position(|event| event_matches_instruction(&invocation.instruction, event))
        else {
            continue;
        };
        let event = pending_events.remove(event_index);
        trades.push(build_trade(invocation, event));
    }

    trades
}

fn event_matches_instruction(instruction: &PumpfunInstruction, event: &TradeEvent) -> bool {
    let Some(expected_ix_name) = instruction.ix_name() else {
        return false;
    };
    let Some(accounts) = instruction.accounts() else {
        return false;
    };

    let expected_is_buy = matches!(
        instruction,
        PumpfunInstruction::Buy(_) | PumpfunInstruction::BuyExactSolIn(_)
    );

    event.ix_name == expected_ix_name
        && event.is_buy == expected_is_buy
        && event.mint == accounts.mint
        && event.user == accounts.user
}

fn build_trade(invocation: PumpfunInvocation, event: TradeEvent) -> ParsedTrade {
    let accounts = invocation
        .instruction
        .accounts()
        .expect("trade instructions must always carry trade accounts");
    let side = invocation
        .instruction
        .side()
        .expect("trade instructions must always map to a side");

    ParsedTrade {
        source: invocation.source.clone(),
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
        instruction: invocation.instruction.clone(),
        event,
    }
}

#[cfg(test)]
mod tests {
    use super::extract_trades;
    use crate::transaction_view::TransactionView;

    fn load_fixture(file_name: &str) -> TransactionView {
        let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("samples")
            .join("views")
            .join(file_name);
        let content = std::fs::read_to_string(path).unwrap();
        serde_json::from_str(&content).unwrap()
    }

    #[test]
    fn pumpfun_direct_buy_trade() {
        let view = load_fixture(
            "408960897-5c6muSpp3Poda2NHHau5AdANHmy9p6sPZJqWCSfZrs3KCU7s1RYcR3rtUkysFopqPxVVMwCqD7kDxtf4N9VyNeqk.json",
        );
        let trades = extract_trades(&view);
        assert_eq!(trades.len(), 1);
        let trade = &trades[0];

        assert!(trade.is_buy);
        assert!(trade.sol_amount > 0);
        assert!(trade.token_amount > 0);
        assert_eq!(trade.mint, trade.event.mint);
        assert_eq!(trade.user, trade.event.user);
        assert_eq!(trade.ix_name, trade.event.ix_name);
        assert_eq!(trade.is_buy, trade.event.is_buy);
    }

    #[test]
    fn pumpfun_direct_sell_trade() {
        let view = load_fixture(
            "408960897-3pJc2Qcj2Zr1xHESpjzfoaRveDRc22czs6yahyaGuYbY4rZUbojHPnPaThAUduxAQNrmoyVGggjdXAR1zGUETihm.json",
        );
        let trades = extract_trades(&view);
        assert_eq!(trades.len(), 1);
        let trade = &trades[0];

        assert!(!trade.is_buy);
        assert!(trade.sol_amount > 0);
        assert!(trade.token_amount > 0);
        assert_eq!(trade.mint, trade.event.mint);
        assert_eq!(trade.user, trade.event.user);
        assert_eq!(trade.ix_name, trade.event.ix_name);
        assert_eq!(trade.is_buy, trade.event.is_buy);
    }

    #[test]
    fn pumpfun_direct_buy_exact_sol_in_trade() {
        let view = load_fixture(
            "408960898-uSsCNYLqCgsvWNdgRaaPBCN8jz47NDA5fwPtXH9MrZYzRMr9JJe4EFaV2onKy4X6hzBEUsjubZ5WEb7gRR7zM7Q.json",
        );
        let trades = extract_trades(&view);
        assert_eq!(trades.len(), 1);
        let trade = &trades[0];

        assert!(trade.is_buy);
        assert!(trade.sol_amount > 0);
        assert!(trade.token_amount > 0);
        assert_eq!(trade.mint, trade.event.mint);
        assert_eq!(trade.user, trade.event.user);
        assert_eq!(trade.ix_name, trade.event.ix_name);
        assert_eq!(trade.is_buy, trade.event.is_buy);
    }

    #[test]
    fn pumpfun_wrapped_trade() {
        let view = load_fixture(
            "408960896-3NHdGpk2tq6t8pmD6HYZGxKpDbNT5maTrHNpqxwioA1NQoQRRNYC4WLYKemz8t1WRiG9PXfSXrTAMu5kfFbbxtQs.json",
        );
        let trades = extract_trades(&view);
        assert!(!trades.is_empty());
    }
}
