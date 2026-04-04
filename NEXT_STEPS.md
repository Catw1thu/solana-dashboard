# Go Next Steps

## Data Modeling

- Build the first token-centric read models on top of `service_events`, starting with the tracked-token lifecycle assumptions already enforced in Rust.
- Revisit `markets` and `trades` with more live data, especially around one mint spanning `pumpfun` and `pumpamm`.
- Add the next high-value table for dashboard queries, likely `token_market_state`.
- Decide when to introduce token metadata fields like name, symbol, and image separately from core lifecycle data.

## Ingest and Projection

- Make projector writes idempotent and easier to replay from `service_events`.
- Add projection logging/metrics so we can distinguish ingest failures from projection failures quickly.
- Define replay/backfill commands for rebuilding read models from stored events.
- Decide whether `service_events` remains the canonical tracked-event log or needs stricter retention/replay tooling.

## Performance Hotspots To Revisit

- Reduce per-event ingest logs on the hot path once we have metrics in place, because the current logs will become noisy and expensive under sustained tracked traffic.
- Revisit the single-goroutine JetStream consumer when tracked coverage expands, since it will cap throughput before protobuf decoding does.
- Replace projector polling with a lower-latency trigger or queue once read models and websocket topics need tighter freshness than the current poll interval.
- Revisit replay/backfill decoding cost if projector catch-up becomes slow, because replay currently decodes JSON payloads from `service_events`.
- Keep an eye on realtime fan-out and subscriber backpressure once `/ws` evolves beyond the current in-process broadcast hub.

## Realtime API

- Add room/topic subscriptions on top of the current WebSocket broadcast path.
- Define subscription keys around `mint`, `market`, and later page-specific feeds.
- Add connection metrics, subscriber counts, and drop/backpressure visibility for realtime delivery.

## Storage and Querying

- Add repository/query paths for dashboard pages instead of reading directly from raw event tables.
- Add `service_events` query indexes around `refs.mint` and `refs.pool` so token and market event feeds stay cheap as history grows.
- Evaluate TimescaleDB for time-window aggregations and candle/stat queries once `trades` stabilizes.
- Plan indexes only after we observe real query patterns from the first frontend pages.

## Product Surface

- Start the first API endpoints around tracked tokens, token detail, and recent trades.
- Keep the schema mint-centric so a token can show one continuous lifecycle from `pumpfun` to `pumpamm`.
- Delay low-priority metadata and cosmetic enrichment until the lifecycle/trade queries are stable.
