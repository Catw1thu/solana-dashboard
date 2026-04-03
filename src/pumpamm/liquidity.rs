use super::model::{
    CreatePoolEvent, LiquidityEvent, ParsedLiquidityAction, ParsedPoolCreation, PumpAmmInstruction,
    PumpAmmInvocation,
};
use crate::event_origin::EventOrigin;
#[cfg(test)]
use crate::{
    transaction_view::TransactionView,
    unified_parser::{ParsedEvent, parse_view},
};

#[cfg(test)]
pub fn extract_pool_creations(view: &TransactionView) -> Vec<ParsedPoolCreation> {
    parse_view(view)
        .into_iter()
        .filter_map(|event| match event {
            ParsedEvent::PumpAmmCreatePool(creation) => Some(creation),
            _ => None,
        })
        .collect()
}

#[cfg(test)]
pub fn extract_liquidity_actions(view: &TransactionView) -> Vec<ParsedLiquidityAction> {
    parse_view(view)
        .into_iter()
        .filter_map(|event| match event {
            ParsedEvent::PumpAmmLiquidity(action) => Some(action),
            _ => None,
        })
        .collect()
}

pub(crate) fn event_matches_create_pool(
    instruction: &PumpAmmInstruction,
    event: &CreatePoolEvent,
) -> bool {
    let Some(accounts) = instruction.create_pool_accounts() else {
        return false;
    };

    match instruction {
        PumpAmmInstruction::CreatePool(ix) => {
            event.pool == accounts.pool
                && event.creator == accounts.creator
                && event.base_mint == accounts.base_mint
                && event.quote_mint == accounts.quote_mint
                && event.base_amount_in == ix.base_amount_in
                && event.quote_amount_in == ix.quote_amount_in
                && event.coin_creator == ix.coin_creator
                && event.is_mayhem_mode == ix.is_mayhem_mode
        }
        _ => false,
    }
}

pub(crate) fn event_matches_liquidity(
    instruction: &PumpAmmInstruction,
    event: &LiquidityEvent,
) -> bool {
    let Some(accounts) = instruction.liquidity_accounts() else {
        return false;
    };

    match (instruction, event) {
        (PumpAmmInstruction::Deposit(ix), LiquidityEvent::Deposit(event)) => {
            event.pool == accounts.pool
                && event.user == accounts.user
                && event.lp_token_amount_out == ix.lp_token_amount_out
                && event.max_base_amount_in == ix.max_base_amount_in
                && event.max_quote_amount_in == ix.max_quote_amount_in
        }
        (PumpAmmInstruction::Withdraw(ix), LiquidityEvent::Withdraw(event)) => {
            event.pool == accounts.pool
                && event.user == accounts.user
                && event.lp_token_amount_in == ix.lp_token_amount_in
                && event.min_base_amount_out == ix.min_base_amount_out
                && event.min_quote_amount_out == ix.min_quote_amount_out
        }
        _ => false,
    }
}

pub(crate) fn build_pool_creation(
    invocation: PumpAmmInvocation,
    event_source: EventOrigin,
    event: CreatePoolEvent,
) -> ParsedPoolCreation {
    ParsedPoolCreation {
        source: invocation.source.clone(),
        event_source,
        pool: event.pool.clone(),
        creator: event.creator.clone(),
        base_mint: event.base_mint.clone(),
        quote_mint: event.quote_mint.clone(),
        lp_mint: event.lp_mint.clone(),
        base_amount_in: event.base_amount_in,
        quote_amount_in: event.quote_amount_in,
        initial_liquidity: event.initial_liquidity,
        coin_creator: event.coin_creator.clone(),
        is_mayhem_mode: event.is_mayhem_mode,
        instruction: invocation.instruction.clone(),
        event,
    }
}

pub(crate) fn build_liquidity_action(
    invocation: PumpAmmInvocation,
    event_source: EventOrigin,
    event: LiquidityEvent,
) -> ParsedLiquidityAction {
    let accounts = invocation
        .instruction
        .liquidity_accounts()
        .expect("liquidity instructions must carry liquidity accounts");
    let action = invocation
        .instruction
        .liquidity_action()
        .expect("liquidity instructions must map to an action");

    let (pool, user, timestamp) = match &event {
        LiquidityEvent::Deposit(event) => (&event.pool, &event.user, event.timestamp),
        LiquidityEvent::Withdraw(event) => (&event.pool, &event.user, event.timestamp),
    };

    ParsedLiquidityAction {
        source: invocation.source.clone(),
        event_source,
        action,
        pool: pool.clone(),
        user: user.clone(),
        base_mint: accounts.base_mint.clone(),
        quote_mint: accounts.quote_mint.clone(),
        lp_mint: accounts.lp_mint.clone(),
        timestamp,
        instruction: invocation.instruction.clone(),
        event,
    }
}

