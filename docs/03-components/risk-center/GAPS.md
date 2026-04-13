# Risk Center — Known Gaps

## Summary

This document consolidates all known gaps in the Risk Center component with their current status, impact, and planned resolution. Gaps are tracked with IDs prefixed by their category: `G-SEM` (semantic), `G-REP` (runtime event processing), `G-RE` (risk engine), `G-EXP` (explainability), `G-DB` (database), `G-API` (API), `G-R` (roadmap/runtime), `G-UI` (UI/UX), `G-E2E` (testing), and `SBOM-*` (SBOM pipeline).

---

## Gap Details

### G-SEM-01: Capability class vs progression ADR + validators (P0) ✅

- **Status:** Completed
- **Impact:** Without formal separation of capability classes (declared/observed/effective) and progression states, the capability model lacked semantic rigor.
- **Planned resolution:** Delivered. ADR-001 documents semantics. Code: `core/pkg/capability/semantics.go` (`ValidateCapabilityClass`, `ValidateProgressionState`). Spec: `FORTUNA_CAPABILITY_MODEL.md`.
- **Tests:** `go test ./core/pkg/capability/...`

### G-SEM-02: Signal/incident lifecycle + source matrix (P0) ✅

- **Status:** Completed
- **Impact:** Signals and incidents lacked formalized lifecycle and source provenance tracking.
- **Planned resolution:** Delivered. ADR-002 documents lifecycle. Spec: `FORTUNA_RUNTIME_SIGNAL_MODEL.md`. Correlator tests exercise behavior.

### G-SEM-03: Five schema groups + JSON validation (P0) ✅

- **Status:** Completed
- **Impact:** `asset_security_state` lacked structured schema validation.
- **Planned resolution:** Delivered. ADR-003 documents schema groups. Code: `core/pkg/models/asset_security_state_discipline.go`, projector calls `ValidateAssetSecurityStateJSON`.
- **Tests:** `go test ./core/pkg/models/... -run AssetSecurity`

### G-REP-GOV-01: Detector registry + metadata in incidents (P0) ✅

- **Status:** Completed
- **Impact:** No centralized registry for detector metadata; incidents lacked provenance.
- **Planned resolution:** Delivered. Code: `core/pkg/rep/detector_registry.go`, `incident_correlator.go` uses registry.
- **Tests:** `go test ./core/pkg/rep/...`

### G-EXP-01: Explanation chain + enrich API (P1) ✅

- **Status:** Completed
- **Impact:** Insights were not explainable end-to-end (event → fact → signal → incident → capability → insight).
- **Planned resolution:** Delivered. `explanation_chain` in `core/pkg/explainability/insight_chain.go`, `enrich.go`. API: `?enrich=1` loads facts from DB. Handler: `insights_handlers.go`.
- **Tests:** `core/pkg/explainability/` tests + `insights_explainability_test.go`

### G-RE-01: Score v3 preview (P1) — Partially complete

- **Status:** Preview delivered; full cut-over deferred
- **Impact:** V2 scoring lacks factorized dimensions from `asset_security_state`. V3 preview provides `factors.score_v3_preview` per Pod but does not yet replace V2 total.
- **Planned resolution:** (Completed) Preview: `core/pkg/riskengine/score_v3_types.go`, `core/pkg/risk/score_v3_preview.go`, `scorer.go` hook. (Deferred) Replace V2 total with v3 dimensions and persist `scorerVersion` beyond preview map.
- **Tests:** `go test ./pkg/risk/... -run ScoreV3Preview`, `./pkg/riskengine/... -run MergeDimension`

### G-REP-01: Confidence merge for duplicate incidents (P1) — Partially complete

- **Status:** Basic merge delivered; advanced features deferred
- **Impact:** Duplicate incidents could inflate noise without confidence-aware suppression.
- **Planned resolution:** (Completed) `core/pkg/rep/incident_confidence.go` + `incident_correlator.go`. (Deferred) Per-detector dynamic cooldown from confidence (registry has static cooldown); time-based decay.
- **Tests:** `go test ./pkg/rep/... -run MergeIncident`

### G-DB-01: Capability table strategy (P0) ✅

- **Status:** Completed
- **Impact:** Needed a decision on single-table vs multi-table for capabilities.
- **Planned resolution:** Delivered. ADR-005 documents decision: keep single `pod_capabilities` table with `capability_class` column.

### G-API-01: v2 capabilities route (P1) ✅

- **Status:** Completed
- **Impact:** No v2 API for pod capabilities with new semantic model.
- **Planned resolution:** Delivered. `routes_runtime.go`, dashboard integration.
- **Tests:** `pod_capability_handlers_sync_test.go`

### G-R10: Admission risk gate baseline (P1) ✅

- **Status:** Completed
- **Impact:** No admission-level risk enforcement.
- **Planned resolution:** Delivered. Runbook, CI verify script, env vars (`ADMISSION_RISK_GATE_ENABLED`, `ADMISSION_RISK_GATE_MODE`).

### G-R9: Real eBPF exec/connect + PID→pod (P2) — Deferred

