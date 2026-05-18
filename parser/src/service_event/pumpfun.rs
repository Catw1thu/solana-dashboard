use super::model::{
    SERVICE_EVENT_SCHEMA_VERSION, ServiceCommitmentLevel, ServiceEventChain, ServiceEventEnvelope,
    ServiceEventProtocol, ServiceEventRefs, ServiceEventType, ServiceInstructionPath,
    ServiceInstructionSource, build_service_event_id,
};
use crate::{
    pumpfun::model::{ParsedCreate, ParsedMigrate, ParsedTrade, PumpfunInstruction},
    transaction_view::TransactionView,
};
use base64::{Engine as _, engine::general_purpose::STANDARD};
use serde::Serialize;

#[derive(Debug, Serialize)]
struct PumpfunTradeInstructionArgs {
    amount: Option<String>,
    max_sol_cost: Option<String>,
    min_sol_output: Option<String>,
    spendable_sol_in: Option<String>,
    min_tokens_out: Option<String>,
}

#[derive(Debug, Serialize)]
struct PumpfunTradePayload {
    side: String,
    ix_name: String,
    mint: String,
    user: String,
    bonding_curve: String,
    associated_bonding_curve: String,
    creator: String,
    creator_vault: String,
    token_program: String,
    sol_amount: String,
    token_amount: String,
    fee: String,
    creator_fee: String,
    virtual_sol_reserves: String,
    virtual_token_reserves: String,
    real_sol_reserves: String,
    real_token_reserves: String,
    track_volume: bool,
    mayhem_mode: bool,
    cashback: String,
    instruction_args: PumpfunTradeInstructionArgs,
}

#[derive(Debug, Serialize)]
struct PumpfunCreatePayload {
    ix_name: String,
    mint: String,
    bonding_curve: String,
    user: String,
    creator: String,
    name: String,
    symbol: String,
    uri: String,
    token_program: String,
    virtual_token_reserves: String,
    virtual_sol_reserves: String,
    real_token_reserves: String,
    token_total_supply: String,
    is_mayhem_mode: bool,
    is_cashback_enabled: bool,
    mint_decimals: Option<u8>,
}

#[derive(Debug, Serialize)]
struct PumpfunMigratePayload {
    mint: String,
    user: String,
    bonding_curve: String,
    pool: String,
    mint_amount: String,
    sol_amount: String,
    pool_migration_fee: String,
    withdraw_authority: String,
    associated_bonding_curve: String,
    token_program: String,
    pump_amm: String,
    pool_authority: String,
    lp_mint: String,
}

pub fn build_pumpfun_trade_service_event(
    view: &TransactionView,
    trade: &ParsedTrade,
) -> ServiceEventEnvelope {
    let instruction_path = build_instruction_path(&trade.source);
    let refs = ServiceEventRefs {
        mint: Some(trade.mint.clone()),
        pool: None,
        bonding_curve: Some(trade.bonding_curve.clone()),
        user: Some(trade.user.clone()),
        creator: Some(trade.creator.clone()),
        base_mint: None,
        quote_mint: None,
        lp_mint: None,
    };
    let payload = build_payload(trade);
    let protocol = ServiceEventProtocol::Pumpfun;
    let event_type = ServiceEventType::Trade;

    ServiceEventEnvelope {
        schema_version: SERVICE_EVENT_SCHEMA_VERSION,
        event_id: build_service_event_id(
            protocol.clone(),
            event_type.clone(),
            &view.signature,
            &instruction_path,
        ),
        chain: ServiceEventChain::Solana,
        protocol,
        event_type,
        commitment: ServiceCommitmentLevel::Processed,
        slot: view.slot,
        tx_signature: view.signature.clone(),
        tx_index: view.tx_index,
        instruction_path,
        event_source: trade.event_source,
        event_unix_ts: trade.timestamp,
        refs,
        payload: serde_json::to_value(payload).expect("pumpfun trade payload must serialize"),
    }
}

