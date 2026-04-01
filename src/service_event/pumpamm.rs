use super::model::{
    SERVICE_EVENT_SCHEMA_VERSION, ServiceCommitmentLevel, ServiceEventChain, ServiceEventEnvelope,
    ServiceEventProtocol, ServiceEventRefs, ServiceEventType, ServiceInstructionPath,
    ServiceInstructionSource, build_service_event_id,
};
use crate::{
    pumpamm::model::{
        LiquidityAction, LiquidityEvent, ParsedLiquidityAction, ParsedPoolCreation, ParsedSwap,
        PumpAmmInstruction, SwapEvent, SwapSide,
    },
    transaction_view::TransactionView,
};
use serde::Serialize;

#[derive(Debug, Serialize)]
struct PumpAmmSwapInstructionArgs {
    base_amount_in: Option<String>,
    min_quote_amount_out: Option<String>,
    base_amount_out: Option<String>,
    max_quote_amount_in: Option<String>,
    spendable_quote_in: Option<String>,
    min_base_amount_out: Option<String>,
}

#[derive(Debug, Serialize)]
struct PumpAmmSwapPayload {
    side: String,
    ix_name: String,
    pool: String,
    user: String,
    base_mint: String,
    quote_mint: String,
    coin_creator: String,
    base_amount_in: Option<String>,
    base_amount_out: Option<String>,
    quote_amount_in: Option<String>,
    quote_amount_out: Option<String>,
    lp_fee: String,
    protocol_fee: String,
    coin_creator_fee: String,
    cashback: String,
    pool_base_token_reserves: String,
    pool_quote_token_reserves: String,
    instruction_args: PumpAmmSwapInstructionArgs,
}

#[derive(Debug, Serialize)]
struct PumpAmmCreatePoolPayload {
    pool: String,
    creator: String,
    base_mint: String,
    quote_mint: String,
    lp_mint: String,
    base_amount_in: String,
    quote_amount_in: String,
    initial_liquidity: String,
    coin_creator: String,
    is_mayhem_mode: bool,
    instruction_args: PumpAmmCreatePoolInstructionArgs,
}

#[derive(Debug, Serialize)]
struct PumpAmmCreatePoolInstructionArgs {
    index: u16,
    coin_creator: String,
    is_mayhem_mode: bool,
    is_cashback_coin: Option<bool>,
}

#[derive(Debug, Serialize)]
struct PumpAmmLiquidityPayload {
    action: String,
    pool: String,
    user: String,
    base_mint: String,
    quote_mint: String,
    lp_mint: String,
    lp_token_amount_in: Option<String>,
    lp_token_amount_out: Option<String>,
    base_amount_in: Option<String>,
    quote_amount_in: Option<String>,
    base_amount_out: Option<String>,
    quote_amount_out: Option<String>,
    lp_mint_supply: String,
    instruction_args: PumpAmmLiquidityInstructionArgs,
}

#[derive(Debug, Serialize)]
struct PumpAmmLiquidityInstructionArgs {
    lp_token_amount_in: Option<String>,
    lp_token_amount_out: Option<String>,
    max_base_amount_in: Option<String>,
    max_quote_amount_in: Option<String>,
    min_base_amount_out: Option<String>,
    min_quote_amount_out: Option<String>,
}

pub fn build_pumpamm_swap_service_event(
    view: &TransactionView,
    swap: &ParsedSwap,
) -> ServiceEventEnvelope {
    let instruction_path = build_instruction_path(&swap.source);
    let refs = ServiceEventRefs {
        mint: None,
        pool: Some(swap.pool.clone()),
        bonding_curve: None,
        user: Some(swap.user.clone()),
        creator: Some(extract_coin_creator(&swap.event)),
        base_mint: Some(swap.base_mint.clone()),
        quote_mint: Some(swap.quote_mint.clone()),
        lp_mint: None,
    };
    let payload = build_payload(swap);
    let protocol = ServiceEventProtocol::Pumpamm;
    let event_type = ServiceEventType::Swap;

    ServiceEventEnvelope {
        schema_version: SERVICE_EVENT_SCHEMA_VERSION,
        event_id: build_service_event_id(
            protocol.clone(),
            event_type.clone(),
            &view.signature,
            &instruction_path,
        ),
        chain: ServiceEventChain::Solana,
        protocol,
        event_type,
        commitment: ServiceCommitmentLevel::Processed,
        slot: view.slot,
        tx_signature: view.signature.clone(),
        tx_index: view.tx_index,
        instruction_path,
        event_source: swap.event_source,
        event_unix_ts: swap.timestamp,
        refs,
        payload: serde_json::to_value(payload).expect("pumpamm swap payload must serialize"),
    }
}