- **Status:** Deferred (multi-sprint)
- **Impact:** Runtime event collection relies on Falco; no native eBPF probes for direct syscall-level visibility with PID-to-pod resolution.
- **Planned resolution:** Roadmap item. Requires eBPF probe development and PID→pod mapping infrastructure.

### G-R6: Full multi-signal chain calibration (P2) — Deferred

- **Status:** Deferred
- **Impact:** Detector thresholds and confidence levels are not calibrated across multi-signal chains.
- **Planned resolution:** Requires production traffic data to tune. Planned after initial detector set stabilizes.

### G-R5: Byte/flow per connection (P2) — Deferred

- **Status:** Deferred
- **Impact:** Network signals lack volume/flow metrics (bytes transferred per connection), limiting exfiltration detection accuracy.
- **Planned resolution:** Requires eBPF probe enhancements (G-R9).

### G-UI-01: Pod Detail IA redesign (P2) — Deferred

- **Status:** Deferred
- **Impact:** Pod Detail page does not clearly separate runtime events, signals, incidents, capabilities, and insights into distinct layered views.
- **Planned resolution:** Planned for post-launch UX iteration.

### G-E2E-01: Full detector matrix automation (P2) — Deferred

- **Status:** Deferred
- **Impact:** No automated E2E coverage for all detectors across all runtime sources.
- **Planned resolution:** Planned after detector catalog stabilizes.

### G-P2-COV: Formal coverage store (P2) — Deferred

- **Status:** Deferred
- **Impact:** No formal model tracking signal/MITRE/source coverage per cluster.
- **Planned resolution:** Requires coverage model design and persistence layer.

### G-P2-GRAPH: Attack-path graph edges (P2) — Deferred

- **Status:** Deferred
- **Impact:** Attack path visualization lacks formal graph edges between capabilities and blast radius.
- **Planned resolution:** Requires graph data model and UI rendering.

### SBOM-REL-01: DLQ replay orchestration (P1) ✅

- **Status:** Completed
- **Impact:** Failed SBOM processing had no retry mechanism.
- **Planned resolution:** Delivered. `core/pkg/worker/sbom_dlq_worker.go` with bounded retries.
- **Tests:** `go test ./pkg/worker/... -run SBOMDLQ`

### SBOM-REL-02: SBOMMatchRun explicit lifecycle (P1) ✅

- **Status:** Completed
- **Impact:** SBOM match runs lacked explicit status tracking, timeout handling, and error codes.
- **Planned resolution:** Delivered. `core/pkg/models/sbom_match_run.go`, `core/internal/repository/sbom_repository.go`, `core/pkg/worker/cve_matcher_worker.go`, migration 111.
- **Tests:** `go test ./internal/repository/...`

### SBOM-REL-03: Configurable SBOM pod phase policy (P1) ✅

- **Status:** Completed
- **Impact:** SBOM processing did not respect pod lifecycle phases.
- **Planned resolution:** Delivered. `core/pkg/worker/sbom_worker.go` reads `FORTUNA_SBOM_POD_PHASE_POLICY`.
- **Tests:** `go test ./pkg/worker/... -run ShouldProcessSBOMForPhase`

### SBOM-REL-04: Configurable orphan grace + runtime evidence guard (P1) ✅

- **Status:** Completed
- **Impact:** Orphan SBOMs could be deleted before runtime evidence was collected.
- **Planned resolution:** Delivered. `core/pkg/reconciler/sbom_reconciler.go` reads `FORTUNA_SBOM_ORPHAN_GRACE_PERIOD`.
- **Tests:** `go test ./pkg/reconciler/...`

### SBOM-DATA-01: Enum normalization + DB constraints (P1) ✅

- **Status:** Completed
- **Impact:** `sbom_source` and `confidence` lacked normalization and DB-level constraints.
- **Planned resolution:** Delivered. `core/pkg/models/sbom.go`, migration 111.
- **Tests:** `go test ./internal/repository/... -run NormalizeSourceAndConfidence`

### SBOM-PERF-01: Remove N+1 CVE severity queries (P1) ✅

- **Status:** Completed
- **Impact:** SBOM list API had N+1 query performance issue for CVE severity lookups.
- **Planned resolution:** Delivered. `core/internal/api/sbom_handlers.go`.
- **Tests:** `go test ./internal/api/...`

---

## Operational / Infrastructure Gaps

### GAP-OBS-01: Grafana dashboard for Risk Center metrics (Low)

- **Status:** Backlog — requires Grafana/Prometheus infrastructure
- **Impact:** Prometheus metrics (`RiskEvaluationDuration`, `InsightsBatchSize`) are exposed but have no pre-built dashboard.
- **Planned resolution:** Create Grafana dashboard JSON when Grafana is available in the deployment.

### GAP-OBS-02: Prometheus alerting rules (Low)

- **Status:** Backlog — requires Prometheus/Alertmanager infrastructure
- **Impact:** No alert rules for critical insight spikes or risk evaluation anomalies.
- **Planned resolution:** Deploy `deploy/prometheus/risk-center.alerts.yaml` when Prometheus is available.

### GAP-OBS-03: OpenTelemetry spans (Low)

