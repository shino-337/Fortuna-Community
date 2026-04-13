# Risk Center Component

## Overview

The Risk Center is a page within the Fortuna Dashboard (route `/risks`) that provides a unified workspace for security risk management. It is **not** a separate micro-frontend; it is a React page in the Dashboard codebase that calls Core APIs via `/api/v1/risk/*` and a WebSocket at `/api/v1/ws/risks`.

**Key capabilities:**

- View and manage **risk findings** (insights): CVE, RBAC, misconfiguration, capability exposure.
- View **Pod Capability Exposure (PCE)**: 7-day trend and namespace × severity heatmap.
- View **Evidence & References**: runtime signals, audit trail.
- Real-time updates via NATS → WebSocket push + client polling fallback.
- Export findings as CSV or PDF. Bulk acknowledge / resolve / dismiss.

**User personas:** SecOps, Platform Engineer, Compliance.

**Dashboard routes:**

| Route | Content |
|-------|---------|
| `/risks` | Overview — KPI cards, Risk Trend (7 days), Histogram, Risk Level, Risks by Cluster, Quick Links |
| `/risks/findings` | Findings table with filters, sort, pagination, bulk actions, Export CSV/PDF |
| `/risks/pce` | PCE Trend line chart (7 days), Exposure by Namespace heatmap |
| `/risks/evidence` | Runtime Signals table, Audit Trail tab |

---

## Architecture

### Core Components

#### Rules Engine

The risk engine evaluates normalized resources against configurable rules. Rules are loaded from the database table `risk_rules` (priority) or from YAML files in `FORTUNA_RULES_DIR/risk/`. Each rule has an id, name, severity, category, conditions (CEL expressions or resource match), aggregation (AND/OR/THRESHOLD), and base_score.

- Code: `core/pkg/riskengine/engine.go` — `Engine.EvaluateResource()`, `createInsight()`
- Rule loading: `LoadRulesFromDB()`, `ReloadFromDB()`, or YAML via `ListRuleSummariesFromDir()`
- CEL is used for deterministic predicates, boolean composition, and threshold logic. Temporal sequence detection, correlation windows, and probabilistic scoring happen upstream in the REP pipeline.
- Users can manage rules via **Settings → Risk Rules** (Add/Edit/Delete when source=db; Import/Export YAML).

#### Risk Scorer

Calculates a 0–100 risk score per resource and assigns a priority level.

- Code: `core/pkg/risk/scorer.go` — `Scorer.CalculateScore()`, `SaveScore()`
- Automatically triggered after insight creation/update via `InsightManager.scheduleRiskScoreCalculation()`.
- Batch recalculation: `POST /api/v1/risk/scores/sync` (202 Accepted, runs in background goroutines).

#### Admission Controller

Runtime admission risk gate for Kubernetes workloads (G-R10).

- Env vars: `ADMISSION_RISK_GATE_ENABLED`, `ADMISSION_RISK_SENSITIVE_NAMESPACES`, `ADMISSION_RISK_BLOCK_THRESHOLD`, `ADMISSION_RISK_GATE_MODE` (`audit` or `enforce`).
- Baseline deployment: `deploy/fortuna-core-deployment.yaml`.
- Webhook: `ValidatingWebhookConfiguration` `fortuna-policy-webhook` and Service `fortuna-webhook` (see `deploy/webhook-config.yaml`).
- Verification: `bash scripts/verify/verify-admission-risk-gate.sh`.

---

## Processing Flow

### Unified Risk Pipeline (5-Layer Architecture — Phase 1–4, 2026-04-13)

The codebase was refactored from 6 independent risk systems into a clear pipeline where each layer's output feeds into the next:

