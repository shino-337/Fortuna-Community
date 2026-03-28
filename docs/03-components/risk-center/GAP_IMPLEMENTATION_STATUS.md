# GAP implementation status (sync with backlog §9)

Last updated: 2026-03-28 (iteration 2).

## Completed in this delivery

| ID | Deliverable | Code / docs | Tests |
|----|-------------|-------------|-------|
| **G-SEM-01** | Capability class vs progression ADR + validators | `docs/adr/001-*`, `FORTUNA_CAPABILITY_MODEL.md`, `core/pkg/capability/semantics.go` | `go test ./core/pkg/capability/...` |
| **G-SEM-02** | Signal/incident lifecycle + source matrix | `docs/adr/002-*`, `FORTUNA_RUNTIME_SIGNAL_MODEL.md` (mermaid fix + headings) | — (spec); correlator tests exercise behavior |
| **G-SEM-03** | Five schema groups + JSON validation | `docs/adr/003-*`, `core/pkg/models/asset_security_state_discipline.go`, projector calls `ValidateAssetSecurityStateJSON` | `go test ./core/pkg/models/... -run AssetSecurity` |
| **G-REP-GOV-01** | Detector registry + metadata in incidents | `core/pkg/rep/detector_registry.go`, `incident_correlator.go` uses registry | `go test ./core/pkg/rep/...` |
| **G-EXP-01** | Chain + enrich: `explanation_chain`; `?enrich=1` loads facts from DB | `explainability/insight_chain.go`, `enrich.go`, `insights_handlers.go` | `explainability` tests + `insights_explainability_test.go` |
| **G-RE-01** | Types + **preview**: `factors.score_v3_preview` (Pod + `asset_security_state`) | `riskengine/score_v3_types.go`, `risk/score_v3_preview.go`, `scorer.go` hook | `go test ./pkg/risk/... -run ScoreV3Preview`, `./pkg/riskengine/... -run MergeDimension` |
| **G-REP-01** | Confidence merge when suppressing duplicate incidents | `rep/incident_confidence.go` + `incident_correlator.go` | `go test ./pkg/rep/... -run MergeIncident` |
| **G-DB-01** | Decision: keep single table | `docs/adr/005-*` | — |
| **G-API-01** | (prior) v2 capabilities route | `routes_runtime.go`, dashboard | `pod_capability_handlers_sync_test.go` |
| **G-R10** (baseline) | Runbook + CI verify | `R10-ADMISSION-RUNBOOK.md`, workflow | script smoke |

## Deferred (multi-sprint / not done here)

| ID | Reason |
|----|--------|
| **G-RE-01** cut-over | Replace V2 total with v3 dimensions + persist `scorerVersion` beyond preview map |
| **G-REP-01** advanced | Per-detector dynamic cooldown from confidence (registry has static cooldown); time-based decay |
| **G-R9** | Real eBPF exec/connect + PID→pod |
| **G-R6** | Full multi-signal chain calibration |
| **G-R5** | Byte/flow per connection |
| **G-P2-COV / G-P2-GRAPH** | Formal coverage store + graph edges |
| **G-UI-01** | Pod Detail IA redesign |
| **G-E2E-01** | Full detector matrix automation |

## Suggested test command bundle
```bash
cd core && go test ./pkg/capability/... ./pkg/models/... ./pkg/rep/... ./pkg/explainability/... ./pkg/riskengine/... ./pkg/risk/... ./internal/api/... -count=1
```
