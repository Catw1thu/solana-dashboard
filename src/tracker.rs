use std::collections::HashSet;

use anyhow::{Context, Result};
use redis::AsyncCommands;
use tokio_postgres::NoTls;

use crate::service_event::model::ServiceEventEnvelope;

const TRACKED_MINTS_KEY: &str = "tracked:mints";
const SOL_MINT: &str = "So11111111111111111111111111111111111111112";
const STAGE_BONDING_CURVE: &str = "bonding_curve";
const STAGE_AMM: &str = "amm";
const MARKET_TYPE_PUMPFUN_CURVE: &str = "pumpfun_curve";
const MARKET_TYPE_PUMPAMM_POOL: &str = "pumpamm_pool";

const LOAD_TRACKED_TOKENS_SQL: &str = r#"
select mint
from tracked_tokens
"#;

const UPSERT_TRACKED_TOKEN_SQL: &str = r#"
insert into tracked_tokens (
    mint,
    creator,
    bonding_curve,
    token_program,
    create_event_id,
    accepted_at,
    current_stage,
    current_market_type,
    current_market_id,
    migrated_at,
    updated_at
) values (
    $1,
    $2,
    $3,
    $4,
    $5,
    to_timestamp($6),
    $7,
    $8,
    $9,
    to_timestamp($10),
    now()
)
on conflict (mint) do update set
    creator = excluded.creator,
    bonding_curve = excluded.bonding_curve,
    token_program = excluded.token_program,
    updated_at = now()
"#;

const MARK_MIGRATED_SQL: &str = r#"
update tracked_tokens
set
    current_stage = $2,
    current_market_type = $3,
    current_market_id = $4,
    migrated_at = to_timestamp($5),
    updated_at = now()
where mint = $1
"#;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct MintKey([u8; 32]);

impl MintKey {
    pub fn parse(value: &str) -> Result<Self> {
        let decoded = bs58::decode(value)
            .into_vec()
            .with_context(|| format!("decode mint {value} from base58"))?;

        let bytes: [u8; 32] = decoded
            .try_into()
            .map_err(|_| anyhow::anyhow!("mint {value} is not 32 bytes"))?;

        Ok(Self(bytes))
    }
}

pub struct TrackedMintTracker {
    tracked_mints: HashSet<MintKey>,
    redis_client: redis::Client,
    pg_client: tokio_postgres::Client,
}

impl TrackedMintTracker {
    pub async fn new(database_url: &str, redis_url: &str) -> Result<Self> {
        let (pg_client, connection) = tokio_postgres::connect(database_url, NoTls).await?;
        tokio::spawn(async move {
            if let Err(err) = connection.await {
                eprintln!("tracked token postgres connection error: {err}");
            }
        });

        let redis_client = redis::Client::open(redis_url)?;

        let mut tracker = Self {
            tracked_mints: HashSet::new(),
            redis_client,
            pg_client,
        };
        tracker.bootstrap().await?;

        Ok(tracker)
    }

    pub fn should_forward(&self, event: &ServiceEventEnvelope) -> bool {
        let Some(mint) = tracked_mint_from_event(event) else {
            return false;
        };

        self.tracked_mints.contains(&mint)
    }

    pub async fn accept_create_event(&mut self, event: &ServiceEventEnvelope) -> Result<()> {
        let Some(mint_str) = event.refs.mint.as_deref() else {
            return Ok(());
        };
        let mint = MintKey::parse(mint_str)?;

        if !self.tracked_mints.insert(mint) {
            return Ok(());
        }

        self.write_mint_to_redis(mint_str).await?;
        self.upsert_tracked_token(event).await?;

        Ok(())
    }

    pub async fn record_migration_event(&self, event: &ServiceEventEnvelope) -> Result<()> {
        let Some(mint) = event.refs.mint.as_deref() else {
            return Ok(());
        };
        let pool = event.refs.pool.as_deref();
        let migrated_at = event.event_unix_ts as f64;

        self.pg_client
            .execute(
                MARK_MIGRATED_SQL,
                &[
                    &mint,
                    &STAGE_AMM,
                    &MARKET_TYPE_PUMPAMM_POOL,
                    &pool,
                    &migrated_at,
                ],
            )
            .await?;

        Ok(())
    }

    async fn bootstrap(&mut self) -> Result<()> {
        match self.load_from_redis().await {
            Ok(mints) if !mints.is_empty() => {
                self.tracked_mints = mints;
                return Ok(());
            }
            Ok(_) => {}
            Err(err) => {
                eprintln!(
                    "failed to load tracked mints from redis, falling back to postgres: {err}"
                );
            }
        }

        let mint_strings = self.load_tracked_mint_strings_from_postgres().await?;
        self.tracked_mints = mint_strings
            .iter()
            .filter_map(|mint| MintKey::parse(mint).ok())
            .collect();

        if !mint_strings.is_empty() {
            if let Err(err) = self.backfill_redis_strings(&mint_strings).await {
                eprintln!("failed to backfill tracked mints into redis: {err}");
            }
        }

        Ok(())
    }