```
Layer 1: FACT DISCOVERY
  PCE evaluator  → pod_capabilities (11 rules, state: detected/confirmed/exploited/chained)
  Risk Engine    → insights (RBAC, capability, vulnerability, misconfiguration)
  CVE Matcher    → vulnerability insights (CVSS, SBOM-linked)

Layer 2: RUNTIME ENRICHMENT
  CSC (CapabilityStateController) → state promotion
  AttackStepInference             → pod_attack_steps

Layer 3: PATH ANALYSIS  ← NEW: now reads PCE
  RelationalPathBuilder.BuildPathsForPod()
    reads pod_capabilities → adds escape edges (ESC_PRIV_POD, ESC_HOSTPATH_NODE, ESC_RUNTIME_ACTIVE)
    reads pod_attack_steps → adds active step edges (e.g. NODE_CRED_DUMP → CAN_STEAL_CREDENTIALS)
    boosts TotalRisk by capability state (confirmed +1.0, exploited +2.5/difficulty -0.3)
    persists to attack_paths table (migration 118, upsert)

Layer 4: UNIFIED SCORING  ← NEW: replaces 4 independent scoring systems
  UnifiedScorerV3 (core/pkg/risk/unified_scorer.go)
    reads: insights + pod_capabilities + attack_paths + runtime_signals
    7 dimensions (max pts): VULNERABILITY(15), CAPABILITY_EXPOSURE(15), ATTACK_PATH(15),
                            RBAC_POLICY(15), RUNTIME_THREAT(15), EXPOSURE(15), BLAST_RADIUS(10)
    toxic-combo boosts: CVE critical + internet-exposed (+10),
                        privileged + escape confirmed + cluster-admin path (+15),
                        token theft + external egress (+10)
    formula: min(100, (Σdimensions + toxic_boost) × time_decay)
    persists to risk_scores with scorer_version="v3" and dimension columns (migration 119)
    triggered by: PCE evaluator completion, InsightManager.scheduleRiskScoreCalculation()

Layer 5: PRESENTATION
  Dashboard reads risk_scores.total_score (V3) for authoritative number
  PodDetail.tsx: Unified Risk Summary card with 7-dimension breakdown + toxic combos
  Dashboard.tsx: Top Risky Pods card (V3 scores), exploited capability count, attack path count
  Metrics.tsx: Pipeline Health section (Layer 1–4 status)
```

**Shared RBAC Analyzer (PCE-1 / PCE-3 gap closure):**
- `core/pkg/rbac/analyzer.go` — single shared implementation for RBAC analysis
- PCE evaluator's `hasAPIWriteAccess()` → calls `rbac.AnalyzePod()`
- Attack Path's `classifyRoleRisk()` → replaced by `rbac.ClassifyRoleRisk()`
- Eliminates the previous 3-way duplication

**Backward Compatibility:**
- V2 scorer (`core/pkg/risk/scorer.go`) continues to run unchanged alongside V3
- `pod_risk_profiles` table still populated by PCE (not yet deprecated)
- V3 records identified by `scorer_version="v3"` in `risk_scores`

### Risk Evaluation Pipeline (V2 — unchanged)

```
[Agent / Sync] → pods, pod_capabilities, SBOM (DB)
[NATS: fortuna.normalized.*] → RiskWorker.Process
    → riskEngine.EvaluateResource(kind, normalizedData)
    → []*Insight (createInsight per rule match)
    → insightMgr.BatchCreateOrUpdateInsights(insights)
    → INSERT/UPDATE insights (dedup by resource_uid + insight_type + title or cve_id)
    → js.Publish("fortuna.insights.updated", {})
[Main: subscriber fortuna.insights.updated] → BroadcastRisksUpdate()
    → ClearByPrefix("risks:list:", "insights:summary:", "risk:histogram:")
    → RisksWSHub.Broadcast({"type":"insights_updated"})
[Dashboard: WebSocket /ws/risks] receives message → refetch getRisks()
[Risk score] V2 + V3 calculated per-resource → stored in risk_scores
    GET /risk/insights?withScores=1 joins risk_scores → returns totalScore, priorityLevel
```

**Insight sources:**

| Source | Mechanism |
|--------|-----------|
| Risk engine (RBAC/rule-based) | RiskWorker subscribes `fortuna.normalized.>`, evaluates rules, creates insights |
| CVE matcher | Processes SBOM/CVE data, creates vulnerability insights |
| Capability evaluator | Creates capability exposure insights |
| Historical risk evaluator | Re-evaluates historical data |

**Deduplication:** Vulnerability insights dedup by `(resource_uid, cve_id)`. Capability with cve_id: `(resource_uid, cve_id, insight_type)`. Others: `(resource_uid, insight_type, title)`. Re-activates resolved/dismissed insights on new evidence.

