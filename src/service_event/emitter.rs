use anyhow::{Context, Result};
use tokio::sync::mpsc;

use crate::service_event::{model::ServiceEventEnvelope, protobuf::encode_event};

const STREAM_NAME: &str = "SERVICE_EVENTS";
const STREAM_SUBJECT: &str = "solana.tracked.>";

struct PublishTask {
    event_id: String,
    subject: String,
    payload: Vec<u8>,
}

pub struct ServiceEventEmitter {
    publish_tx: Option<mpsc::UnboundedSender<PublishTask>>,
}

impl ServiceEventEmitter {
    pub async fn new(raw_url: Option<&str>) -> Result<Self> {
        let Some(url) = raw_url else {
            return Ok(Self { publish_tx: None });
        };

        let client = async_nats::connect(url)
            .await
            .with_context(|| format!("connect to nats at {url}"))?;
        let jetstream = async_nats::jetstream::new(client);
        jetstream
            .get_or_create_stream(async_nats::jetstream::stream::Config {
                name: STREAM_NAME.to_string(),
                subjects: vec![STREAM_SUBJECT.to_string()],
                ..Default::default()
            })
            .await
            .context("ensure jetstream stream")?;
        let (publish_tx, mut publish_rx) = mpsc::unbounded_channel::<PublishTask>();

        tokio::spawn(async move {
            while let Some(task) = publish_rx.recv().await {
                let publish = jetstream
                    .publish(task.subject.clone(), task.payload.into())
                    .await;
                match publish {
                    Ok(ack) => {
                        if let Err(err) = ack.await {
                            eprintln!(
                                "failed to ack jetstream publish for {} on {}: {err}",
                                task.event_id, task.subject
                            );
                        }
                    }
                    Err(err) => {
                        eprintln!(
                            "failed to publish service event {} to {}: {err}",
                            task.event_id, task.subject
                        );
                    }
                }
            }
        });

        Ok(Self {
            publish_tx: Some(publish_tx),
        })
    }

    pub async fn emit(&self, event: &ServiceEventEnvelope) -> Result<()> {
        let Some(publish_tx) = &self.publish_tx else {
            return Ok(());
        };

        let (subject, payload) = encode_event(event)?;
        publish_tx
            .send(PublishTask {
                event_id: event.event_id.clone(),
                subject,
                payload,
            })
            .map_err(|err| anyhow::anyhow!("queue nats publish task: {err}"))?;

        Ok(())
    }
}
