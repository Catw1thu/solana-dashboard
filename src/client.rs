use crate::{config::Config, pumpamm::PUMP_AMM_PROGRAM_ID, pumpfun::PUMPFUN_PROGRAM_ID};
use anyhow::Result;
use futures::{Sink, SinkExt, Stream, channel::mpsc};
use std::{collections::HashMap, time::Duration};
use tonic::transport::ClientTlsConfig;
use yellowstone_grpc_client::GeyserGrpcClient;
use yellowstone_grpc_proto::geyser::{
    CommitmentLevel, SubscribeRequest, SubscribeRequestFilterTransactions, SubscribeRequestPing,
    SubscribeUpdate,
};

const CONNECT_TIMEOUT: Duration = Duration::from_secs(15);
const HTTP2_KEEPALIVE_INTERVAL: Duration = Duration::from_secs(15);
const KEEPALIVE_TIMEOUT: Duration = Duration::from_secs(5);
const TCP_KEEPALIVE_INTERVAL: Duration = Duration::from_secs(15);

pub type SubscribeSink = Box<dyn Sink<SubscribeRequest, Error = mpsc::SendError> + Unpin + Send>;
pub type UpdateStream =
    Box<dyn Stream<Item = Result<SubscribeUpdate, tonic::Status>> + Unpin + Send>;

pub struct Subscription {
    pub sink: SubscribeSink,
    pub stream: UpdateStream,
}

pub async fn subscribe_pump_ecosystem(config: &Config) -> Result<Subscription> {
    let _ = rustls::crypto::ring::default_provider().install_default();

    let mut builder = GeyserGrpcClient::build_from_shared(config.grpc_endpoint.clone())?
        .x_token(Some(config.grpc_token.clone()))?
        .max_decoding_message_size(1024 * 1024 * 1024)
        .connect_timeout(CONNECT_TIMEOUT)
        .http2_keep_alive_interval(HTTP2_KEEPALIVE_INTERVAL)
        .keep_alive_timeout(KEEPALIVE_TIMEOUT)
        .keep_alive_while_idle(true)
        .tcp_keepalive(Some(TCP_KEEPALIVE_INTERVAL))
        .tcp_nodelay(true);

    builder = builder.tls_config(ClientTlsConfig::new().with_native_roots())?;

    let mut client = builder.connect().await?;
    println!("connected to yellowstone grpc");

    let request = build_subscribe_request();
    let (sink, stream) = client.subscribe_with_request(Some(request)).await?;
    println!("subscribed to pump ecosystem transactions");

    Ok(Subscription {
        sink: Box::new(sink),
        stream: Box::new(stream),
    })
}

pub async fn reply_to_ping(sink: &mut SubscribeSink, id: i32) -> Result<()> {
    sink.send(SubscribeRequest {
        ping: Some(SubscribeRequestPing { id }),
        ..Default::default()
    })
    .await?;
    Ok(())
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

    transactions.insert(
        "pumpamm".to_string(),
        SubscribeRequestFilterTransactions {
            vote: Some(false),
            failed: Some(false),
            signature: None,
            account_include: vec![PUMP_AMM_PROGRAM_ID.to_string()],
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
