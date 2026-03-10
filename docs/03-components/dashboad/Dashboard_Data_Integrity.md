Dashboard Data Integrity & De-Mock Initiative
ROLE

You are a senior backend / platform engineer responsible for data correctness and system integrity, not UI cosmetics.

Your task is to eliminate misleading dashboard data by fixing data sources, logic flow, and system contracts.

You are not allowed to:

fabricate data

infer values not produced by the system

keep mock data in production paths

“make the dashboard look nice” at the cost of correctness

🚨 CORE PRINCIPLE (NON-NEGOTIABLE)

If data cannot be traced end-to-end, it must NOT be shown.

End-to-end means:

Agent → Core → Storage → API → Dashboard


If any link is missing → data is invalid.

🧭 OBJECTIVES

Identify and remove all mock / test / hardcoded data affecting the dashboard

Ensure every dashboard widget is backed by real system signals

Disable, hide, or explicitly label unsupported features

Add data integrity checks to prevent future regressions

1️⃣ INVENTORY ALL DASHBOARD DATA PATHS
Tasks

Enumerate all APIs consumed by the dashboard

For each API, determine:

data origin: mock | test | real

producing agent (if any)

triggering condition (event / scan / runtime / schedule)

Output (REQUIRED)

Create a table or structured output equivalent to:

Widget → API → Source → Agent → Core Logic → Status


If you cannot identify the agent or event → mark as INVALID.

2️⃣ MOCK DATA ERADICATION (ZERO TOLERANCE)
You MUST:

Locate all:

mock generators

seed scripts

fallback / default responses

Classify each as:

DEV-ONLY

DEMO-ONLY

INVALID / LEGACY

Enforcement Rules

Production code must fail loudly, not fallback silently

Mock data must be:

removed, OR

hard-gated by DEV_MODE

Explicitly REMOVE these patterns:

Returning demo data when DB is empty

Defaulting severity, status, or counts

Randomized numbers for charts

3️⃣ WIDGET-BY-WIDGET REALITY CHECK

For each dashboard widget:

Validate

What exact signal is being displayed?

Does the system actually generate this signal?

Which agent sends it?

What event/schema/version?

Decision Logic

If fully supported → keep

If partially supported → mark Experimental

If unsupported → disable or hide

❗ Never show 0 or placeholder values to “look complete”.

4️⃣ AGENT ↔ CORE ↔ DASHBOARD CONTRACT AUDIT
Agent

Verify:

event emission

schema versioning

heartbeat / liveness signals

Core

Validate:

schema enforcement

rejected / dropped data logging

aggregation ownership (core, not UI)

Dashboard

Must be purely declarative

No inference, no aggregation, no severity guessing

If logic exists in the UI → move it to core or remove it.

5️⃣ UNSUPPORTED FEATURES HANDLING

For any UI feature without backend support:

Choose ONE:

Disable with explicit tooltip

Hide completely

Mark clearly as Experimental

Never silently fake data.

6️⃣ DATA INTEGRITY & ANOMALY CHECKS

You must add at least:

Cross-checks:

active agents vs dashboard agents

raw events vs aggregated counts

Alerts when:

data exists but no agents are active

severity totals exceed signal totals

Recommended:

GET /health/dashboard-data-integrity

7️⃣ DEFINITION OF DONE (STRICT)

You are NOT DONE if:

Any widget cannot be traced to an agent event

You cannot answer:

“Which agent produced this number, and when?”

You are DONE ONLY WHEN:

All dashboard data is real, traceable, or explicitly disabled

No mock data can reach production

Dashboard correctly shows empty / no-data states

🧨 FINAL REMINDER

This task is about truth, not aesthetics.

A broken dashboard that looks honest
is infinitely better than a beautiful one that lies.

Proceed methodically.
Document every assumption you invalidate.

---

## Implementation Summary (Completed)

### 1. Inventory: Widget → API → Source → Agent → Status

