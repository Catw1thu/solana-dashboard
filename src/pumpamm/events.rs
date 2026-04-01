use super::{
    constants::PUMP_AMM_PROGRAM_ID,
    discriminators::{
        BUY_EVENT_DISC, CREATE_POOL_EVENT_DISC, DEPOSIT_EVENT_DISC, SELL_EVENT_DISC,
        WITHDRAW_EVENT_DISC,
    },
    model::{
        BuyEvent, CreatePoolEvent, DepositEvent, LiquidityEvent, SellEvent, SwapEvent,
        WithdrawEvent,
    },
};
use crate::transaction_view::InnerInstructionGroup;
use base64::{Engine as _, engine::general_purpose::STANDARD};

struct ByteReader<'a> {
    data: &'a [u8],
    offset: usize,
}

impl<'a> ByteReader<'a> {
    fn new(data: &'a [u8]) -> Self {
        Self { data, offset: 0 }
    }

    fn is_finished(&self) -> bool {
        self.offset == self.data.len()
    }

    fn read_bytes(&mut self, len: usize) -> Option<&'a [u8]> {
        let bytes = self.data.get(self.offset..self.offset + len)?;
        self.offset += len;
        Some(bytes)
    }

    fn read_u8(&mut self) -> Option<u8> {
        Some(*self.read_bytes(1)?.first()?)
    }

    fn read_u16_le(&mut self) -> Option<u16> {
        let bytes: [u8; 2] = self.read_bytes(2)?.try_into().ok()?;
        Some(u16::from_le_bytes(bytes))
    }

    fn read_bool(&mut self) -> Option<bool> {
        Some(self.read_u8()? != 0)
    }

    fn read_u64_le(&mut self) -> Option<u64> {
        let bytes: [u8; 8] = self.read_bytes(8)?.try_into().ok()?;
        Some(u64::from_le_bytes(bytes))
    }

    fn read_i64_le(&mut self) -> Option<i64> {
        let bytes: [u8; 8] = self.read_bytes(8)?.try_into().ok()?;
        Some(i64::from_le_bytes(bytes))
    }

    fn read_pubkey(&mut self) -> Option<String> {
        Some(bs58::encode(self.read_bytes(32)?).into_string())
    }

    fn read_string(&mut self) -> Option<String> {
        let len_bytes: [u8; 4] = self.read_bytes(4)?.try_into().ok()?;
        let len = u32::from_le_bytes(len_bytes) as usize;
        let bytes = self.read_bytes(len)?;
        std::str::from_utf8(bytes)
            .ok()
            .map(|value| value.to_string())
    }
}

fn parse_buy_event_bytes(data: &[u8]) -> Option<BuyEvent> {
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    if disc != BUY_EVENT_DISC {
        return None;
    }

    let mut reader = ByteReader::new(data.get(8..)?);
    let event = BuyEvent {
        timestamp: reader.read_i64_le()?,
        base_amount_out: reader.read_u64_le()?,
        max_quote_amount_in: reader.read_u64_le()?,
        user_base_token_reserves: reader.read_u64_le()?,
        user_quote_token_reserves: reader.read_u64_le()?,
        pool_base_token_reserves: reader.read_u64_le()?,
        pool_quote_token_reserves: reader.read_u64_le()?,
        quote_amount_in: reader.read_u64_le()?,
        lp_fee_basis_points: reader.read_u64_le()?,
        lp_fee: reader.read_u64_le()?,
        protocol_fee_basis_points: reader.read_u64_le()?,
        protocol_fee: reader.read_u64_le()?,
        quote_amount_in_with_lp_fee: reader.read_u64_le()?,
        user_quote_amount_in: reader.read_u64_le()?,
        pool: reader.read_pubkey()?,
        user: reader.read_pubkey()?,
        user_base_token_account: reader.read_pubkey()?,
        user_quote_token_account: reader.read_pubkey()?,
        protocol_fee_recipient: reader.read_pubkey()?,
        protocol_fee_recipient_token_account: reader.read_pubkey()?,
        coin_creator: reader.read_pubkey()?,
        coin_creator_fee_basis_points: reader.read_u64_le()?,
        coin_creator_fee: reader.read_u64_le()?,
        track_volume: reader.read_bool()?,
        total_unclaimed_tokens: reader.read_u64_le()?,
        total_claimed_tokens: reader.read_u64_le()?,
        current_sol_volume: reader.read_u64_le()?,
        last_update_timestamp: reader.read_i64_le()?,
        min_base_amount_out: reader.read_u64_le()?,
        ix_name: reader.read_string()?,
        cashback_fee_basis_points: reader.read_u64_le()?,
        cashback: reader.read_u64_le()?,
    };

    if !reader.is_finished() {
        return None;
    }

    Some(event)
}