- **Status:** Backlog — `go.opentelemetry.io/otel` is indirect dependency
- **Impact:** No distributed tracing for risk evaluation pipeline.
- **Planned resolution:** Instrument key handlers (`GET /risks`, `BatchCreateOrUpdateInsights`, CVE matcher) when otel SDK is adopted in the project.

### GAP-INFRA-01: Redis for cache/WebSocket scale-out (Low)

- **Status:** Backlog — requires Redis infrastructure
- **Impact:** In-memory cache and WebSocket hub are single-instance; no horizontal scaling.
- **Planned resolution:** Migrate to Redis-backed cache and pub/sub when Redis is available.

### GAP-OPS-01: Helm values documentation (Medium)

- **Status:** Partially complete
- **Impact:** Environment variables are documented in Core README and deploy checklist, but not in Helm chart values.
- **Planned resolution:** Add env var documentation to Helm chart `values.yaml` when Helm chart is formalized.

---

## UI/UX Gaps

### GAP-UI-01: Sidebar filter 280px (Low)

- **Status:** Backlog — UX enhancement
- **Impact:** Findings page uses inline filters; wireframe specifies a fixed 280px sidebar filter panel.
- **Planned resolution:** Planned for advanced UI iteration.

### GAP-UI-02: Risk Score vs Capability Severity view (Low)

- **Status:** Backlog — future feature
- **Impact:** No comparative view of risk scores against capability severity.
- **Planned resolution:** Coming soon; no timeline.

### GAP-UI-03: MITRE ATT&CK mapping in drawer (Low)

- **Status:** Backlog — conditional feature
- **Impact:** Risk Detail drawer's "PCE & Attack Path" tab does not display MITRE ATT&CK mappings.
- **Planned resolution:** When MITRE data is available in runtime signals.

### GAP-UI-04: Delta WebSocket updates (Medium)

- **Status:** Open
- **Impact:** WebSocket broadcasts `insights_updated` with no payload; client performs full refetch instead of incremental merge.
- **Planned resolution:** Send `changed_ids` in WebSocket payload; client merges/refetches only changed items.

### GAP-UI-05: Quick Links on Overview (Low)

- **Status:** Open
- **Impact:** Overview page lacks the 3 quick-link cards (View All Findings, PCE Heatmap, Evidence & References) from the wireframe.
- **Planned resolution:** Add card components to Overview route.

---

## Testing Gaps

### GAP-TEST-01: End-to-end flow test (Medium)

- **Status:** Open
- **Impact:** No E2E test covering the full flow: Agent → Risk Worker → insights → WebSocket → Dashboard UI.
- **Planned resolution:** Build integration test exercising the complete pipeline.

### GAP-TEST-02: UI-level tests (Medium)

- **Status:** Open
- **Impact:** No Cypress/Playwright tests for `/risks/*` routes (bulk actions, WebSocket, cache invalidation, PCE cleanup).
- **Planned resolution:** Add browser-based E2E tests.

### GAP-TEST-03: Bulk action E2E (Low)

- **Status:** Open
- **Impact:** `POST /risk/insights/bulk` not covered by E2E script.
- **Planned resolution:** Add TC to `e2e-risk-center-full.sh`.

---

## Runtime Architecture Gaps (from target model analysis)

### GAP-ARCH-01: Canonical RuntimeEventDTO v2 ingest cut-over (P1) — Deferred

- **Status:** Deferred
- **Impact:** Core still accepts both flattened and nested source fields for backward compatibility. Canonical v2 contract not fully enforced.
- **Planned resolution:** Cut over after all agents emit v2 contract.

### GAP-ARCH-02: Risk scorer full cut-over to asset_security_state (P1) — Deferred

- **Status:** Deferred
- **Impact:** V2 scorer does not yet consume full `asset_security_state` dimensions. V3 preview exists but doesn't replace V2.
- **Planned resolution:** After v3 preview validation in production.

### GAP-ARCH-03: Explainability refs end-to-end (P1) — Deferred

- **Status:** Deferred
- **Impact:** Full chain `event → fact → signal → incident → capability → insight` explainability refs not yet wired end-to-end.
- **Planned resolution:** Incremental wiring as each pipeline layer stabilizes.

### GAP-ARCH-04: JetStream/NATS durable consumer integration (Medium) — Deferred

- **Status:** Deferred
- **Impact:** Tests don't run nats-server with durable consumers. Message delivery guarantees not fully tested.
- **Planned resolution:** Add integration test with embedded NATS JetStream.

### GAP-ARCH-05: Full Pod manifest storage (Low) — Deferred

- **Status:** Deferred
- **Impact:** No `raw_json` column in `pods` table for complete Pod manifest storage.
- **Planned resolution:** Add column and populate from Agent sync when needed for forensic analysis.

### GAP-ARCH-06: ValidatingWebhook enforcement (Medium) — Deferred

- **Status:** Deferred (G-R10 provides audit mode only)
- **Impact:** Admission risk gate operates in audit mode. Full enforcement via `ValidatingWebhookConfiguration` not yet enabled by default.
- **Planned resolution:** Enable when admission control policies are validated in production.
