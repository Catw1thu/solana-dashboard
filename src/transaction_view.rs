use base64::{Engine as _, engine::general_purpose::STANDARD};
use serde::{Deserialize, Serialize};
use yellowstone_grpc_proto::{
    geyser::SubscribeUpdateTransaction,
    prelude::{CompiledInstruction, InnerInstruction, InnerInstructions},
};

#[derive(Debug, Serialize, Deserialize)]
pub struct TransactionView {
    pub slot: u64,
    pub signature: String,
    pub tx_index: u64,
    pub is_vote: bool,
    pub account_keys: Vec<String>,
    pub loaded_writable_addresses: Vec<String>,
    pub loaded_readonly_addresses: Vec<String>,
    pub all_accounts: Vec<String>,
    pub outer_instructions: Vec<OuterInstructionView>,
    pub inner_instruction_groups: Vec<InnerInstructionGroup>,
    pub log_messages: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OuterInstructionView {
    pub program_id_index: u32,
    pub program_id: String,
    pub account_indices: Vec<u32>,
    pub account_pubkeys: Vec<String>,
    pub data_len: usize,
    pub data_prefix: Vec<u8>,
    pub data_base64: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct InnerInstructionGroup {
    pub outer_instruction_index: u32,
    pub instructions: Vec<InnerInstructionView>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct InnerInstructionView {
    pub program_id_index: u32,
    pub program_id: String,
    pub account_indices: Vec<u32>,
    pub account_pubkeys: Vec<String>,
    pub data_len: usize,
    pub data_prefix: Vec<u8>,
    pub data_base64: String,
    pub stack_height: Option<u32>,
}

pub fn build_transaction_view(tx: &SubscribeUpdateTransaction) -> Option<TransactionView> {
    let info = tx.transaction.as_ref()?;
    let raw_tx = info.transaction.as_ref()?;
    let message = raw_tx.message.as_ref()?;
    let meta = info.meta.as_ref()?;

    let signature = bs58::encode(&info.signature).into_string();

    let account_keys = message
        .account_keys
        .iter()
        .filter_map(|bytes| bytes_to_address(bytes))
        .collect::<Vec<_>>();

    let loaded_writable_addresses = meta
        .loaded_writable_addresses
        .iter()
        .filter_map(|bytes| bytes_to_address(bytes))
        .collect::<Vec<_>>();

    let loaded_readonly_addresses = meta
        .loaded_readonly_addresses
        .iter()
        .filter_map(|bytes| bytes_to_address(bytes))
        .collect::<Vec<_>>();

    let mut all_accounts = account_keys.clone();
    all_accounts.extend(loaded_writable_addresses.iter().cloned());
    all_accounts.extend(loaded_readonly_addresses.iter().cloned());

    let outer_instructions = message
        .instructions
        .iter()
        .map(|ix| build_outer_instruction_view(ix, &all_accounts))
        .collect::<Vec<_>>();

    let inner_instruction_groups = meta
        .inner_instructions
        .iter()
        .map(|group| build_inner_instruction_group(group, &all_accounts))
        .collect::<Vec<_>>();

    Some(TransactionView {
        slot: tx.slot,
        signature,
        tx_index: info.index,
        is_vote: info.is_vote,
        account_keys,
        loaded_writable_addresses,
        loaded_readonly_addresses,
        all_accounts,
        outer_instructions,
        inner_instruction_groups,
        log_messages: meta.log_messages.clone(),
    })
}

fn bytes_to_address(bytes: &[u8]) -> Option<String> {
    if bytes.len() != 32 {
        return None;
    }

    Some(bs58::encode(bytes).into_string())
}

fn lookup_account(all_accounts: &[String], index: usize) -> String {
    all_accounts
        .get(index)
        .cloned()
        .unwrap_or_else(|| format!("<missing:{}>", index))
}

fn build_outer_instruction_view(
    ix: &CompiledInstruction,
    all_accounts: &[String],
) -> OuterInstructionView {
    let account_indices = ix.accounts.iter().map(|&i| i as u32).collect::<Vec<_>>();
    let account_pubkeys = ix
        .accounts
        .iter()
        .map(|&i| lookup_account(all_accounts, i as usize))
        .collect::<Vec<_>>();

    OuterInstructionView {
        program_id_index: ix.program_id_index,
        program_id: lookup_account(all_accounts, ix.program_id_index as usize),
        account_indices,
        account_pubkeys,
        data_len: ix.data.len(),
        data_prefix: ix.data.iter().take(16).copied().collect(),
        data_base64: STANDARD.encode(&ix.data),
    }
}

fn build_inner_instruction_view(
    ix: &InnerInstruction,
    all_accounts: &[String],
) -> InnerInstructionView {
    let account_indices = ix.accounts.iter().map(|&i| i as u32).collect::<Vec<_>>();
    let account_pubkeys = ix
        .accounts
        .iter()
        .map(|&i| lookup_account(all_accounts, i as usize))
        .collect::<Vec<_>>();

    InnerInstructionView {
        program_id_index: ix.program_id_index,
        program_id: lookup_account(all_accounts, ix.program_id_index as usize),
        account_indices,
        account_pubkeys,
        data_len: ix.data.len(),
        data_prefix: ix.data.iter().take(16).copied().collect(),
        data_base64: STANDARD.encode(&ix.data),
        stack_height: ix.stack_height,
    }
}

fn build_inner_instruction_group(
    group: &InnerInstructions,
    all_accounts: &[String],
) -> InnerInstructionGroup {
    InnerInstructionGroup {
        outer_instruction_index: group.index,
        instructions: group
            .instructions
            .iter()
            .map(|ix| build_inner_instruction_view(ix, all_accounts))
            .collect(),
    }
}
