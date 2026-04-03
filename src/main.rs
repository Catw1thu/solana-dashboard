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
use client::subscribe_pump_ecosystem;
use config::load_config;
use futures::StreamExt;
use service_event::{ServiceEventEmitter, collect_service_events};
use tokio::time::{Duration, Instant};
use tracker::{TrackedMintTracker, is_create_event, is_migrate_event};
use transaction_view::build_transaction_view;
use writer::{write_raw_sample, write_transaction_view_sample};
use yellowstone_grpc_proto::geyser::{SubscribeUpdateTransaction, subscribe_update::UpdateOneof};

async fn persist_transaction_samples(
    tx: &SubscribeUpdateTransaction,
    emitter: &ServiceEventEmitter,
    tracker: &mut TrackedMintTracker,
    capture_samples: bool,
) -> Result<()> {
    let parse_started = Instant::now();
    let Some(info) = &tx.transaction else {
        return Ok(());
    };

    let signature = bs58::encode(&info.signature).into_string();
    if capture_samples {
        write_raw_sample(tx.slot, &signature, tx)?;
    }

    let view_started = Instant::now();
    if let Some(view) = build_transaction_view(tx) {
        let view_elapsed = view_started.elapsed();
        if capture_samples {
            write_transaction_view_sample(&view)?;
        }

        let collect_started = Instant::now();
        let service_events = collect_service_events(&view);
        let collect_elapsed = collect_started.elapsed();
        let collected_event_count = service_events.len();
        let mut forwarded_event_count = 0usize;
        let mut first_forward_elapsed_ms = None;
        let mut tracker_elapsed = Duration::ZERO;
        let mut accept_create_elapsed = Duration::ZERO;
        let mut should_forward_elapsed = Duration::ZERO;
        let mut record_migration_elapsed = Duration::ZERO;
        let mut emit_elapsed = Duration::ZERO;

        for service_event in service_events {
            let tracker_started = Instant::now();
            if is_create_event(&service_event) {
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
            let json = serde_json::to_string(&service_event)?;
            println!("Service event: {json}");
            emitter.emit(&service_event).await?;
            emit_elapsed += emit_started.elapsed();
            forwarded_event_count += 1;
        }

        if collected_event_count > 0 {
            println!(
                "Parse timing: sig={} collected={} forwarded={} view_ms={:.3} collect_ms={:.3} tracker_ms={:.3} accept_create_ms={:.3} should_forward_ms={:.3} record_migration_ms={:.3} emit_ms={:.3} total_ms={:.3} first_forward_ms={}",
                signature,
                collected_event_count,
                forwarded_event_count,
                view_elapsed.as_secs_f64() * 1000.0,
                collect_elapsed.as_secs_f64() * 1000.0,
                tracker_elapsed.as_secs_f64() * 1000.0,
                accept_create_elapsed.as_secs_f64() * 1000.0,
                should_forward_elapsed.as_secs_f64() * 1000.0,
                record_migration_elapsed.as_secs_f64() * 1000.0,
                emit_elapsed.as_secs_f64() * 1000.0,
                parse_started.elapsed().as_secs_f64() * 1000.0,
                first_forward_elapsed_ms
                    .map(|ms| format!("{ms:.3}"))
                    .unwrap_or_else(|| "none".to_string())
            );
        }
    }

    Ok(())
}

#[tokio::main]
async fn main() -> Result<()> {
    let config = load_config()?;
    let service_event_emitter = ServiceEventEmitter::new(config.nats_url.as_deref()).await?;
    let mut tracker = TrackedMintTracker::new(&config.database_url, &config.redis_url).await?;
    let client::Subscription {
        sink: _sink,
        mut stream,
    } = subscribe_pump_ecosystem(&config).await?;

    let listen_seconds = std::env::var("LISTEN_SECONDS")
        .ok()
        .and_then(|value| value.parse::<u64>().ok())
        .unwrap_or(1);

    let started = Instant::now();
    while started.elapsed() < Duration::from_secs(listen_seconds) {
        if let Some(message) = stream.next().await {
            let update = message?;
            match update.update_oneof {
                Some(UpdateOneof::Transaction(tx)) => {
                    persist_transaction_samples(
                        &tx,
                        &service_event_emitter,
                        &mut tracker,
                        config.capture_samples,
                    )
                    .await?;
                }
                _ => {}
            }
        }
    }
    Ok(())
}
