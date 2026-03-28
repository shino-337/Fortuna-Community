# ADR-003: `asset_security_state` — five schema groups (anti junk-drawer)

## Status
Accepted (2026-03-28)

## Context
`asset_security_state` is the risk-engine backbone. Ad-hoc columns or JSON keys invite inconsistent semantics and break CEL projections.

## Decision
All present and **future** fields on `asset_security_state` must map to exactly one of:

| Group | Purpose | Current columns / JSON |
|-------|---------|-------------------------|
| **identity_context** | Who runs the workload, privilege bindings | `namespace`, `cluster_id`, `service_account_bound_to_cluster_admin` |
| **exposure_context** | Host / namespace exposure from spec | `host_network`, `host_pid`, `host_ipc` |
| **software_risk_context** | CVE/SBOM-derived (future) | *(none in P0 row; add only with ADR)* |
| **runtime_security_context** | Live runtime posture | `signal_total_24h`, `has_suspicious_exec`, `has_network_queue_anomaly`, `has_escape_related`, `last_runtime_activity_at`, **`runtime_signals_by_type` JSON** |
| **effective_capability_context** | Summarized capability ids / classes | **`effective_capabilities` JSON** |

**Rule:** New SQL columns or JSON keys require an ADR amendment or new ADR; PR template should ask “which group?”

## Consequences
- Validation helper: `models.ValidateAssetSecurityStateDiscipline` checks JSON shape for the two JSON columns.
- Projector / migrations must not add columns without updating this ADR.

## Related
- G-SEM-03, `core/pkg/models/asset_security_state.go`
