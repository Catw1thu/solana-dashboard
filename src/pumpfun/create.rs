use super::{
    events::{extract_create_cpi_events, extract_create_events},
    invocation::extract_invocations,
    model::{CreateAnalysis, CreateEvent, ParsedCreate, PumpfunInstruction, PumpfunInvocation},
};
use crate::{event_origin::EventOrigin, transaction_view::TransactionView};

pub fn extract_creates(view: &TransactionView) -> Vec<ParsedCreate> {
    analyze_creates(view).creates
}

pub fn analyze_creates(view: &TransactionView) -> CreateAnalysis {
    let mut pending_events = extract_create_events(&view.log_messages)
        .into_iter()
        .map(|event| (EventOrigin::Logs, event))
        .collect::<Vec<_>>();
    for event in extract_create_cpi_events(&view.inner_instruction_groups) {
        if !pending_events
            .iter()
            .any(|(_, existing)| existing == &event)
        {
            pending_events.push((EventOrigin::InnerCpi, event));
        }
    }
    let invocations = extract_invocations(view)
        .into_iter()
        .filter(|invocation| invocation.instruction.is_create())
        .collect::<Vec<_>>();

    let mut creates = Vec::new();
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
        creates.push(build_create(invocation, event_source, event));
    }

    CreateAnalysis {
        creates,
        unmatched_invocations,
        unmatched_events: pending_events.into_iter().map(|(_, event)| event).collect(),
    }
}

fn event_matches_instruction(instruction: &PumpfunInstruction, event: &CreateEvent) -> bool {
    let Some(accounts) = instruction.create_accounts() else {
        return false;
    };

    match instruction {
        PumpfunInstruction::Create(ix) => {
            event.mint == accounts.mint
                && event.bonding_curve == accounts.bonding_curve
                && event.user == accounts.user
                && event.creator == ix.creator
                && event.name == ix.name
                && event.symbol == ix.symbol
                && event.uri == ix.uri
        }
        PumpfunInstruction::CreateV2(ix) => {
            event.mint == accounts.mint
                && event.bonding_curve == accounts.bonding_curve
                && event.user == accounts.user
                && event.creator == ix.creator
                && event.name == ix.name
                && event.symbol == ix.symbol
                && event.uri == ix.uri
                && event.is_mayhem_mode == ix.is_mayhem_mode
                && ix
                    .is_cashback_enabled
                    .is_none_or(|enabled| event.is_cashback_enabled == enabled)
        }
        PumpfunInstruction::Buy(_)
        | PumpfunInstruction::Sell(_)
        | PumpfunInstruction::BuyExactSolIn(_)
        | PumpfunInstruction::Migrate(_) => false,
    }
}

fn build_create(
    invocation: PumpfunInvocation,
    event_source: EventOrigin,
    event: CreateEvent,
) -> ParsedCreate {
    ParsedCreate {
        source: invocation.source.clone(),
        event_source,
        mint: event.mint.clone(),
        bonding_curve: event.bonding_curve.clone(),
        user: event.user.clone(),
        creator: event.creator.clone(),
        name: event.name.clone(),
        symbol: event.symbol.clone(),
        uri: event.uri.clone(),
        timestamp: event.timestamp,
        virtual_token_reserves: event.virtual_token_reserves,
        virtual_sol_reserves: event.virtual_sol_reserves,
        real_token_reserves: event.real_token_reserves,
        token_total_supply: event.token_total_supply,
        token_program: event.token_program.clone(),
        is_mayhem_mode: event.is_mayhem_mode,
        is_cashback_enabled: event.is_cashback_enabled,
        instruction: invocation.instruction.clone(),
        event,
    }
}

#[cfg(test)]
mod tests {
    use super::{analyze_creates, extract_creates};
    use crate::{
        pumpfun::{
            PUMPFUN_PROGRAM_ID,
            discriminators::{CREATE_V2_EVENT_DISC, CREATE_V2_IX_DISC},
            events::extract_create_events,
            invocation::extract_invocations,
            model::{InvocationSource, PumpfunInstruction},
        },
        transaction_view::{
            InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
        },
    };
    use base64::{Engine as _, engine::general_purpose::STANDARD};
    use std::{fs, path::Path};

    fn load_fixture(file_name: &str) -> TransactionView {
        let path = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("samples")
            .join("tests")
            .join("views")
            .join(file_name);

        let content = fs::read_to_string(path).unwrap();
        serde_json::from_str(&content).unwrap()
    }

