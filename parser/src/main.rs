mod client;
mod config;
mod event_origin;
mod proto;
mod pumpamm;
mod pumpfun;
mod service_event;
mod tracker;
mod transaction_view;
mod unified_parser;
mod writer;

use anyhow::Result;
use base64::{Engine as _, engine::general_purpose::STANDARD};
use client::{reply_to_ping, subscribe_pump_ecosystem};
use config::load_config;
use futures::StreamExt;
use pumpfun::discriminators::{
    CREATE_EVENT_DISC as PUMPFUN_CREATE_EVENT_DISC,
    CREATE_V2_EVENT_DISC as PUMPFUN_CREATE_V2_EVENT_DISC,
};
use pumpfun::{
    PUMPFUN_PROGRAM_ID,
    discriminators::{
        BUY_EXACT_SOL_IN_IX_DISC, BUY_IX_DISC, CREATE_IX_DISC, CREATE_V2_IX_DISC, MIGRATE_IX_DISC,
        SELL_IX_DISC,
    },
    events::parse_create_event_bytes,
    instruction::parse_decoded_instruction as parse_pumpfun_instruction,
    model::PumpfunInstruction,
};
use service_event::{ServiceEventEmitter, collect_service_events};
use tokio::time::{Duration, Instant, sleep, sleep_until};
use tracker::{TrackedMintTracker, is_create_event, is_migrate_event};
use transaction_view::build_transaction_view;
use writer::{write_raw_sample, write_transaction_view_sample};
use yellowstone_grpc_proto::geyser::{SubscribeUpdateTransaction, subscribe_update::UpdateOneof};

const STABLE_SESSION_RESET: Duration = Duration::from_secs(60);
const PARSER_STATS_LOG_INTERVAL: Duration = Duration::from_secs(30);

#[derive(Default, Clone, Copy)]
struct TransactionStats {
    transaction_updates: u64,
    parsed_views: u64,
    collected_events: u64,
    forwarded_events: u64,
    create_events: u64,
    migrate_events: u64,
    pumpfun_trade_events: u64,
    pumpamm_events: u64,
    pumpfun_buy_ix: u64,
    pumpfun_sell_ix: u64,
    pumpfun_buy_exact_sol_ix: u64,
    pumpfun_create_ix: u64,
    pumpfun_create_v2_ix: u64,
    pumpfun_create_v2_parseable_ix: u64,
    pumpfun_create_event_bytes: u64,
    pumpfun_create_event_parseable: u64,
    pumpfun_migrate_ix: u64,
    pumpfun_other_ix: u64,
}

#[derive(Default)]
struct SessionStats {
    transaction_updates: u64,
    parsed_views: u64,
    collected_events: u64,
    forwarded_events: u64,
    create_events: u64,
    migrate_events: u64,
    pumpfun_trade_events: u64,
    pumpamm_events: u64,
    pumpfun_buy_ix: u64,
    pumpfun_sell_ix: u64,
    pumpfun_buy_exact_sol_ix: u64,
    pumpfun_create_ix: u64,
    pumpfun_create_v2_ix: u64,
    pumpfun_create_v2_parseable_ix: u64,
    pumpfun_create_event_bytes: u64,
    pumpfun_create_event_parseable: u64,
    pumpfun_migrate_ix: u64,
    pumpfun_other_ix: u64,
    pings: u64,
    pongs: u64,
}

impl SessionStats {
    fn add_transaction(&mut self, stats: TransactionStats) {
        self.transaction_updates += stats.transaction_updates;
        self.parsed_views += stats.parsed_views;
        self.collected_events += stats.collected_events;
        self.forwarded_events += stats.forwarded_events;
        self.create_events += stats.create_events;
        self.migrate_events += stats.migrate_events;
        self.pumpfun_trade_events += stats.pumpfun_trade_events;
        self.pumpamm_events += stats.pumpamm_events;
        self.pumpfun_buy_ix += stats.pumpfun_buy_ix;
        self.pumpfun_sell_ix += stats.pumpfun_sell_ix;
        self.pumpfun_buy_exact_sol_ix += stats.pumpfun_buy_exact_sol_ix;
        self.pumpfun_create_ix += stats.pumpfun_create_ix;
        self.pumpfun_create_v2_ix += stats.pumpfun_create_v2_ix;
        self.pumpfun_create_v2_parseable_ix += stats.pumpfun_create_v2_parseable_ix;
        self.pumpfun_create_event_bytes += stats.pumpfun_create_event_bytes;
        self.pumpfun_create_event_parseable += stats.pumpfun_create_event_parseable;
        self.pumpfun_migrate_ix += stats.pumpfun_migrate_ix;
        self.pumpfun_other_ix += stats.pumpfun_other_ix;
    }

