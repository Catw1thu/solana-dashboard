use crate::event_origin::EventOrigin;
use serde::Serialize;
use serde_json::Value;

pub const SERVICE_EVENT_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ServiceEventChain {
    Solana,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ServiceEventProtocol {
    Pumpfun,
    Pumpamm,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[allow(dead_code)]
#[serde(rename_all = "snake_case")]
pub enum ServiceEventType {
    Trade,
    Create,
    Migrate,
    Swap,
    CreatePool,
    Deposit,
    Withdraw,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[allow(dead_code)]
#[serde(rename_all = "snake_case")]
pub enum ServiceCommitmentLevel {
    Processed,
    Confirmed,
    Finalized,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ServiceInstructionSource {
    Outer,
    Inner,
}

#[derive(Debug, Clone, Serialize)]
pub struct ServiceInstructionPath {
    pub source: ServiceInstructionSource,
    pub outer_index: usize,
    pub inner_index: Option<usize>,
}

#[derive(Debug, Clone, Serialize)]
pub struct ServiceEventRefs {
    pub mint: Option<String>,
    pub pool: Option<String>,
    pub bonding_curve: Option<String>,
    pub user: Option<String>,
    pub creator: Option<String>,
    pub base_mint: Option<String>,
    pub quote_mint: Option<String>,
    pub lp_mint: Option<String>,
}

#[derive(Debug, Clone, Serialize)]
pub struct ServiceEventEnvelope {
    pub schema_version: u32,
    pub event_id: String,
    pub chain: ServiceEventChain,
    pub protocol: ServiceEventProtocol,
    pub event_type: ServiceEventType,
    pub commitment: ServiceCommitmentLevel,
    pub slot: u64,
    pub tx_signature: String,
    pub tx_index: u64,
    pub instruction_path: ServiceInstructionPath,
    pub event_source: EventOrigin,
    pub event_unix_ts: i64,
    pub refs: ServiceEventRefs,
    pub payload: Value,
}

pub fn build_service_event_id(
    protocol: ServiceEventProtocol,
    event_type: ServiceEventType,
    tx_signature: &str,
    instruction_path: &ServiceInstructionPath,
) -> String {
    let protocol = match protocol {
        ServiceEventProtocol::Pumpfun => "pumpfun",
        ServiceEventProtocol::Pumpamm => "pumpamm",
    };

    let event_type = match event_type {
        ServiceEventType::Trade => "trade",
        ServiceEventType::Create => "create",
        ServiceEventType::Migrate => "migrate",
        ServiceEventType::Swap => "swap",
        ServiceEventType::CreatePool => "create_pool",
        ServiceEventType::Deposit => "deposit",
        ServiceEventType::Withdraw => "withdraw",
    };

    match instruction_path.inner_index {
        Some(inner_index) => format!(
            "solana:{protocol}:{event_type}:{tx_signature}:inner:{}:{inner_index}",
            instruction_path.outer_index
        ),
        None => format!(
            "solana:{protocol}:{event_type}:{tx_signature}:outer:{}",
            instruction_path.outer_index
        ),
    }
}
