use crate::types::InstructionView;
use base64::{Engine, engine::general_purpose::STANDARD};

const BUY: [u8; 8] = [102, 6, 61, 18, 1, 218, 235, 234];
const SELL: [u8; 8] = [51, 230, 133, 164, 1, 127, 131, 173];
const BUY_EXACT_SOL_IN: [u8; 8] = [56, 252, 116, 8, 158, 223, 205, 95];

const CREATE: [u8; 8] = [24, 30, 200, 40, 5, 28, 7, 119];
const CREATE_V2: [u8; 8] = [214, 144, 76, 236, 95, 139, 49, 180];

#[derive(Debug)]
pub enum PumpfunOuterInstruction {
    Buy(PumpfunBuyIx),
    Sell(PumpfunSellIx),
    BuyExactSolIn(PumpfunBuyExactSolInIx),
    Create,
    CreateV2,
}

#[derive(Debug)]
pub struct PumpfunTradeAccounts {
    pub global: String,
    pub fee_recipient: String,
    pub mint: String,
    pub bonding_curve: String,
    pub associated_bonding_curve: String,
    pub associated_user: String,
    pub user: String,
    pub system_program: String,
    pub token_program: String,
    pub creator_vault: String,
    pub event_authority: String,
    pub program: String,
    pub global_volume_accumulator: Option<String>,
    pub user_volume_accumulator: Option<String>,
    pub fee_config: String,
    pub fee_program: String,
}

#[derive(Debug)]
pub struct PumpfunBuyIx {
    pub amount: u64,
    pub max_sol_cost: u64,
    pub track_volume: bool,
    pub accounts: PumpfunTradeAccounts,
}

#[derive(Debug)]
pub struct PumpfunSellIx {
    pub amount: u64,
    pub min_sol_output: u64,
    pub accounts: PumpfunTradeAccounts,
}

#[derive(Debug)]
pub struct PumpfunBuyExactSolInIx {
    pub spendable_sol_in: u64,
    pub min_token_out: u64,
    pub track_volume: bool,
    pub accounts: PumpfunTradeAccounts,
}

fn read_u64_le(data: &[u8], offset: usize) -> Option<u64> {
    let bytes = data.get(offset..offset + 8)?.try_into().ok()?;
    Some(u64::from_le_bytes(bytes))
}

fn read_bool_flag(data: &[u8], offset: usize) -> Option<bool> {
    Some(*data.get(offset)? != 0)
}

fn account_at(ix: &InstructionView, index: usize) -> Option<String> {
    ix.account_pubkeys.get(index).cloned()
}

fn parse_buy_accounts(ix: &InstructionView) -> Option<PumpfunTradeAccounts> {
    Some(PumpfunTradeAccounts {
        global: account_at(ix, 0)?,
        fee_recipient: account_at(ix, 1)?,
        mint: account_at(ix, 2)?,
        bonding_curve: account_at(ix, 3)?,
        associated_bonding_curve: account_at(ix, 4)?,
        associated_user: account_at(ix, 5)?,
        user: account_at(ix, 6)?,
        system_program: account_at(ix, 7)?,
        token_program: account_at(ix, 8)?,
        creator_vault: account_at(ix, 9)?,
        event_authority: account_at(ix, 10)?,
        program: account_at(ix, 11)?,
        global_volume_accumulator: account_at(ix, 12),
        user_volume_accumulator: account_at(ix, 13),
        fee_config: account_at(ix, 14)?,
        fee_program: account_at(ix, 15)?,
    })
}

fn parse_sell_accounts(ix: &InstructionView) -> Option<PumpfunTradeAccounts> {
    Some(PumpfunTradeAccounts {
        global: account_at(ix, 0)?,
        fee_recipient: account_at(ix, 1)?,
        mint: account_at(ix, 2)?,
        bonding_curve: account_at(ix, 3)?,
        associated_bonding_curve: account_at(ix, 4)?,
        associated_user: account_at(ix, 5)?,
        user: account_at(ix, 6)?,
        system_program: account_at(ix, 7)?,
        creator_vault: account_at(ix, 8)?,
        token_program: account_at(ix, 9)?,
        event_authority: account_at(ix, 10)?,
        program: account_at(ix, 11)?,
        global_volume_accumulator: None,
        user_volume_accumulator: None,
        fee_config: account_at(ix, 12)?,
        fee_program: account_at(ix, 13)?,
    })
}

// 过滤 program_id == PUMPFUN_PROGRAM_ID
// base64 解 data_base64
// 看前 8 字节 discriminator
// 按 IDL 顺序把 accounts 映射成字段
// 读取 args
pub fn parse_pumpfun_outer_instruction(ix: &InstructionView) -> Option<PumpfunOuterInstruction> {
    if ix.program_id != "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P" {
        return None;
    }

    let data = STANDARD.decode(&ix.data_base64).ok()?;
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;

    match disc {
        BUY => {
            let amount = read_u64_le(&data, 8)?;
            let max_sol_cost = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24)?;
            let accounts = parse_buy_accounts(ix)?;

            Some(PumpfunOuterInstruction::Buy(PumpfunBuyIx {
                amount,
                max_sol_cost,
                track_volume,
                accounts,
            }))
        }
        SELL => {
            let amount = read_u64_le(&data, 8)?;
            let min_sol_output = read_u64_le(&data, 16)?;
            let accounts = parse_sell_accounts(ix)?;

            Some(PumpfunOuterInstruction::Sell(PumpfunSellIx {
                amount,
                min_sol_output,
                accounts,
            }))
        }
        BUY_EXACT_SOL_IN => {
            let spendable_sol_in = read_u64_le(&data, 8)?;
            let min_token_out = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24)?;
            let accounts = parse_buy_accounts(ix)?;

            Some(PumpfunOuterInstruction::BuyExactSolIn(
                PumpfunBuyExactSolInIx {
                    spendable_sol_in,
                    min_token_out,
                    track_volume,
                    accounts,
                },
            ))
        }
        CREATE => Some(PumpfunOuterInstruction::Create),
        CREATE_V2 => Some(PumpfunOuterInstruction::CreateV2),
        _ => None,
    }
}
