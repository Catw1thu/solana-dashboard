mod client;
mod config;
mod normalize;
mod pumpfun;
mod types;
mod writer;

use anyhow::Result;
use client::subscribe_pumpfun;
use config::load_config;
use futures::StreamExt;
use normalize::normalize_tx;
use pumpfun::extract_merged_trades;
use tokio::time::{Duration, Instant};
use writer::{write_normalized_sample, write_raw_sample};
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

    if let Some(view) = normalize_tx(tx) {
        write_normalized_sample(&view)?;

        for trade in extract_merged_trades(&view) {
            println!("Merged pumpfun trade: {:?}", trade);
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
    } = subscribe_pumpfun(&config).await?;

    let started = Instant::now();
    while started.elapsed() < Duration::from_secs(1) {
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
