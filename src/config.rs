use anyhow::{Result, bail};

pub struct Config {
    pub grpc_endpoint: String,
    pub grpc_token: String,
}

pub fn load_config() -> Result<Config> {
    dotenvy::dotenv().ok();

    let grpc_endpoint = std::env::var("GRPC_ENDPOINT")?.trim().to_string();
    let grpc_token = std::env::var("GRPC_TOKEN")?.trim().to_string();

    if grpc_endpoint.is_empty() {
        bail!("GRPC_ENDPOINT is empty");
    }
    if grpc_token.is_empty() {
        bail!("GRPC_TOKEN is empty");
    }

    Ok(Config {
        grpc_endpoint,
        grpc_token,
    })
}