    async fn load_from_redis(&self) -> Result<HashSet<MintKey>> {
        let mut connection = self.redis_client.get_multiplexed_async_connection().await?;
        let raw_mints: Vec<String> = connection.smembers(TRACKED_MINTS_KEY).await?;

        let mut mints = HashSet::with_capacity(raw_mints.len());
        for mint in raw_mints {
            if let Ok(key) = MintKey::parse(&mint) {
                mints.insert(key);
            }
        }

        Ok(mints)
    }

    async fn load_tracked_mint_strings_from_postgres(&self) -> Result<Vec<String>> {
        let rows = self.pg_client.query(LOAD_TRACKED_TOKENS_SQL, &[]).await?;
        Ok(rows.into_iter().map(|row| row.get("mint")).collect())
    }

    async fn backfill_redis_strings(&self, mint_strings: &[String]) -> Result<()> {
        if mint_strings.is_empty() {
            return Ok(());
        }

        let mut connection = self.redis_client.get_multiplexed_async_connection().await?;
        let _: usize = connection.sadd(TRACKED_MINTS_KEY, mint_strings).await?;
        Ok(())
    }

    async fn write_mint_to_redis(&self, mint: &str) -> Result<()> {
        let mut connection = self.redis_client.get_multiplexed_async_connection().await?;
        let _: usize = connection.sadd(TRACKED_MINTS_KEY, mint).await?;
        Ok(())
    }

    async fn upsert_tracked_token(&self, event: &ServiceEventEnvelope) -> Result<()> {
        let mint = event
            .refs
            .mint
            .as_deref()
            .context("create event missing mint ref")?;
        let creator = event.refs.creator.as_deref();
        let bonding_curve = event.refs.bonding_curve.as_deref();
        let token_program = event
            .payload
            .get("token_program")
            .and_then(|value| value.as_str());
        let accepted_at = event.event_unix_ts as f64;
        let current_market_id = bonding_curve;
        let migrated_at: Option<f64> = None;

        self.pg_client
            .execute(
                UPSERT_TRACKED_TOKEN_SQL,
                &[
                    &mint,
                    &creator,
                    &bonding_curve,
                    &token_program,
                    &event.event_id,
                    &accepted_at,
                    &STAGE_BONDING_CURVE,
                    &MARKET_TYPE_PUMPFUN_CURVE,
                    &current_market_id,
                    &migrated_at,
                ],
            )
            .await?;

        Ok(())
    }
}

pub fn tracked_mint_from_event(event: &ServiceEventEnvelope) -> Option<MintKey> {
    if let Some(mint) = event.refs.mint.as_deref() {
        return MintKey::parse(mint).ok();
    }

    let base_mint = event.refs.base_mint.as_deref()?;
    let quote_mint = event.refs.quote_mint.as_deref()?;

    resolve_non_sol_mint(base_mint, quote_mint).and_then(|mint| MintKey::parse(mint).ok())
}

pub fn resolve_non_sol_mint<'a>(base_mint: &'a str, quote_mint: &'a str) -> Option<&'a str> {
    match (base_mint == SOL_MINT, quote_mint == SOL_MINT) {
        (true, false) => Some(quote_mint),
        (false, true) => Some(base_mint),
        _ => None,
    }
}

pub fn is_create_event(event: &ServiceEventEnvelope) -> bool {
    event.protocol == crate::service_event::model::ServiceEventProtocol::Pumpfun
        && event.event_type == crate::service_event::model::ServiceEventType::Create
}

pub fn is_migrate_event(event: &ServiceEventEnvelope) -> bool {
    event.protocol == crate::service_event::model::ServiceEventProtocol::Pumpfun
        && event.event_type == crate::service_event::model::ServiceEventType::Migrate
}

#[cfg(test)]
mod tests {
    use super::{MintKey, resolve_non_sol_mint};

    #[test]
    fn parses_compact_mint_key() {
        let mint = MintKey::parse("So11111111111111111111111111111111111111112").unwrap();
        assert_eq!(
            mint,
            MintKey::parse("So11111111111111111111111111111111111111112").unwrap()
        );
    }

    #[test]
    fn resolves_non_sol_side() {
        assert_eq!(
            resolve_non_sol_mint(
                "So11111111111111111111111111111111111111112",
                "FWRmWAueX7BH8CcMcTso18hbciYgiSdZL6ko7AwUpump"
            ),
            Some("FWRmWAueX7BH8CcMcTso18hbciYgiSdZL6ko7AwUpump")
        );
        assert_eq!(
            resolve_non_sol_mint(
                "FWRmWAueX7BH8CcMcTso18hbciYgiSdZL6ko7AwUpump",
                "So11111111111111111111111111111111111111112"
            ),
            Some("FWRmWAueX7BH8CcMcTso18hbciYgiSdZL6ko7AwUpump")
        );
    }
}