pub fn build_pumpfun_create_service_event(
    view: &TransactionView,
    create: &ParsedCreate,
) -> ServiceEventEnvelope {
    let instruction_path = build_instruction_path(&create.source);
    let refs = ServiceEventRefs {
        mint: Some(create.mint.clone()),
        pool: None,
        bonding_curve: Some(create.bonding_curve.clone()),
        user: Some(create.user.clone()),
        creator: Some(create.creator.clone()),
        base_mint: None,
        quote_mint: None,
        lp_mint: None,
    };
    let payload = PumpfunCreatePayload {
        ix_name: create
            .instruction
            .ix_name()
            .expect("create instruction must have ix_name")
            .to_string(),
        mint: create.mint.clone(),
        bonding_curve: create.bonding_curve.clone(),
        user: create.user.clone(),
        creator: create.creator.clone(),
        name: create.name.clone(),
        symbol: create.symbol.clone(),
        uri: create.uri.clone(),
        token_program: create.token_program.clone(),
        virtual_token_reserves: create.virtual_token_reserves.to_string(),
        virtual_sol_reserves: create.virtual_sol_reserves.to_string(),
        real_token_reserves: create.real_token_reserves.to_string(),
        token_total_supply: create.token_total_supply.to_string(),
        is_mayhem_mode: create.is_mayhem_mode,
        is_cashback_enabled: create.is_cashback_enabled,
        mint_decimals: extract_mint_decimals(view, &create.mint),
    };
    let protocol = ServiceEventProtocol::Pumpfun;
    let event_type = ServiceEventType::Create;

    ServiceEventEnvelope {
        schema_version: SERVICE_EVENT_SCHEMA_VERSION,
        event_id: build_service_event_id(
            protocol.clone(),
            event_type.clone(),
            &view.signature,
            &instruction_path,
        ),
        chain: ServiceEventChain::Solana,
        protocol,
        event_type,
        commitment: ServiceCommitmentLevel::Processed,
        slot: view.slot,
        tx_signature: view.signature.clone(),
        tx_index: view.tx_index,
        instruction_path,
        event_source: create.event_source,
        event_unix_ts: create.timestamp,
        refs,
        payload: serde_json::to_value(payload).expect("pumpfun create payload must serialize"),
    }
}

pub fn build_pumpfun_migrate_service_event(
    view: &TransactionView,
    migration: &ParsedMigrate,
) -> ServiceEventEnvelope {
    let instruction_path = build_instruction_path(&migration.source);
    let refs = ServiceEventRefs {
        mint: Some(migration.mint.clone()),
        pool: Some(migration.pool.clone()),
        bonding_curve: Some(migration.bonding_curve.clone()),
        user: Some(migration.user.clone()),
        creator: None,
        base_mint: None,
        quote_mint: None,
        lp_mint: Some(migration.lp_mint.clone()),
    };
    let payload = PumpfunMigratePayload {
        mint: migration.mint.clone(),
        user: migration.user.clone(),
        bonding_curve: migration.bonding_curve.clone(),
        pool: migration.pool.clone(),
        mint_amount: migration.mint_amount.to_string(),
        sol_amount: migration.sol_amount.to_string(),
        pool_migration_fee: migration.pool_migration_fee.to_string(),
        withdraw_authority: migration.withdraw_authority.clone(),
        associated_bonding_curve: migration.associated_bonding_curve.clone(),
        token_program: migration.token_program.clone(),
        pump_amm: migration.pump_amm.clone(),
        pool_authority: migration.pool_authority.clone(),
        lp_mint: migration.lp_mint.clone(),
    };
    let protocol = ServiceEventProtocol::Pumpfun;
    let event_type = ServiceEventType::Migrate;

    ServiceEventEnvelope {
        schema_version: SERVICE_EVENT_SCHEMA_VERSION,
        event_id: build_service_event_id(
            protocol.clone(),
            event_type.clone(),
            &view.signature,
            &instruction_path,
        ),
        chain: ServiceEventChain::Solana,
        protocol,
        event_type,
        commitment: ServiceCommitmentLevel::Processed,
        slot: view.slot,
        tx_signature: view.signature.clone(),
        tx_index: view.tx_index,
        instruction_path,
        event_source: migration.event_source,
        event_unix_ts: migration.timestamp,
        refs,
        payload: serde_json::to_value(payload).expect("pumpfun migrate payload must serialize"),
    }
}

fn build_instruction_path(
    source: &crate::pumpfun::model::InvocationSource,
) -> ServiceInstructionPath {
    match source {
        crate::pumpfun::model::InvocationSource::Outer { outer_index } => ServiceInstructionPath {
            source: ServiceInstructionSource::Outer,
            outer_index: *outer_index,
            inner_index: None,
        },
        crate::pumpfun::model::InvocationSource::Inner {
            outer_index,
            inner_index,
        } => ServiceInstructionPath {
            source: ServiceInstructionSource::Inner,
            outer_index: *outer_index,
            inner_index: Some(*inner_index),
        },
    }
}

