mod client;
mod config;
mod pumpamm;
mod pumpfun;
mod transaction_view;
mod writer;

use anyhow::Result;
use client::subscribe_pump_ecosystem;
use config::load_config;
use futures::StreamExt;
use pumpamm::{extract_liquidity_actions, extract_pool_creations, extract_swaps};
use pumpfun::{extract_creates, extract_migrations, extract_trades};
use tokio::time::{Duration, Instant};
use transaction_view::build_transaction_view;
use writer::{write_raw_sample, write_transaction_view_sample};
use yellowstone_grpc_proto::geyser::{SubscribeUpdateTransaction, subscribe_update::UpdateOneof};

#[allow(dead_code)]
fn print_tx_summary(tx: &SubscribeUpdateTransaction) {
    let Some(info) = &tx.transaction else {
        return;
    };
    let Some(raw_tx) = &info.transaction else {
        return;
    };
    let Some(meta) = &info.meta else {
        return;
    };
    let Some(tx_message) = &raw_tx.message else {
        return;
    };

    let signature = bs58::encode(&info.signature).into_string();
    println!(
        "slot={} sig={} outer_ix={} inner_groups={} logs={}",
        tx.slot,
        signature,
        tx_message.instructions.len(),
        meta.inner_instructions.len(),
        meta.log_messages.len(),
    );
}

fn persist_transaction_samples(tx: &SubscribeUpdateTransaction) -> Result<()> {
    let Some(info) = &tx.transaction else {
        return Ok(());
    };

    let signature = bs58::encode(&info.signature).into_string();
    write_raw_sample(tx.slot, &signature, tx)?;

    if let Some(view) = build_transaction_view(tx) {
        write_transaction_view_sample(&view)?;

        for create in extract_creates(&view) {
            println!("Parsed pumpfun create: {:?}", create);
        }

        for migration in extract_migrations(&view) {
            println!("Parsed pumpfun migrate: {:?}", migration);
        }

        for trade in extract_trades(&view) {
            println!("Parsed pumpfun trade: {:?}", trade);
        }

        for creation in extract_pool_creations(&view) {
            println!("Parsed pump_amm create_pool: {:?}", creation);
        }

        for action in extract_liquidity_actions(&view) {
            println!("Parsed pump_amm liquidity: {:?}", action);
        }

        for swap in extract_swaps(&view) {
            println!("Parsed pump_amm swap: {:?}", swap);
        }
    }

    Ok(())
}

#[tokio::main]
async fn main() -> Result<()> {
    let config = load_config()?;
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
                    persist_transaction_samples(&tx)?;
                }
                _ => {}
            }
        }
    }
    Ok(())
}
