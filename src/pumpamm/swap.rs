use super::{
    events::{extract_swap_cpi_events, extract_swap_events},
    invocation::extract_invocations,
    model::{ParsedSwap, PumpAmmInstruction, PumpAmmInvocation, SwapAnalysis, SwapEvent},
};
use crate::{event_origin::EventOrigin, transaction_view::TransactionView};

pub fn extract_swaps(view: &TransactionView) -> Vec<ParsedSwap> {
    analyze_swaps(view).swaps
}

pub fn analyze_swaps(view: &TransactionView) -> SwapAnalysis {
    let mut pending_events = extract_swap_events(&view.log_messages)
        .into_iter()
        .map(|event| (EventOrigin::Logs, event))
        .collect::<Vec<_>>();
    for event in extract_swap_cpi_events(&view.inner_instruction_groups) {
        if !pending_events
            .iter()
            .any(|(_, existing)| existing == &event)
        {
            pending_events.push((EventOrigin::InnerCpi, event));
        }
    }
    let invocations = extract_invocations(view)
        .into_iter()
        .filter(|invocation| invocation.instruction.is_swap())
        .collect::<Vec<_>>();

    let mut swaps = Vec::new();
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
        swaps.push(build_swap(invocation, event_source, event));
    }

    SwapAnalysis {
        swaps,
        unmatched_invocations,
        unmatched_events: pending_events.into_iter().map(|(_, event)| event).collect(),
    }
}

fn event_matches_instruction(instruction: &PumpAmmInstruction, event: &SwapEvent) -> bool {
    let Some(accounts) = instruction.swap_accounts() else {
        return false;
    };

    match (instruction, event) {
        (PumpAmmInstruction::Buy(ix), SwapEvent::Buy(event)) => {
            event.ix_name == instruction.ix_name()
                && event.pool == accounts.pool
                && event.user == accounts.user
                && event.base_amount_out == ix.base_amount_out
                && event.max_quote_amount_in == ix.max_quote_amount_in
                && ix
                    .track_volume
                    .is_none_or(|track_volume| event.track_volume == track_volume)
        }
        (PumpAmmInstruction::BuyExactQuoteIn(ix), SwapEvent::Buy(event)) => {
            event.ix_name == instruction.ix_name()
                && event.pool == accounts.pool
                && event.user == accounts.user
                && event.max_quote_amount_in == ix.spendable_quote_in
                && event.min_base_amount_out == ix.min_base_amount_out
                && ix
                    .track_volume
                    .is_none_or(|track_volume| event.track_volume == track_volume)
        }
        (PumpAmmInstruction::Sell(ix), SwapEvent::Sell(event)) => {
            event.pool == accounts.pool
                && event.user == accounts.user
                && event.base_amount_in == ix.base_amount_in
                && event.min_quote_amount_out == ix.min_quote_amount_out
        }
        _ => false,
    }
}

fn build_swap(
    invocation: PumpAmmInvocation,
    event_source: EventOrigin,
    event: SwapEvent,
) -> ParsedSwap {
    let accounts = invocation
        .instruction
        .swap_accounts()
        .expect("swap instructions must carry swap accounts");
    let side = invocation
        .instruction
        .swap_side()
        .expect("swap instructions must map to a side");

    let (pool, user, timestamp) = match &event {
        SwapEvent::Buy(event) => (&event.pool, &event.user, event.timestamp),
        SwapEvent::Sell(event) => (&event.pool, &event.user, event.timestamp),
    };

    ParsedSwap {
        source: invocation.source.clone(),
        event_source,
        side,
        pool: pool.clone(),
        user: user.clone(),
        base_mint: accounts.base_mint.clone(),
        quote_mint: accounts.quote_mint.clone(),
        timestamp,
        instruction: invocation.instruction.clone(),
        event,
    }
}

#[cfg(test)]
mod tests {
    use super::{analyze_swaps, extract_swaps};
    use crate::pumpamm::{
        constants::PUMP_AMM_PROGRAM_ID,
        discriminators::{BUY_EVENT_DISC, BUY_EXACT_QUOTE_IN_IX_DISC},
        model::{InvocationSource, PumpAmmInstruction, SwapEvent, SwapSide},
    };
    use crate::transaction_view::{
        InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
    };
    use base64::{Engine as _, engine::general_purpose::STANDARD};
    use std::{fs, path::Path};

    #[test]
    fn pumpamm_buy_exact_quote_in_from_inner_merges_successfully() {
        let accounts = fake_accounts(23);
        let view = TransactionView {
            slot: 0,
            signature: "sig".to_string(),
            tx_index: 0,
            is_vote: false,
            account_keys: vec![],
            loaded_writable_addresses: vec![],
            loaded_readonly_addresses: vec![],
            all_accounts: accounts.clone(),
            outer_instructions: vec![other_outer_ix()],
            inner_instruction_groups: vec![InnerInstructionGroup {
                outer_instruction_index: 0,
                instructions: vec![
                    buy_exact_quote_in_inner_ix(accounts.clone()),
                    buy_event_inner_ix(),
                ],
            }],
            log_messages: vec![],
        };

        let swaps = extract_swaps(&view);
        assert_eq!(swaps.len(), 1);

        let swap = &swaps[0];
        assert!(matches!(
            swap.source,
            InvocationSource::Inner {
                outer_index: 0,
                inner_index: 0
            }
        ));
        assert!(matches!(
            swap.instruction,
            PumpAmmInstruction::BuyExactQuoteIn(_)
        ));
        assert!(matches!(swap.side, SwapSide::BuyExactQuoteIn));
        assert_eq!(swap.pool, accounts[0]);
        assert_eq!(swap.user, accounts[1]);
        assert_eq!(swap.base_mint, accounts[3]);
        assert_eq!(swap.quote_mint, accounts[4]);

        match &swap.event {
            SwapEvent::Buy(event) => {
                assert_eq!(event.ix_name, "buy_exact_quote_in");
                assert_eq!(event.pool, accounts[0]);
                assert_eq!(event.user, accounts[1]);
                assert_eq!(event.max_quote_amount_in, 500);
                assert_eq!(event.min_base_amount_out, 400);
            }
            SwapEvent::Sell(_) => panic!("expected buy event"),
        }
    }

