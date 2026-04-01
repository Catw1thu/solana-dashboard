use anyhow::{Result, bail};

pub struct Config {
    pub grpc_endpoint: String,
    pub grpc_token: String,
    pub service_event_ingest_url: Option<String>,
    pub database_url: String,
    pub redis_url: String,
}

pub fn load_config() -> Result<Config> {
    dotenvy::dotenv().ok();

    let grpc_endpoint = std::env::var("GRPC_ENDPOINT")?.trim().to_string();
    let grpc_token = std::env::var("GRPC_TOKEN")?.trim().to_string();
    let service_event_ingest_url = std::env::var("SERVICE_EVENT_INGEST_URL")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty());
    let database_url = std::env::var("DATABASE_URL")?.trim().to_string();
    let redis_url = std::env::var("REDIS_URL")?.trim().to_string();

    if grpc_endpoint.is_empty() {
        bail!("GRPC_ENDPOINT is empty");
    }
    if grpc_token.is_empty() {
        bail!("GRPC_TOKEN is empty");
    }
    if database_url.is_empty() {
        bail!("DATABASE_URL is empty");
    }
    if redis_url.is_empty() {
        bail!("REDIS_URL is empty");
    }

    Ok(Config {
        grpc_endpoint,
        grpc_token,
        service_event_ingest_url,
        database_url,
        redis_url,
    })
}
