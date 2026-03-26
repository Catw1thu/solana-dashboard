use crate::config::Config;
use anyhow::Result;
use futures::{channel::mpsc, Sink, Stream};
use std::{collections::HashMap, pin::Pin};
use tonic::transport::ClientTlsConfig;
use yellowstone_grpc_client::GeyserGrpcClient;
use yellowstone_grpc_proto::geyser::{
    CommitmentLevel, SubscribeRequest, SubscribeRequestFilterTransactions, SubscribeUpdate,
};

pub const PUMPFUN_PROGRAM_ID: &str = "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P";

pub type SubscribeSink = Pin<Box<dyn Sink<SubscribeRequest, Error = mpsc::SendError> + Send>>;
pub type UpdateStream = Pin<Box<dyn Stream<Item = Result<SubscribeUpdate, tonic::Status>> + Send>>;

pub struct Subscription {
    pub sink: SubscribeSink,
    pub stream: UpdateStream,
}

pub async fn subscribe_pumpfun(config: &Config) -> Result<Subscription> {
    let _ = rustls::crypto::ring::default_provider().install_default();

    let mut builder = GeyserGrpcClient::build_from_shared(config.grpc_endpoint.clone())?
        .x_token(Some(config.grpc_token.clone()))?
        .max_decoding_message_size(1024 * 1024 * 1024);

    builder = builder.tls_config(ClientTlsConfig::new().with_native_roots())?;

    let mut client = builder.connect().await?;
    println!("connected to yellowstone grpc");

    let request = build_subscribe_request();
    let (sink, stream) = client.subscribe_with_request(Some(request)).await?;
    println!("subscribed to pumpfun transactions");

    Ok(Subscription {
        sink: Box::pin(sink),
        stream: Box::pin(stream),
    })
}

fn build_subscribe_request() -> SubscribeRequest {
    let mut transactions = HashMap::new();

    transactions.insert(
        "pumpfun".to_string(),
        SubscribeRequestFilterTransactions {
            vote: Some(false),
            failed: Some(false),
            signature: None,
            account_include: vec![PUMPFUN_PROGRAM_ID.to_string()],
            account_exclude: vec![],
            account_required: vec![],
        },
    );

    SubscribeRequest {
        accounts: HashMap::new(),
        slots: HashMap::new(),
        transactions,
        transactions_status: HashMap::new(),
        blocks: HashMap::new(),
        blocks_meta: HashMap::new(),
        entry: HashMap::new(),
        commitment: Some(CommitmentLevel::Processed as i32),
        accounts_data_slice: vec![],
        ping: None,
        from_slot: None,
    }
}