fn parse_sell_event_bytes(data: &[u8]) -> Option<SellEvent> {
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    if disc != SELL_EVENT_DISC {
        return None;
    }

    let mut reader = ByteReader::new(data.get(8..)?);
    let event = SellEvent {
        timestamp: reader.read_i64_le()?,
        base_amount_in: reader.read_u64_le()?,
        min_quote_amount_out: reader.read_u64_le()?,
        user_base_token_reserves: reader.read_u64_le()?,
        user_quote_token_reserves: reader.read_u64_le()?,
        pool_base_token_reserves: reader.read_u64_le()?,
        pool_quote_token_reserves: reader.read_u64_le()?,
        quote_amount_out: reader.read_u64_le()?,
        lp_fee_basis_points: reader.read_u64_le()?,
        lp_fee: reader.read_u64_le()?,
        protocol_fee_basis_points: reader.read_u64_le()?,
        protocol_fee: reader.read_u64_le()?,
        quote_amount_out_without_lp_fee: reader.read_u64_le()?,
        user_quote_amount_out: reader.read_u64_le()?,
        pool: reader.read_pubkey()?,
        user: reader.read_pubkey()?,
        user_base_token_account: reader.read_pubkey()?,
        user_quote_token_account: reader.read_pubkey()?,
        protocol_fee_recipient: reader.read_pubkey()?,
        protocol_fee_recipient_token_account: reader.read_pubkey()?,
        coin_creator: reader.read_pubkey()?,
        coin_creator_fee_basis_points: reader.read_u64_le()?,
        coin_creator_fee: reader.read_u64_le()?,
        cashback_fee_basis_points: reader.read_u64_le()?,
        cashback: reader.read_u64_le()?,
    };

    if !reader.is_finished() {
        return None;
    }

    Some(event)
}

fn parse_create_pool_event_bytes(data: &[u8]) -> Option<CreatePoolEvent> {
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    if disc != CREATE_POOL_EVENT_DISC {
        return None;
    }

    let mut reader = ByteReader::new(data.get(8..)?);
    let event = CreatePoolEvent {
        timestamp: reader.read_i64_le()?,
        index: reader.read_u16_le()?,
        creator: reader.read_pubkey()?,
        base_mint: reader.read_pubkey()?,
        quote_mint: reader.read_pubkey()?,
        base_mint_decimals: reader.read_u8()?,
        quote_mint_decimals: reader.read_u8()?,
        base_amount_in: reader.read_u64_le()?,
        quote_amount_in: reader.read_u64_le()?,
        pool_base_amount: reader.read_u64_le()?,
        pool_quote_amount: reader.read_u64_le()?,
        minimum_liquidity: reader.read_u64_le()?,
        initial_liquidity: reader.read_u64_le()?,
        lp_token_amount_out: reader.read_u64_le()?,
        pool_bump: reader.read_u8()?,
        pool: reader.read_pubkey()?,
        lp_mint: reader.read_pubkey()?,
        user_base_token_account: reader.read_pubkey()?,
        user_quote_token_account: reader.read_pubkey()?,
        coin_creator: reader.read_pubkey()?,
        is_mayhem_mode: reader.read_bool()?,
    };

    if !reader.is_finished() {
        return None;
    }

    Some(event)
}

fn parse_deposit_event_bytes(data: &[u8]) -> Option<DepositEvent> {
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    if disc != DEPOSIT_EVENT_DISC {
        return None;
    }

    let mut reader = ByteReader::new(data.get(8..)?);
    let event = DepositEvent {
        timestamp: reader.read_i64_le()?,
        lp_token_amount_out: reader.read_u64_le()?,
        max_base_amount_in: reader.read_u64_le()?,
        max_quote_amount_in: reader.read_u64_le()?,
        user_base_token_reserves: reader.read_u64_le()?,
        user_quote_token_reserves: reader.read_u64_le()?,
        pool_base_token_reserves: reader.read_u64_le()?,
        pool_quote_token_reserves: reader.read_u64_le()?,
        base_amount_in: reader.read_u64_le()?,
        quote_amount_in: reader.read_u64_le()?,
        lp_mint_supply: reader.read_u64_le()?,
        pool: reader.read_pubkey()?,
        user: reader.read_pubkey()?,
        user_base_token_account: reader.read_pubkey()?,
        user_quote_token_account: reader.read_pubkey()?,
        user_pool_token_account: reader.read_pubkey()?,
    };

    if !reader.is_finished() {
        return None;
    }

    Some(event)
}

