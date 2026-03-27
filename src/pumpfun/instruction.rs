use super::{
    constants::PUMPFUN_PROGRAM_ID,
    discriminators::{
        BUY_EXACT_SOL_IN_IX_DISC, BUY_IX_DISC, CREATE_IX_DISC, CREATE_V2_IX_DISC, SELL_IX_DISC,
    },
    model::{BuyExactSolInIx, BuyIx, InstructionInput, PumpfunInstruction, SellIx, TradeAccounts},
};
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

fn account_at(input: &InstructionInput, index: usize) -> Option<String> {
    input.account_pubkeys.get(index).cloned()
}

fn parse_buy_accounts(input: &InstructionInput) -> Option<TradeAccounts> {
    Some(TradeAccounts {
        global: account_at(input, 0)?,
        fee_recipient: account_at(input, 1)?,
        mint: account_at(input, 2)?,
        bonding_curve: account_at(input, 3)?,
        associated_bonding_curve: account_at(input, 4)?,
        associated_user: account_at(input, 5)?,
        user: account_at(input, 6)?,
        system_program: account_at(input, 7)?,
        token_program: account_at(input, 8)?,
        creator_vault: account_at(input, 9)?,
        event_authority: account_at(input, 10)?,
        program: account_at(input, 11)?,
        global_volume_accumulator: account_at(input, 12),
        user_volume_accumulator: account_at(input, 13),
        fee_config: account_at(input, 14)?,
        fee_program: account_at(input, 15)?,
    })
}

fn parse_sell_accounts(input: &InstructionInput) -> Option<TradeAccounts> {
    Some(TradeAccounts {
        global: account_at(input, 0)?,
        fee_recipient: account_at(input, 1)?,
        mint: account_at(input, 2)?,
        bonding_curve: account_at(input, 3)?,
        associated_bonding_curve: account_at(input, 4)?,
        associated_user: account_at(input, 5)?,
        user: account_at(input, 6)?,
        system_program: account_at(input, 7)?,
        creator_vault: account_at(input, 8)?,
        token_program: account_at(input, 9)?,
        event_authority: account_at(input, 10)?,
        program: account_at(input, 11)?,
        global_volume_accumulator: None,
        user_volume_accumulator: None,
        fee_config: account_at(input, 12)?,
        fee_program: account_at(input, 13)?,
    })
}

pub fn parse_instruction(input: &InstructionInput) -> Option<PumpfunInstruction> {
    if input.program_id != PUMPFUN_PROGRAM_ID {
        return None;
    }

    let data = STANDARD.decode(&input.data_base64).ok()?;
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;

    match disc {
        BUY_IX_DISC => {
            let amount = read_u64_le(&data, 8)?;
            let max_sol_cost = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24);
            let accounts = parse_buy_accounts(input)?;

            Some(PumpfunInstruction::Buy(BuyIx {
                amount,
                max_sol_cost,
                track_volume,
                accounts,
            }))
        }
        SELL_IX_DISC => {
            let amount = read_u64_le(&data, 8)?;
            let min_sol_output = read_u64_le(&data, 16)?;
            let accounts = parse_sell_accounts(input)?;

            Some(PumpfunInstruction::Sell(SellIx {
                amount,
                min_sol_output,
                accounts,
            }))
        }
        BUY_EXACT_SOL_IN_IX_DISC => {
            let spendable_sol_in = read_u64_le(&data, 8)?;
            let min_tokens_out = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24);
            let accounts = parse_buy_accounts(input)?;

            Some(PumpfunInstruction::BuyExactSolIn(BuyExactSolInIx {
                spendable_sol_in,
                min_tokens_out,
                track_volume,
                accounts,
            }))
        }
        CREATE_IX_DISC => Some(PumpfunInstruction::Create),
        CREATE_V2_IX_DISC => Some(PumpfunInstruction::CreateV2),
        _ => None,
    }
}