**Real-time vs batch:** NATS messages trigger workers in real-time. WebSocket broadcasts `insights_updated` to Dashboard. Dashboard also uses `usePolling` with configurable intervals as fallback.

### Pipeline Health API

```
GET /api/v1/monitoring/pipeline-health
```

Returns per-layer health status:
- Layer 1: `lastPceEval`, `lastRiskEngineEval`, `insightCount`
- Layer 2: `lastStateChange`, `activePromotionRules`, `exploitedCapCount`
- Layer 3: `lastPathComputation`, `totalPaths`, `criticalPaths` (≥ 9.0 risk)
- Layer 4: `lastScoreCalc`, `resourcesScored`, `avgScore`, `v3Resources`

Visible in Dashboard → Monitoring page → "Pipeline Health" section.

### Runtime Security Detection

Fortuna's runtime pipeline transforms telemetry into security intelligence through layered processing:

```
Layer 1 — Runtime Evidence
  Sensors (Falco, eBPF/LSM, Tetragon, Tracee) → Source Adapters → Canonical RuntimeEventDTO
  → Agent ingest client (retry/batch/backpressure) → Core Runtime Ingest API → runtime_events

Layer 2 — Behavior Semantics
  runtime_events → Behavior Fact Extractor (REP-A) → runtime_behavior_facts
  → Signal Synthesizer (REP-B) → runtime_signals
  → Stateful Correlators (REP-C) → runtime_incidents

Layer 3 — Capability Reasoning
  runtime_signals + runtime_incidents + static inventory/RBAC/exposure
  → Capability Engine (init / promote / suppress / decay) → effective_capabilities

Layer 4 — Unified Security State
  inventory + exposure + RBAC + SBOM/vulns + runtime signals + incidents + capabilities
  → Security State Projector → asset_security_state

Layer 5 — Risk Evaluation
  asset_security_state → Risk Engine (CEL rules + risk conditions) → insights → risk_scores
```

**Implemented REP-C detectors:**

| Detector | Type | Window | Description |
|----------|------|--------|-------------|
| `RECON_BURST` | Stateful | 60s | ≥10 unique destinations within window |
| `POST_EXPLOIT_EXEC_CHAIN` | Stateful | 10m | Execution chain correlator |
| `EXFIL_LIKE_SEQUENCE` | Stateful | 5m | Ordered token-read → external-connect chain |

**Capability model** — three classes:

1. **Declared** — from pod spec, security context, RBAC, mounts (e.g., `CAN_REACH_K8S_API`, `CAN_ACCESS_HOST_FS`)
2. **Observed** — from runtime signals/incidents (e.g., `OBSERVED_K8S_API_ACCESS`, `OBSERVED_EXTERNAL_EGRESS`)
3. **Effective** — union of declared + observed + inference (e.g., `EFFECTIVE_K8S_CONTROL_PLANE_ACCESS`)

Code: `core/pkg/capability/semantics.go` (`ValidateCapabilityClass`, `ValidateProgressionState`).

### Admission Control Flow

When `ADMISSION_RISK_GATE_ENABLED=true`, the admission webhook evaluates incoming workload requests against risk scores:

1. Kubernetes API server sends admission review to `fortuna-policy-webhook`.
2. Core checks if the target namespace is in `ADMISSION_RISK_SENSITIVE_NAMESPACES`.
3. If the resource's risk score ≥ `ADMISSION_RISK_BLOCK_THRESHOLD`, action depends on `ADMISSION_RISK_GATE_MODE`:
   - `audit`: log only (allows the request)
   - `enforce`: deny the request

---

## Technical Details

### Risk Score Calculation (v2)

Scores are stored in the `risk_scores` table with fields: `total_score` (0–100), `base_score`, `severity_weight`, `impact_multiplier`, `time_decay`, `exploitability_score`, `business_impact_score`, `scorer_version`, `factors` (jsonb).

**Priority levels (V2):**

| Priority | Score Range | Meaning |
|----------|------------|---------|
| P0 | ≥ 80 | Critical |
| P1 | ≥ 60 | High |
| P2 | ≥ 35 | Medium |
| P3 | ≥ 10 | Low |
| P4 | 0–9 | Informational |