pub fn build_pumpamm_create_pool_service_event(
    view: &TransactionView,
    creation: &ParsedPoolCreation,
) -> ServiceEventEnvelope {
    let instruction_path = build_instruction_path(&creation.source);
    let refs = ServiceEventRefs {
        mint: None,
        pool: Some(creation.pool.clone()),
        bonding_curve: None,
        user: None,
        creator: Some(creation.creator.clone()),
        base_mint: Some(creation.base_mint.clone()),
        quote_mint: Some(creation.quote_mint.clone()),
        lp_mint: Some(creation.lp_mint.clone()),
    };
    let payload = PumpAmmCreatePoolPayload {
        pool: creation.pool.clone(),
        creator: creation.creator.clone(),
        base_mint: creation.base_mint.clone(),
        quote_mint: creation.quote_mint.clone(),
        lp_mint: creation.lp_mint.clone(),
        base_amount_in: creation.base_amount_in.to_string(),
        quote_amount_in: creation.quote_amount_in.to_string(),
        initial_liquidity: creation.initial_liquidity.to_string(),
        coin_creator: creation.coin_creator.clone(),
        is_mayhem_mode: creation.is_mayhem_mode,
        instruction_args: match &creation.instruction {
            PumpAmmInstruction::CreatePool(ix) => PumpAmmCreatePoolInstructionArgs {
                index: ix.index,
                coin_creator: ix.coin_creator.clone(),
                is_mayhem_mode: ix.is_mayhem_mode,
                is_cashback_coin: ix.is_cashback_coin,
            },
            _ => unreachable!("pool creation must come from create_pool instruction"),
        },
    };
    let protocol = ServiceEventProtocol::Pumpamm;
    let event_type = ServiceEventType::CreatePool;

    ServiceEventEnvelope {
        schema_version: SERVICE_EVENT_SCHEMA_VERSION,
        event_id: build_service_event_id(
            protocol.clone(),
            event_type.clone(),
            &view.signature,
            &instruction_path,
        ),
        chain: ServiceEventChain::Solana,
        protocol,
        event_type,
        commitment: ServiceCommitmentLevel::Processed,
        slot: view.slot,
        tx_signature: view.signature.clone(),
        tx_index: view.tx_index,
        instruction_path,
        event_source: creation.event_source,
        event_unix_ts: creation.event.timestamp,
        refs,
        payload: serde_json::to_value(payload)
            .expect("pumpamm create_pool payload must serialize"),
    }
}

pub fn build_pumpamm_liquidity_service_event(
    view: &TransactionView,
    action: &ParsedLiquidityAction,
) -> ServiceEventEnvelope {
    let instruction_path = build_instruction_path(&action.source);
    let refs = ServiceEventRefs {
        mint: None,
        pool: Some(action.pool.clone()),
        bonding_curve: None,
        user: Some(action.user.clone()),
        creator: None,
        base_mint: Some(action.base_mint.clone()),
        quote_mint: Some(action.quote_mint.clone()),
        lp_mint: Some(action.lp_mint.clone()),
    };
    let payload = build_liquidity_payload(action);
    let protocol = ServiceEventProtocol::Pumpamm;
    let event_type = match action.action {
        LiquidityAction::Deposit => ServiceEventType::Deposit,
        LiquidityAction::Withdraw => ServiceEventType::Withdraw,
    };

    ServiceEventEnvelope {
        schema_version: SERVICE_EVENT_SCHEMA_VERSION,
        event_id: build_service_event_id(
            protocol.clone(),
            event_type.clone(),
            &view.signature,
            &instruction_path,
        ),
        chain: ServiceEventChain::Solana,
        protocol,
        event_type,
        commitment: ServiceCommitmentLevel::Processed,
        slot: view.slot,
        tx_signature: view.signature.clone(),
        tx_index: view.tx_index,
        instruction_path,
        event_source: action.event_source,
        event_unix_ts: action.timestamp,
        refs,
        payload: serde_json::to_value(payload).expect("pumpamm liquidity payload must serialize"),
    }
}

