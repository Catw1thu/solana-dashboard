use anyhow::{Context, Result};
use prost::Message;
use serde::Deserialize;

use crate::{
    event_origin::EventOrigin,
    proto::serviceevent::v1::{
        self as pb, Chain, CommitmentLevel, EventEnvelope, EventOrigin as PbOrigin, EventRefs,
        EventType, InstructionPath, InstructionSource, Protocol, event_envelope,
    },
};

use super::model::{
    ServiceCommitmentLevel, ServiceEventChain, ServiceEventEnvelope, ServiceEventProtocol,
    ServiceEventType, ServiceInstructionSource,
};

#[derive(Debug, Deserialize)]
struct PumpfunTradeInstructionArgs {
    amount: Option<String>,
    max_sol_cost: Option<String>,
    min_sol_output: Option<String>,
    spendable_sol_in: Option<String>,
    min_tokens_out: Option<String>,
}

#[derive(Debug, Deserialize)]
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

#[derive(Debug, Deserialize)]
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

#[derive(Debug, Deserialize)]
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

#[derive(Debug, Deserialize)]
struct PumpAmmSwapInstructionArgs {
    base_amount_in: Option<String>,
    min_quote_amount_out: Option<String>,
    base_amount_out: Option<String>,
    max_quote_amount_in: Option<String>,
    spendable_quote_in: Option<String>,
    min_base_amount_out: Option<String>,
}

#[derive(Debug, Deserialize)]
struct PumpAmmSwapPayload {
    side: String,
    ix_name: String,
    pool: String,
    user: String,
    base_mint: String,
    quote_mint: String,
    coin_creator: String,
    base_amount_in: Option<String>,
    base_amount_out: Option<String>,
    quote_amount_in: Option<String>,
    quote_amount_out: Option<String>,
    lp_fee: String,
    protocol_fee: String,
    coin_creator_fee: String,
    cashback: String,
    pool_base_token_reserves: String,
    pool_quote_token_reserves: String,
    instruction_args: PumpAmmSwapInstructionArgs,
}

#[derive(Debug, Deserialize)]
struct PumpAmmCreatePoolInstructionArgs {
    index: u16,
    coin_creator: String,
    is_mayhem_mode: bool,
    is_cashback_coin: Option<bool>,
}

#[derive(Debug, Deserialize)]
struct PumpAmmCreatePoolPayload {
    pool: String,
    creator: String,
    base_mint: String,
    quote_mint: String,
    lp_mint: String,
    base_amount_in: String,
    quote_amount_in: String,
    initial_liquidity: String,
    coin_creator: String,
    is_mayhem_mode: bool,
    instruction_args: PumpAmmCreatePoolInstructionArgs,
    base_mint_decimals: u8,
    quote_mint_decimals: u8,
}

#[derive(Debug, Deserialize)]
struct PumpAmmLiquidityInstructionArgs {
    lp_token_amount_in: Option<String>,
    lp_token_amount_out: Option<String>,
    max_base_amount_in: Option<String>,
    max_quote_amount_in: Option<String>,
    min_base_amount_out: Option<String>,
    min_quote_amount_out: Option<String>,
}

#[derive(Debug, Deserialize)]
struct PumpAmmLiquidityPayload {
    action: String,
    pool: String,
    user: String,
    base_mint: String,
    quote_mint: String,
    lp_mint: String,
    lp_token_amount_in: Option<String>,
    lp_token_amount_out: Option<String>,
    base_amount_in: Option<String>,
    quote_amount_in: Option<String>,
    base_amount_out: Option<String>,
    quote_amount_out: Option<String>,
    lp_mint_supply: String,
    instruction_args: PumpAmmLiquidityInstructionArgs,
}

pub fn encode_event(event: &ServiceEventEnvelope) -> Result<(String, Vec<u8>)> {
    let subject = build_subject(event);
    let proto_event = build_proto_event(event)?;
    let bytes = proto_event.encode_to_vec();
    Ok((subject, bytes))
}