| Widget / Page | API | Source | Agent / Core | Status |
|---------------|-----|--------|--------------|--------|
| Dashboard stats (clusters, pods, agents, risks) | GET /api/v1/dashboard/stats | real | agent (sync) → DB | OK |
| Dashboard clusters list | GET /api/v1/clusters | real | agent (sync) → DB | OK |
| Dashboard risks / insights | GET /api/v1/risks | real | core (insights) | OK |
| Dashboard notifications | GET /api/v1/notifications | **stub** | — | Returns empty; `_dataSource: stub` |
| Dashboard threat velocity | GET /api/v1/dashboard/metrics/threat-velocity | real | core (insights by date) | OK |
| Dashboard PCE summary/trend | GET /api/v1/pod-capabilities/summary/* | real | agent (PCE) | OK |
| Clusters page | GET /api/v1/clusters | real | agent (sync) | OK |
| Insights / Risks page | GET /api/v1/risks, pod-capabilities | real | core, agent | OK |
| SBOM page | GET /api/v1/sbom, /sbom/:podId | real | agent (SBOM) | OK |
| Metrics: agents | GET /api/v1/agents/status | real | agent (heartbeat) | OK |
| Metrics: queue depth | GET /api/v1/metrics/queue | **placeholder** | — | `_dataSource: unsupported` (Prometheus required) |
| Metrics: workers | GET /api/v1/metrics/workers | **placeholder** | — | `_dataSource: unsupported` |
| Metrics: system / sync status | GET /api/v1/metrics/system | real | core (DB) | OK |
| Metrics: error logs | GET /api/v1/error-logs | **stub** | — | `_dataSource: unsupported`; UI shows "not yet available" |
| Attack Paths | GET /api/v1/attack-paths/graph | real | core (graph) | OK |
| Resources | GET /api/v1/resources | real | agent (sync) | OK |
| Rules | GET /api/v1/rules | real | core | OK |
| Audit / Reports | GET /api/v1/audit, /reports | real | core (audit_logs) | OK |
| Certificates | GET /api/v1/certificates/info | real | core (CertManager) | OK when TLS enabled |
| Users | GET /api/v1/users | **stub** | — | No route; 404 → UI returns [] |
| Cert rotation history | GET /api/v1/certificates/rotation/history | **stub** | — | Not implemented; 404 |

Live inventory and cross-checks: **GET /health/dashboard-data-integrity** (no auth). Returns `crossChecks` (active agents, clusters, pods, insights), `alerts` (e.g. data_exists_no_agents), and `endpoints` (path, source, agent, note).

### 2. Mock / Stub / Placeholder Handling

- **Stubs** (no backend table or route): Return empty payload + `_dataSource: "stub"` or `"unsupported"`. Dashboard does not show fake counts; unsupported widgets show explicit "not yet available" or "—".
- **Placeholders** (e.g. workers, queue): No fake healthy/queue numbers. API returns `_dataSource: "unsupported"` and optional `message`. Dashboard treats as no data (e.g. Queue depth "—", Error logs "Error log aggregation not yet available").
- **Dashboard stats**: No silent fallback to zeros on API failure. `getStats()` throws; Dashboard shows error state + "Retry" instead of fake 0s.
- **GetSystemMetrics**: Removed hardcoded `avgLatency`. **GetPolicyEvaluationCost**: Only DB-derived `totalEvaluations` returned; other fields omitted (require Prometheus).

### 3. Data Integrity Endpoint

- **GET /health/dashboard-data-integrity**
  - **Cross-checks**: active agents count, dashboard agents count, clusters (recent sync), pods, insights, critical insights.
  - **Alerts**: e.g. `data_exists_no_agents` when insights or pods exist but no active agents in last 10m.
  - **Endpoints**: list of path, source (real | stub | placeholder), agent, note.

### 4. Definition of Done Checklist

| Criterion | Status |
|-----------|--------|
| All dashboard data traceable to agent/core or explicitly disabled | Done |
| No mock data in production paths; stubs return `_dataSource` and empty data | Done |
| Dashboard shows error state when stats API fails (no fake zeros) | Done |
| Unsupported features: explicit "not available" or "—", no fake values | Done |
| GET /health/dashboard-data-integrity implemented with cross-checks and endpoint list | Done |
| Empty / no-data states shown correctly (e.g. Error logs, Queue, Workers) | Done |

### 5. UI Data Verification & Refresh Intervals

All displayed data must be traceable and refreshed on an interval so the UI shows up-to-date values.

**Refresh intervals (dashboard `usePolling` + `REFRESH_INTERVALS`):**

| Page / Data | API | Refresh interval | Backend source |
|-------------|-----|------------------|----------------|
| Dashboard (stats, clusters, risks, PCE, threat velocity) | getStats, getClusters, getRisks, getThreatVelocity, getPceSummaryByCapability, getPceTrend | 30s | Agent sync (SYNC_INTERVAL), Core DB |
| SBOM list & detail | getSbomList, getPodSbom | 1 min | Agent SBOM → Core DB |
| Insights / Risk Center (risks, PCE summary, clusters) | getRisks, getPceSummaryBySeverity, getPceCapabilities, getClusters, getStats | 1 min | Core insights + PCE |
| Clusters | getClusters | 30s | Agent sync → clusters table |
| Metrics (agents, certs, sync status) | getAgents, getCertificates, getSyncStatus | 30s | Core DB / CertManager |

**Constants (dashboard/hooks/usePolling.ts):**

- `STATS_CLUSTERS`: 30s — stats, clusters, agents
- `SBOM_RISK_LIST`: 1 min — SBOM list, Risk list (insights)
- `PCE_TREND`: 5 min — PCE summary/trend (heavier)
- `METRICS`: 30s — metrics page

**Backend alignment:**

- Agent `SYNC_INTERVAL` (env, e.g. 30s/5m) drives how often cluster/pod/RBAC data is sent to Core.
- Core risk evaluation and PCE run on schedulers (e.g. 6h risk, PCE_SCHEDULER_INTERVAL). Dashboard polling refreshes what is already in DB so the UI reflects latest stored data.

**User-selectable refresh interval:**

- Layout header (desktop and mobile) includes a **Refresh interval** dropdown (Off, 30s, 1 min, 5 min, 30 min).
- Selection is stored in `fortuna-refresh-interval` (localStorage) and applied to all pages that use polling (Dashboard, SBOM, Risk Center, Clusters, Monitoring).
- **Off** disables auto-refresh; user can still use manual Refresh where available.