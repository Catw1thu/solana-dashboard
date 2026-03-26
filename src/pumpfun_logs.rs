const TRADE_EVENT_DISC: [u8; 8] = [189, 219, 127, 211, 78, 230, 97, 238];
use base64::{Engine, engine::general_purpose::STANDARD};

#[derive(Debug)]
pub struct PumpfunTradeEvent {
    pub mint: String,
    pub sol_amount: u64,
    pub token_amount: u64,
    pub is_buy: bool,
    pub user: String,
    pub timestamp: i64,
    pub virtual_sol_reserves: u64,
    pub virtual_token_reserves: u64,
    pub real_sol_reserves: u64,
    pub real_token_reserves: u64,
    pub fee_recipient: String,
    pub fee_basis_points: u64,
    pub fee: u64,
    pub creator: String,
    pub creator_fee_basis_points: u64,
    pub creator_fee: u64,
    pub track_volume: bool,
    pub total_unclaimed_tokens: u64,
    pub total_claimed_tokens: u64,
    pub current_sol_volume: u64,
    pub last_update_timestamp: i64,
    pub ix_name: String,
    pub mayhem_mode: bool,
    pub cashback_fee_basis_points: u64,
    pub cashback: u64,
}

struct ByteReader<'a> {
    data: &'a [u8],
    offset: usize,
}
impl<'a> ByteReader<'a> {
    fn new(data: &'a [u8]) -> Self {
        Self { data, offset: 0 }
    }
    fn read_u8(&mut self) -> Option<u8> {
        let value = *self.data.get(self.offset)?;
        self.offset += 1;
        Some(value)
    }
    fn read_bool(&mut self) -> Option<bool> {
        Some(self.read_u8()? != 0)
    }
    fn read_u32_le(&mut self) -> Option<u32> {
        let bytes = self
            .data
            .get(self.offset..self.offset + 4)?
            .try_into()
            .ok()?;
        self.offset += 4;
        Some(u32::from_le_bytes(bytes))
    }
    fn read_u64_le(&mut self) -> Option<u64> {
        let bytes = self
            .data
            .get(self.offset..self.offset + 8)?
            .try_into()
            .ok()?;
        self.offset += 8;
        Some(u64::from_le_bytes(bytes))
    }
    fn read_i64_le(&mut self) -> Option<i64> {
        let bytes = self
            .data
            .get(self.offset..self.offset + 8)?
            .try_into()
            .ok()?;
        self.offset += 8;
        Some(i64::from_le_bytes(bytes))
    }
    fn read_pubkey(&mut self) -> Option<String> {
        let bytes = self.data.get(self.offset..self.offset + 32)?;
        self.offset += 32;
        Some(bs58::encode(bytes).into_string())
    }
    fn read_string(&mut self) -> Option<String> {
        let len = self.read_u32_le()? as usize;
        let bytes = self.data.get(self.offset..self.offset + len)?;
        self.offset += len;
        std::str::from_utf8(bytes).ok().map(|s| s.to_string())
    }
}

pub fn parse_trade_event_bytes(data: &[u8]) -> Option<PumpfunTradeEvent> {
    let disc: [u8; 8] = data.get(0..8)?.try_into().ok()?;
    if disc != TRADE_EVENT_DISC {
        return None;
    }
    let mut r = ByteReader::new(&data[8..]);
    let pumpfunTradeEvent = PumpfunTradeEvent {
        mint: r.read_pubkey()?,
        sol_amount: r.read_u64_le()?,
        token_amount: r.read_u64_le()?,
        is_buy: r.read_bool()?,
        user: r.read_pubkey()?,
        timestamp: r.read_i64_le()?,
        virtual_sol_reserves: r.read_u64_le()?,
        virtual_token_reserves: r.read_u64_le()?,
        real_sol_reserves: r.read_u64_le()?,
        real_token_reserves: r.read_u64_le()?,
        fee_recipient: r.read_pubkey()?,
        fee_basis_points: r.read_u64_le()?,
        fee: r.read_u64_le()?,
        creator: r.read_pubkey()?,
        creator_fee_basis_points: r.read_u64_le()?,
        creator_fee: r.read_u64_le()?,
        track_volume: r.read_bool()?,
        total_unclaimed_tokens: r.read_u64_le()?,
        total_claimed_tokens: r.read_u64_le()?,
        current_sol_volume: r.read_u64_le()?,
        last_update_timestamp: r.read_i64_le()?,
        ix_name: r.read_string()?,
        mayhem_mode: r.read_bool()?,
        cashback_fee_basis_points: r.read_u64_le()?,
        cashback: r.read_u64_le()?,
    };
    if r.offset != data.len() - 8 {
        // 解析后应该正好读完所有字节
        return None;
    }
    Some(pumpfunTradeEvent)
}

pub fn extract_trade_event_from_log(logs: &[String]) -> Vec<PumpfunTradeEvent> {
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
