# Fortuna Core Component Analysis

This document summarizes the Core codebase by component/package, the concrete logic each component implements,
and highlights components that are unused, partially used, or gated behind optional dependencies.

---

## 1) Entry Points and Binaries

### `core/cmd/main.go` (Core service)
**Purpose**
- Primary Core service bootstrap: config, DB, migrations, NATS, workers, schedulers, gRPC/HTTP servers.

**Key logic**
- DB connection + migrations before starting servers.
- NATS connection (optional); worker pool created only when NATS is available.
- Workers: `CorrelatorWorker`, `RiskWorker` (normalizer is intentionally skipped).
- Policy evaluator + policy worker (used by admission webhook).
- Schedulers: risk evaluation, pod cleanup, insights cleanup, SBOM reconciliation.
- gRPC server for agent traffic (SBOM ingest, agent registration).
- REST API server with auth middleware and health endpoints.
- Admission webhook HTTPS server (optional).

**Notes**
- If NATS is unavailable, worker pool is skipped (risk pipeline and correlator do not run).
- Some jobs are started only when DB is ready.

### `core/cmd/cve-loader` (CVE loader binary)
**Purpose**
- Loads CVE data into `cves` and `package_vulnerabilities` tables.

**Usage**
- Used by `scripts/load-cve-data.sh` to populate the DB.

---

## 2) Internal Packages (`core/internal`)

### `internal/api`
**Purpose**
- REST API for dashboard and management endpoints.

**Key logic**
- Routes and handlers for:
  - SBOM list/detail (`/sbom`, `/sbom/:podId`)
  - Risks and insights (`/risks`, `/insights`)
  - Risk analytics (`/risk/*`)
  - Graph/attack path (`/graph/*`, `/attack-paths/*`)
  - Resources (`/resources`)
  - Notifications, audit, monitoring, rules, policies.

**Notes**
- `/risks` returns insights; can be filtered by type/severity/status.
- Pod risk report endpoint: `/risks/pods/:podUid/report`.

### `internal/auth`
**Purpose**
- Password hashing, user auth helpers.

### `internal/config`
**Purpose**
- Loads configuration/env settings for core (TLS, ports, DB, NATS).

### `internal/grpc`
**Purpose**
- gRPC server for agent traffic.

**Key logic**
- SBOM ingest: upsert SBOM + components, publish `fortuna.sbom.created` event.
- Agent registration persists agent metadata.

### `internal/health`
**Purpose**
- Liveness/readiness/status endpoints.

### `internal/middleware`
**Purpose**
- CORS, security headers, metrics, rate limiting (HTTP).

### `internal/scheduler`
**Purpose**
- Scheduled background tasks.

**Key logic**
- Risk evaluation scheduler (historical, periodic).
- Pod cleanup, insights cleanup, SBOM reconciliation.

### `internal/service`
**Purpose**
- Core business logic for agent sync (HTTP).

**Key logic**
- Full sync of pods, service accounts, roles, bindings.
- Maintains `linkedPods` for service accounts (risk evaluation accuracy).
- Audit logs for sync changes.

### `internal/storage`
**Purpose**
- DB connection and migrations orchestration.

### `internal/webhook`
**Purpose**
- Admission webhook for policy enforcement.

**Key logic**
- Validates admission requests, evaluates policies, publishes violations.

### `internal/metrics`
**Purpose**
- Internal metrics registry used by HTTP middleware.

### `internal/k8s`
**Purpose**
- K8s client utilities used by policy remediation and some handlers.

### `internal/ingest` (UNUSED)
**Purpose**
- Agent rate limiter implementation.

**Status**
- Not referenced by Core runtime; not wired into gRPC/HTTP ingest paths.

---

## 3) Core Packages (`core/pkg`)

### `pkg/messaging`
**Purpose**
- NATS JetStream publisher abstraction.

**Key logic**
- Publish inventory, SBOM created, insight created events.

### `pkg/worker`
**Purpose**
- Event-driven processing using NATS streams.

**Key logic**
- `CorrelatorWorker`: stores normalized K8s resources into DB.
- `RiskWorker`: evaluates risk rules on normalized resources.
- `CVEMatcherWorker`: matches SBOM → CVE, persists `cve_matches`, creates vulnerability insights.
- `SBOMWorker`: listens to normalized pods, links pod image scans to SBOM, publishes `sbom.created`.
- `HistoricalRiskEvaluator`: evaluates RBAC risks from existing DB state.
- Worker pool with DLQ/backpressure support.

