use super::{
    constants::PUMPFUN_PROGRAM_ID,
    discriminators::{
        BUY_EXACT_SOL_IN_IX_DISC, BUY_IX_DISC, CREATE_IX_DISC, CREATE_V2_IX_DISC, MIGRATE_IX_DISC,
        SELL_IX_DISC,
    },
    model::{
        BuyExactSolInIx, BuyIx, CreateAccounts, CreateIx, CreateV2Ix, InstructionInput,
        MigrateAccounts, MigrateIx, PumpfunInstruction, SellIx, TradeAccounts,
    },
};
use base64::{Engine as _, engine::general_purpose::STANDARD};

struct InstructionInputRef<'a> {
    account_pubkeys: &'a [String],
}

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

struct ArgReader<'a> {
    data: &'a [u8],
    offset: usize,
}

impl<'a> ArgReader<'a> {
    fn new(data: &'a [u8], offset: usize) -> Self {
        Self { data, offset }
    }

    fn read_bytes(&mut self, len: usize) -> Option<&'a [u8]> {
        let bytes = self.data.get(self.offset..self.offset + len)?;
        self.offset += len;
        Some(bytes)
    }

    fn read_u32_le(&mut self) -> Option<u32> {
        let bytes: [u8; 4] = self.read_bytes(4)?.try_into().ok()?;
        Some(u32::from_le_bytes(bytes))
    }

    fn read_string(&mut self) -> Option<String> {
        let len = self.read_u32_le()? as usize;
        let bytes = self.read_bytes(len)?;
        std::str::from_utf8(bytes)
            .ok()
            .map(|value| value.to_string())
    }

    fn read_pubkey(&mut self) -> Option<String> {
        Some(bs58::encode(self.read_bytes(32)?).into_string())
    }

    fn read_bool(&mut self) -> Option<bool> {
        let byte = *self.read_bytes(1)?.first()?;
        match byte {
            0 => Some(false),
            1 => Some(true),
            _ => None,
        }
    }
}

fn account_at(input: &InstructionInputRef<'_>, index: usize) -> Option<String> {
    input.account_pubkeys.get(index).cloned()
}

