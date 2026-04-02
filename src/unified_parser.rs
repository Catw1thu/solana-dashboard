use std::collections::BTreeMap;

use base64::{Engine as _, engine::general_purpose::STANDARD};

use crate::{
    event_origin::EventOrigin,
    pumpamm::{
        PUMP_AMM_PROGRAM_ID,
        constants::PUMP_AMM_PROGRAM_ID as PUMP_AMM_ID,
        discriminators::{
            BUY_EVENT_DISC as PUMPAMM_BUY_EVENT_DISC,
            CREATE_POOL_EVENT_DISC as PUMPAMM_CREATE_POOL_EVENT_DISC,
            DEPOSIT_EVENT_DISC as PUMPAMM_DEPOSIT_EVENT_DISC,
            SELL_EVENT_DISC as PUMPAMM_SELL_EVENT_DISC,
            WITHDRAW_EVENT_DISC as PUMPAMM_WITHDRAW_EVENT_DISC,
        },
        events::{
            parse_buy_event_bytes as parse_pumpamm_buy_event_bytes,
            parse_create_pool_event_bytes as parse_pumpamm_create_pool_event_bytes,
            parse_deposit_event_bytes as parse_pumpamm_deposit_event_bytes,
            parse_sell_event_bytes as parse_pumpamm_sell_event_bytes,
            parse_withdraw_event_bytes as parse_pumpamm_withdraw_event_bytes,
        },
        instruction::parse_decoded_instruction as parse_pumpamm_instruction,
        liquidity::{
            build_liquidity_action, build_pool_creation, event_matches_create_pool,
            event_matches_liquidity,
        },
        model::{
            CreatePoolEvent, LiquidityEvent, ParsedLiquidityAction, ParsedPoolCreation, ParsedSwap,
            PumpAmmInvocation, SwapEvent,
        },
        swap::{build_swap, event_matches_instruction as pumpamm_swap_matches},
    },
    pumpfun::{
        PUMPFUN_PROGRAM_ID,
        constants::PUMPFUN_PROGRAM_ID as PUMPFUN_ID,
        create::{build_create, event_matches_instruction as pumpfun_create_matches},
        discriminators::{
            CREATE_EVENT_DISC as PUMPFUN_CREATE_EVENT_DISC,
            CREATE_V2_EVENT_DISC as PUMPFUN_CREATE_V2_EVENT_DISC,
            MIGRATE_EVENT_DISC as PUMPFUN_MIGRATE_EVENT_DISC,
            TRADE_EVENT_DISC as PUMPFUN_TRADE_EVENT_DISC,
        },
        events::{
            parse_create_event_bytes, parse_migrate_event_bytes, parse_trade_event_bytes,
        },
        instruction::parse_decoded_instruction as parse_pumpfun_instruction,
        migrate::{build_migration, event_matches_instruction as pumpfun_migrate_matches},
        model::{CreateEvent, MigrateEvent, ParsedCreate, ParsedMigrate, ParsedTrade, PumpfunInvocation, TradeEvent},
        trade::{build_trade, event_matches_instruction as pumpfun_trade_matches},
    },
    transaction_view::{
        InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
    },
};

#[derive(Debug, Clone)]
pub enum ParsedEvent {
    PumpfunTrade(ParsedTrade),
    PumpfunCreate(ParsedCreate),
    PumpfunMigrate(ParsedMigrate),
    PumpAmmSwap(ParsedSwap),
    PumpAmmCreatePool(ParsedPoolCreation),
    PumpAmmLiquidity(ParsedLiquidityAction),
}

struct EventQueue<T> {
    seen: Vec<T>,
    pending: Vec<(EventOrigin, T)>,
}

impl<T> Default for EventQueue<T> {
    fn default() -> Self {
        Self {
            seen: Vec::new(),
            pending: Vec::new(),
        }
    }
}

impl<T: Clone + PartialEq> EventQueue<T> {
    fn push_unique(&mut self, origin: EventOrigin, event: T) {
        if self.seen.iter().any(|seen| seen == &event) {
            return;
        }

        self.seen.push(event.clone());
        self.pending.push((origin, event));
    }

    fn remove_first_match<F>(&mut self, predicate: F) -> Option<(EventOrigin, T)>
    where
        F: Fn(&T) -> bool,
    {
        let index = self
            .pending
            .iter()
            .position(|(_, event)| predicate(event))?;
        Some(self.pending.remove(index))
    }
}

#[derive(Default)]
struct PendingEvents {
    pumpfun_trades: EventQueue<TradeEvent>,
    pumpfun_creates: EventQueue<CreateEvent>,
    pumpfun_migrations: EventQueue<MigrateEvent>,
    pumpamm_swaps: EventQueue<SwapEvent>,
    pumpamm_create_pools: EventQueue<CreatePoolEvent>,
    pumpamm_liquidity: EventQueue<LiquidityEvent>,
}

enum UnifiedInvocation {
    Pumpfun(PumpfunInvocation),
    Pumpamm(PumpAmmInvocation),
}

