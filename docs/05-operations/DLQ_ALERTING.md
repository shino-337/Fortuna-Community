# DLQ Alerting Runbook

## What this covers

Dead letter queue (DLQ) alerting when the JetStream DLQ stream backlog grows beyond `AlertThreshold`.

## Trigger

In `core/pkg/worker/dlq.go`, the DLQ manager checks `StreamInfo.State.Msgs` for the configured DLQ stream.

An alert is emitted when:
- `message_count > AlertThreshold`

To avoid alert/log spam, alerts are rate-limited by a cooldown in-memory (default `5m`) while the DLQ remains over threshold.

## Signals

1. Prometheus metric
- `fortuna_dlq_threshold_exceeded_total{stream_name="<stream>"}` (counter)

2. Logs
- `[DLQ] ALERT: DLQ message count (...) exceeds threshold (...) (stream=...)`

## Where to adjust

Defaults:
- Stream: `fortuna-dlq`
- Threshold: `AlertThreshold: 100`
- Cooldown: `5m`

Configuration is defined in `core/pkg/worker/dlq.go` (`DefaultDLQConfig()` and `NewDLQManager()`).

## Operational response checklist

When this alert fires:
1. Verify which worker is failing (look at DLQ messages: `fortuna.dlq.<worker_name>`).
2. Triage the top `error_type` (`fatal|non-retryable|retryable`) from DLQ payloads.
3. Fix the underlying ingestion/matching bug, then re-drive or wait for backlog to drain.

