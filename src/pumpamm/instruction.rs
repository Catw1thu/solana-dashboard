use super::{
    constants::PUMP_AMM_PROGRAM_ID,
    discriminators::{
        BUY_EXACT_QUOTE_IN_IX_DISC, BUY_IX_DISC, CREATE_POOL_IX_DISC, DEPOSIT_IX_DISC,
        SELL_IX_DISC, WITHDRAW_IX_DISC,
    },
    model::{
        BuyExactQuoteInIx, BuyIx, CreatePoolAccounts, CreatePoolIx, DepositIx, InstructionInput,
        LiquidityAccounts, PumpAmmInstruction, SellIx, SwapAccounts, WithdrawIx,
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

fn account_at(input: &InstructionInputRef<'_>, index: usize) -> Option<String> {
    input.account_pubkeys.get(index).cloned()
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

    fn read_u16_le(&mut self) -> Option<u16> {
        let bytes: [u8; 2] = self.read_bytes(2)?.try_into().ok()?;
        Some(u16::from_le_bytes(bytes))
    }

    fn read_u64_le(&mut self) -> Option<u64> {
        let bytes: [u8; 8] = self.read_bytes(8)?.try_into().ok()?;
        Some(u64::from_le_bytes(bytes))
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

fn parse_buy_accounts(input: &InstructionInputRef<'_>) -> Option<SwapAccounts> {
    Some(SwapAccounts {
        pool: account_at(input, 0)?,
        user: account_at(input, 1)?,
        global_config: account_at(input, 2)?,
        base_mint: account_at(input, 3)?,
        quote_mint: account_at(input, 4)?,
        user_base_token_account: account_at(input, 5)?,
        user_quote_token_account: account_at(input, 6)?,
        pool_base_token_account: account_at(input, 7)?,
        pool_quote_token_account: account_at(input, 8)?,
        protocol_fee_recipient: account_at(input, 9)?,
        protocol_fee_recipient_token_account: account_at(input, 10)?,
        base_token_program: account_at(input, 11)?,
        quote_token_program: account_at(input, 12)?,
        system_program: account_at(input, 13)?,
        associated_token_program: account_at(input, 14)?,
        event_authority: account_at(input, 15)?,
        program: account_at(input, 16)?,
        coin_creator_vault_ata: account_at(input, 17)?,
        coin_creator_vault_authority: account_at(input, 18)?,
        global_volume_accumulator: account_at(input, 19),
        user_volume_accumulator: account_at(input, 20),
        fee_config: account_at(input, 21)?,
        fee_program: account_at(input, 22)?,
    })
}

fn parse_sell_accounts(input: &InstructionInputRef<'_>) -> Option<SwapAccounts> {
    Some(SwapAccounts {
        pool: account_at(input, 0)?,
        user: account_at(input, 1)?,
        global_config: account_at(input, 2)?,
        base_mint: account_at(input, 3)?,
        quote_mint: account_at(input, 4)?,
        user_base_token_account: account_at(input, 5)?,
        user_quote_token_account: account_at(input, 6)?,
        pool_base_token_account: account_at(input, 7)?,
        pool_quote_token_account: account_at(input, 8)?,
        protocol_fee_recipient: account_at(input, 9)?,
        protocol_fee_recipient_token_account: account_at(input, 10)?,
        base_token_program: account_at(input, 11)?,
        quote_token_program: account_at(input, 12)?,
        system_program: account_at(input, 13)?,
        associated_token_program: account_at(input, 14)?,
        event_authority: account_at(input, 15)?,
        program: account_at(input, 16)?,
        coin_creator_vault_ata: account_at(input, 17)?,
        coin_creator_vault_authority: account_at(input, 18)?,
        global_volume_accumulator: None,
        user_volume_accumulator: None,
        fee_config: account_at(input, 19)?,
        fee_program: account_at(input, 20)?,
    })
}

fn parse_create_pool_accounts(input: &InstructionInputRef<'_>) -> Option<CreatePoolAccounts> {
    Some(CreatePoolAccounts {
        pool: account_at(input, 0)?,
        global_config: account_at(input, 1)?,
        creator: account_at(input, 2)?,
        base_mint: account_at(input, 3)?,
        quote_mint: account_at(input, 4)?,
        lp_mint: account_at(input, 5)?,
        user_base_token_account: account_at(input, 6)?,
        user_quote_token_account: account_at(input, 7)?,
        user_pool_token_account: account_at(input, 8)?,
        pool_base_token_account: account_at(input, 9)?,
        pool_quote_token_account: account_at(input, 10)?,
        system_program: account_at(input, 11)?,
        token_2022_program: account_at(input, 12)?,
        base_token_program: account_at(input, 13)?,
        quote_token_program: account_at(input, 14)?,
        associated_token_program: account_at(input, 15)?,
        event_authority: account_at(input, 16)?,
        program: account_at(input, 17)?,
    })
}

fn parse_liquidity_accounts(input: &InstructionInputRef<'_>) -> Option<LiquidityAccounts> {
    Some(LiquidityAccounts {
        pool: account_at(input, 0)?,
        global_config: account_at(input, 1)?,
        user: account_at(input, 2)?,
        base_mint: account_at(input, 3)?,
        quote_mint: account_at(input, 4)?,
        lp_mint: account_at(input, 5)?,
        user_base_token_account: account_at(input, 6)?,
        user_quote_token_account: account_at(input, 7)?,
        user_pool_token_account: account_at(input, 8)?,
        pool_base_token_account: account_at(input, 9)?,
        pool_quote_token_account: account_at(input, 10)?,
        token_program: account_at(input, 11)?,
        token_2022_program: account_at(input, 12)?,
        event_authority: account_at(input, 13)?,
        program: account_at(input, 14)?,
    })
}

#[allow(dead_code)]
pub fn parse_instruction(input: &InstructionInput) -> Option<PumpAmmInstruction> {
    let data = STANDARD.decode(&input.data_base64).ok()?;
    parse_decoded_instruction(&input.program_id, &input.account_pubkeys, &data)
}

pub(crate) fn parse_decoded_instruction(
    program_id: &str,
    account_pubkeys: &[String],
    data: &[u8],
) -> Option<PumpAmmInstruction> {
    if program_id != PUMP_AMM_PROGRAM_ID {
        return None;
    }

    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    let input = InstructionInputRef {
        account_pubkeys,
    };

    match disc {
        BUY_IX_DISC => {
            let base_amount_out = read_u64_le(&data, 8)?;
            let max_quote_amount_in = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24);
            let accounts = parse_buy_accounts(&input)?;

            Some(PumpAmmInstruction::Buy(BuyIx {
                base_amount_out,
                max_quote_amount_in,
                track_volume,
                accounts,
            }))
        }
        BUY_EXACT_QUOTE_IN_IX_DISC => {
            let spendable_quote_in = read_u64_le(&data, 8)?;
            let min_base_amount_out = read_u64_le(&data, 16)?;
            let track_volume = read_bool_flag(&data, 24);
            let accounts = parse_buy_accounts(&input)?;

            Some(PumpAmmInstruction::BuyExactQuoteIn(BuyExactQuoteInIx {
                spendable_quote_in,
                min_base_amount_out,
                track_volume,
                accounts,
            }))
        }
        SELL_IX_DISC => {
            let base_amount_in = read_u64_le(&data, 8)?;
            let min_quote_amount_out = read_u64_le(&data, 16)?;
            let accounts = parse_sell_accounts(&input)?;

            Some(PumpAmmInstruction::Sell(SellIx {
                base_amount_in,
                min_quote_amount_out,
                accounts,
            }))
        }
        CREATE_POOL_IX_DISC => {
            let mut reader = ArgReader::new(&data, 8);
            let index = reader.read_u16_le()?;
            let base_amount_in = reader.read_u64_le()?;
            let quote_amount_in = reader.read_u64_le()?;
            let coin_creator = reader.read_pubkey()?;
            let is_mayhem_mode = reader.read_bool()?;
            let is_cashback_coin = reader.read_bool();
            let accounts = parse_create_pool_accounts(&input)?;

            Some(PumpAmmInstruction::CreatePool(CreatePoolIx {
                index,
                base_amount_in,
                quote_amount_in,
                coin_creator,
                is_mayhem_mode,
                is_cashback_coin,
                accounts,
            }))
        }
        DEPOSIT_IX_DISC => {
            let lp_token_amount_out = read_u64_le(&data, 8)?;
            let max_base_amount_in = read_u64_le(&data, 16)?;
            let max_quote_amount_in = read_u64_le(&data, 24)?;
            let accounts = parse_liquidity_accounts(&input)?;

            Some(PumpAmmInstruction::Deposit(DepositIx {
                lp_token_amount_out,
                max_base_amount_in,
                max_quote_amount_in,
                accounts,
            }))
        }
        WITHDRAW_IX_DISC => {
            let lp_token_amount_in = read_u64_le(&data, 8)?;
            let min_base_amount_out = read_u64_le(&data, 16)?;
            let min_quote_amount_out = read_u64_le(&data, 24)?;
            let accounts = parse_liquidity_accounts(&input)?;

            Some(PumpAmmInstruction::Withdraw(WithdrawIx {
                lp_token_amount_in,
                min_base_amount_out,
                min_quote_amount_out,
                accounts,
            }))
        }
        _ => None,
    }
}
