use super::{
    discriminators::{CREATE_EVENT_DISC, TRADE_EVENT_DISC},
    model::{CreateEvent, TradeEvent},
};
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

    fn read_bool(&mut self) -> Option<bool> {
        Some(self.read_u8()? != 0)
    }

    fn read_u32_le(&mut self) -> Option<u32> {
        let bytes: [u8; 4] = self.read_bytes(4)?.try_into().ok()?;
        Some(u32::from_le_bytes(bytes))
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
        let len = self.read_u32_le()? as usize;
        let bytes = self.read_bytes(len)?;
        std::str::from_utf8(bytes).ok().map(|s| s.to_string())
    }
}

pub fn parse_trade_event_bytes(data: &[u8]) -> Option<TradeEvent> {
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    if disc != TRADE_EVENT_DISC {
        return None;
    }

    let mut reader = ByteReader::new(data.get(8..)?);
    let trade_event = TradeEvent {
        mint: reader.read_pubkey()?,
        sol_amount: reader.read_u64_le()?,
        token_amount: reader.read_u64_le()?,
        is_buy: reader.read_bool()?,
        user: reader.read_pubkey()?,
        timestamp: reader.read_i64_le()?,
        virtual_sol_reserves: reader.read_u64_le()?,
        virtual_token_reserves: reader.read_u64_le()?,
        real_sol_reserves: reader.read_u64_le()?,
        real_token_reserves: reader.read_u64_le()?,
        fee_recipient: reader.read_pubkey()?,
        fee_basis_points: reader.read_u64_le()?,
        fee: reader.read_u64_le()?,
        creator: reader.read_pubkey()?,
        creator_fee_basis_points: reader.read_u64_le()?,
        creator_fee: reader.read_u64_le()?,
        track_volume: reader.read_bool()?,
        total_unclaimed_tokens: reader.read_u64_le()?,
        total_claimed_tokens: reader.read_u64_le()?,
        current_sol_volume: reader.read_u64_le()?,
        last_update_timestamp: reader.read_i64_le()?,
        ix_name: reader.read_string()?,
        mayhem_mode: reader.read_bool()?,
        cashback_fee_basis_points: reader.read_u64_le()?,
        cashback: reader.read_u64_le()?,
    };

    if !reader.is_finished() {
        return None;
    }

    Some(trade_event)
}

pub fn parse_create_event_bytes(data: &[u8]) -> Option<CreateEvent> {
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    if disc != CREATE_EVENT_DISC {
        return None;
    }

    let mut reader = ByteReader::new(data.get(8..)?);
    let create_event = CreateEvent {
        name: reader.read_string()?,
        symbol: reader.read_string()?,
        uri: reader.read_string()?,
        mint: reader.read_pubkey()?,
        bonding_curve: reader.read_pubkey()?,
        user: reader.read_pubkey()?,
        creator: reader.read_pubkey()?,
        timestamp: reader.read_i64_le()?,
        virtual_token_reserves: reader.read_u64_le()?,
        virtual_sol_reserves: reader.read_u64_le()?,
        real_token_reserves: reader.read_u64_le()?,
        token_total_supply: reader.read_u64_le()?,
        token_program: reader.read_pubkey()?,
        is_mayhem_mode: reader.read_bool()?,
        is_cashback_enabled: reader.read_bool()?,
    };

    if !reader.is_finished() {
        return None;
    }

    Some(create_event)
}

pub fn extract_trade_events(logs: &[String]) -> Vec<TradeEvent> {
    let mut events = Vec::new();

    for log in logs {
        let Some(encoded) = log.strip_prefix("Program data: ") else {
            continue;
        };

        let Ok(bytes) = STANDARD.decode(encoded) else {
            continue;
        };

        if let Some(event) = parse_trade_event_bytes(&bytes) {
            events.push(event);
        }
    }

    events
}

pub fn extract_create_events(logs: &[String]) -> Vec<CreateEvent> {
    let mut events = Vec::new();

    for log in logs {
        let Some(encoded) = log.strip_prefix("Program data: ") else {
            continue;
        };

        let Ok(bytes) = STANDARD.decode(encoded) else {
            continue;
        };

        if let Some(event) = parse_create_event_bytes(&bytes) {
            events.push(event);
        }
    }

    events
}