/// SPL Token `InitializeMint2` instruction type index.
const INITIALIZE_MINT2_IX_TYPE: u8 = 20;

const SPL_TOKEN_PROGRAM_ID: &str = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA";
const SPL_TOKEN_2022_PROGRAM_ID: &str = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb";

/// Extract `decimals` from the `InitializeMint2` inner instruction whose first
/// account matches `mint`.
///
/// `InitializeMint2` data layout:
///   [0]    – instruction type (20)
///   [1]    – decimals (u8)
///   [2..34] – mint_authority (Pubkey)
///   [34]   – freeze_authority option (0 or 1)
///   [35..67] – freeze_authority (Pubkey), present only if option == 1
fn extract_mint_decimals(view: &TransactionView, mint: &str) -> Option<u8> {
    for group in &view.inner_instruction_groups {
        for ix in &group.instructions {
            if ix.program_id != SPL_TOKEN_PROGRAM_ID && ix.program_id != SPL_TOKEN_2022_PROGRAM_ID {
                continue;
            }

            // Quick check via prefix to avoid full base64 decode
            if ix.data_prefix.first().copied() != Some(INITIALIZE_MINT2_IX_TYPE) {
                continue;
            }

            // The first account must be the mint
            if ix.account_pubkeys.first().map(|s| s.as_str()) != Some(mint) {
                continue;
            }

            let Ok(data) = STANDARD.decode(&ix.data_base64) else {
                continue;
            };

            if data.len() >= 2 && data[0] == INITIALIZE_MINT2_IX_TYPE {
                return Some(data[1]);
            }
        }
    }
    None
}

fn build_payload(trade: &ParsedTrade) -> PumpfunTradePayload {
    PumpfunTradePayload {
        side: match trade.side {
            crate::pumpfun::model::TradeSide::Buy => "buy".to_string(),
            crate::pumpfun::model::TradeSide::Sell => "sell".to_string(),
            crate::pumpfun::model::TradeSide::BuyExactSolIn => "buy_exact_sol_in".to_string(),
        },
        ix_name: trade.ix_name.clone(),
        mint: trade.mint.clone(),
        user: trade.user.clone(),
        bonding_curve: trade.bonding_curve.clone(),
        associated_bonding_curve: trade.associated_bonding_curve.clone(),
        creator: trade.creator.clone(),
        creator_vault: trade.creator_vault.clone(),
        token_program: trade.token_program.clone(),
        sol_amount: trade.sol_amount.to_string(),
        token_amount: trade.token_amount.to_string(),
        fee: trade.fee.to_string(),
        creator_fee: trade.creator_fee.to_string(),
        virtual_sol_reserves: trade.virtual_sol_reserves.to_string(),
        virtual_token_reserves: trade.virtual_token_reserves.to_string(),
        real_sol_reserves: trade.real_sol_reserves.to_string(),
        real_token_reserves: trade.real_token_reserves.to_string(),
        track_volume: trade.track_volume,
        mayhem_mode: trade.event.mayhem_mode,
        cashback: trade.cashback.to_string(),
        instruction_args: build_instruction_args(&trade.instruction),
    }
}

fn build_instruction_args(instruction: &PumpfunInstruction) -> PumpfunTradeInstructionArgs {
    match instruction {
        PumpfunInstruction::Buy(ix) => PumpfunTradeInstructionArgs {
            amount: Some(ix.amount.to_string()),
            max_sol_cost: Some(ix.max_sol_cost.to_string()),
            min_sol_output: None,
            spendable_sol_in: None,
            min_tokens_out: None,
        },
        PumpfunInstruction::Sell(ix) => PumpfunTradeInstructionArgs {
            amount: Some(ix.amount.to_string()),
            max_sol_cost: None,
            min_sol_output: Some(ix.min_sol_output.to_string()),
            spendable_sol_in: None,
            min_tokens_out: None,
        },
        PumpfunInstruction::BuyExactSolIn(ix) => PumpfunTradeInstructionArgs {
            amount: None,
            max_sol_cost: None,
            min_sol_output: None,
            spendable_sol_in: Some(ix.spendable_sol_in.to_string()),
            min_tokens_out: Some(ix.min_tokens_out.to_string()),
        },
        PumpfunInstruction::Create(_)
        | PumpfunInstruction::CreateV2(_)
        | PumpfunInstruction::Migrate(_) => PumpfunTradeInstructionArgs {
            amount: None,
            max_sol_cost: None,
            min_sol_output: None,
            spendable_sol_in: None,
            min_tokens_out: None,
        },
    }
}

