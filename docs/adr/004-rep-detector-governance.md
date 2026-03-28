# ADR-004: REP-C detector governance & replay expectations

## Status
Accepted (2026-03-28)

## Context
Stateful correlators need shared metadata (inputs, windows, confidence, cooldown) and **deterministic replay** for debugging and CI.

## Decision
1. Each REP-C correlator has a **registry entry** (`core/pkg/rep/detector_registry.go`): `id`, `version`, `incident_type`, input fact types, window label, minimum distinct evidence, `confidence`, `cooldown`, `expected_fp_class`.

2. **Confidence**
   - Facts: optional future per-fact confidence (not required for P0).
   - Signals: important for suppression / UI (existing signal path).
   - Incidents: **mandatory** on `runtime_incidents.confidence` (float 0–1).

3. **Confidence on suppress (G-REP-01)**  
   When an incident update is **suppressed** into the latest row (same pod + type within cooldown), confidence is merged from prior and incoming rows using evidence ref counts: more distinct evidence → small reinforcement; same evidence footprint → slight decay. See `core/pkg/rep/incident_confidence.go`.

4. **Replay / determinism**
   - **Idempotent key:** `incident_id` as defined per detector (bucketed wall time).
   - **Late events:** facts with `observed_at` inside the detector window participate in counts; ordering-sensitive detectors (e.g. exfil) use explicit timestamp compare.
   - **Duplicate replay:** ingesting the same fact set twice must not create extra incidents beyond upsert/suppression rules (see tests).

5. **Tests per detector (target bar):** positive, negative, suppression, duplicate replay, out-of-order (where applicable). P0 correlators meet at least positive + negative + suppression + duplicate replay for `RECON_BURST`.

## Consequences
- Metadata JSON written by correlators should include `detector_id` matching the registry entry’s `ID`.

## Related
- G-REP-GOV-01, `incident_correlator_test.go`