#[cfg(test)]
mod tests {
    use super::{extract_liquidity_actions, extract_pool_creations};
    use crate::pumpamm::{
        constants::PUMP_AMM_PROGRAM_ID,
        discriminators::{
            CREATE_POOL_EVENT_DISC, CREATE_POOL_IX_DISC, DEPOSIT_EVENT_DISC, DEPOSIT_IX_DISC,
            WITHDRAW_EVENT_DISC, WITHDRAW_IX_DISC,
        },
        model::{InvocationSource, LiquidityAction, LiquidityEvent, PumpAmmInstruction},
    };
    use crate::transaction_view::{
        InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
    };
    use base64::{Engine as _, engine::general_purpose::STANDARD};
    use std::{fs, path::Path};

    #[test]
    fn pumpamm_create_pool_merges_successfully() {
        let accounts = fake_accounts(18);
        let view = TransactionView {
            slot: 0,
            signature: "sig".to_string(),
            tx_index: 0,
            is_vote: false,
            account_keys: vec![],
            loaded_writable_addresses: vec![],
            loaded_readonly_addresses: vec![],
            all_accounts: accounts.clone(),
            outer_instructions: vec![create_pool_outer_ix(accounts.clone())],
            inner_instruction_groups: vec![InnerInstructionGroup {
                outer_instruction_index: 0,
                instructions: vec![create_pool_event_inner_ix()],
            }],
            log_messages: vec![],
        };

        let creations = extract_pool_creations(&view);
        assert_eq!(creations.len(), 1);
        let creation = &creations[0];

        assert!(matches!(
            creation.source,
            InvocationSource::Outer { outer_index: 0 }
        ));
        assert!(matches!(
            creation.instruction,
            PumpAmmInstruction::CreatePool(_)
        ));
        assert_eq!(creation.pool, accounts[0]);
        assert_eq!(creation.creator, accounts[2]);
        assert_eq!(creation.base_mint, accounts[3]);
        assert_eq!(creation.quote_mint, accounts[4]);
        assert_eq!(creation.lp_mint, accounts[5]);
        assert_eq!(creation.base_amount_in, 100);
        assert_eq!(creation.quote_amount_in, 200);
    }

    #[test]
    fn pumpamm_create_pool_from_migration_fixture_merges_successfully() {
        let view = load_fixture(
            "409576108-3zCwTozsNVfMaSftorXKLbbdVAmNPaPy3oZXN5ch6eMBdYdKfoB9GAgsiwhAFq786wnYoP9Lv64XjC8LbaKnbijZ.json",
        );

        let creations = extract_pool_creations(&view);
        assert_eq!(creations.len(), 1);

        let creation = &creations[0];
        assert!(matches!(
            creation.source,
            InvocationSource::Inner {
                outer_index: 2,
                inner_index: 22,
            }
        ));
        assert!(matches!(
            creation.instruction,
            PumpAmmInstruction::CreatePool(_)
        ));
        assert_eq!(
            creation.pool,
            "DnxHHLuC5GbtqT5bo9jmrg4QQPSTZUzb2AcTNf93iwP5"
        );
        assert_eq!(
            creation.creator,
            "428jPeuGES9SfDBCkiBvoAKhL6TraBGFZ2oLDriWS2ya"
        );
        assert_eq!(
            creation.base_mint,
            "GVmgdyiK6xNTdAeqshXT3iGqhK36AvqZT1iQpKampump"
        );
        assert_eq!(
            creation.quote_mint,
            "So11111111111111111111111111111111111111112"
        );
        assert_eq!(
            creation.lp_mint,
            "9dFt11yagNYwqeL4iodHm8AUYS9Jtj9j8z1D4STevzbG"
        );
        assert_eq!(creation.pool, creation.event.pool);
        assert_eq!(creation.creator, creation.event.creator);
        assert!(creation.base_amount_in > 0);
        assert!(creation.quote_amount_in > 0);
        assert!(creation.initial_liquidity > 0);
    }