    fn log(&self, uptime: Duration) {
        eprintln!(
            "parser heartbeat uptime={:.0}s tx={} views={} collected_events={} forwarded_events={} pumpfun_trades={} creates={} migrations={} pumpamm_events={} pumpfun_ix={{buy:{} sell:{} buy_exact_sol:{} create:{} create_v2:{} create_v2_parseable:{} migrate:{} other:{}}} pumpfun_create_event={{bytes:{} parseable:{}}} pings={} pongs={}",
            uptime.as_secs_f64(),
            self.transaction_updates,
            self.parsed_views,
            self.collected_events,
            self.forwarded_events,
            self.pumpfun_trade_events,
            self.create_events,
            self.migrate_events,
            self.pumpamm_events,
            self.pumpfun_buy_ix,
            self.pumpfun_sell_ix,
            self.pumpfun_buy_exact_sol_ix,
            self.pumpfun_create_ix,
            self.pumpfun_create_v2_ix,
            self.pumpfun_create_v2_parseable_ix,
            self.pumpfun_migrate_ix,
            self.pumpfun_other_ix,
            self.pumpfun_create_event_bytes,
            self.pumpfun_create_event_parseable,
            self.pings,
            self.pongs
        );
    }
}

async fn persist_transaction_samples(
    tx: &SubscribeUpdateTransaction,
    emitter: &ServiceEventEmitter,
    tracker: &mut TrackedMintTracker,
    capture_samples: bool,
) -> Result<TransactionStats> {
    let mut stats = TransactionStats {
        transaction_updates: 1,
        ..Default::default()
    };
    let parse_started = Instant::now();
    let Some(info) = &tx.transaction else {
        return Ok(stats);
    };

    let signature = bs58::encode(&info.signature).into_string();
    if capture_samples {
        write_raw_sample(tx.slot, &signature, tx)?;
    }

    let view_started = Instant::now();
    if let Some(view) = build_transaction_view(tx) {
        stats.parsed_views += 1;
        count_pumpfun_instruction_discriminators(&view, &mut stats);
        count_pumpfun_create_event_bytes(&view, &mut stats);
        let _view_elapsed = view_started.elapsed();
        if capture_samples {
            write_transaction_view_sample(&view)?;
        }

        let collect_started = Instant::now();
        let service_events = collect_service_events(&view);
        let _collect_elapsed = collect_started.elapsed();
        let _collected_event_count = service_events.len();
        stats.collected_events += service_events.len() as u64;
        let mut _forwarded_event_count = 0usize;
        let mut first_forward_elapsed_ms = None;
        let mut tracker_elapsed = Duration::ZERO;
        let mut accept_create_elapsed = Duration::ZERO;
        let mut should_forward_elapsed = Duration::ZERO;
        let mut record_migration_elapsed = Duration::ZERO;
        let mut emit_elapsed = Duration::ZERO;

        for service_event in service_events {
            let tracker_started = Instant::now();
            match (&service_event.protocol, &service_event.event_type) {
                (
                    service_event::model::ServiceEventProtocol::Pumpfun,
                    service_event::model::ServiceEventType::Trade,
                ) => stats.pumpfun_trade_events += 1,
                (service_event::model::ServiceEventProtocol::Pumpamm, _) => {
                    stats.pumpamm_events += 1
                }
                _ => {}
            }

            if is_create_event(&service_event) {
                stats.create_events += 1;
                let accept_create_started = Instant::now();
                if let Err(err) = tracker.accept_create_event(&service_event) {
                    eprintln!(
                        "failed to accept tracked token from create event {}: {err}",
                        service_event.event_id
                    );
                }
                accept_create_elapsed += accept_create_started.elapsed();
            }

            let should_forward_started = Instant::now();
            let should_forward = tracker.should_forward(&service_event);
            should_forward_elapsed += should_forward_started.elapsed();

            if !should_forward {
                tracker_elapsed += tracker_started.elapsed();
                continue;
            }

            if is_migrate_event(&service_event) {
                stats.migrate_events += 1;
                let record_migration_started = Instant::now();
                if let Err(err) = tracker.record_migration_event(&service_event).await {
                    eprintln!(
                        "failed to record migration for event {}: {err}",
                        service_event.event_id
                    );
                }
                record_migration_elapsed += record_migration_started.elapsed();
            }
            tracker_elapsed += tracker_started.elapsed();

            if first_forward_elapsed_ms.is_none() {
                first_forward_elapsed_ms = Some(parse_started.elapsed().as_secs_f64() * 1000.0);
            }

            let emit_started = Instant::now();
            // let json = serde_json::to_string(&service_event)?;
            //println!("Service event: {json}");
            emitter.emit(&service_event).await?;
            emit_elapsed += emit_started.elapsed();
            _forwarded_event_count += 1;
            stats.forwarded_events += 1;
        }

        // if collected_event_count > 0 {
        //     println!(
        //         "Parse timing: sig={} collected={} forwarded={} view_ms={:.3} collect_ms={:.3} tracker_ms={:.3} accept_create_ms={:.3} should_forward_ms={:.3} record_migration_ms={:.3} emit_ms={:.3} total_ms={:.3} first_forward_ms={}",
        //         signature,
        //         collected_event_count,
        //         forwarded_event_count,
        //         view_elapsed.as_secs_f64() * 1000.0,
        //         collect_elapsed.as_secs_f64() * 1000.0,
        //         tracker_elapsed.as_secs_f64() * 1000.0,
        //         accept_create_elapsed.as_secs_f64() * 1000.0,
        //         should_forward_elapsed.as_secs_f64() * 1000.0,
        //         record_migration_elapsed.as_secs_f64() * 1000.0,
        //         emit_elapsed.as_secs_f64() * 1000.0,
        //         parse_started.elapsed().as_secs_f64() * 1000.0,
        //         first_forward_elapsed_ms
        //             .map(|ms| format!("{ms:.3}"))
        //             .unwrap_or_else(|| "none".to_string())
        //     );
        // }
    }

    Ok(stats)
}