> Note: V1 thresholds were P0 ≥ 90, P1 ≥ 70, P2 ≥ 40. V2 is the current version. Code: `models.GetPriorityLevel(score)`.

**Weighted scoring dimensions:**

| Dimension | Description |
|-----------|-------------|
| CVE / Exploitability | CVSS-based severity + known exploit availability |
| SBOM / Software Risk | Critical/high vulnerabilities, fix availability |
| Runtime Threat | Recent suspicious runtime behavior signals |
| Network / Exposure | Internet exposure, service type, public ingress |
| Config / Privilege | Root execution, host mounts, Linux capabilities, RBAC |

**Score factors (v3 preview):** `exposure`, `exploitability`, `privilege`, `runtime_threat`, `blast_radius`, `confidence`, optionally `criticality`. Stored in `risk_scores.factors` as `score_v3_preview`. Code: `core/pkg/riskengine/score_v3_types.go`, `core/pkg/risk/score_v3_preview.go`.

**Score boosters** (toxic combinations that materially increase priority):

- Internet exposed + known exploitable vulnerability
- Service account token read + external egress
- Root + hostPath mount + host file access observed
- Shell execution + payload fetch + tmp binary exec

**Decay:** Runtime-heavy scores decay over time. Recent runtime incidents carry more weight; stale runtime-only signals lose impact. Score is recomputed as state changes.

### Runtime Detectors

#### Detector Catalog (priority build set)

| ID | Detector | Type | Input Facts | Emits | Confidence |
|----|----------|------|-------------|-------|------------|
| D1 | `SHELL_EXEC_DETECTED` | Stateless | `PROCESS_EXEC`, `INTERACTIVE_SHELL` | Signal: `SUSPICIOUS_SHELL_EXEC` | 0.7 |
| D2 | `SERVICEACCOUNT_TOKEN_READ_DETECTED` | Stateless | `FILE_READ` (sensitive path) | Signal: `SERVICEACCOUNT_TOKEN_READ` | 0.8 |
| D3 | `EXTERNAL_EGRESS_DETECTED` | Stateless | `NETWORK_CONNECT` (public dst) | Signal: `EXTERNAL_EGRESS` | 0.65 |
| D4 | `REMOTE_PAYLOAD_FETCH_DETECTED` | Stateless | `NETWORK_CONNECT`, `FILE_WRITE`, `PROCESS_EXEC` | Signal: `REMOTE_PAYLOAD_FETCH` | 0.7 |
| D5 | `TMP_EXEC_DETECTED` | Stateless | `PROCESS_EXEC` (path `/tmp/*`, `/dev/shm/*`) | Signal: `TMP_BINARY_EXEC` | 0.75 |
| D6 | `HOST_PATH_ACCESS_DETECTED` | Stateless | `FILE_READ`/`FILE_WRITE`/`HOST_PATH_TOUCH` | Signal: `HOST_PATH_ACCESS` | 0.75 |
| D7 | `RECON_BURST_DETECTED` | Stateful (60s) | Repeated `NETWORK_CONNECT` to many distinct dsts | Incident: `RECON_BURST` | medium–high |
| D8 | `EXFIL_LIKE_SEQUENCE_DETECTED` | Stateful (5m) | Token read + external egress (ordered) | Incident: `EXFIL_LIKE_SEQUENCE` | medium–high |

- P0 ship set: D1–D7. P1 add-on: D8.
- Detector registry: `core/pkg/rep/detector_registry.go`
- Incident correlator: `core/pkg/rep/incident_correlator.go`

#### Runtime Signal Taxonomy (8 domains)

