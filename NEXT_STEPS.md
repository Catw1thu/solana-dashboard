# Rust Next Steps

## Parser

- Split `accept_create_event` off the hot path so Redis/Postgres writes do not dominate create-event latency.
- Add a bounded async persistence queue for tracked-token side effects and define failure/retry behavior.
- Keep measuring `view_ms`, `collect_ms`, and tracker sub-stages while optimizing to avoid regressing parser accuracy.
- Add an optional benchmark mode that disables stdout `Service event` printing for cleaner latency measurements.
- Verify whether `build_transaction_view` can be made more allocation-efficient before changing semantics.

## Tracking

- Persist more lifecycle state for tracked tokens, especially market transitions and migration linkage.
- Decide whether tracker writes should be best-effort or guaranteed before forwarding downstream events.
- Add startup metrics for Redis hit, Postgres fallback, and tracked mint count.
- Add safeguards for repeated create events and duplicate persistence attempts.

## Emission

- Reuse HTTP connections when sending events to Go instead of per-event request setup.
- Consider batched emission mode for bursts, while keeping event ordering intact.
- Add delivery counters and failure counters around emitter calls.

## Validation

- Run longer live captures focused on create-heavy windows to validate tracker latency after async persistence changes.
- Add regression fixtures for transactions that mix create and immediate trade in the same transaction.
- Sample and inspect long-tail parser transactions to confirm they are caused by expected external I/O rather than parser regressions.

## Future Optimization Ideas

- Evaluate zero-copy or lower-copy parsing in the Rust pipeline, especially around instruction/event byte handling and string allocation.
- Evaluate storing tracked mints in a more compact representation end to end, not just inside the in-process `HashSet`.
- Consider separating measurement of gRPC receive, view build, parse, tracker, and emit into structured metrics.