fn parse_withdraw_event_bytes(data: &[u8]) -> Option<WithdrawEvent> {
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    if disc != WITHDRAW_EVENT_DISC {
        return None;
    }

    let mut reader = ByteReader::new(data.get(8..)?);
    let event = WithdrawEvent {
        timestamp: reader.read_i64_le()?,
        lp_token_amount_in: reader.read_u64_le()?,
        min_base_amount_out: reader.read_u64_le()?,
        min_quote_amount_out: reader.read_u64_le()?,
        user_base_token_reserves: reader.read_u64_le()?,
        user_quote_token_reserves: reader.read_u64_le()?,
        pool_base_token_reserves: reader.read_u64_le()?,
        pool_quote_token_reserves: reader.read_u64_le()?,
        base_amount_out: reader.read_u64_le()?,
        quote_amount_out: reader.read_u64_le()?,
        lp_mint_supply: reader.read_u64_le()?,
        pool: reader.read_pubkey()?,
        user: reader.read_pubkey()?,
        user_base_token_account: reader.read_pubkey()?,
        user_quote_token_account: reader.read_pubkey()?,
        user_pool_token_account: reader.read_pubkey()?,
    };

    if !reader.is_finished() {
        return None;
    }

    Some(event)
}

pub fn extract_swap_events(logs: &[String]) -> Vec<SwapEvent> {
    let mut events = Vec::new();

    for log in logs {
        let Some(encoded) = log.strip_prefix("Program data: ") else {
            continue;
        };

        let Ok(bytes) = STANDARD.decode(encoded) else {
            continue;
        };

        if let Some(event) = parse_buy_event_bytes(&bytes) {
            events.push(SwapEvent::Buy(event));
            continue;
        }

        if let Some(event) = parse_sell_event_bytes(&bytes) {
            events.push(SwapEvent::Sell(event));
        }
    }

    events
}

pub fn extract_swap_cpi_events(inner_groups: &[InnerInstructionGroup]) -> Vec<SwapEvent> {
    extract_inner_program_events(inner_groups, PUMP_AMM_PROGRAM_ID, |bytes| {
        parse_buy_event_bytes(bytes)
            .map(SwapEvent::Buy)
            .or_else(|| parse_sell_event_bytes(bytes).map(SwapEvent::Sell))
    })
}

pub fn extract_create_pool_events(logs: &[String]) -> Vec<CreatePoolEvent> {
    let mut events = Vec::new();

    for log in logs {
        let Some(encoded) = log.strip_prefix("Program data: ") else {
            continue;
        };

        let Ok(bytes) = STANDARD.decode(encoded) else {
            continue;
        };

        if let Some(event) = parse_create_pool_event_bytes(&bytes) {
            events.push(event);
        }
    }

    events
}

pub fn extract_create_pool_cpi_events(
    inner_groups: &[InnerInstructionGroup],
) -> Vec<CreatePoolEvent> {
    extract_inner_program_events(
        inner_groups,
        PUMP_AMM_PROGRAM_ID,
        parse_create_pool_event_bytes,
    )
}

pub fn extract_liquidity_events(logs: &[String]) -> Vec<LiquidityEvent> {
    let mut events = Vec::new();

    for log in logs {
        let Some(encoded) = log.strip_prefix("Program data: ") else {
            continue;
        };

        let Ok(bytes) = STANDARD.decode(encoded) else {
            continue;
        };

        if let Some(event) = parse_deposit_event_bytes(&bytes) {
            events.push(LiquidityEvent::Deposit(event));
            continue;
        }

        if let Some(event) = parse_withdraw_event_bytes(&bytes) {
            events.push(LiquidityEvent::Withdraw(event));
        }
    }

    events
}

pub fn extract_liquidity_cpi_events(inner_groups: &[InnerInstructionGroup]) -> Vec<LiquidityEvent> {
    extract_inner_program_events(inner_groups, PUMP_AMM_PROGRAM_ID, |bytes| {
        parse_deposit_event_bytes(bytes)
            .map(LiquidityEvent::Deposit)
            .or_else(|| parse_withdraw_event_bytes(bytes).map(LiquidityEvent::Withdraw))
    })
}

fn extract_inner_program_events<T, F>(
    inner_groups: &[InnerInstructionGroup],
    program_id: &str,
    parser: F,
) -> Vec<T>
where
    F: Fn(&[u8]) -> Option<T>,
{
    let mut events = Vec::new();

    for group in inner_groups {
        for ix in &group.instructions {
            if ix.program_id != program_id {
                continue;
            }

            let Ok(bytes) = STANDARD.decode(&ix.data_base64) else {
                continue;
            };

            if let Some(event) = parser(&bytes).or_else(|| bytes.get(8..).and_then(&parser)) {
                events.push(event);
            }
        }
    }

    events
}