| Domain | Example Signal Types |
|--------|---------------------|
| Execution | `SUSPICIOUS_PROCESS_EXEC`, `INTERACTIVE_SHELL_EXEC`, `TMP_BINARY_EXECUTION` |
| Filesystem | `SENSITIVE_FILE_ACCESS`, `SERVICEACCOUNT_TOKEN_READ`, `HOST_PATH_ACCESS`, `BINARY_STAGING` |
| Network | `EXTERNAL_EGRESS`, `DNS_ANOMALY`, `PORT_SCAN_LIKE_ACTIVITY`, `C2_LIKE_CONNECTION` |
| Privilege / Escape | `PRIVILEGE_ESCALATION_ATTEMPT`, `CAPABILITY_ABUSE`, `HOST_NAMESPACE_ACCESS`, `CONTAINER_ESCAPE_PRIMITIVE` |
| Credential / Secret | `CREDENTIAL_ACCESS`, `SECRET_MATERIAL_ACCESS`, `K8S_API_TOKEN_USE`, `CLOUD_METADATA_ACCESS` |
| Defense Evasion | `SECURITY_TOOL_TAMPERING`, `PROCESS_HIDE_ATTEMPT`, `HISTORY_CLEARING`, `AUDIT_EVASION` |
| Discovery / Recon | `ENVIRONMENT_DISCOVERY`, `K8S_RECON_ACTIVITY`, `NETWORK_RECON_ACTIVITY` |
| Persistence / Staging | `PERSISTENCE_ATTEMPT`, `REMOTE_PAYLOAD_FETCH`, `CRON_MODIFICATION` |

Signals are mapped to MITRE ATT&CK tactics and techniques where applicable (e.g., `SERVICEACCOUNT_TOKEN_READ` → Credential Access / T1552).

### Database Schema

#### Key Tables

**`insights`** — Risk findings.

| Column | Type | Description |
|--------|------|-------------|
| `id` | serial PK | |
| `resource_type`, `resource_namespace`, `resource_name`, `resource_uid` | varchar | Resource identification |
| `insight_type` | varchar | vulnerability, capability, policy, etc. |
| `severity` | varchar | critical, high, medium, low |
| `title`, `description`, `recommendation` | text | Human-readable details |
| `cve_id`, `cvss`, `affected_component`, `affected_version`, `fixed_version` | varchar/float | CVE-specific fields |
| `evidence` | jsonb | Structured evidence payload |
| `violated_rules` | jsonb | Rules that triggered this insight |
| `status` | varchar | active, resolved, acknowledged, dismissed |
| `detected_at`, `resolved_at` | timestamptz | Lifecycle timestamps |
| `created_at`, `updated_at`, `deleted_at` | timestamptz | Standard timestamps |

**`risk_scores`** — Per-resource risk scores.

| Column | Type | Description |
|--------|------|-------------|
| `id` | serial PK | |
| `resource_type`, `resource_uid`, `resource_name`, `namespace`, `cluster_id` | varchar | Resource identification |
| `total_score` | float | 0–100 composite score |
| `base_score`, `severity_weight`, `impact_multiplier`, `time_decay` | float | Score components |
| `exploitability_score`, `business_impact_score` | float | V2 dimensions |
| `scorer_version` | varchar | Algorithm version |
| `factors` | jsonb | V2/V3 factor breakdown |
| `insights_count` | int | Number of active insights |
| `highest_severity` | varchar | Highest severity among insights |
| `priority_level` | varchar | P0–P4 |
| `calculated_at` | timestamptz | Last calculation time |
| `created_at`, `updated_at`, `deleted_at` | timestamptz | Standard timestamps |

Unique constraint: `(resource_type, resource_uid, cluster_id)`.
Indexes: `total_score`, `resource_uid`, `cluster_id`, `priority_level`, `calculated_at`, `deleted_at`.

**`risk_rules`** — Configurable risk evaluation rules.

| Column | Type | Description |
|--------|------|-------------|
| `id`, `rule_id` | serial/varchar | Identifiers |
| `name`, `category`, `severity`, `description` | varchar/text | Rule metadata |
| `enabled` | boolean | Active flag |
| `conditions` | text/json | CEL expression or match conditions |
| `aggregation` | varchar | AND, OR, THRESHOLD |
| `base_score` | float | Score contribution |
| `tags` | text/json | Classification tags |
| `created_at`, `updated_at`, `deleted_at` | timestamptz | Standard timestamps |

History tracked in `risk_rules_history` (snapshot before update).

**`admission_policies`** — Admission control policies.

Referenced by the admission risk gate. Configuration primarily via env vars (see Admission Controller section).