    #[test]
    fn direct_create_v2_merges_successfully() {
        let view = load_fixture(
            "409401849-47d52VVYGHEkKtC5hRddTUzK7RwL5Tt1cnwjMB2rzSau7td1Nu6CSJ2Da2k71Huxjt2JuLi5JU2QDa7PkV5dCii1.json",
        );

        assert_eq!(view.outer_instructions.len(), 1);
        assert_eq!(view.outer_instructions[0].program_id, PUMPFUN_PROGRAM_ID);

        let invocations = extract_invocations(&view)
            .into_iter()
            .filter(|invocation| invocation.instruction.is_create())
            .collect::<Vec<_>>();
        assert_eq!(invocations.len(), 1);

        let events = extract_create_events(&view.log_messages);
        assert_eq!(events.len(), 1);

        let analysis = analyze_creates(&view);
        assert_eq!(analysis.creates.len(), 1);
        assert!(analysis.unmatched_invocations.is_empty());
        assert!(analysis.unmatched_events.is_empty());

        let creates = extract_creates(&view);
        assert_eq!(creates.len(), 1);

        let create = &creates[0];
        assert!(matches!(
            create.source,
            InvocationSource::Outer { outer_index: 0 }
        ));
        assert!(matches!(
            create.instruction,
            PumpfunInstruction::CreateV2(_)
        ));

        assert_eq!(create.name, "DONT FEED GAY BUNDLE");
        assert_eq!(create.symbol, "IMAGINE");
        assert_eq!(create.uri, "https://mwgy.us/m/lvOc.json");
        assert_eq!(create.mint, "6KVh7yXJJWHtz3zbKZeVmBkVXjYfyFfah5oYLcTvqFu6");
        assert_eq!(
            create.bonding_curve,
            "CsegWwbZeArmR9emteKde1jC9PQjKBa6peTqTahURK9L"
        );
        assert_eq!(create.user, "38fqKr9R7duGnEBwLvNamVCkGpHqDhuoJZnVReyfZx4r");
        assert_eq!(
            create.token_program,
            "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
        );

        assert_eq!(create.mint, create.event.mint);
        assert_eq!(create.bonding_curve, create.event.bonding_curve);
        assert_eq!(create.user, create.event.user);
        assert_eq!(create.name, create.event.name);
        assert_eq!(create.symbol, create.event.symbol);
        assert_eq!(create.uri, create.event.uri);
    }

    #[test]
    fn create_can_fallback_to_inner_cpi_event() {
        let accounts = fake_accounts(16);
        let creator = accounts[5].clone();
        let view = TransactionView {
            slot: 0,
            signature: "sig".to_string(),
            tx_index: 0,
            is_vote: false,
            account_keys: vec![],
            loaded_writable_addresses: vec![],
            loaded_readonly_addresses: vec![],
            all_accounts: accounts.clone(),
            outer_instructions: vec![create_v2_outer_ix(accounts.clone(), &creator)],
            inner_instruction_groups: vec![InnerInstructionGroup {
                outer_instruction_index: 0,
                instructions: vec![create_event_inner_ix(accounts.clone(), &creator)],
            }],
            log_messages: vec![],
        };

        let creates = extract_creates(&view);
        assert_eq!(creates.len(), 1);

        let create = &creates[0];
        assert!(matches!(
            create.source,
            InvocationSource::Outer { outer_index: 0 }
        ));
        assert!(matches!(
            create.instruction,
            PumpfunInstruction::CreateV2(_)
        ));
        assert_eq!(create.name, "Inner Launch");
        assert_eq!(create.symbol, "INNER");
        assert_eq!(create.uri, "https://example.com/token.json");
        assert_eq!(create.mint, accounts[0]);
        assert_eq!(create.bonding_curve, accounts[2]);
        assert_eq!(create.user, accounts[5]);
        assert_eq!(create.creator, creator);
        assert_eq!(create.token_program, accounts[7]);
        assert!(create.is_mayhem_mode);
        assert!(!create.is_cashback_enabled);
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

    fn create_v2_outer_ix(accounts: Vec<String>, creator: &str) -> OuterInstructionView {
        let creator_bytes = bs58::decode(creator).into_vec().unwrap();

        let mut data = Vec::from(CREATE_V2_IX_DISC);
        push_string(&mut data, "Inner Launch");
        push_string(&mut data, "INNER");
        push_string(&mut data, "https://example.com/token.json");
        data.extend_from_slice(&creator_bytes);
        data.push(1);
        data.push(0);

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

    fn create_event_inner_ix(accounts: Vec<String>, creator: &str) -> InnerInstructionView {
        let mut data = vec![9, 8, 7, 6, 5, 4, 3, 2];
        data.extend_from_slice(&create_event_bytes(&accounts, creator));

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

    fn create_event_bytes(accounts: &[String], creator: &str) -> Vec<u8> {
        let mint = bs58::decode(&accounts[0]).into_vec().unwrap();
        let bonding_curve = bs58::decode(&accounts[2]).into_vec().unwrap();
        let user = bs58::decode(&accounts[5]).into_vec().unwrap();
        let creator_bytes = bs58::decode(creator).into_vec().unwrap();
        let token_program = bs58::decode(&accounts[7]).into_vec().unwrap();

        let mut data = Vec::from(CREATE_V2_EVENT_DISC);
        push_string(&mut data, "Inner Launch");
        push_string(&mut data, "INNER");
        push_string(&mut data, "https://example.com/token.json");
        data.extend_from_slice(&mint);
        data.extend_from_slice(&bonding_curve);
        data.extend_from_slice(&user);
        data.extend_from_slice(&creator_bytes);
        data.extend_from_slice(&1_777_777_777i64.to_le_bytes());
        data.extend_from_slice(&10u64.to_le_bytes());
        data.extend_from_slice(&11u64.to_le_bytes());
        data.extend_from_slice(&12u64.to_le_bytes());
        data.extend_from_slice(&13u64.to_le_bytes());
        data.extend_from_slice(&token_program);
        data.push(1);
        data.push(0);
        data
    }
}