#[cfg(test)]
mod tests {
    use super::{
        build_pumpfun_create_service_event, build_pumpfun_migrate_service_event,
        build_pumpfun_trade_service_event,
    };
    use crate::{
        pumpfun::{extract_creates, extract_migrations, extract_trades},
        transaction_view::TransactionView,
    };

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
    fn pumpfun_trade_service_event_is_stable() {
        let view = load_fixture(
            "408960897-5c6muSpp3Poda2NHHau5AdANHmy9p6sPZJqWCSfZrs3KCU7s1RYcR3rtUkysFopqPxVVMwCqD7kDxtf4N9VyNeqk.json",
        );
        let trade = extract_trades(&view).remove(0);
        let service_event = build_pumpfun_trade_service_event(&view, &trade);

        assert_eq!(service_event.schema_version, 1);
        assert_eq!(service_event.tx_signature, view.signature);
        assert_eq!(service_event.slot, view.slot);
        assert_eq!(
            service_event.refs.mint.as_deref(),
            Some(trade.mint.as_str())
        );
        assert_eq!(
            service_event.refs.bonding_curve.as_deref(),
            Some(trade.bonding_curve.as_str())
        );
        assert_eq!(
            service_event.refs.user.as_deref(),
            Some(trade.user.as_str())
        );
        assert!(service_event.event_id.contains(":pumpfun:trade:"));
    }

    #[test]
    fn pumpfun_create_service_event_is_stable() {
        let view = load_fixture(
            "409401849-47d52VVYGHEkKtC5hRddTUzK7RwL5Tt1cnwjMB2rzSau7td1Nu6CSJ2Da2k71Huxjt2JuLi5JU2QDa7PkV5dCii1.json",
        );
        let create = extract_creates(&view).remove(0);
        let service_event = build_pumpfun_create_service_event(&view, &create);

        assert_eq!(service_event.schema_version, 1);
        assert_eq!(service_event.tx_signature, view.signature);
        assert_eq!(
            service_event.refs.mint.as_deref(),
            Some(create.mint.as_str())
        );
        assert_eq!(
            service_event.refs.bonding_curve.as_deref(),
            Some(create.bonding_curve.as_str())
        );
        assert_eq!(
            service_event.refs.user.as_deref(),
            Some(create.user.as_str())
        );
        assert_eq!(
            service_event.refs.creator.as_deref(),
            Some(create.creator.as_str())
        );
        assert!(service_event.event_id.contains(":pumpfun:create:"));

        // Verify mint_decimals is extracted from InitializeMint2 inner instruction
        let mint_decimals = service_event.payload.get("mint_decimals").unwrap();
        assert_eq!(mint_decimals.as_u64(), Some(6));
    }

    #[test]
    fn pumpfun_migrate_service_event_is_stable() {
        let view = load_fixture(
            "409576108-3zCwTozsNVfMaSftorXKLbbdVAmNPaPy3oZXN5ch6eMBdYdKfoB9GAgsiwhAFq786wnYoP9Lv64XjC8LbaKnbijZ.json",
        );
        let migration = extract_migrations(&view).remove(0);
        let service_event = build_pumpfun_migrate_service_event(&view, &migration);

        assert_eq!(service_event.schema_version, 1);
        assert_eq!(service_event.tx_signature, view.signature);
        assert_eq!(
            service_event.refs.mint.as_deref(),
            Some(migration.mint.as_str())
        );
        assert_eq!(
            service_event.refs.bonding_curve.as_deref(),
            Some(migration.bonding_curve.as_str())
        );
        assert_eq!(
            service_event.refs.user.as_deref(),
            Some(migration.user.as_str())
        );
        assert_eq!(
            service_event.refs.pool.as_deref(),
            Some(migration.pool.as_str())
        );
        assert_eq!(
            service_event.refs.lp_mint.as_deref(),
            Some(migration.lp_mint.as_str())
        );
        assert!(service_event.event_id.contains(":pumpfun:migrate:"));
    }
}