fn build_proto_event(event: &ServiceEventEnvelope) -> Result<EventEnvelope> {
    Ok(EventEnvelope {
        schema_version: event.schema_version,
        event_id: event.event_id.clone(),
        chain: map_chain(&event.chain) as i32,
        protocol: map_protocol(&event.protocol) as i32,
        event_type: map_event_type(&event.event_type) as i32,
        commitment: map_commitment(&event.commitment) as i32,
        slot: event.slot,
        tx_signature: event.tx_signature.clone(),
        tx_index: event.tx_index,
        instruction_path: Some(InstructionPath {
            source: map_instruction_source(&event.instruction_path.source) as i32,
            outer_index: event.instruction_path.outer_index as u32,
            inner_index: event.instruction_path.inner_index.map(|index| index as u32),
        }),
        event_source: map_event_origin(event.event_source) as i32,
        event_unix_ts: event.event_unix_ts,
        refs: Some(EventRefs {
            mint: event.refs.mint.clone(),
            pool: event.refs.pool.clone(),
            bonding_curve: event.refs.bonding_curve.clone(),
            user: event.refs.user.clone(),
            creator: event.refs.creator.clone(),
            base_mint: event.refs.base_mint.clone(),
            quote_mint: event.refs.quote_mint.clone(),
            lp_mint: event.refs.lp_mint.clone(),
        }),
        payload: Some(build_payload(event)?),
    })
}