**Runtime tables:**

| Table | Purpose |
|-------|---------|
| `runtime_events` | Immutable raw runtime evidence (pod_uid, namespace, source_kind, event_type, syscall, severity, mitre_technique, payload_json) |
| `runtime_behavior_facts` | Normalized behavioral primitives (fact_type, domain, attributes jsonb) |
| `runtime_signals` | Semantic security signals (signal_type, category, confidence, evidence jsonb, count) |
| `runtime_incidents` | Stateful correlated conditions (incident_type, severity_hint, confidence, window, evidence_refs) |
| `pod_capabilities` | Pod capability exposure (capability_id, capability_group, severity, state, confidence, evidence jsonb) |
| `asset_security_state` | Unified per-asset risk context snapshot (identity, exposure, privilege, software, runtime, capabilities, blast_radius) |
| `audit_logs` | User action audit trail (action, resource, resource_id, user_id, details, cluster_id) |

### API Endpoints

Base path: `/api/v1/risk/`

#### Insights / Findings

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/risk/insights` | `GetInsightsListCached` | List findings with pagination, filters (severity, status, clusterId, sinceMinutes, priorityLevel, scoreBin, withScores). Cache TTL 60s. |
| GET | `/risk/insights/summary` | `GetInsightsSummaryCached` | Summary by severity (total, critical, high, medium, low) |
| GET | `/risk/insights/summary/by-cluster` | `GetInsightsSummaryByCluster` | Summary grouped by cluster |
| GET | `/risk/insights/summary/global` | `GetInsightsSummaryGlobalCached` | Global summary across all clusters |
| GET | `/risk/insights/:id` | `GetInsight` | Single insight detail |
| POST | `/risk/insights/:id/acknowledge` | `AcknowledgeInsight` | Acknowledge finding |
| POST | `/risk/insights/:id/resolve` | `ResolveInsight` | Resolve finding |
| POST | `/risk/insights/:id/dismiss` | `DismissInsight` | Dismiss finding |
| POST | `/risk/insights/bulk` | Bulk action | Bulk acknowledge/resolve/dismiss (insightIds[]) |
| GET | `/risk/insights/export` | `ExportRisksCSV` | Export CSV (streaming, chunked 500, limit 10k) |
| GET | `/risk/insights/export?format=pdf` | Export PDF | HTML optimized for print |

#### Risk Scores

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/risk/scores` | | List all risk scores |
| GET | `/risk/scores/:uid` | | Score for specific resource |
| POST | `/risk/scores/:uid/calculate` | `CalculateRiskScore` | Calculate and save score for one resource |
| POST | `/risk/scores/sync` | `SyncRiskScores` | Recalculate all scores (202 Accepted) |

#### Histogram / Trends

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/risk/histogram` | `GetRiskHistogram` | Score distribution bins 0–90 with severity breakdown. Cache key: `risk:histogram:{clusterId}:{sinceMinutes}`, TTL 30s. |
| GET | `/risk/trends` | `GetRiskTrends` | 7-day trend data |
| GET | `/dashboard/metrics/threat-velocity` | `GetThreatVelocity` | Threat velocity (supports `byType=all`) |
| GET | `/dashboard/stats` | `GetDashboardStats` | KPI stats (total, critical, resolved24h) |

#### Risk Rules

| Method | Path | Description |
|--------|------|-------------|
| GET | `/risk/rules` | List rules (DB or YAML) |
| GET | `/risk/rules/:id` | Single rule detail |
| POST | `/risk/rules` | Create rule (source=db only) |
| PUT | `/risk/rules/:id` | Update rule |
| DELETE | `/risk/rules/:id` | Delete rule |
| GET | `/risk/rules/export` | Export rules as YAML |
| POST | `/risk/rules/validate` | Validate rule definition |
| POST | `/risk/rules/import` | Import rules from YAML |

#### PCE (Pod Capability Exposure)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/risk/pod-capabilities/trends` | 7-day PCE trend |
| GET | `/risk/pod-capabilities/summary/namespace` | PCE summary by namespace (heatmap data) |

#### Runtime

