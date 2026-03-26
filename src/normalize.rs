use crate::types::{InnerInstructionGroupView, InnerInstructionView, InstructionView, RawTxView};
use base64::{engine::general_purpose::STANDARD, Engine as _};
use yellowstone_grpc_proto::{
    geyser::SubscribeUpdateTransaction,
    prelude::{CompiledInstruction, InnerInstruction, InnerInstructions},
};

pub fn normalize_tx(tx: &SubscribeUpdateTransaction) -> Option<RawTxView> {
    let info = tx.transaction.as_ref()?;
    let raw_tx = info.transaction.as_ref()?;
    let msg = raw_tx.message.as_ref()?;
    let meta = info.meta.as_ref()?;

    let signature = bs58::encode(&info.signature).into_string();

    let account_keys = msg
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

    let outer_instructions = msg
        .instructions
        .iter()
        .map(|ix| normalize_instruction(ix, &all_accounts))
        .collect::<Vec<_>>();

    let inner_instruction_groups = meta
        .inner_instructions
        .iter()
        .map(|group| normalize_inner_group(group, &all_accounts))
        .collect::<Vec<_>>();

    Some(RawTxView {
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

fn normalize_instruction(ix: &CompiledInstruction, all_accounts: &[String]) -> InstructionView {
    let account_indices = ix.accounts.iter().map(|&i| i as u32).collect::<Vec<_>>();
    let account_pubkeys = ix
        .accounts
        .iter()
        .map(|&i| lookup_account(all_accounts, i as usize))
        .collect::<Vec<_>>();

    InstructionView {
        program_id_index: ix.program_id_index,
        program_id: lookup_account(all_accounts, ix.program_id_index as usize),
        account_indices,
        account_pubkeys,
        data_len: ix.data.len(),
        data_prefix: ix.data.iter().take(16).copied().collect(),
        data_base64: STANDARD.encode(&ix.data),
    }
}

fn normalize_inner_instruction(
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

fn normalize_inner_group(
    group: &InnerInstructions,
    all_accounts: &[String],
) -> InnerInstructionGroupView {
    InnerInstructionGroupView {
        outer_instruction_index: group.index,
        instructions: group
            .instructions
            .iter()
            .map(|ix| normalize_inner_instruction(ix, all_accounts))
            .collect(),
    }
}
