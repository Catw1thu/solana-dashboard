use super::{
    constants::PROGRAM_ID,
    discriminators::{
        BUY_EXACT_SOL_IN_IX_DISC, BUY_IX_DISC, CREATE_IX_DISC, CREATE_V2_IX_DISC, SELL_IX_DISC,
    },
    types::{BuyExactSolInIx, BuyIx, OuterInstruction, SellIx, TradeAccounts},
};
use crate::types::InstructionView;
use base64::{Engine as _, engine::general_purpose::STANDARD};

fn read_u64_le(data: &[u8], offset: usize) -> Option<u64> {
    let bytes: [u8; 8] = data.get(offset..offset + 8)?.try_into().ok()?;
    Some(u64::from_le_bytes(bytes))
}

fn read_bool_flag(data: &[u8], offset: usize) -> Option<bool> {
    let byte = data.get(offset);
    match byte {
        Some(0) => Some(false),
        Some(1) => Some(true),
        _ => None,
    }
}

fn account_at(ix: &InstructionView, index: usize) -> Option<String> {
    ix.account_pubkeys.get(index).cloned()
}

fn parse_buy_accounts(ix: &InstructionView) -> Option<TradeAccounts> {
    Some(TradeAccounts {
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

fn parse_sell_accounts(ix: &InstructionView) -> Option<TradeAccounts> {
    Some(TradeAccounts {
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

pub fn parse_outer_instruction(ix: &InstructionView) -> Option<OuterInstruction> {
    if ix.program_id != PROGRAM_ID {
        return None;
    }

    let data = STANDARD.decode(&ix.data_base64).ok()?;
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;

    match disc {
        BUY_IX_DISC => {
            let amount = read_u64_le(&data, 8)?;
            let max_sol_cost = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24);
            let accounts = parse_buy_accounts(ix)?;

            Some(OuterInstruction::Buy(BuyIx {
                amount,
                max_sol_cost,
                track_volume,
                accounts,
            }))
        }
        SELL_IX_DISC => {
            let amount = read_u64_le(&data, 8)?;
            let min_sol_output = read_u64_le(&data, 16)?;
            let accounts = parse_sell_accounts(ix)?;

            Some(OuterInstruction::Sell(SellIx {
                amount,
                min_sol_output,
                accounts,
            }))
        }
        BUY_EXACT_SOL_IN_IX_DISC => {
            let spendable_sol_in = read_u64_le(&data, 8)?;
            let min_tokens_out = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24);
            let accounts = parse_buy_accounts(ix)?;

            Some(OuterInstruction::BuyExactSolIn(BuyExactSolInIx {
                spendable_sol_in,
                min_tokens_out,
                track_volume,
                accounts,
            }))
        }
        CREATE_IX_DISC => Some(OuterInstruction::Create),
        CREATE_V2_IX_DISC => Some(OuterInstruction::CreateV2),
        _ => None,
    }
}