| Method | Path | Description |
|--------|------|-------------|
| GET | `/risk/runtime/summary` | Runtime signals summary |
| GET | `/risk/pods/:uid/report` | Pod risk report |
| GET | `/risk/pods/:uid/runtime` | Pod runtime overview |
| GET | `/risk/pods/:uid/runtime/events` | Pod runtime events |
| GET | `/runtime/pods/:uid/signals` | Runtime signals for pod (24h) |

#### v2 APIs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v2/runtime/pods/:uid/security-state` | Asset security state snapshot |
| GET | `/api/v2/runtime/pods/:uid/facts` | Runtime behavior facts |
| GET | `/api/v2/runtime/pods/:uid/incidents` | Runtime incidents |
| GET | `/api/v2/runtime/pods/:uid/capabilities` | Pod capabilities (v2) |

#### WebSocket

| Path | Description |
|------|-------------|
| `/ws/risks` | Real-time updates. Broadcasts `{"type":"insights_updated"}` on insight changes. Rate-limited per IP via `RISKS_WS_MAX_CONNS_PER_IP`. |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PCE_CLEANUP_RETENTION_DAYS` | `30` | Days to keep `pod_capabilities` records; cleanup job removes entries older than `last_seen_at`. |
| `RISKS_WS_MAX_CONNS_PER_IP` | `10` | Max WebSocket connections to `/ws/risks` per IP. |
| `INSIGHTS_RESOLVED_RETENTION_DAYS` | `30` | Resolved insights are soft-deleted after N days (InsightsCleanupJob). |
| `INSIGHTS_ACTIVE_RETENTION_DAYS` | `90` | Active insights without updates are soft-deleted after N days. |
| `FORTUNA_RULES_DIR` | — | Directory for YAML risk rules. |
| `FORTUNA_RUNTIME_RISK_LOOKBACK_HOURS` | `24` | Runtime signals lookback window (max 168). |
| `ADMISSION_RISK_GATE_ENABLED` | `false` | Enable admission risk gate. |
| `ADMISSION_RISK_SENSITIVE_NAMESPACES` | — | Comma-separated namespaces for admission enforcement. |
| `ADMISSION_RISK_BLOCK_THRESHOLD` | `70` | Risk score threshold for blocking. |
| `ADMISSION_RISK_GATE_MODE` | `audit` | `audit` (log only) or `enforce`. |
| `FORTUNA_SBOM_POD_PHASE_POLICY` | — | Configurable SBOM pod phase policy. |
| `FORTUNA_SBOM_ORPHAN_GRACE_PERIOD` | — | Grace period before orphan SBOM cleanup. |

### Key Code Paths

| Area | Path |
|------|------|
| Risk engine | `core/pkg/riskengine/engine.go` |
| Insight manager | `core/pkg/riskengine/insight_manager.go` |
| Risk scorer | `core/pkg/risk/scorer.go` |
| Score v3 preview | `core/pkg/risk/score_v3_preview.go`, `core/pkg/riskengine/score_v3_types.go` |
| Risk worker | `core/pkg/worker/risk_worker.go` |
| CVE matcher worker | `core/pkg/worker/cve_matcher_worker.go` |
| API handlers (insights) | `core/internal/api/insights_handlers.go` |
| API handlers (dashboard/histogram) | `core/internal/api/dashboard_handlers.go` |
| API routes (risk) | `core/internal/api/routes_risk.go` |
| Cache | `core/internal/api/risks_cache.go` |
| WebSocket hub | `core/internal/api/risks_ws_hub.go` |
| Insights cleanup job | `core/internal/scheduler/insights_cleanup_job.go` |
| PCE cleanup job | `core/internal/scheduler/pce_cleanup_job.go` |
| Evidence masking | `core/pkg/evidence/mask.go` |
| Capability semantics | `core/pkg/capability/semantics.go` |
| Detector registry | `core/pkg/rep/detector_registry.go` |
| Incident correlator | `core/pkg/rep/incident_correlator.go` |
| Explainability chain | `core/pkg/explainability/insight_chain.go`, `enrich.go` |
| Metrics | `core/pkg/metrics/metrics.go` |
| Dashboard page | `dashboard/pages/Insights.tsx` |
| Risk detail page | `dashboard/pages/RiskDetail.tsx` |
| Dashboard API client | `dashboard/lib/api.ts` |

### NATS Subjects

| Subject | Purpose |
|---------|---------|
| `fortuna.normalized.>` | Normalized resource events consumed by RiskWorker and other workers |
| `fortuna.insights.updated` | Published after insight creation/update; triggers WebSocket broadcast and cache invalidation |
| `fortuna.siem.events` | SIEM integration: published for critical/high insights |

### Prometheus Metrics

- `RiskEvaluationDuration` — Risk evaluation latency
- `InsightsBatchSize` — Batch size for insight operations
- `InsightsCreatedTotal` (labels: insight_type, severity) — Counter of created insights
- `InsightsActiveTotal` — Gauge of active insights
- `RiskScoresCalculatedTotal` — Counter of risk score calculations
- `RiskScoreDistribution` — Distribution of risk scores

Endpoint: `/metrics` (promhttp).

---

## Testing

### E2E Test Cases

Script: `scripts/e2e/e2e-risk-center-full.sh`
Report: `test-results/risk-center-e2e-YYYYMMDD-HHMMSS.md`

```bash
# Run from repo root (requires Core pod running in cluster)
./scripts/e2e/e2e-risk-center-full.sh
# Custom namespace
NAMESPACE=my-ns ./scripts/e2e/e2e-risk-center-full.sh
```

| ID | Test Case | Description |
|----|-----------|-------------|
| TC-01 | GET /risks (list, pagination) | Returns insights[], total with page/pageSize |
| TC-02 | GET /risks?withScores=1 | Unified score: totalScore, priorityLevel from risk_scores |
| TC-03 | GET /risks?priorityLevel=P0 | Filter by priority level P0–P4 |
| TC-04 | GET /insights/summary | Severity breakdown: total, critical, high, medium, low |
| TC-05 | GET /insights/summary/by-cluster | By-cluster summary array |
| TC-05b | GET /insights/summary/global | Global summary (HTTP 200) |
| TC-05c | GET /risk/histogram | Score distribution bins[], totalFindings |
| TC-06 | GET /risks/export | Export CSV (HTTP 200) |
| TC-07 | GET /risks/export?format=pdf | Export HTML for PDF (HTTP 200) |
| TC-08 | GET /risk-rules | List risk rules |
| TC-09 | GET /risk-rules/:id | Single rule detail (SKIP if no rules) |
| TC-10 | POST /risk-rules | Create rule (source=db only) |
| TC-11 | PUT /risk-rules/:id | Update rule |
| TC-12 | DELETE /risk-rules/:id | Delete rule |
| TC-13 | GET /pod-capabilities/trends | PCE 7-day trend |
| TC-14 | GET /pod-capabilities/summary/namespace | PCE heatmap by namespace |
| TC-15 | GET /runtime-signals | Runtime signals list |
| TC-16 | WebSocket GET /ws/risks | WebSocket endpoint responds (101/400/401) |
| TC-17 | GET /risk/pods/:uid/report | Pod risk report |
| TC-18 | GET /runtime/pods/:uid/signals | Runtime signals for pod (24h) |

### Unit Tests

```bash
cd core
go test ./pkg/capability/... ./pkg/models/... ./pkg/rep/... ./pkg/explainability/... \
  ./pkg/riskengine/... ./pkg/risk/... ./internal/api/... ./internal/scheduler/... -count=1
```

Key test files: `insights_audit_test.go`, `risks_export_test.go`, `insights_cleanup_job_test.go`, `pod_capability_handlers_sync_test.go`, `insights_explainability_test.go`.

---

## Related ADRs

| ADR | Title |
|-----|-------|
| [ADR-001](../../adr/001-capability-reasoning-semantics.md) | Capability reasoning semantics (class vs progression) |
| [ADR-002](../../adr/002-runtime-signal-incident-lifecycle.md) | Signal & incident lifecycle |
| [ADR-003](../../adr/003-asset-security-state-schema-groups.md) | Asset security state schema groups |
| [ADR-004](../../adr/004-rep-detector-governance.md) | REP-C detector governance |
| [ADR-005](../../adr/005-pod-capability-single-table.md) | Single-table `pod_capabilities` decision |