**Notes**
- Worker pool only runs when NATS is available.
- Normalizer worker is removed; normalization happens in handlers.

### `pkg/cve`
**Purpose**
- CVE matching and DB access.

**Key logic**
- `database.Manager`: bulk lookup of CVEs by package/ecosystem.
- `matcher.Matcher`: converts SBOM components into CVE matches.

### `pkg/sbom`
**Purpose**
- SBOM service layer and events.

**Key logic**
- Upsert PodImageScan entries.
- SBOMCreatedEvent struct used across workers.

### `pkg/riskengine`
**Purpose**
- Risk rules engine and insight creation.

**Key logic**
- Built-in rules (cluster-admin binding, wildcard permissions, overprivileged roles).
- Optional YAML rules engine (when `FORTUNA_RULES_DIR` is set).
- InsightManager for dedup/upsert and reactivation.

### `pkg/risk`
**Purpose**
- Risk scoring V2 for resources.

### `pkg/graph` (PARTIALLY USED)
**Purpose**
- Graph-based attack paths and risk queries via Apache AGE.

**Key logic**
- `AgeGraphEngine` + `QueryService` implement attack paths, permissions graph, risky pods.

**Status**
- If AGE extension is not enabled, queries return empty results (functional but no data).

### `pkg/policy`
**Purpose**
- Policy evaluation, remediation, violations, YAML parsing.

**Used by**
- Admission webhook.
- Policy worker and evaluator in Core startup.

### `pkg/security`
**Purpose**
- TLS certificate management (rotation/expiry monitoring).

**Used by**
- gRPC server initialization; API routes exposed when cert manager is available.

### `pkg/metrics`
**Purpose**
- Prometheus metrics (HTTP, admission webhook).

### `pkg/models`
**Purpose**
- DB models for clusters, pods, insights, SBOM, CVE, policy, etc.

### `pkg/reconciler`
**Purpose**
- SBOM reconciliation loop (detect missing/orphaned SBOMs).

### `pkg/scanner` (UNUSED/LEGACY)
**Purpose**
- Legacy Trivy-based scanning (deprecated).

**Status**
- Not referenced by Core runtime; kept as stub for build/test compatibility.

---

## 4) Risk Pipeline (Detailed)

1. **Resource ingestion**
   - From HTTP `/api/v1/agent/sync` or NATS normalized streams.
   - Stored in DB by `AgentService` and/or `CorrelatorWorker`.

2. **Real-time risk**
   - `RiskWorker` consumes `fortuna.normalized.>` and calls `riskengine.Engine.EvaluateResource`.
   - Creates insights via `InsightManager`.

3. **Historical/batch risk**
   - `HistoricalRiskEvaluator` scans DB tables and creates insights.
   - Triggered by scheduler and on full agent sync.

4. **Exposure**
   - `/risks` API (insights list) and `/risk/*` analytics.
   - `/risks/pods/:podUid/report` builds detailed RBAC risk report.

---

## 5) SBOM + CVE Pipeline (Detailed)

1. **SBOM ingest (gRPC)**
   - Agent sends SBOM.
   - Core upserts SBOM, inserts components if new.
   - Publishes `fortuna.sbom.created`.

2. **CVE matching**
   - `CVEMatcherWorker` consumes `sbom.created`.
   - Matches packages → CVEs via `pkg/cve`.
   - Persists `cve_matches`, creates vulnerability insights.

3. **SBOM API**
   - `/sbom` returns latest SBOM per pod, with vuln counts.
   - `/sbom/:podId` returns components + matches.

---

## 6) Components Not Used or Gated

### Unused
- `internal/ingest` (rate limiter): implemented but never wired.
- `pkg/scanner` (legacy Trivy): deprecated, not referenced.

### Partially used / optional
- `pkg/graph`: requires Apache AGE; without AGE, APIs return empty datasets.
- `pkg/security` routes: only exposed if TLS cert manager initializes.
- Worker pool: disabled when NATS is unavailable.

---

## 7) Known Gaps / Potential Improvements

- Normalization pipeline: normalizer worker removed; ensure all ingested events are normalized before reaching workers.
- Rate limiting: `internal/ingest` could be integrated into gRPC/HTTP ingest paths.
- Graph features depend on AGE; consider fallback or migration docs.