pub fn parse_view(view: &TransactionView) -> Vec<ParsedEvent> {
    let mut pending_events = PendingEvents::default();
    collect_log_events(&view.log_messages, &mut pending_events);

    let mut parsed_events = Vec::new();
    let mut pending_inner_groups = view
        .inner_instruction_groups
        .iter()
        .map(|group| (group.outer_instruction_index as usize, group))
        .collect::<BTreeMap<_, _>>();

    for (outer_index, outer_ix) in view.outer_instructions.iter().enumerate() {
        let outer_invocation = parse_outer_invocation(outer_ix, outer_index);
        let inner_invocations = pending_inner_groups
            .remove(&outer_index)
            .map(|group| scan_inner_group(group, outer_index, &mut pending_events))
            .unwrap_or_default();

        if let Some(invocation) = outer_invocation {
            process_invocation(invocation, &mut pending_events, &mut parsed_events);
        }

        for invocation in inner_invocations {
            process_invocation(invocation, &mut pending_events, &mut parsed_events);
        }
    }

    for (outer_index, group) in pending_inner_groups {
        let inner_invocations = scan_inner_group(group, outer_index, &mut pending_events);
        for invocation in inner_invocations {
            process_invocation(invocation, &mut pending_events, &mut parsed_events);
        }
    }

    parsed_events
}

fn collect_log_events(logs: &[String], pending_events: &mut PendingEvents) {
    for log in logs {
        let Some(encoded) = log.strip_prefix("Program data: ") else {
            continue;
        };

        let Ok(bytes) = STANDARD.decode(encoded) else {
            continue;
        };

        collect_event_bytes(&bytes, EventOrigin::Logs, pending_events);
    }
}

fn parse_outer_invocation(ix: &OuterInstructionView, outer_index: usize) -> Option<UnifiedInvocation> {
    let bytes = STANDARD.decode(&ix.data_base64).ok()?;

    if ix.program_id == PUMPFUN_ID {
        let instruction = parse_pumpfun_instruction(&ix.program_id, &ix.account_pubkeys, &bytes)?;
        return Some(UnifiedInvocation::Pumpfun(PumpfunInvocation {
            source: crate::pumpfun::model::InvocationSource::Outer { outer_index },
            instruction,
        }));
    }

    if ix.program_id == PUMP_AMM_ID {
        let instruction = parse_pumpamm_instruction(&ix.program_id, &ix.account_pubkeys, &bytes)?;
        return Some(UnifiedInvocation::Pumpamm(PumpAmmInvocation {
            source: crate::pumpamm::model::InvocationSource::Outer { outer_index },
            instruction,
        }));
    }

    None
}

fn scan_inner_group(
    group: &InnerInstructionGroup,
    outer_index: usize,
    pending_events: &mut PendingEvents,
) -> Vec<UnifiedInvocation> {
    let mut invocations = Vec::new();

    for (inner_index, ix) in group.instructions.iter().enumerate() {
        let Ok(bytes) = STANDARD.decode(&ix.data_base64) else {
            continue;
        };

        collect_inner_instruction_events(&ix.program_id, &bytes, pending_events);

        if let Some(invocation) = parse_inner_invocation(ix, outer_index, inner_index, &bytes) {
            invocations.push(invocation);
        }
    }

    invocations
}

fn collect_inner_instruction_events(
    program_id: &str,
    bytes: &[u8],
    pending_events: &mut PendingEvents,
) {
    if program_id != PUMPFUN_PROGRAM_ID && program_id != PUMP_AMM_PROGRAM_ID {
        return;
    }

    collect_event_bytes(bytes, EventOrigin::InnerCpi, pending_events);
    if bytes.len() > 8 {
        collect_event_bytes(&bytes[8..], EventOrigin::InnerCpi, pending_events);
    }
}

fn parse_inner_invocation(
    ix: &InnerInstructionView,
    outer_index: usize,
    inner_index: usize,
    bytes: &[u8],
) -> Option<UnifiedInvocation> {
    if ix.program_id == PUMPFUN_ID {
        let instruction = parse_pumpfun_instruction(&ix.program_id, &ix.account_pubkeys, bytes)?;
        return Some(UnifiedInvocation::Pumpfun(PumpfunInvocation {
            source: crate::pumpfun::model::InvocationSource::Inner {
                outer_index,
                inner_index,
            },
            instruction,
        }));
    }

    if ix.program_id == PUMP_AMM_ID {
        let instruction = parse_pumpamm_instruction(&ix.program_id, &ix.account_pubkeys, bytes)?;
        return Some(UnifiedInvocation::Pumpamm(PumpAmmInvocation {
            source: crate::pumpamm::model::InvocationSource::Inner {
                outer_index,
                inner_index,
            },
            instruction,
        }));
    }

    None
}

fn process_invocation(
    invocation: UnifiedInvocation,
    pending_events: &mut PendingEvents,
    parsed_events: &mut Vec<ParsedEvent>,
) {
    match invocation {
        UnifiedInvocation::Pumpfun(invocation) => {
            process_pumpfun_invocation(invocation, pending_events, parsed_events);
        }
        UnifiedInvocation::Pumpamm(invocation) => {
            process_pumpamm_invocation(invocation, pending_events, parsed_events);
        }
    }
}

