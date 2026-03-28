# ADR-002: Runtime signal vs runtime incident lifecycle

## Status
Accepted (2026-03-28)

## Context
`runtime_signals` historically mixed **semantic assertions** with **dedupe/counter** behavior. REP-C adds **incidents** as stateful correlated conditions. We need one vocabulary for when to create, update, or escalate.

## Decision
1. **Runtime signal** = *semantic assertion* about observed behavior (possibly aggregated), with taxonomy id (`signal_type`), evidence refs, and lifecycle timestamps (`first_seen_at` / `last_seen_at`). It is **not** only a statistics row.

2. **Emit new signal row** when the assertion identity changes for the same asset + type + window policy (implementation-specific). **Update** the same row when the same assertion is reinforced (refresh `last_seen_at`, optional confidence bump).

3. **Runtime incident** = *stateful correlated hypothesis* over a window (multiple facts/signals), with stable **`incident_id`** = `pod_uid:INCIDENT_TYPE:time_bucket` (bucket size per detector). **Create/upsert** by `incident_id`; **suppress** duplicate buckets per ADR-004 cooldown policy.

4. **Escalation path (logical)**: facts → signals → incidents → capability promotion / `asset_security_state` projection. Incidents do not replace signals; they summarize multi-signal patterns.

## Source independence (audit matrix)

| REP-C detector / incident type | Falco-sourced facts | eBPF-sourced facts | Mixed |
|-------------------------------|--------------------|--------------------|-------|
| RECON_BURST | Yes (NETWORK_CONNECT / EXTERNAL_CONNECT facts) | Yes, if extractor emits same `fact_type` | Yes |
| POST_EXPLOIT_EXEC_CHAIN | Yes | Yes, same condition | Yes |
| EXFIL_LIKE_SEQUENCE | Yes | Yes, same ordering rules | Yes |

Detectors must **only** depend on `fact_type` + timestamps + pod scope, not on `source_kind` in hot path.

## Consequences
- Registry metadata references this ADR for correlators (`core/pkg/rep/detector_registry.go`).
- `FORTUNA_RUNTIME_SIGNAL_MODEL.md` links here for G-SEM-02.

## Related
- ADR-004
- `FORTUNA_RUNTIME_EVENT_MODEL.md`