fn build_instruction_path(source: &crate::pumpamm::model::InvocationSource) -> ServiceInstructionPath {
    match source {
        crate::pumpamm::model::InvocationSource::Outer { outer_index } => ServiceInstructionPath {
            source: ServiceInstructionSource::Outer,
            outer_index: *outer_index,
            inner_index: None,
        },
        crate::pumpamm::model::InvocationSource::Inner {
            outer_index,
            inner_index,
        } => ServiceInstructionPath {
            source: ServiceInstructionSource::Inner,
            outer_index: *outer_index,
            inner_index: Some(*inner_index),
        },
    }
}

fn build_liquidity_payload(action: &ParsedLiquidityAction) -> PumpAmmLiquidityPayload {
    let (lp_token_amount_in, lp_token_amount_out, base_amount_in, quote_amount_in, base_amount_out, quote_amount_out, lp_mint_supply) =
        match &action.event {
            LiquidityEvent::Deposit(event) => (
                None,
                Some(event.lp_token_amount_out.to_string()),
                Some(event.base_amount_in.to_string()),
                Some(event.quote_amount_in.to_string()),
                None,
                None,
                event.lp_mint_supply.to_string(),
            ),
            LiquidityEvent::Withdraw(event) => (
                Some(event.lp_token_amount_in.to_string()),
                None,
                None,
                None,
                Some(event.base_amount_out.to_string()),
                Some(event.quote_amount_out.to_string()),
                event.lp_mint_supply.to_string(),
            ),
        };

    let instruction_args = match &action.instruction {
        PumpAmmInstruction::Deposit(ix) => PumpAmmLiquidityInstructionArgs {
            lp_token_amount_in: None,
            lp_token_amount_out: Some(ix.lp_token_amount_out.to_string()),
            max_base_amount_in: Some(ix.max_base_amount_in.to_string()),
            max_quote_amount_in: Some(ix.max_quote_amount_in.to_string()),
            min_base_amount_out: None,
            min_quote_amount_out: None,
        },
        PumpAmmInstruction::Withdraw(ix) => PumpAmmLiquidityInstructionArgs {
            lp_token_amount_in: Some(ix.lp_token_amount_in.to_string()),
            lp_token_amount_out: None,
            max_base_amount_in: None,
            max_quote_amount_in: None,
            min_base_amount_out: Some(ix.min_base_amount_out.to_string()),
            min_quote_amount_out: Some(ix.min_quote_amount_out.to_string()),
        },
        _ => unreachable!("liquidity service event must come from deposit or withdraw"),
    };

    PumpAmmLiquidityPayload {
        action: match action.action {
            LiquidityAction::Deposit => "deposit".to_string(),
            LiquidityAction::Withdraw => "withdraw".to_string(),
        },
        pool: action.pool.clone(),
        user: action.user.clone(),
        base_mint: action.base_mint.clone(),
        quote_mint: action.quote_mint.clone(),
        lp_mint: action.lp_mint.clone(),
        lp_token_amount_in,
        lp_token_amount_out,
        base_amount_in,
        quote_amount_in,
        base_amount_out,
        quote_amount_out,
        lp_mint_supply,
        instruction_args,
    }
}

fn build_payload(swap: &ParsedSwap) -> PumpAmmSwapPayload {
    PumpAmmSwapPayload {
        side: match swap.side {
            SwapSide::Buy => "buy".to_string(),
            SwapSide::BuyExactQuoteIn => "buy_exact_quote_in".to_string(),
            SwapSide::Sell => "sell".to_string(),
        },
        ix_name: match &swap.event {
            SwapEvent::Buy(event) => event.ix_name.clone(),
            SwapEvent::Sell(_) => "sell".to_string(),
        },
        pool: swap.pool.clone(),
        user: swap.user.clone(),
        base_mint: swap.base_mint.clone(),
        quote_mint: swap.quote_mint.clone(),
        coin_creator: extract_coin_creator(&swap.event),
        base_amount_in: extract_base_amount_in(&swap.event),
        base_amount_out: extract_base_amount_out(&swap.event),
        quote_amount_in: extract_quote_amount_in(&swap.event),
        quote_amount_out: extract_quote_amount_out(&swap.event),
        lp_fee: extract_lp_fee(&swap.event),
        protocol_fee: extract_protocol_fee(&swap.event),
        coin_creator_fee: extract_coin_creator_fee(&swap.event),
        cashback: extract_cashback(&swap.event),
        pool_base_token_reserves: extract_pool_base_token_reserves(&swap.event),
        pool_quote_token_reserves: extract_pool_quote_token_reserves(&swap.event),
        instruction_args: build_instruction_args(&swap.instruction),
    }
}

