use super::{
    events::extract_create_events,
    invocation::extract_invocations,
    model::{CreateAnalysis, CreateEvent, ParsedCreate, PumpfunInstruction, PumpfunInvocation},
};
use crate::transaction_view::TransactionView;

pub fn extract_creates(view: &TransactionView) -> Vec<ParsedCreate> {
    analyze_creates(view).creates
}

pub fn analyze_creates(view: &TransactionView) -> CreateAnalysis {
    let mut pending_events = extract_create_events(&view.log_messages);
    let invocations = extract_invocations(view)
        .into_iter()
        .filter(|invocation| invocation.instruction.is_create())
        .collect::<Vec<_>>();

    let mut creates = Vec::new();
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
        creates.push(build_create(invocation, event));
    }

    CreateAnalysis {
        creates,
        unmatched_invocations,
        unmatched_events: pending_events,
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
        | PumpfunInstruction::BuyExactSolIn(_) => false,
    }
}

fn build_create(invocation: PumpfunInvocation, event: CreateEvent) -> ParsedCreate {
    ParsedCreate {
        source: invocation.source.clone(),
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
            events::extract_create_events,
            invocation::extract_invocations,
            model::{InvocationSource, PumpfunInstruction},
        },
        transaction_view::TransactionView,
    };
    use std::{fs, path::Path};

    fn load_fixture(file_name: &str) -> TransactionView {
        let path = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("samples")
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
}