    #[test]
    fn pumpamm_real_multi_swap_fixture_merges_successfully() {
        let view = load_fixture(
            "409587793-3sbc2gtmRvM5Jfr98dAzV2hwpKtAkLkKhgTGAqG6doxW29tYtiJ3TF2Xh9WNW9Aoq8hNCS5BaqfUPzu2MWo1jhyb.json",
        );

        let analysis = analyze_swaps(&view);
        assert_eq!(analysis.swaps.len(), 2);
        assert!(analysis.unmatched_invocations.is_empty());
        assert!(analysis.unmatched_events.is_empty());

        let swaps = extract_swaps(&view);
        assert_eq!(swaps.len(), 2);
        assert!(
            swaps
                .iter()
                .all(|swap| matches!(swap.source, InvocationSource::Outer { .. }))
        );
        assert!(swaps.iter().any(|swap| matches!(swap.side, SwapSide::Buy)));
        assert!(swaps.iter().any(|swap| matches!(swap.side, SwapSide::Sell)));
        assert!(swaps.iter().all(|swap| !swap.pool.is_empty()));
        assert!(swaps.iter().all(|swap| !swap.user.is_empty()));
    }

    fn load_fixture(file_name: &str) -> TransactionView {
        let path = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("samples")
            .join("tests")
            .join("views")
            .join(file_name);

        let content = fs::read_to_string(path).expect("fixture must exist");
        serde_json::from_str(&content).expect("fixture must deserialize")
    }

    fn other_outer_ix() -> OuterInstructionView {
        OuterInstructionView {
            program_id_index: 0,
            program_id: "other-program".to_string(),
            account_indices: vec![],
            account_pubkeys: vec![],
            data_len: 0,
            data_prefix: vec![],
            data_base64: STANDARD.encode([]),
        }
    }

    fn buy_exact_quote_in_inner_ix(accounts: Vec<String>) -> InnerInstructionView {
        let mut data = Vec::from(BUY_EXACT_QUOTE_IN_IX_DISC);
        data.extend_from_slice(&500u64.to_le_bytes());
        data.extend_from_slice(&400u64.to_le_bytes());
        data.push(1);

        InnerInstructionView {
            program_id_index: 0,
            program_id: PUMP_AMM_PROGRAM_ID.to_string(),
            account_indices: (0..23).collect(),
            account_pubkeys: accounts,
            data_len: data.len(),
            data_prefix: data.iter().take(16).copied().collect(),
            data_base64: STANDARD.encode(data),
            stack_height: Some(2),
        }
    }

    fn buy_event_inner_ix() -> InnerInstructionView {
        let mut data = vec![228, 69, 165, 46, 81, 203, 154, 29];
        data.extend_from_slice(&buy_event_bytes());

        InnerInstructionView {
            program_id_index: 0,
            program_id: PUMP_AMM_PROGRAM_ID.to_string(),
            account_indices: vec![],
            account_pubkeys: vec![],
            data_len: data.len(),
            data_prefix: data.iter().take(16).copied().collect(),
            data_base64: STANDARD.encode(data),
            stack_height: Some(2),
        }
    }

    fn fake_accounts(count: usize) -> Vec<String> {
        (1..=count)
            .map(|seed| bs58::encode([seed as u8; 32]).into_string())
            .collect()
    }

    fn push_string(data: &mut Vec<u8>, value: &str) {
        data.extend_from_slice(&(value.len() as u32).to_le_bytes());
        data.extend_from_slice(value.as_bytes());
    }

    fn buy_event_bytes() -> Vec<u8> {
        let mut data = Vec::from(BUY_EVENT_DISC);
        data.extend_from_slice(&1_777_777_777i64.to_le_bytes());
        data.extend_from_slice(&450u64.to_le_bytes()); // base_amount_out
        data.extend_from_slice(&500u64.to_le_bytes()); // max_quote_amount_in
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&490u64.to_le_bytes()); // quote_amount_in
        data.extend_from_slice(&95u64.to_le_bytes());
        data.extend_from_slice(&5u64.to_le_bytes());
        data.extend_from_slice(&20u64.to_le_bytes());
        data.extend_from_slice(&2u64.to_le_bytes());
        data.extend_from_slice(&495u64.to_le_bytes());
        data.extend_from_slice(&500u64.to_le_bytes()); // user_quote_amount_in
        data.extend_from_slice(&[1u8; 32]); // pool -> accounts[0]
        data.extend_from_slice(&[2u8; 32]); // user -> accounts[1]
        data.extend_from_slice(&[6u8; 32]);
        data.extend_from_slice(&[7u8; 32]);
        data.extend_from_slice(&[10u8; 32]);
        data.extend_from_slice(&[11u8; 32]);
        data.extend_from_slice(&[19u8; 32]);
        data.extend_from_slice(&30u64.to_le_bytes());
        data.extend_from_slice(&1u64.to_le_bytes());
        data.push(1);
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&1_777_777_000i64.to_le_bytes());
        data.extend_from_slice(&400u64.to_le_bytes()); // min_base_amount_out
        push_string(&mut data, "buy_exact_quote_in");
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data
    }
}
