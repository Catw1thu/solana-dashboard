use crate::transaction_view::TransactionView;
use anyhow::Result;
use std::{fs, path::PathBuf};
use yellowstone_grpc_proto::geyser::SubscribeUpdateTransaction;

pub fn write_raw_sample(slot: u64, signature: &str, tx: &SubscribeUpdateTransaction) -> Result<()> {
    let dir = sample_dir("raw");
    fs::create_dir_all(&dir)?;

    let path = dir.join(format!("{}-{}.txt", slot, signature));
    let content = format!("{:#?}", tx);

    fs::write(&path, content)?;
    println!("wrote raw sample -> {}", path.display());
    Ok(())
}

pub fn write_transaction_view_sample(view: &TransactionView) -> Result<()> {
    let dir = sample_dir("views");
    fs::create_dir_all(&dir)?;

    let path = dir.join(format!("{}-{}.json", view.slot, view.signature));
    let content = serde_json::to_string_pretty(view)?;

    fs::write(&path, content)?;
    println!("wrote transaction view sample -> {}", path.display());
    Ok(())
}

fn sample_dir(kind: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("samples")
        .join("captured")
        .join(kind)
}
