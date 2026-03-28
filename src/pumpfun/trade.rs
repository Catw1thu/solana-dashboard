use super::{
    events::extract_trade_events,
    invocation::extract_invocations,
    model::{ParsedTrade, PumpfunInstruction, PumpfunInvocation, TradeEvent},
};
use crate::{pumpfun::model::TradeAnalysis, transaction_view::TransactionView};

pub fn extract_trades(view: &TransactionView) -> Vec<ParsedTrade> {
    analyze_trades(view).trades
}

pub fn analyze_trades(view: &TransactionView) -> TradeAnalysis {
    let mut pending_events = extract_trade_events(&view.log_messages);
    let invocations = extract_invocations(view);
    let mut trades = Vec::new();
    let mut unmatched_invocations = Vec::new();

    for invocation in invocations {
        let Some(event_index) = pending_events
            .iter()
            .position(|event| event_matches_instruction(&invocation.instruction, event))
        else {
            unmatched_invocations.push(invocation);
            continue;
        };
        let event = pending_events.remove(event_index);
        trades.push(build_trade(invocation, event));
    }

    TradeAnalysis {
        trades,
        unmatched_invocations,
        unmatched_events: pending_events,
    }
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
    use super::{analyze_trades, extract_trades};
    use crate::{
        pumpfun::{
            PUMPFUN_PROGRAM_ID,
            model::{InvocationSource, PumpfunInstruction, TradeSide},
        },
        transaction_view::TransactionView,
    };

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
    fn pumpfun_wrapped_buy_trade_from_inner() {
        let view = load_fixture(
            "408960896-3NHdGpk2tq6t8pmD6HYZGxKpDbNT5maTrHNpqxwioA1NQoQRRNYC4WLYKemz8t1WRiG9PXfSXrTAMu5kfFbbxtQs.json",
        );

        assert!(
            view.outer_instructions
                .iter()
                .all(|ix| ix.program_id != PUMPFUN_PROGRAM_ID)
        );

        let trades = extract_trades(&view);

        assert_eq!(trades.len(), 1);
        let trade = &trades[0];

        assert!(matches!(
            trade.source,
            InvocationSource::Inner {
                outer_index: 1,
                inner_index: 2
            }
        ));
        assert!(matches!(trade.instruction, PumpfunInstruction::Buy(_)));
        assert!(matches!(trade.side, TradeSide::Buy));
        assert_eq!(trade.ix_name, "buy");
        assert!(trade.is_buy);
        assert_eq!(trade.mint, "FRSrEK3nr9gQ2gir6pkrFUS4n7vaDJhjyCMcQbQFpump");
        assert_eq!(trade.user, "8gCJYyKWnKGoY6Di3iF1iUErfXHpQyiaGLB1TyRYbhcF");
        assert_eq!(
            trade.bonding_curve,
            "EyVx2fzEjdoZe7ZmRswAkpEjio4NmSMysuGkE4ks63sf"
        );
        assert_eq!(
            trade.creator_vault,
            "96bWWuSF6de5H3wFAMb2gQ2sV2NLuwFJpB4WZ7BZnFJD"
        );
        assert_eq!(
            trade.token_program,
            "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
        );
        assert_eq!(trade.mint, trade.event.mint);
        assert_eq!(trade.user, trade.event.user);
        assert_eq!(trade.ix_name, trade.event.ix_name);
        assert_eq!(trade.is_buy, trade.event.is_buy);
        assert!(trade.sol_amount > 0);
        assert!(trade.token_amount > 0);
    }

    #[test]
    fn pumpfun_multi_trade_sell_many_analysis() {
        let view = load_fixture(
            "409396076-2neiyZdnVzZLxwzPLLbsWt41qjjZEBMK6GSAK9BwmFBc31nKY4i7M1e46UzPHz8yK46Hq7CTzzoN5hCuAKrdTGTV.json",
        );

        let analysis = analyze_trades(&view);

        assert_eq!(analysis.trades.len(), 2);
        assert!(analysis.unmatched_invocations.is_empty());
        assert!(analysis.unmatched_events.is_empty());

        let first = &analysis.trades[0];
        let second = &analysis.trades[1];

        assert!(matches!(
            first.source,
            InvocationSource::Inner {
                outer_index: 2,
                inner_index: 0
            }
        ));
        assert!(matches!(
            second.source,
            InvocationSource::Inner {
                outer_index: 2,
                inner_index: 7
            }
        ));
        assert!(matches!(first.instruction, PumpfunInstruction::Sell(_)));
        assert!(matches!(second.instruction, PumpfunInstruction::Sell(_)));
        assert!(matches!(first.side, TradeSide::Sell));
        assert!(matches!(second.side, TradeSide::Sell));
        assert_eq!(first.ix_name, "sell");
        assert_eq!(second.ix_name, "sell");
        assert_eq!(first.mint, "BAqcCCgMNLwpiNcMq3PHaWF4uDWtpz1ekFuzYCqjpump");
        assert_eq!(second.mint, "BAqcCCgMNLwpiNcMq3PHaWF4uDWtpz1ekFuzYCqjpump");
        assert_ne!(first.user, second.user);
        assert_eq!(first.user, "71WXxTi8LDj95a3pDbHHwxj7UCNBXcufvLQec9956J5x");
        assert_eq!(second.user, "3VvoMU8jCM3iqqNsZNYP9GtC8FmeW2mKT4hyo4CZ29bx");
        assert!(first.sol_amount > 0);
        assert!(second.sol_amount > 0);
        assert!(first.token_amount > 0);
        assert!(second.token_amount > 0);
    }
}