    #[test]
    fn pumpamm_liquidity_actions_merge_successfully() {
        let deposit_accounts = fake_accounts(15);
        let withdraw_accounts = fake_accounts_with_offset(15, 20);
        let view = TransactionView {
            slot: 0,
            signature: "sig".to_string(),
            tx_index: 0,
            is_vote: false,
            account_keys: vec![],
            loaded_writable_addresses: vec![],
            loaded_readonly_addresses: vec![],
            all_accounts: deposit_accounts.clone(),
            outer_instructions: vec![
                deposit_outer_ix(deposit_accounts.clone()),
                withdraw_outer_ix(withdraw_accounts.clone()),
            ],
            inner_instruction_groups: vec![
                InnerInstructionGroup {
                    outer_instruction_index: 0,
                    instructions: vec![deposit_event_inner_ix()],
                },
                InnerInstructionGroup {
                    outer_instruction_index: 1,
                    instructions: vec![withdraw_event_inner_ix()],
                },
            ],
            log_messages: vec![],
        };

        let actions = extract_liquidity_actions(&view);
        assert_eq!(actions.len(), 2);

        assert!(matches!(
            actions[0].source,
            InvocationSource::Outer { outer_index: 0 }
        ));
        assert!(matches!(actions[0].action, LiquidityAction::Deposit));
        assert!(matches!(
            actions[0].instruction,
            PumpAmmInstruction::Deposit(_)
        ));
        match &actions[0].event {
            LiquidityEvent::Deposit(event) => {
                assert_eq!(event.pool, deposit_accounts[0]);
                assert_eq!(event.user, deposit_accounts[2]);
            }
            LiquidityEvent::Withdraw(_) => panic!("expected deposit"),
        }

        assert!(matches!(
            actions[1].source,
            InvocationSource::Outer { outer_index: 1 }
        ));
        assert!(matches!(actions[1].action, LiquidityAction::Withdraw));
        assert!(matches!(
            actions[1].instruction,
            PumpAmmInstruction::Withdraw(_)
        ));
        match &actions[1].event {
            LiquidityEvent::Withdraw(event) => {
                assert_eq!(event.pool, withdraw_accounts[0]);
                assert_eq!(event.user, withdraw_accounts[2]);
            }
            LiquidityEvent::Deposit(_) => panic!("expected withdraw"),
        }
    }

