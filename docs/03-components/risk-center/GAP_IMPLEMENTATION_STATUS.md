# GAP implementation status (sync with backlog §9)

Last updated: 2026-03-31 (iteration 3).

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
| **SBOM-REL-01** | DLQ replay orchestration (bounded retries) | `core/pkg/worker/sbom_dlq_worker.go`, `core/cmd/main.go` | `go test ./pkg/worker/... -run SBOMDLQ` |
| **SBOM-REL-02** | `SBOMMatchRun` explicit lifecycle + timeout/status/error_code | `core/pkg/models/sbom_match_run.go`, `core/internal/repository/sbom_repository.go`, `core/pkg/worker/cve_matcher_worker.go`, `core/migrations/111_*` | `go test ./internal/repository/...` |
| **SBOM-REL-03** | Configurable SBOM pod phase policy | `core/pkg/worker/sbom_worker.go` (`FORTUNA_SBOM_POD_PHASE_POLICY`) | `go test ./pkg/worker/... -run ShouldProcessSBOMForPhase` |
| **SBOM-REL-04** | Configurable orphan grace + runtime evidence guard before delete | `core/pkg/reconciler/sbom_reconciler.go` (`FORTUNA_SBOM_ORPHAN_GRACE_PERIOD`) | `go test ./pkg/reconciler/...` |
| **SBOM-DATA-01** | Enum normalization + DB constraints for `sbom_source`/`confidence` | `core/pkg/models/sbom.go`, `core/internal/repository/sbom_repository.go`, `core/migrations/111_*` | `go test ./internal/repository/... -run NormalizeSourceAndConfidence` |
| **SBOM-PERF-01** | Remove N+1 CVE severity queries in SBOM list API | `core/internal/api/sbom_handlers.go` | `go test ./internal/api/...` |

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