fn build_instruction_args(instruction: &PumpAmmInstruction) -> PumpAmmSwapInstructionArgs {
    match instruction {
        PumpAmmInstruction::Buy(ix) => PumpAmmSwapInstructionArgs {
            base_amount_in: None,
            min_quote_amount_out: None,
            base_amount_out: Some(ix.base_amount_out.to_string()),
            max_quote_amount_in: Some(ix.max_quote_amount_in.to_string()),
            spendable_quote_in: None,
            min_base_amount_out: None,
        },
        PumpAmmInstruction::BuyExactQuoteIn(ix) => PumpAmmSwapInstructionArgs {
            base_amount_in: None,
            min_quote_amount_out: None,
            base_amount_out: None,
            max_quote_amount_in: None,
            spendable_quote_in: Some(ix.spendable_quote_in.to_string()),
            min_base_amount_out: Some(ix.min_base_amount_out.to_string()),
        },
        PumpAmmInstruction::Sell(ix) => PumpAmmSwapInstructionArgs {
            base_amount_in: Some(ix.base_amount_in.to_string()),
            min_quote_amount_out: Some(ix.min_quote_amount_out.to_string()),
            base_amount_out: None,
            max_quote_amount_in: None,
            spendable_quote_in: None,
            min_base_amount_out: None,
        },
        PumpAmmInstruction::CreatePool(_)
        | PumpAmmInstruction::Deposit(_)
        | PumpAmmInstruction::Withdraw(_) => PumpAmmSwapInstructionArgs {
            base_amount_in: None,
            min_quote_amount_out: None,
            base_amount_out: None,
            max_quote_amount_in: None,
            spendable_quote_in: None,
            min_base_amount_out: None,
        },
    }
}

fn extract_coin_creator(event: &SwapEvent) -> String {
    match event {
        SwapEvent::Buy(event) => event.coin_creator.clone(),
        SwapEvent::Sell(event) => event.coin_creator.clone(),
    }
}

fn extract_base_amount_in(event: &SwapEvent) -> Option<String> {
    match event {
        SwapEvent::Buy(_) => None,
        SwapEvent::Sell(event) => Some(event.base_amount_in.to_string()),
    }
}

fn extract_base_amount_out(event: &SwapEvent) -> Option<String> {
    match event {
        SwapEvent::Buy(event) => Some(event.base_amount_out.to_string()),
        SwapEvent::Sell(_) => None,
    }
}

fn extract_quote_amount_in(event: &SwapEvent) -> Option<String> {
    match event {
        SwapEvent::Buy(event) => Some(event.quote_amount_in.to_string()),
        SwapEvent::Sell(_) => None,
    }
}

fn extract_quote_amount_out(event: &SwapEvent) -> Option<String> {
    match event {
        SwapEvent::Buy(_) => None,
        SwapEvent::Sell(event) => Some(event.quote_amount_out.to_string()),
    }
}

fn extract_lp_fee(event: &SwapEvent) -> String {
    match event {
        SwapEvent::Buy(event) => event.lp_fee.to_string(),
        SwapEvent::Sell(event) => event.lp_fee.to_string(),
    }
}

fn extract_protocol_fee(event: &SwapEvent) -> String {
    match event {
        SwapEvent::Buy(event) => event.protocol_fee.to_string(),
        SwapEvent::Sell(event) => event.protocol_fee.to_string(),
    }
}

fn extract_coin_creator_fee(event: &SwapEvent) -> String {
    match event {
        SwapEvent::Buy(event) => event.coin_creator_fee.to_string(),
        SwapEvent::Sell(event) => event.coin_creator_fee.to_string(),
    }
}

fn extract_cashback(event: &SwapEvent) -> String {
    match event {
        SwapEvent::Buy(event) => event.cashback.to_string(),
        SwapEvent::Sell(event) => event.cashback.to_string(),
    }
}

fn extract_pool_base_token_reserves(event: &SwapEvent) -> String {
    match event {
        SwapEvent::Buy(event) => event.pool_base_token_reserves.to_string(),
        SwapEvent::Sell(event) => event.pool_base_token_reserves.to_string(),
    }
}

