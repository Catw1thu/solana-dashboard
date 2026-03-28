use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub enum PumpfunInstruction {
    Buy(BuyIx),
    Sell(SellIx),
    BuyExactSolIn(BuyExactSolInIx),
    Create,
    CreateV2,
}

impl PumpfunInstruction {
    pub fn accounts(&self) -> Option<&TradeAccounts> {
        match self {
            Self::Buy(ix) => Some(&ix.accounts),
            Self::Sell(ix) => Some(&ix.accounts),
            Self::BuyExactSolIn(ix) => Some(&ix.accounts),
            Self::Create | Self::CreateV2 => None,
        }
    }

    pub fn ix_name(&self) -> Option<&'static str> {
        match self {
            Self::Buy(_) => Some("buy"),
            Self::Sell(_) => Some("sell"),
            Self::BuyExactSolIn(_) => Some("buy_exact_sol_in"),
            Self::Create | Self::CreateV2 => None,
        }
    }

    pub fn side(&self) -> Option<TradeSide> {
        match self {
            Self::Buy(_) => Some(TradeSide::Buy),
            Self::Sell(_) => Some(TradeSide::Sell),
            Self::BuyExactSolIn(_) => Some(TradeSide::BuyExactSolIn),
            Self::Create | Self::CreateV2 => None,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub enum TradeSide {
    Buy,
    Sell,
    BuyExactSolIn,
}

#[derive(Debug, Clone, Serialize)]
pub struct TradeAccounts {
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

#[derive(Debug, Clone, Serialize)]
pub struct BuyIx {
    pub amount: u64,
    pub max_sol_cost: u64,
    pub track_volume: Option<bool>,
    pub accounts: TradeAccounts,
}

#[derive(Debug, Clone, Serialize)]
pub struct SellIx {
    pub amount: u64,
    pub min_sol_output: u64,
    pub accounts: TradeAccounts,
}

#[derive(Debug, Clone, Serialize)]
pub struct BuyExactSolInIx {
    pub spendable_sol_in: u64,
    pub min_tokens_out: u64,
    pub track_volume: Option<bool>,
    pub accounts: TradeAccounts,
}

#[derive(Debug, Clone, Serialize)]
pub struct TradeEvent {
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

#[derive(Debug, Clone, Serialize)]
pub struct ParsedTrade {
    pub source: InvocationSource,
    pub side: TradeSide,
    pub mint: String,
    pub user: String,
    pub bonding_curve: String,
    pub associated_bonding_curve: String,
    pub creator_vault: String,
    pub token_program: String,
    pub sol_amount: u64,
    pub token_amount: u64,
    pub is_buy: bool,
    pub track_volume: bool,
    pub timestamp: i64,
    pub ix_name: String,
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
    pub current_sol_volume: u64,
    pub cashback_fee_basis_points: u64,
    pub cashback: u64,
    pub instruction: PumpfunInstruction,
    pub event: TradeEvent,
}

#[derive(Debug, Clone, Serialize)]
pub enum InvocationSource {
    Outer {
        outer_index: usize,
    },
    Inner {
        outer_index: usize,
        inner_index: usize,
    },
}

#[derive(Debug, Clone)]
pub struct InstructionInput {
    pub program_id: String,
    pub account_pubkeys: Vec<String>,
    pub data_base64: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct PumpfunInvocation {
    pub source: InvocationSource,
    pub instruction: PumpfunInstruction,
}

#[derive(Debug, Clone, Serialize)]
pub struct TradeAnalysis {
    pub trades: Vec<ParsedTrade>,
    pub unmatched_invocations: Vec<PumpfunInvocation>,
    pub unmatched_events: Vec<TradeEvent>,
}