fn build_payload(event: &ServiceEventEnvelope) -> Result<event_envelope::Payload> {
    match (&event.protocol, &event.event_type) {
        (ServiceEventProtocol::Pumpfun, ServiceEventType::Trade) => {
            let payload: PumpfunTradePayload = serde_json::from_value(event.payload.clone())
                .context("decode pumpfun trade payload from json")?;
            Ok(event_envelope::Payload::PumpfunTrade(
                pb::PumpfunTradePayload {
                    side: payload.side,
                    ix_name: payload.ix_name,
                    mint: payload.mint,
                    user: payload.user,
                    bonding_curve: payload.bonding_curve,
                    associated_bonding_curve: payload.associated_bonding_curve,
                    creator: payload.creator,
                    creator_vault: payload.creator_vault,
                    token_program: payload.token_program,
                    sol_amount: payload.sol_amount,
                    token_amount: payload.token_amount,
                    fee: payload.fee,
                    creator_fee: payload.creator_fee,
                    virtual_sol_reserves: payload.virtual_sol_reserves,
                    virtual_token_reserves: payload.virtual_token_reserves,
                    real_sol_reserves: payload.real_sol_reserves,
                    real_token_reserves: payload.real_token_reserves,
                    track_volume: payload.track_volume,
                    mayhem_mode: payload.mayhem_mode,
                    cashback: payload.cashback,
                    instruction_args: Some(pb::PumpfunTradeInstructionArgs {
                        amount: payload.instruction_args.amount,
                        max_sol_cost: payload.instruction_args.max_sol_cost,
                        min_sol_output: payload.instruction_args.min_sol_output,
                        spendable_sol_in: payload.instruction_args.spendable_sol_in,
                        min_tokens_out: payload.instruction_args.min_tokens_out,
                    }),
                },
            ))
        }
        (ServiceEventProtocol::Pumpfun, ServiceEventType::Create) => {
            let payload: PumpfunCreatePayload = serde_json::from_value(event.payload.clone())
                .context("decode pumpfun create payload from json")?;
            Ok(event_envelope::Payload::PumpfunCreate(
                pb::PumpfunCreatePayload {
                    ix_name: payload.ix_name,
                    mint: payload.mint,
                    bonding_curve: payload.bonding_curve,
                    user: payload.user,
                    creator: payload.creator,
                    name: payload.name,
                    symbol: payload.symbol,
                    uri: payload.uri,
                    token_program: payload.token_program,
                    virtual_token_reserves: payload.virtual_token_reserves,
                    virtual_sol_reserves: payload.virtual_sol_reserves,
                    real_token_reserves: payload.real_token_reserves,
                    token_total_supply: payload.token_total_supply,
                    is_mayhem_mode: payload.is_mayhem_mode,
                    is_cashback_enabled: payload.is_cashback_enabled,
                    mint_decimals: payload.mint_decimals.map(u32::from),
                },
            ))
        }
        (ServiceEventProtocol::Pumpfun, ServiceEventType::Migrate) => {
            let payload: PumpfunMigratePayload = serde_json::from_value(event.payload.clone())
                .context("decode pumpfun migrate payload from json")?;
            Ok(event_envelope::Payload::PumpfunMigrate(
                pb::PumpfunMigratePayload {
                    mint: payload.mint,
                    user: payload.user,
                    bonding_curve: payload.bonding_curve,
                    pool: payload.pool,
                    mint_amount: payload.mint_amount,
                    sol_amount: payload.sol_amount,
                    pool_migration_fee: payload.pool_migration_fee,
                    withdraw_authority: payload.withdraw_authority,
                    associated_bonding_curve: payload.associated_bonding_curve,
                    token_program: payload.token_program,
                    pump_amm: payload.pump_amm,
                    pool_authority: payload.pool_authority,
                    lp_mint: payload.lp_mint,
                },
            ))
        }
        (ServiceEventProtocol::Pumpamm, ServiceEventType::Swap) => {
            let payload: PumpAmmSwapPayload = serde_json::from_value(event.payload.clone())
                .context("decode pumpamm swap payload from json")?;
            Ok(event_envelope::Payload::PumpammSwap(
                pb::PumpAmmSwapPayload {
                    side: payload.side,
                    ix_name: payload.ix_name,
                    pool: payload.pool,
                    user: payload.user,
                    base_mint: payload.base_mint,
                    quote_mint: payload.quote_mint,
                    coin_creator: payload.coin_creator,
                    base_amount_in: payload.base_amount_in,
                    base_amount_out: payload.base_amount_out,
                    quote_amount_in: payload.quote_amount_in,
                    quote_amount_out: payload.quote_amount_out,
                    lp_fee: payload.lp_fee,
                    protocol_fee: payload.protocol_fee,
                    coin_creator_fee: payload.coin_creator_fee,
                    cashback: payload.cashback,
                    pool_base_token_reserves: payload.pool_base_token_reserves,
                    pool_quote_token_reserves: payload.pool_quote_token_reserves,
                    instruction_args: Some(pb::PumpAmmSwapInstructionArgs {
                        base_amount_in: payload.instruction_args.base_amount_in,
                        min_quote_amount_out: payload.instruction_args.min_quote_amount_out,
                        base_amount_out: payload.instruction_args.base_amount_out,
                        max_quote_amount_in: payload.instruction_args.max_quote_amount_in,
                        spendable_quote_in: payload.instruction_args.spendable_quote_in,
                        min_base_amount_out: payload.instruction_args.min_base_amount_out,
                    }),
                },
            ))
        }
        (ServiceEventProtocol::Pumpamm, ServiceEventType::CreatePool) => {
            let payload: PumpAmmCreatePoolPayload =
                serde_json::from_value(event.payload.clone())
                    .context("decode pumpamm create_pool payload from json")?;
            Ok(event_envelope::Payload::PumpammCreatePool(
                pb::PumpAmmCreatePoolPayload {
                    pool: payload.pool,
                    creator: payload.creator,
                    base_mint: payload.base_mint,
                    quote_mint: payload.quote_mint,
                    lp_mint: payload.lp_mint,
                    base_amount_in: payload.base_amount_in,
                    quote_amount_in: payload.quote_amount_in,
                    initial_liquidity: payload.initial_liquidity,
                    coin_creator: payload.coin_creator,
                    is_mayhem_mode: payload.is_mayhem_mode,
                    base_mint_decimals: payload.base_mint_decimals as u32,
                    quote_mint_decimals: payload.quote_mint_decimals as u32,
                    instruction_args: Some(pb::PumpAmmCreatePoolInstructionArgs {
                        index: payload.instruction_args.index as u32,
                        coin_creator: payload.instruction_args.coin_creator,
                        is_mayhem_mode: payload.instruction_args.is_mayhem_mode,
                        is_cashback_coin: payload.instruction_args.is_cashback_coin,
                    }),
                },
            ))
        }
        (ServiceEventProtocol::Pumpamm, ServiceEventType::Deposit) => {
            let payload: PumpAmmLiquidityPayload = serde_json::from_value(event.payload.clone())
                .context("decode pumpamm deposit payload from json")?;
            Ok(event_envelope::Payload::PumpammDeposit(
                build_liquidity_payload(payload),
            ))
        }
        (ServiceEventProtocol::Pumpamm, ServiceEventType::Withdraw) => {
            let payload: PumpAmmLiquidityPayload = serde_json::from_value(event.payload.clone())
                .context("decode pumpamm withdraw payload from json")?;
            Ok(event_envelope::Payload::PumpammWithdraw(
                build_liquidity_payload(payload),
            ))
        }
        _ => anyhow::bail!(
            "unsupported service event payload for {:?}:{:?}",
            event.protocol,
            event.event_type
        ),
    }
}

