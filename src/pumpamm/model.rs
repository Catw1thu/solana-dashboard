use crate::event_origin::EventOrigin;
use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub enum PumpAmmInstruction {
    Buy(BuyIx),
    BuyExactQuoteIn(BuyExactQuoteInIx),
    Sell(SellIx),
    CreatePool(CreatePoolIx),
    Deposit(DepositIx),
    Withdraw(WithdrawIx),
}

impl PumpAmmInstruction {
    pub fn swap_accounts(&self) -> Option<&SwapAccounts> {
        match self {
            Self::Buy(ix) => Some(&ix.accounts),
            Self::BuyExactQuoteIn(ix) => Some(&ix.accounts),
            Self::Sell(ix) => Some(&ix.accounts),
            Self::CreatePool(_) | Self::Deposit(_) | Self::Withdraw(_) => None,
        }
    }

    pub fn create_pool_accounts(&self) -> Option<&CreatePoolAccounts> {
        match self {
            Self::CreatePool(ix) => Some(&ix.accounts),
            Self::Buy(_)
            | Self::BuyExactQuoteIn(_)
            | Self::Sell(_)
            | Self::Deposit(_)
            | Self::Withdraw(_) => None,
        }
    }

    pub fn liquidity_accounts(&self) -> Option<&LiquidityAccounts> {
        match self {
            Self::Deposit(ix) => Some(&ix.accounts),
            Self::Withdraw(ix) => Some(&ix.accounts),
            Self::Buy(_) | Self::BuyExactQuoteIn(_) | Self::Sell(_) | Self::CreatePool(_) => None,
        }
    }