fn count_pumpfun_instruction_discriminators(
    view: &transaction_view::TransactionView,
    stats: &mut TransactionStats,
) {
    for ix in &view.outer_instructions {
        count_pumpfun_discriminator(
            &ix.program_id,
            &ix.data_prefix,
            &ix.data_base64,
            &ix.account_pubkeys,
            stats,
        );
    }
    for group in &view.inner_instruction_groups {
        for ix in &group.instructions {
            count_pumpfun_discriminator(
                &ix.program_id,
                &ix.data_prefix,
                &ix.data_base64,
                &ix.account_pubkeys,
                stats,
            );
        }
    }
}

fn count_pumpfun_discriminator(
    program_id: &str,
    data_prefix: &[u8],
    data_base64: &str,
    account_pubkeys: &[String],
    stats: &mut TransactionStats,
) {
    if program_id != PUMPFUN_PROGRAM_ID {
        return;
    }

    let Some(discriminator) = data_prefix.get(0..8) else {
        stats.pumpfun_other_ix += 1;
        return;
    };

    match discriminator {
        value if value == BUY_IX_DISC => stats.pumpfun_buy_ix += 1,
        value if value == SELL_IX_DISC => stats.pumpfun_sell_ix += 1,
        value if value == BUY_EXACT_SOL_IN_IX_DISC => stats.pumpfun_buy_exact_sol_ix += 1,
        value if value == CREATE_IX_DISC => stats.pumpfun_create_ix += 1,
        value if value == CREATE_V2_IX_DISC => {
            stats.pumpfun_create_v2_ix += 1;
            if let Ok(bytes) = STANDARD.decode(data_base64)
                && matches!(
                    parse_pumpfun_instruction(program_id, account_pubkeys, &bytes),
                    Some(PumpfunInstruction::CreateV2(_))
                )
            {
                stats.pumpfun_create_v2_parseable_ix += 1;
            }
        }
        value if value == MIGRATE_IX_DISC => stats.pumpfun_migrate_ix += 1,
        _ => stats.pumpfun_other_ix += 1,
    }
}

fn count_pumpfun_create_event_bytes(
    view: &transaction_view::TransactionView,
    stats: &mut TransactionStats,
) {
    for log in &view.log_messages {
        let Some(encoded) = log.strip_prefix("Program data: ") else {
            continue;
        };
        let Ok(bytes) = STANDARD.decode(encoded) else {
            continue;
        };
        count_create_event_bytes(&bytes, stats);
    }

    for group in &view.inner_instruction_groups {
        for ix in &group.instructions {
            if ix.program_id != PUMPFUN_PROGRAM_ID {
                continue;
            }
            let Ok(bytes) = STANDARD.decode(&ix.data_base64) else {
                continue;
            };
            count_create_event_bytes(&bytes, stats);
            if bytes.len() > 8 {
                count_create_event_bytes(&bytes[8..], stats);
            }
        }
    }
}