    #[test]
    fn pumpamm_real_withdraw_fixture_merges_successfully() {
        let view = load_fixture(
            "409590250-yoENqqk48Fq9LJa8wS8RJdtTgYLsSuFXaFJDbbAsD7G1mnGgYgRJDEeyMWFmKwsJzN9GjoFfd5PTceDKawW65pw.json",
        );

        let actions = extract_liquidity_actions(&view);
        assert_eq!(actions.len(), 1);

        let action = &actions[0];
        assert!(matches!(
            action.source,
            InvocationSource::Outer { outer_index: 3 }
        ));
        assert!(matches!(action.action, LiquidityAction::Withdraw));
        assert!(matches!(
            action.instruction,
            PumpAmmInstruction::Withdraw(_)
        ));
        assert_eq!(action.pool, "8naZnQi7SwC7sFP4Y1tEccaaunPGesDyG22CywDz2CJi");
        assert_eq!(action.user, "9dBgK7Gx6ZLAmVSCw549LkEgkK61p6qv8wUG5qmVKn2x");
        assert_eq!(
            action.pool,
            match &action.event {
                LiquidityEvent::Withdraw(event) => event.pool.clone(),
                LiquidityEvent::Deposit(_) => panic!("expected withdraw"),
            }
        );
        assert_eq!(
            action.user,
            match &action.event {
                LiquidityEvent::Withdraw(event) => event.user.clone(),
                LiquidityEvent::Deposit(_) => panic!("expected withdraw"),
            }
        );
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

    fn create_pool_outer_ix(accounts: Vec<String>) -> OuterInstructionView {
        let mut data = Vec::from(CREATE_POOL_IX_DISC);
        data.extend_from_slice(&1u16.to_le_bytes());
        data.extend_from_slice(&100u64.to_le_bytes());
        data.extend_from_slice(&200u64.to_le_bytes());
        data.extend_from_slice(&[19u8; 32]);
        data.push(1);
        data.push(0);

        OuterInstructionView {
            program_id_index: 0,
            program_id: PUMP_AMM_PROGRAM_ID.to_string(),
            account_indices: (0..18).collect(),
            account_pubkeys: accounts,
            data_len: data.len(),
            data_prefix: data.iter().take(16).copied().collect(),
            data_base64: STANDARD.encode(data),
        }
    }

    fn create_pool_event_inner_ix() -> InnerInstructionView {
        let mut data = vec![228, 69, 165, 46, 81, 203, 154, 29];
        data.extend_from_slice(&create_pool_event_bytes());

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

    fn deposit_outer_ix(accounts: Vec<String>) -> OuterInstructionView {
        let mut data = Vec::from(DEPOSIT_IX_DISC);
        data.extend_from_slice(&10u64.to_le_bytes());
        data.extend_from_slice(&100u64.to_le_bytes());
        data.extend_from_slice(&200u64.to_le_bytes());

        OuterInstructionView {
            program_id_index: 0,
            program_id: PUMP_AMM_PROGRAM_ID.to_string(),
            account_indices: (0..15).collect(),
            account_pubkeys: accounts,
            data_len: data.len(),
            data_prefix: data.iter().take(16).copied().collect(),
            data_base64: STANDARD.encode(data),
        }
    }

    fn deposit_event_inner_ix() -> InnerInstructionView {
        let mut data = vec![228, 69, 165, 46, 81, 203, 154, 29];
        data.extend_from_slice(&deposit_event_bytes());

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

    fn withdraw_outer_ix(accounts: Vec<String>) -> OuterInstructionView {
        let mut data = Vec::from(WITHDRAW_IX_DISC);
        data.extend_from_slice(&11u64.to_le_bytes());
        data.extend_from_slice(&101u64.to_le_bytes());
        data.extend_from_slice(&201u64.to_le_bytes());

        OuterInstructionView {
            program_id_index: 0,
            program_id: PUMP_AMM_PROGRAM_ID.to_string(),
            account_indices: (0..15).collect(),
            account_pubkeys: accounts,
            data_len: data.len(),
            data_prefix: data.iter().take(16).copied().collect(),
            data_base64: STANDARD.encode(data),
        }
    }

    fn withdraw_event_inner_ix() -> InnerInstructionView {
        let mut data = vec![228, 69, 165, 46, 81, 203, 154, 29];
        data.extend_from_slice(&withdraw_event_bytes());

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
        fake_accounts_with_offset(count, 0)
    }

    fn fake_accounts_with_offset(count: usize, offset: u8) -> Vec<String> {
        (1..=count)
            .map(|seed| bs58::encode([seed as u8 + offset; 32]).into_string())
            .collect()
    }

    fn create_pool_event_bytes() -> Vec<u8> {
        let mut data = Vec::from(CREATE_POOL_EVENT_DISC);
        data.extend_from_slice(&1_777_777_777i64.to_le_bytes());
        data.extend_from_slice(&1u16.to_le_bytes());
        data.extend_from_slice(&[3u8; 32]); // creator accounts[2]
        data.extend_from_slice(&[4u8; 32]); // base_mint
        data.extend_from_slice(&[5u8; 32]); // quote_mint
        data.push(6);
        data.push(9);
        data.extend_from_slice(&100u64.to_le_bytes());
        data.extend_from_slice(&200u64.to_le_bytes());
        data.extend_from_slice(&100u64.to_le_bytes());
        data.extend_from_slice(&200u64.to_le_bytes());
        data.extend_from_slice(&1u64.to_le_bytes());
        data.extend_from_slice(&50u64.to_le_bytes());
        data.extend_from_slice(&10u64.to_le_bytes());
        data.push(255);
        data.extend_from_slice(&[1u8; 32]); // pool accounts[0]
        data.extend_from_slice(&[6u8; 32]); // lp mint accounts[5]
        data.extend_from_slice(&[7u8; 32]);
        data.extend_from_slice(&[8u8; 32]);
        data.extend_from_slice(&[19u8; 32]); // coin_creator
        data.push(1);
        data
    }

    fn deposit_event_bytes() -> Vec<u8> {
        let mut data = Vec::from(DEPOSIT_EVENT_DISC);
        data.extend_from_slice(&1_777_777_777i64.to_le_bytes());
        data.extend_from_slice(&10u64.to_le_bytes());
        data.extend_from_slice(&100u64.to_le_bytes());
        data.extend_from_slice(&200u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&90u64.to_le_bytes());
        data.extend_from_slice(&180u64.to_le_bytes());
        data.extend_from_slice(&1_000u64.to_le_bytes());
        data.extend_from_slice(&[1u8; 32]); // pool
        data.extend_from_slice(&[3u8; 32]); // user
        data.extend_from_slice(&[7u8; 32]);
        data.extend_from_slice(&[8u8; 32]);
        data.extend_from_slice(&[9u8; 32]);
        data
    }

    fn withdraw_event_bytes() -> Vec<u8> {
        let mut data = Vec::from(WITHDRAW_EVENT_DISC);
        data.extend_from_slice(&1_777_777_778i64.to_le_bytes());
        data.extend_from_slice(&11u64.to_le_bytes());
        data.extend_from_slice(&101u64.to_le_bytes());
        data.extend_from_slice(&201u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&0u64.to_le_bytes());
        data.extend_from_slice(&91u64.to_le_bytes());
        data.extend_from_slice(&181u64.to_le_bytes());
        data.extend_from_slice(&1_001u64.to_le_bytes());
        data.extend_from_slice(&[21u8; 32]); // pool
        data.extend_from_slice(&[23u8; 32]); // user
        data.extend_from_slice(&[27u8; 32]);
        data.extend_from_slice(&[28u8; 32]);
        data.extend_from_slice(&[29u8; 32]);
        data
    }
}
