use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct RawTxView {
    pub slot: u64,
    pub signature: String,
    pub tx_index: u64,
    pub is_vote: bool,

    pub account_keys: Vec<String>,
    pub loaded_writable_addresses: Vec<String>,
    pub loaded_readonly_addresses: Vec<String>,
    pub all_accounts: Vec<String>,

    pub outer_instructions: Vec<InstructionView>,
    pub inner_instruction_groups: Vec<InnerInstructionGroupView>,
    pub log_messages: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct InstructionView {
    pub program_id_index: u32,
    pub program_id: String,

    pub account_indices: Vec<u32>,
    pub account_pubkeys: Vec<String>,

    pub data_len: usize,
    pub data_prefix: Vec<u8>,
    pub data_base64: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct InnerInstructionGroupView {
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