    pub fn ix_name(&self) -> &'static str {
        match self {
            Self::Buy(_) => "buy",
            Self::BuyExactQuoteIn(_) => "buy_exact_quote_in",
            Self::Sell(_) => "sell",
            Self::CreatePool(_) => "create_pool",
            Self::Deposit(_) => "deposit",
            Self::Withdraw(_) => "withdraw",
        }
    }

    pub fn swap_side(&self) -> Option<SwapSide> {
        match self {
            Self::Buy(_) => Some(SwapSide::Buy),
            Self::BuyExactQuoteIn(_) => Some(SwapSide::BuyExactQuoteIn),
            Self::Sell(_) => Some(SwapSide::Sell),
            Self::CreatePool(_) | Self::Deposit(_) | Self::Withdraw(_) => None,
        }
    }

    pub fn liquidity_action(&self) -> Option<LiquidityAction> {
        match self {
            Self::Deposit(_) => Some(LiquidityAction::Deposit),
            Self::Withdraw(_) => Some(LiquidityAction::Withdraw),
            Self::Buy(_) | Self::BuyExactQuoteIn(_) | Self::Sell(_) | Self::CreatePool(_) => None,
        }
    }

    pub fn is_swap(&self) -> bool {
        matches!(
            self,
            Self::Buy(_) | Self::BuyExactQuoteIn(_) | Self::Sell(_)
        )
    }

    pub fn is_create_pool(&self) -> bool {
        matches!(self, Self::CreatePool(_))
    }

    pub fn is_liquidity(&self) -> bool {
        matches!(self, Self::Deposit(_) | Self::Withdraw(_))
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SwapSide {
    Buy,
    BuyExactQuoteIn,
    Sell,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum LiquidityAction {
    Deposit,
    Withdraw,
}

#[derive(Debug, Clone, Serialize)]
pub struct SwapAccounts {
    pub pool: String,
    pub user: String,
    pub global_config: String,
    pub base_mint: String,
    pub quote_mint: String,
    pub user_base_token_account: String,
    pub user_quote_token_account: String,
    pub pool_base_token_account: String,
    pub pool_quote_token_account: String,
    pub protocol_fee_recipient: String,
    pub protocol_fee_recipient_token_account: String,
    pub base_token_program: String,
    pub quote_token_program: String,
    pub system_program: String,
    pub associated_token_program: String,
    pub event_authority: String,
    pub program: String,
    pub coin_creator_vault_ata: String,
    pub coin_creator_vault_authority: String,
    pub global_volume_accumulator: Option<String>,
    pub user_volume_accumulator: Option<String>,
    pub fee_config: String,
    pub fee_program: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct BuyIx {
    pub base_amount_out: u64,
    pub max_quote_amount_in: u64,
    pub track_volume: Option<bool>,
    pub accounts: SwapAccounts,
}

#[derive(Debug, Clone, Serialize)]
pub struct BuyExactQuoteInIx {
    pub spendable_quote_in: u64,
    pub min_base_amount_out: u64,
    pub track_volume: Option<bool>,
    pub accounts: SwapAccounts,
}

#[derive(Debug, Clone, Serialize)]
pub struct SellIx {
    pub base_amount_in: u64,
    pub min_quote_amount_out: u64,
    pub accounts: SwapAccounts,
}

#[derive(Debug, Clone, Serialize)]
pub struct CreatePoolAccounts {
    pub pool: String,
    pub global_config: String,
    pub creator: String,
    pub base_mint: String,
    pub quote_mint: String,
    pub lp_mint: String,
    pub user_base_token_account: String,
    pub user_quote_token_account: String,
    pub user_pool_token_account: String,
    pub pool_base_token_account: String,
    pub pool_quote_token_account: String,
    pub system_program: String,
    pub token_2022_program: String,
    pub base_token_program: String,
    pub quote_token_program: String,
    pub associated_token_program: String,
    pub event_authority: String,
    pub program: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct CreatePoolIx {
    pub index: u16,
    pub base_amount_in: u64,
    pub quote_amount_in: u64,
    pub coin_creator: String,
    pub is_mayhem_mode: bool,
    pub is_cashback_coin: Option<bool>,
    pub accounts: CreatePoolAccounts,
}

#[derive(Debug, Clone, Serialize)]
pub struct LiquidityAccounts {
    pub pool: String,
    pub global_config: String,
    pub user: String,
    pub base_mint: String,
    pub quote_mint: String,
    pub lp_mint: String,
    pub user_base_token_account: String,
    pub user_quote_token_account: String,
    pub user_pool_token_account: String,
    pub pool_base_token_account: String,
    pub pool_quote_token_account: String,
    pub token_program: String,
    pub token_2022_program: String,
    pub event_authority: String,
    pub program: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct DepositIx {
    pub lp_token_amount_out: u64,
    pub max_base_amount_in: u64,
    pub max_quote_amount_in: u64,
    pub accounts: LiquidityAccounts,
}

#[derive(Debug, Clone, Serialize)]
pub struct WithdrawIx {
    pub lp_token_amount_in: u64,
    pub min_base_amount_out: u64,
    pub min_quote_amount_out: u64,
    pub accounts: LiquidityAccounts,
}

#[derive(Debug, Clone, Serialize, PartialEq)]
pub struct BuyEvent {
    pub timestamp: i64,
    pub base_amount_out: u64,
    pub max_quote_amount_in: u64,
    pub user_base_token_reserves: u64,
    pub user_quote_token_reserves: u64,
    pub pool_base_token_reserves: u64,
    pub pool_quote_token_reserves: u64,
    pub quote_amount_in: u64,
    pub lp_fee_basis_points: u64,
    pub lp_fee: u64,
    pub protocol_fee_basis_points: u64,
    pub protocol_fee: u64,
    pub quote_amount_in_with_lp_fee: u64,
    pub user_quote_amount_in: u64,
    pub pool: String,
    pub user: String,
    pub user_base_token_account: String,
    pub user_quote_token_account: String,
    pub protocol_fee_recipient: String,
    pub protocol_fee_recipient_token_account: String,
    pub coin_creator: String,
    pub coin_creator_fee_basis_points: u64,
    pub coin_creator_fee: u64,
    pub track_volume: bool,
    pub total_unclaimed_tokens: u64,
    pub total_claimed_tokens: u64,
    pub current_sol_volume: u64,
    pub last_update_timestamp: i64,
    pub min_base_amount_out: u64,
    pub ix_name: String,
    pub cashback_fee_basis_points: u64,
    pub cashback: u64,
}

#[derive(Debug, Clone, Serialize, PartialEq)]
pub struct SellEvent {
    pub timestamp: i64,
    pub base_amount_in: u64,
    pub min_quote_amount_out: u64,
    pub user_base_token_reserves: u64,
    pub user_quote_token_reserves: u64,
    pub pool_base_token_reserves: u64,
    pub pool_quote_token_reserves: u64,
    pub quote_amount_out: u64,
    pub lp_fee_basis_points: u64,
    pub lp_fee: u64,
    pub protocol_fee_basis_points: u64,
    pub protocol_fee: u64,
    pub quote_amount_out_without_lp_fee: u64,
    pub user_quote_amount_out: u64,
    pub pool: String,
    pub user: String,
    pub user_base_token_account: String,
    pub user_quote_token_account: String,
    pub protocol_fee_recipient: String,
    pub protocol_fee_recipient_token_account: String,
    pub coin_creator: String,
    pub coin_creator_fee_basis_points: u64,
    pub coin_creator_fee: u64,
    pub cashback_fee_basis_points: u64,
    pub cashback: u64,
}

#[derive(Debug, Clone, Serialize, PartialEq)]
pub enum SwapEvent {
    Buy(BuyEvent),
    Sell(SellEvent),
}

#[derive(Debug, Clone, Serialize, PartialEq)]
pub struct CreatePoolEvent {
    pub timestamp: i64,
    pub index: u16,
    pub creator: String,
    pub base_mint: String,
    pub quote_mint: String,
    pub base_mint_decimals: u8,
    pub quote_mint_decimals: u8,
    pub base_amount_in: u64,
    pub quote_amount_in: u64,
    pub pool_base_amount: u64,
    pub pool_quote_amount: u64,
    pub minimum_liquidity: u64,
    pub initial_liquidity: u64,
    pub lp_token_amount_out: u64,
    pub pool_bump: u8,
    pub pool: String,
    pub lp_mint: String,
    pub user_base_token_account: String,
    pub user_quote_token_account: String,
    pub coin_creator: String,
    pub is_mayhem_mode: bool,
}

#[derive(Debug, Clone, Serialize, PartialEq)]
pub struct DepositEvent {
    pub timestamp: i64,
    pub lp_token_amount_out: u64,
    pub max_base_amount_in: u64,
    pub max_quote_amount_in: u64,
    pub user_base_token_reserves: u64,
    pub user_quote_token_reserves: u64,
    pub pool_base_token_reserves: u64,
    pub pool_quote_token_reserves: u64,
    pub base_amount_in: u64,
    pub quote_amount_in: u64,
    pub lp_mint_supply: u64,
    pub pool: String,
    pub user: String,
    pub user_base_token_account: String,
    pub user_quote_token_account: String,
    pub user_pool_token_account: String,
}

#[derive(Debug, Clone, Serialize, PartialEq)]
pub struct WithdrawEvent {
    pub timestamp: i64,
    pub lp_token_amount_in: u64,
    pub min_base_amount_out: u64,
    pub min_quote_amount_out: u64,
    pub user_base_token_reserves: u64,
    pub user_quote_token_reserves: u64,
    pub pool_base_token_reserves: u64,
    pub pool_quote_token_reserves: u64,
    pub base_amount_out: u64,
    pub quote_amount_out: u64,
    pub lp_mint_supply: u64,
    pub pool: String,
    pub user: String,
    pub user_base_token_account: String,
    pub user_quote_token_account: String,
    pub user_pool_token_account: String,
}

#[derive(Debug, Clone, Serialize, PartialEq)]
pub enum LiquidityEvent {
    Deposit(DepositEvent),
    Withdraw(WithdrawEvent),
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "snake_case")]
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
pub struct PumpAmmInvocation {
    pub source: InvocationSource,
    pub instruction: PumpAmmInstruction,
}

#[derive(Debug, Clone, Serialize)]
pub struct ParsedSwap {
    pub source: InvocationSource,
    pub event_source: EventOrigin,
    pub side: SwapSide,
    pub pool: String,
    pub user: String,
    pub base_mint: String,
    pub quote_mint: String,
    pub timestamp: i64,
    pub instruction: PumpAmmInstruction,
    pub event: SwapEvent,
}

#[derive(Debug, Clone, Serialize)]
pub struct ParsedPoolCreation {
    pub source: InvocationSource,
    pub event_source: EventOrigin,
    pub pool: String,
    pub creator: String,
    pub base_mint: String,
    pub quote_mint: String,
    pub lp_mint: String,
    pub base_amount_in: u64,
    pub quote_amount_in: u64,
    pub initial_liquidity: u64,
    pub coin_creator: String,
    pub is_mayhem_mode: bool,
    pub instruction: PumpAmmInstruction,
    pub event: CreatePoolEvent,
}

#[derive(Debug, Clone, Serialize)]
pub struct ParsedLiquidityAction {
    pub source: InvocationSource,
    pub event_source: EventOrigin,
    pub action: LiquidityAction,
    pub pool: String,
    pub user: String,
    pub base_mint: String,
    pub quote_mint: String,
    pub lp_mint: String,
    pub timestamp: i64,
    pub instruction: PumpAmmInstruction,
    pub event: LiquidityEvent,
}

#[derive(Debug, Clone, Serialize)]
pub struct SwapAnalysis {
    pub swaps: Vec<ParsedSwap>,
    pub unmatched_invocations: Vec<PumpAmmInvocation>,
    pub unmatched_events: Vec<SwapEvent>,
}

#[derive(Debug, Clone, Serialize)]
pub struct PoolCreationAnalysis {
    pub pool_creations: Vec<ParsedPoolCreation>,
    pub unmatched_invocations: Vec<PumpAmmInvocation>,
    pub unmatched_events: Vec<CreatePoolEvent>,
}

#[derive(Debug, Clone, Serialize)]
pub struct LiquidityAnalysis {
    pub actions: Vec<ParsedLiquidityAction>,
    pub unmatched_invocations: Vec<PumpAmmInvocation>,
    pub unmatched_events: Vec<LiquidityEvent>,
}