fn process_pumpfun_invocation(
    invocation: PumpfunInvocation,
    pending_events: &mut PendingEvents,
    parsed_events: &mut Vec<ParsedEvent>,
) {
    if invocation.instruction.is_trade() {
        if let Some((event_source, event)) = pending_events
            .pumpfun_trades
            .remove_first_match(|event| pumpfun_trade_matches(&invocation.instruction, event))
        {
            parsed_events.push(ParsedEvent::PumpfunTrade(build_trade(
                invocation,
                event_source,
                event,
            )));
        }
        return;
    }

    if invocation.instruction.is_create() {
        if let Some((event_source, event)) = pending_events
            .pumpfun_creates
            .remove_first_match(|event| pumpfun_create_matches(&invocation.instruction, event))
        {
            parsed_events.push(ParsedEvent::PumpfunCreate(build_create(
                invocation,
                event_source,
                event,
            )));
        }
        return;
    }

    if invocation.instruction.is_migrate() {
        if let Some((event_source, event)) = pending_events
            .pumpfun_migrations
            .remove_first_match(|event| pumpfun_migrate_matches(&invocation.instruction, event))
        {
            parsed_events.push(ParsedEvent::PumpfunMigrate(build_migration(
                invocation,
                event_source,
                event,
            )));
        }
    }
}

fn process_pumpamm_invocation(
    invocation: PumpAmmInvocation,
    pending_events: &mut PendingEvents,
    parsed_events: &mut Vec<ParsedEvent>,
) {
    if invocation.instruction.is_swap() {
        if let Some((event_source, event)) = pending_events
            .pumpamm_swaps
            .remove_first_match(|event| pumpamm_swap_matches(&invocation.instruction, event))
        {
            parsed_events.push(ParsedEvent::PumpAmmSwap(build_swap(
                invocation,
                event_source,
                event,
            )));
        }
        return;
    }

    if invocation.instruction.is_create_pool() {
        if let Some((event_source, event)) = pending_events
            .pumpamm_create_pools
            .remove_first_match(|event| event_matches_create_pool(&invocation.instruction, event))
        {
            parsed_events.push(ParsedEvent::PumpAmmCreatePool(build_pool_creation(
                invocation,
                event_source,
                event,
            )));
        }
        return;
    }

    if invocation.instruction.is_liquidity() {
        if let Some((event_source, event)) = pending_events
            .pumpamm_liquidity
            .remove_first_match(|event| event_matches_liquidity(&invocation.instruction, event))
        {
            parsed_events.push(ParsedEvent::PumpAmmLiquidity(build_liquidity_action(
                invocation,
                event_source,
                event,
            )));
        }
    }
}

fn collect_event_bytes(data: &[u8], origin: EventOrigin, pending_events: &mut PendingEvents) {
    let Some(discriminator) = data.get(0..8).and_then(|bytes| bytes.try_into().ok()) else {
        return;
    };

    match discriminator {
        PUMPFUN_TRADE_EVENT_DISC => {
            if let Some(event) = parse_trade_event_bytes(data) {
                pending_events.pumpfun_trades.push_unique(origin, event);
            }
        }
        PUMPFUN_CREATE_EVENT_DISC | PUMPFUN_CREATE_V2_EVENT_DISC => {
            if let Some(event) = parse_create_event_bytes(data) {
                pending_events.pumpfun_creates.push_unique(origin, event);
            }
        }
        PUMPFUN_MIGRATE_EVENT_DISC => {
            if let Some(event) = parse_migrate_event_bytes(data) {
                pending_events.pumpfun_migrations.push_unique(origin, event);
            }
        }
        PUMPAMM_BUY_EVENT_DISC => {
            if let Some(event) = parse_pumpamm_buy_event_bytes(data) {
                pending_events
                    .pumpamm_swaps
                    .push_unique(origin, SwapEvent::Buy(event));
            }
        }
        PUMPAMM_SELL_EVENT_DISC => {
            if let Some(event) = parse_pumpamm_sell_event_bytes(data) {
                pending_events
                    .pumpamm_swaps
                    .push_unique(origin, SwapEvent::Sell(event));
            }
        }
        PUMPAMM_CREATE_POOL_EVENT_DISC => {
            if let Some(event) = parse_pumpamm_create_pool_event_bytes(data) {
                pending_events.pumpamm_create_pools.push_unique(origin, event);
            }
        }
        PUMPAMM_DEPOSIT_EVENT_DISC => {
            if let Some(event) = parse_pumpamm_deposit_event_bytes(data) {
                pending_events
                    .pumpamm_liquidity
                    .push_unique(origin, LiquidityEvent::Deposit(event));
            }
        }
        PUMPAMM_WITHDRAW_EVENT_DISC => {
            if let Some(event) = parse_pumpamm_withdraw_event_bytes(data) {
                pending_events
                    .pumpamm_liquidity
                    .push_unique(origin, LiquidityEvent::Withdraw(event));
            }
        }
        _ => {}
    }
}