fn build_liquidity_payload(payload: PumpAmmLiquidityPayload) -> pb::PumpAmmLiquidityPayload {
    pb::PumpAmmLiquidityPayload {
        action: payload.action,
        pool: payload.pool,
        user: payload.user,
        base_mint: payload.base_mint,
        quote_mint: payload.quote_mint,
        lp_mint: payload.lp_mint,
        lp_token_amount_in: payload.lp_token_amount_in,
        lp_token_amount_out: payload.lp_token_amount_out,
        base_amount_in: payload.base_amount_in,
        quote_amount_in: payload.quote_amount_in,
        base_amount_out: payload.base_amount_out,
        quote_amount_out: payload.quote_amount_out,
        lp_mint_supply: payload.lp_mint_supply,
        instruction_args: Some(pb::PumpAmmLiquidityInstructionArgs {
            lp_token_amount_in: payload.instruction_args.lp_token_amount_in,
            lp_token_amount_out: payload.instruction_args.lp_token_amount_out,
            max_base_amount_in: payload.instruction_args.max_base_amount_in,
            max_quote_amount_in: payload.instruction_args.max_quote_amount_in,
            min_base_amount_out: payload.instruction_args.min_base_amount_out,
            min_quote_amount_out: payload.instruction_args.min_quote_amount_out,
        }),
    }
}

fn build_subject(event: &ServiceEventEnvelope) -> String {
    format!(
        "solana.tracked.{}.{}",
        protocol_slug(&event.protocol),
        event_type_slug(&event.event_type)
    )
}

fn protocol_slug(protocol: &ServiceEventProtocol) -> &'static str {
    match protocol {
        ServiceEventProtocol::Pumpfun => "pumpfun",
        ServiceEventProtocol::Pumpamm => "pumpamm",
    }
}

fn event_type_slug(event_type: &ServiceEventType) -> &'static str {
    match event_type {
        ServiceEventType::Trade => "trade",
        ServiceEventType::Create => "create",
        ServiceEventType::Migrate => "migrate",
        ServiceEventType::Swap => "swap",
        ServiceEventType::CreatePool => "create_pool",
        ServiceEventType::Deposit => "deposit",
        ServiceEventType::Withdraw => "withdraw",
    }
}

fn map_chain(chain: &ServiceEventChain) -> Chain {
    match chain {
        ServiceEventChain::Solana => Chain::Solana,
    }
}

fn map_protocol(protocol: &ServiceEventProtocol) -> Protocol {
    match protocol {
        ServiceEventProtocol::Pumpfun => Protocol::Pumpfun,
        ServiceEventProtocol::Pumpamm => Protocol::Pumpamm,
    }
}

fn map_event_type(event_type: &ServiceEventType) -> EventType {
    match event_type {
        ServiceEventType::Trade => EventType::Trade,
        ServiceEventType::Create => EventType::Create,
        ServiceEventType::Migrate => EventType::Migrate,
        ServiceEventType::Swap => EventType::Swap,
        ServiceEventType::CreatePool => EventType::CreatePool,
        ServiceEventType::Deposit => EventType::Deposit,
        ServiceEventType::Withdraw => EventType::Withdraw,
    }
}

fn map_commitment(commitment: &ServiceCommitmentLevel) -> CommitmentLevel {
    match commitment {
        ServiceCommitmentLevel::Processed => CommitmentLevel::Processed,
        ServiceCommitmentLevel::Confirmed => CommitmentLevel::Confirmed,
        ServiceCommitmentLevel::Finalized => CommitmentLevel::Finalized,
    }
}

fn map_instruction_source(source: &ServiceInstructionSource) -> InstructionSource {
    match source {
        ServiceInstructionSource::Outer => InstructionSource::Outer,
        ServiceInstructionSource::Inner => InstructionSource::Inner,
    }
}