fn extract_pool_quote_token_reserves(event: &SwapEvent) -> String {
    match event {
        SwapEvent::Buy(event) => event.pool_quote_token_reserves.to_string(),
        SwapEvent::Sell(event) => event.pool_quote_token_reserves.to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::{
        build_pumpamm_create_pool_service_event, build_pumpamm_liquidity_service_event,
        build_pumpamm_swap_service_event,
    };
    use crate::{
        pumpamm::{extract_liquidity_actions, extract_pool_creations, extract_swaps},
        transaction_view::TransactionView,
    };

    fn load_fixture(file_name: &str) -> TransactionView {
        let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("samples")
            .join("tests")
            .join("views")
            .join(file_name);
        let content = std::fs::read_to_string(path).unwrap();
        serde_json::from_str(&content).unwrap()
    }

    #[test]
    fn pumpamm_swap_service_event_is_stable() {
        let view = load_fixture(
            "409587793-3sbc2gtmRvM5Jfr98dAzV2hwpKtAkLkKhgTGAqG6doxW29tYtiJ3TF2Xh9WNW9Aoq8hNCS5BaqfUPzu2MWo1jhyb.json",
        );
        let swap = extract_swaps(&view).remove(0);
        let service_event = build_pumpamm_swap_service_event(&view, &swap);

        assert_eq!(service_event.schema_version, 1);
        assert_eq!(service_event.tx_signature, view.signature);
        assert_eq!(service_event.slot, view.slot);
        assert_eq!(service_event.refs.pool.as_deref(), Some(swap.pool.as_str()));
        assert_eq!(service_event.refs.user.as_deref(), Some(swap.user.as_str()));
        assert_eq!(
            service_event.refs.base_mint.as_deref(),
            Some(swap.base_mint.as_str())
        );
        assert_eq!(
            service_event.refs.quote_mint.as_deref(),
            Some(swap.quote_mint.as_str())
        );
        assert!(service_event.event_id.contains(":pumpamm:swap:"));
    }

    #[test]
    fn pumpamm_create_pool_service_event_is_stable() {
        let view = load_fixture(
            "409576108-3zCwTozsNVfMaSftorXKLbbdVAmNPaPy3oZXN5ch6eMBdYdKfoB9GAgsiwhAFq786wnYoP9Lv64XjC8LbaKnbijZ.json",
        );
        let creation = extract_pool_creations(&view).remove(0);
        let service_event = build_pumpamm_create_pool_service_event(&view, &creation);

        assert_eq!(service_event.schema_version, 1);
        assert_eq!(service_event.tx_signature, view.signature);
        assert_eq!(service_event.refs.pool.as_deref(), Some(creation.pool.as_str()));
        assert_eq!(
            service_event.refs.creator.as_deref(),
            Some(creation.creator.as_str())
        );
        assert_eq!(
            service_event.refs.base_mint.as_deref(),
            Some(creation.base_mint.as_str())
        );
        assert_eq!(
            service_event.refs.quote_mint.as_deref(),
            Some(creation.quote_mint.as_str())
        );
        assert_eq!(
            service_event.refs.lp_mint.as_deref(),
            Some(creation.lp_mint.as_str())
        );
        assert!(service_event.event_id.contains(":pumpamm:create_pool:"));
    }

    #[test]
    fn pumpamm_liquidity_service_event_is_stable() {
        let view = load_fixture(
            "409590250-yoENqqk48Fq9LJa8wS8RJdtTgYLsSuFXaFJDbbAsD7G1mnGgYgRJDEeyMWFmKwsJzN9GjoFfd5PTceDKawW65pw.json",
        );
        let action = extract_liquidity_actions(&view).remove(0);
        let service_event = build_pumpamm_liquidity_service_event(&view, &action);

        assert_eq!(service_event.schema_version, 1);
        assert_eq!(service_event.tx_signature, view.signature);
        assert_eq!(service_event.refs.pool.as_deref(), Some(action.pool.as_str()));
        assert_eq!(service_event.refs.user.as_deref(), Some(action.user.as_str()));
        assert_eq!(
            service_event.refs.base_mint.as_deref(),
            Some(action.base_mint.as_str())
        );
        assert_eq!(
            service_event.refs.quote_mint.as_deref(),
            Some(action.quote_mint.as_str())
        );
        assert_eq!(
            service_event.refs.lp_mint.as_deref(),
            Some(action.lp_mint.as_str())
        );
        assert!(service_event.event_id.contains(":pumpamm:withdraw:"));
    }
}