fn count_create_event_bytes(bytes: &[u8], stats: &mut TransactionStats) {
    let Some(discriminator) = bytes.get(0..8) else {
        return;
    };
    if discriminator != PUMPFUN_CREATE_EVENT_DISC && discriminator != PUMPFUN_CREATE_V2_EVENT_DISC {
        return;
    }

    stats.pumpfun_create_event_bytes += 1;
    if parse_create_event_bytes(bytes).is_some() {
        stats.pumpfun_create_event_parseable += 1;
    }
}

async fn run_subscription_session(
    config: &config::Config,
    emitter: &ServiceEventEmitter,
    tracker: &mut TrackedMintTracker,
    deadline: Instant,
) -> Result<()> {
    let client::Subscription {
        mut sink,
        mut stream,
    } = subscribe_pump_ecosystem(config).await?;
    let mut next_ping_id = 1_i32;
    let session_started = Instant::now();
    let mut next_stats_log = Instant::now() + PARSER_STATS_LOG_INTERVAL;
    let mut stats = SessionStats::default();

    loop {
        let message = tokio::select! {
            _ = sleep_until(deadline) => return Ok(()),
            _ = sleep_until(next_stats_log) => {
                stats.log(session_started.elapsed());
                next_stats_log = Instant::now() + PARSER_STATS_LOG_INTERVAL;
                continue;
            },
            message = stream.next() => message,
        };

        let Some(message) = message else {
            anyhow::bail!("yellowstone subscription stream ended");
        };

        let update = message?;
        match update.update_oneof {
            Some(UpdateOneof::Transaction(tx)) => {
                let tx_stats =
                    persist_transaction_samples(&tx, emitter, tracker, config.capture_samples)
                        .await?;
                stats.add_transaction(tx_stats);
            }
            Some(UpdateOneof::Ping(_)) => {
                stats.pings += 1;
                reply_to_ping(&mut sink, next_ping_id).await?;
                next_ping_id = if next_ping_id == i32::MAX {
                    1
                } else {
                    next_ping_id + 1
                };
            }
            Some(UpdateOneof::Pong(_)) => {
                stats.pongs += 1;
            }
            _ => {}
        }
    }
}

fn reconnect_delay(attempt: u32) -> Duration {
    let capped_attempt = attempt.min(5);
    let seconds = (1_u64 << capped_attempt).min(30);
    Duration::from_secs(seconds)
}

#[tokio::main]
async fn main() -> Result<()> {
    let config = load_config()?;
    let service_event_emitter = ServiceEventEmitter::new(config.nats_url.as_deref()).await?;
    let mut tracker = TrackedMintTracker::new(&config.database_url, &config.redis_url).await?;

    let listen_seconds = std::env::var("LISTEN_SECONDS")
        .ok()
        .and_then(|value| value.parse::<u64>().ok())
        .unwrap_or(1);

    let deadline = Instant::now() + Duration::from_secs(listen_seconds);
    let mut reconnect_attempt = 0_u32;

    while Instant::now() < deadline {
        let session_started = Instant::now();
        match run_subscription_session(&config, &service_event_emitter, &mut tracker, deadline)
            .await
        {
            Ok(()) => {
                if Instant::now() >= deadline {
                    break;
                }
            }
            Err(err) => {
                let session_uptime = session_started.elapsed();
                if session_uptime >= STABLE_SESSION_RESET {
                    reconnect_attempt = 0;
                }

                let wait_for = reconnect_delay(reconnect_attempt);
                reconnect_attempt = reconnect_attempt.saturating_add(1);
                eprintln!(
                    "yellowstone stream disconnected after {:.1}s: {err:#}",
                    session_uptime.as_secs_f64()
                );

                let now = Instant::now();
                if now >= deadline {
                    break;
                }

                let remaining = deadline - now;
                let sleep_for = wait_for.min(remaining);
                eprintln!(
                    "reconnecting to yellowstone in {:.1}s (attempt {})",
                    sleep_for.as_secs_f64(),
                    reconnect_attempt
                );
                sleep(sleep_for).await;
            }
        }
    }

    Ok(())
}
