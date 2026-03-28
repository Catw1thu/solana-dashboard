use super::{
    events::extract_migrate_events,
    invocation::extract_invocations,
    model::{MigrateAnalysis, MigrateEvent, ParsedMigrate, PumpfunInstruction, PumpfunInvocation},
};
use crate::transaction_view::TransactionView;

pub fn extract_migrations(view: &TransactionView) -> Vec<ParsedMigrate> {
    analyze_migrations(view).migrations
}

pub fn analyze_migrations(view: &TransactionView) -> MigrateAnalysis {
    let mut pending_events = extract_migrate_events(&view.log_messages);
    let invocations = extract_invocations(view)
        .into_iter()
        .filter(|invocation| invocation.instruction.is_migrate())
        .collect::<Vec<_>>();

    let mut migrations = Vec::new();
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
        migrations.push(build_migration(invocation, event));
    }

    MigrateAnalysis {
        migrations,
        unmatched_invocations,
        unmatched_events: pending_events,
    }
}

fn event_matches_instruction(instruction: &PumpfunInstruction, event: &MigrateEvent) -> bool {
    let Some(accounts) = instruction.migrate_accounts() else {
        return false;
    };

    event.mint == accounts.mint
        && event.user == accounts.user
        && event.bonding_curve == accounts.bonding_curve
        && event.pool == accounts.pool
}

fn build_migration(invocation: PumpfunInvocation, event: MigrateEvent) -> ParsedMigrate {
    let accounts = invocation
        .instruction
        .migrate_accounts()
        .expect("migrate instructions must always carry migrate accounts");

    ParsedMigrate {
        source: invocation.source.clone(),
        mint: event.mint.clone(),
        user: event.user.clone(),
        bonding_curve: event.bonding_curve.clone(),
        pool: event.pool.clone(),
        mint_amount: event.mint_amount,
        sol_amount: event.sol_amount,
        pool_migration_fee: event.pool_migration_fee,
        timestamp: event.timestamp,
        withdraw_authority: accounts.withdraw_authority.clone(),
        associated_bonding_curve: accounts.associated_bonding_curve.clone(),
        token_program: accounts.token_program.clone(),
        pump_amm: accounts.pump_amm.clone(),
        pool_authority: accounts.pool_authority.clone(),
        lp_mint: accounts.lp_mint.clone(),
        instruction: invocation.instruction.clone(),
        event,
    }
}

#[cfg(test)]
mod tests {
    use super::extract_migrations;
    use crate::pumpfun::{
        PUMPFUN_PROGRAM_ID,
        discriminators::{MIGRATE_EVENT_DISC, MIGRATE_IX_DISC},
        model::{InvocationSource, PumpfunInstruction},
    };
    use crate::transaction_view::{
        InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
    };
    use base64::{Engine as _, engine::general_purpose::STANDARD};

    #[test]
    fn pumpfun_migrate_from_inner_invocation_merges_successfully() {
        let accounts = fake_accounts(24);
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
                instructions: vec![migrate_inner_ix(accounts.clone())],
            }],
            log_messages: vec![format!(
                "Program data: {}",
                STANDARD.encode(migrate_event_bytes())
            )],
        };

        let migrations = extract_migrations(&view);
        assert_eq!(migrations.len(), 1);

        let migration = &migrations[0];
        assert!(matches!(
            migration.source,
            InvocationSource::Inner {
                outer_index: 0,
                inner_index: 0,
            }
        ));
        assert!(matches!(
            migration.instruction,
            PumpfunInstruction::Migrate(_)
        ));
        assert_eq!(migration.user, accounts[5]);
        assert_eq!(migration.mint, accounts[2]);
        assert_eq!(migration.bonding_curve, accounts[3]);
        assert_eq!(migration.pool, accounts[9]);
        assert_eq!(migration.withdraw_authority, accounts[1]);
        assert_eq!(migration.pump_amm, accounts[8]);
        assert_eq!(migration.lp_mint, accounts[15]);
        assert_eq!(migration.mint_amount, 123);
        assert_eq!(migration.sol_amount, 456);
        assert_eq!(migration.pool_migration_fee, 7);
        assert_eq!(migration.timestamp, 1_777_777_777);
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

    fn migrate_inner_ix(accounts: Vec<String>) -> InnerInstructionView {
        let data = Vec::from(MIGRATE_IX_DISC);

        InnerInstructionView {
            program_id_index: 0,
            program_id: PUMPFUN_PROGRAM_ID.to_string(),
            account_indices: (0..24).collect(),
            account_pubkeys: accounts,
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

    fn migrate_event_bytes() -> Vec<u8> {
        let mut data = Vec::from(MIGRATE_EVENT_DISC);
        data.extend_from_slice(&[6u8; 32]); // user -> accounts[5]
        data.extend_from_slice(&[3u8; 32]); // mint -> accounts[2]
        data.extend_from_slice(&123u64.to_le_bytes());
        data.extend_from_slice(&456u64.to_le_bytes());
        data.extend_from_slice(&7u64.to_le_bytes());
        data.extend_from_slice(&[4u8; 32]); // bonding_curve -> accounts[3]
        data.extend_from_slice(&1_777_777_777i64.to_le_bytes());
        data.extend_from_slice(&[10u8; 32]); // pool -> accounts[9]
        data
    }
}