fn map_event_origin(origin: EventOrigin) -> PbOrigin {
    match origin {
        EventOrigin::Logs => PbOrigin::Logs,
        EventOrigin::InnerCpi => PbOrigin::InnerCpi,
    }
}

#[cfg(test)]
mod tests {
    use prost::Message;
    use serde_json::json;

    use crate::{
        event_origin::EventOrigin,
        proto::serviceevent::v1::{
            self as pb, Chain, CommitmentLevel, EventType, InstructionSource, Protocol,
            event_envelope,
        },
        service_event::model::{
            SERVICE_EVENT_SCHEMA_VERSION, ServiceCommitmentLevel, ServiceEventChain,
            ServiceEventEnvelope, ServiceEventProtocol, ServiceEventRefs, ServiceEventType,
            ServiceInstructionPath, ServiceInstructionSource,
        },
    };

    use super::encode_event;

    #[test]
    fn encodes_pumpfun_trade_event_to_protobuf() {
        let event = ServiceEventEnvelope {
            schema_version: SERVICE_EVENT_SCHEMA_VERSION,
            event_id: "solana:pumpfun:trade:testsig:outer:3".to_string(),
            chain: ServiceEventChain::Solana,
            protocol: ServiceEventProtocol::Pumpfun,
            event_type: ServiceEventType::Trade,
            commitment: ServiceCommitmentLevel::Processed,
            slot: 42,
            tx_signature: "testsig".to_string(),
            tx_index: 1,
            instruction_path: ServiceInstructionPath {
                source: ServiceInstructionSource::Outer,
                outer_index: 3,
                inner_index: None,
            },
            event_source: EventOrigin::Logs,
            event_unix_ts: 1_700_000_000,
            refs: ServiceEventRefs {
                mint: Some("mint_1".to_string()),
                pool: None,
                bonding_curve: Some("curve_1".to_string()),
                user: Some("user_1".to_string()),
                creator: Some("creator_1".to_string()),
                base_mint: None,
                quote_mint: None,
                lp_mint: None,
            },
            payload: json!({
                "side": "buy",
                "ix_name": "buy",
                "mint": "mint_1",
                "user": "user_1",
                "bonding_curve": "curve_1",
                "associated_bonding_curve": "assoc_curve_1",
                "creator": "creator_1",
                "creator_vault": "vault_1",
                "token_program": "token_program_1",
                "sol_amount": "100",
                "token_amount": "200",
                "fee": "1",
                "creator_fee": "2",
                "virtual_sol_reserves": "300",
                "virtual_token_reserves": "400",
                "real_sol_reserves": "500",
                "real_token_reserves": "600",
                "track_volume": true,
                "mayhem_mode": false,
                "cashback": "0",
                "instruction_args": {
                    "amount": "1000",
                    "max_sol_cost": "2000",
                    "min_sol_output": null,
                    "spendable_sol_in": null,
                    "min_tokens_out": null
                }
            }),
        };

        let (subject, bytes) = encode_event(&event).expect("encode service event");

        assert_eq!(subject, "solana.tracked.pumpfun.trade");

        let decoded = pb::EventEnvelope::decode(bytes.as_slice()).expect("decode protobuf");
        assert_eq!(decoded.schema_version, SERVICE_EVENT_SCHEMA_VERSION);
        assert_eq!(decoded.event_id, event.event_id);
        assert_eq!(decoded.chain, Chain::Solana as i32);
        assert_eq!(decoded.protocol, Protocol::Pumpfun as i32);
        assert_eq!(decoded.event_type, EventType::Trade as i32);
        assert_eq!(decoded.commitment, CommitmentLevel::Processed as i32);

        let path = decoded.instruction_path.expect("instruction path");
        assert_eq!(path.source, InstructionSource::Outer as i32);
        assert_eq!(path.outer_index, 3);
        assert_eq!(path.inner_index, None);

        match decoded.payload.expect("payload") {
            event_envelope::Payload::PumpfunTrade(payload) => {
                assert_eq!(payload.mint, "mint_1");
                assert_eq!(payload.side, "buy");
                assert_eq!(payload.token_amount, "200");
                let args = payload.instruction_args.expect("instruction args");
                assert_eq!(args.amount.as_deref(), Some("1000"));
            }
            other => panic!("unexpected payload: {other:?}"),
        }
    }
}