fn parse_buy_accounts(input: &InstructionInputRef<'_>) -> Option<TradeAccounts> {
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

fn parse_sell_accounts(input: &InstructionInputRef<'_>) -> Option<TradeAccounts> {
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

fn parse_create_accounts(input: &InstructionInputRef<'_>) -> Option<CreateAccounts> {
    Some(CreateAccounts {
        mint: account_at(input, 0)?,
        mint_authority: account_at(input, 1)?,
        bonding_curve: account_at(input, 2)?,
        associated_bonding_curve: account_at(input, 3)?,
        global: account_at(input, 4)?,
        user: account_at(input, 7)?,
        system_program: account_at(input, 8)?,
        token_program: account_at(input, 9)?,
        associated_token_program: account_at(input, 10)?,
        event_authority: account_at(input, 12)?,
        program: account_at(input, 13)?,
        mpl_token_metadata: account_at(input, 5),
        metadata: account_at(input, 6),
        rent: account_at(input, 11),
        mayhem_program_id: None,
        global_params: None,
        sol_vault: None,
        mayhem_state: None,
        mayhem_token_vault: None,
    })
}

fn parse_create_v2_accounts(input: &InstructionInputRef<'_>) -> Option<CreateAccounts> {
    Some(CreateAccounts {
        mint: account_at(input, 0)?,
        mint_authority: account_at(input, 1)?,
        bonding_curve: account_at(input, 2)?,
        associated_bonding_curve: account_at(input, 3)?,
        global: account_at(input, 4)?,
        user: account_at(input, 5)?,
        system_program: account_at(input, 6)?,
        token_program: account_at(input, 7)?,
        associated_token_program: account_at(input, 8)?,
        event_authority: account_at(input, 14)?,
        program: account_at(input, 15)?,
        mpl_token_metadata: None,
        metadata: None,
        rent: None,
        mayhem_program_id: account_at(input, 9),
        global_params: account_at(input, 10),
        sol_vault: account_at(input, 11),
        mayhem_state: account_at(input, 12),
        mayhem_token_vault: account_at(input, 13),
    })
}

fn parse_migrate_accounts(input: &InstructionInputRef<'_>) -> Option<MigrateAccounts> {
    Some(MigrateAccounts {
        global: account_at(input, 0)?,
        withdraw_authority: account_at(input, 1)?,
        mint: account_at(input, 2)?,
        bonding_curve: account_at(input, 3)?,
        associated_bonding_curve: account_at(input, 4)?,
        user: account_at(input, 5)?,
        system_program: account_at(input, 6)?,
        token_program: account_at(input, 7)?,
        pump_amm: account_at(input, 8)?,
        pool: account_at(input, 9)?,
        pool_authority: account_at(input, 10)?,
        pool_authority_mint_account: account_at(input, 11)?,
        pool_authority_wsol_account: account_at(input, 12)?,
        amm_global_config: account_at(input, 13)?,
        wsol_mint: account_at(input, 14)?,
        lp_mint: account_at(input, 15)?,
        user_pool_token_account: account_at(input, 16)?,
        pool_base_token_account: account_at(input, 17)?,
        pool_quote_token_account: account_at(input, 18)?,
        token_2022_program: account_at(input, 19)?,
        associated_token_program: account_at(input, 20)?,
        pump_amm_event_authority: account_at(input, 21)?,
        event_authority: account_at(input, 22)?,
        program: account_at(input, 23)?,
    })
}

#[allow(dead_code)]
pub fn parse_instruction(input: &InstructionInput) -> Option<PumpfunInstruction> {
    let data = STANDARD.decode(&input.data_base64).ok()?;
    parse_decoded_instruction(&input.program_id, &input.account_pubkeys, &data)
}

pub(crate) fn parse_decoded_instruction(
    program_id: &str,
    account_pubkeys: &[String],
    data: &[u8],
) -> Option<PumpfunInstruction> {
    if program_id != PUMPFUN_PROGRAM_ID {
        return None;
    }

    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    let input = InstructionInputRef {
        account_pubkeys,
    };

    match disc {
        BUY_IX_DISC => {
            let amount = read_u64_le(&data, 8)?;
            let max_sol_cost = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24);
            let accounts = parse_buy_accounts(&input)?;

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
            let accounts = parse_sell_accounts(&input)?;

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
            let accounts = parse_buy_accounts(&input)?;

            Some(PumpfunInstruction::BuyExactSolIn(BuyExactSolInIx {
                spendable_sol_in,
                min_tokens_out,
                track_volume,
                accounts,
            }))
        }
        CREATE_IX_DISC => {
            let mut reader = ArgReader::new(&data, 8);
            let name = reader.read_string()?;
            let symbol = reader.read_string()?;
            let uri = reader.read_string()?;
            let creator = reader.read_pubkey()?;
            let accounts = parse_create_accounts(&input)?;

            Some(PumpfunInstruction::Create(CreateIx {
                name,
                symbol,
                uri,
                creator,
                accounts,
            }))
        }
        CREATE_V2_IX_DISC => {
            let mut reader = ArgReader::new(&data, 8);
            let name = reader.read_string()?;
            let symbol = reader.read_string()?;
            let uri = reader.read_string()?;
            let creator = reader.read_pubkey()?;
            let is_mayhem_mode = reader.read_bool()?;
            let is_cashback_enabled = reader.read_bool();
            let accounts = parse_create_v2_accounts(&input)?;

            Some(PumpfunInstruction::CreateV2(CreateV2Ix {
                name,
                symbol,
                uri,
                creator,
                is_mayhem_mode,
                is_cashback_enabled,
                accounts,
            }))
        }
        MIGRATE_IX_DISC => {
            let accounts = parse_migrate_accounts(&input)?;
            Some(PumpfunInstruction::Migrate(MigrateIx { accounts }))
        }
        _ => None,
    }
}
