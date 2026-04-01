use super::{
    events::{extract_trade_cpi_events, extract_trade_events},
    invocation::extract_invocations,
    model::{ParsedTrade, PumpfunInstruction, PumpfunInvocation, TradeEvent},
};
use crate::{
    event_origin::EventOrigin,
    pumpfun::model::TradeAnalysis,
    transaction_view::TransactionView,
};

pub fn extract_trades(view: &TransactionView) -> Vec<ParsedTrade> {
    analyze_trades(view).trades
}

pub fn analyze_trades(view: &TransactionView) -> TradeAnalysis {
    let mut pending_events = extract_trade_events(&view.log_messages)
        .into_iter()
        .map(|event| (EventOrigin::Logs, event))
        .collect::<Vec<_>>();
    for event in extract_trade_cpi_events(&view.inner_instruction_groups) {
        if !pending_events.iter().any(|(_, existing)| existing == &event) {
            pending_events.push((EventOrigin::InnerCpi, event));
        }
    }
    let invocations = extract_invocations(view)
        .into_iter()
        .filter(|invocation| invocation.instruction.is_trade())
        .collect::<Vec<_>>();
    let mut trades = Vec::new();
    let mut unmatched_invocations = Vec::new();

    for invocation in invocations {
        let Some(event_index) = pending_events
            .iter()
            .position(|(_, event)| event_matches_instruction(&invocation.instruction, event))
        else {
            unmatched_invocations.push(invocation);
            continue;
        };
        let (event_source, event) = pending_events.remove(event_index);
        trades.push(build_trade(invocation, event_source, event));
    }

    TradeAnalysis {
        trades,
        unmatched_invocations,
        unmatched_events: pending_events.into_iter().map(|(_, event)| event).collect(),
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

fn build_trade(
    invocation: PumpfunInvocation,
    event_source: EventOrigin,
    event: TradeEvent,
) -> ParsedTrade {
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
        event_source,
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
            discriminators::{BUY_IX_DISC, TRADE_EVENT_DISC},
            model::{InvocationSource, PumpfunInstruction, TradeSide},
        },
        transaction_view::{
            InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
        },
    };
    use base64::{Engine as _, engine::general_purpose::STANDARD};

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

    #[test]
    fn pumpfun_trade_can_fallback_to_inner_cpi_event() {
        let accounts = fake_accounts(16);
        let view = TransactionView {
            slot: 0,
            signature: "sig".to_string(),
            tx_index: 0,
            is_vote: false,
            account_keys: vec![],
            loaded_writable_addresses: vec![],
            loaded_readonly_addresses: vec![],
            all_accounts: accounts.clone(),
            outer_instructions: vec![buy_outer_ix(accounts.clone())],
            inner_instruction_groups: vec![InnerInstructionGroup {
                outer_instruction_index: 0,
                instructions: vec![trade_event_inner_ix(accounts.clone())],
            }],
            log_messages: vec![],
        };

        let trades = extract_trades(&view);
        assert_eq!(trades.len(), 1);

        let trade = &trades[0];
        assert!(matches!(
            trade.source,
            InvocationSource::Outer { outer_index: 0 }
        ));
        assert!(matches!(trade.instruction, PumpfunInstruction::Buy(_)));
        assert!(matches!(trade.side, TradeSide::Buy));
        assert_eq!(trade.ix_name, "buy");
        assert!(trade.is_buy);
        assert_eq!(trade.mint, accounts[2]);
        assert_eq!(trade.user, accounts[6]);
        assert_eq!(trade.fee_recipient, accounts[1]);
        assert_eq!(trade.creator, accounts[9]);
        assert_eq!(trade.sol_amount, 111);
        assert_eq!(trade.token_amount, 222);
    }

    fn fake_accounts(count: usize) -> Vec<String> {
        (1..=count)
            .map(|seed| bs58::encode([seed as u8; 32]).into_string())
            .collect()
    }

    fn buy_outer_ix(accounts: Vec<String>) -> OuterInstructionView {
        let mut data = Vec::from(BUY_IX_DISC);
        data.extend_from_slice(&222u64.to_le_bytes());
        data.extend_from_slice(&333u64.to_le_bytes());
        data.push(1);

        OuterInstructionView {
            program_id_index: 0,
            program_id: PUMPFUN_PROGRAM_ID.to_string(),
            account_indices: (0..16).collect(),
            account_pubkeys: accounts,
            data_len: data.len(),
            data_prefix: data.iter().take(16).copied().collect(),
            data_base64: STANDARD.encode(data),
        }
    }

    fn trade_event_inner_ix(accounts: Vec<String>) -> InnerInstructionView {
        let mut data = vec![1, 2, 3, 4, 5, 6, 7, 8];
        data.extend_from_slice(&trade_event_bytes(&accounts));

        InnerInstructionView {
            program_id_index: 0,
            program_id: PUMPFUN_PROGRAM_ID.to_string(),
            account_indices: vec![],
            account_pubkeys: vec![],
            data_len: data.len(),
            data_prefix: data.iter().take(16).copied().collect(),
            data_base64: STANDARD.encode(data),
            stack_height: Some(2),
        }
    }

    fn push_string(data: &mut Vec<u8>, value: &str) {
        data.extend_from_slice(&(value.len() as u32).to_le_bytes());
        data.extend_from_slice(value.as_bytes());
    }

    fn trade_event_bytes(accounts: &[String]) -> Vec<u8> {
        let mint = bs58::decode(&accounts[2]).into_vec().unwrap();
        let fee_recipient = bs58::decode(&accounts[1]).into_vec().unwrap();
        let user = bs58::decode(&accounts[6]).into_vec().unwrap();
        let creator = bs58::decode(&accounts[9]).into_vec().unwrap();

        let mut data = Vec::from(TRADE_EVENT_DISC);
        data.extend_from_slice(&mint);
        data.extend_from_slice(&111u64.to_le_bytes());
        data.extend_from_slice(&222u64.to_le_bytes());
        data.push(1);
        data.extend_from_slice(&user);
        data.extend_from_slice(&1_777_777_777i64.to_le_bytes());
        data.extend_from_slice(&10u64.to_le_bytes());
        data.extend_from_slice(&11u64.to_le_bytes());
        data.extend_from_slice(&12u64.to_le_bytes());
        data.extend_from_slice(&13u64.to_le_bytes());
        data.extend_from_slice(&fee_recipient);
        data.extend_from_slice(&30u64.to_le_bytes());
        data.extend_from_slice(&3u64.to_le_bytes());
        data.extend_from_slice(&creator);
        data.extend_from_slice(&40u64.to_le_bytes());
        data.extend_from_slice(&4u64.to_le_bytes());
        data.push(1);
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&1_777_777_000i64.to_le_bytes());
        push_string(&mut data, "buy");
        data.push(0);
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data
    }
}
