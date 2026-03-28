# ADR-001: Capability reasoning semantics (class vs progression)

## Status
Accepted (2026-03-28)

## Context
`pod_capabilities` carries both **capability_class** (`declared` | `observed` | `effective`) and **state** (`detected` | `confirmed` | `exploited` | `chained`). Without explicit rules, teams mix “what the workload can do” with “how far along an attack narrative we think we are.”

## Decision
1. **Capability class** answers: *from which evidence family does this capability row derive?*
   - **declared**: static / inventory / admission / manifest-derived (no runtime proof required).
   - **observed**: runtime facts or signals prove the behavior occurred.
   - **effective**: fused / promoted conclusion used for risk and UI “current posture.”

2. **Progression state** (`state` column) answers: *how strong is Fortuna’s belief this capability is actionable or chained*, independent of class. It must not be used interchangeably with class.

3. **`derived_from` (JSON)** is a **contract**, not a dump. Preferred keys (extensible only via ADR):
   - `source`: `inventory` | `runtime_signal` | `runtime_incident` | `risk_rule` | `manual`
   - `refs`: object with optional `signal_types`, `incident_ids`, `event_ids`, `fact_ids`, `rule_ids`

4. Rulebook examples (non-exhaustive):
   - HostPath mount only → **declared** host exposure; runtime open on host path → may add **observed**; CSC promotion → **effective**.
   - Token path read at runtime → **observed** API/cred access; static SA binding alone → **declared** privilege context.

## Consequences
- Code: `core/pkg/capability/semantics.go` validates class + state strings.
- Docs: `FORTUNA_CAPABILITY_MODEL.md` links here as normative for G-SEM-01.
- Scoring / graph work must not proceed as “production truth” until promotion paths honor this ADR.

## Related
- ADR-005 (single-table storage)
- G-SEM-01 in `FORTUNA_RUNTIME_IMPLEMENTATION_BACKLOG.md`
