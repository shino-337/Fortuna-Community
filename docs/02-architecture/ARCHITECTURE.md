# Fortuna Platform Architecture

**Version**: 2.0 | **Last Updated**: 2026-04-13

## Overview

Fortuna is a comprehensive security and risk management platform for Kubernetes clusters. It provides real-time vulnerability detection, SBOM extraction, CVE matching, security insights, pod capability analysis (MITRE ATT&CK), runtime signal detection, and admission control.

### System Diagram

```
┌──────────────────────┐     ┌──────────────────────────┐
│   Dashboard (React)  │     │   Kubernetes API Server   │
│   Nginx + SPA        │     │                          │
└──────────┬───────────┘     └────────────┬─────────────┘
           │ /api/* proxy                 │
           ▼                              │
┌──────────────────────┐                  │
│   Core (Go/Gin)      │◄─── gRPC/mTLS ──┤
│   REST API :8080     │                  │
│   gRPC :9090         │     ┌────────────▼─────────────┐
│   Workers, Scheduler │     │   Agent (DaemonSet)      │
└──────────┬───────────┘     │   Per-node monitoring    │
           │                 │   SBOM extraction        │
     ┌─────┴──────┐          │   Runtime collection     │
     ▼            ▼          └──────────────────────────┘
┌─────────┐  ┌─────────┐
│PostgreSQL│  │  NATS   │
│  :5432   │  │JetStream│
└─────────┘  └─────────┘
```

## Core Components

### Agent (DaemonSet)

**Purpose:** Node-level security monitoring, SBOM extraction, runtime data collection

| Feature | Description |
|---------|-------------|
| Pod detection | K8s Informer watches pods on local node |
| SBOM extraction | Multi-parser (dpkg, apk, rpm, npm, pip, gomod) via containerd socket |
| Pod Detail collection | Runtime metrics, processes, network connections, K8s events |
| Runtime events | Falco JSONL reader, eBPF sensor (noop health-check phase) |
| Communication | gRPC + mTLS to Core; HTTP fallback for pod detail ingest |

**Key env vars:** `CORE_GRPC_ENDPOINT`, `CLUSTER_ID`, `SYNC_INTERVAL`, `POD_DETAIL_RUNTIME_SOURCE`, `FALCO_EVENTS_ENABLED`, `EBPF_ENABLED`

**Resource:** Memory limit 6Gi (due to SBOM extraction). Deploy: `deploy/fortuna-agent-daemonset.yaml`

### Core (Go/Gin)

**Purpose:** Central API server, event processing, risk evaluation, data persistence

| Feature | Description |
|---------|-------------|
| REST API | Gin framework, JWT auth, domain-organized routes |
| gRPC server | Agent sync endpoint with mTLS |
| SBOM pipeline | NATS JetStream async queue → CVE matching → insights |
| Risk engine | Rule-based evaluation, risk scoring (v2), insight generation |
| PCE integration | Triggers `EvaluateAndUpsertPod` after pod sync |
| Admission controller | Webhook for risk-based admission control |
| Scheduler | Retention jobs, periodic evaluations |

**Key env vars:** `DATABASE_URL`, `NATS_URL`, `AUTH_ENABLED`, `ADMISSION_RISK_GATE_ENABLED`

### Dashboard (React SPA)

**Purpose:** Security investigation and monitoring UI

**Stack:** React 19, Vite, TypeScript, Tailwind CSS, Zustand, Recharts/D3, Lucide React

**Deployment:** Nginx serves static + proxies `/api/*` to Core. See [Dashboard component docs](../03-components/dashboard/README.md).

### Infrastructure

| Component | Purpose | Config |
|-----------|---------|--------|
| PostgreSQL | Primary data store (96+ migrations) | `DATABASE_URL` |
| NATS JetStream | Async event bus (SBOM queue, runtime events) | `NATS_URL` |
| Redis | Optional: caching, dedup (not required for MVP) | `REDIS_URL` |

## Data Flow: End-to-End Pipeline

### 1. Pod Discovery → SBOM → CVE → Insights

```
K8s Pod created on Node
    ↓
Agent detects (Informer) → extracts SBOM (containerd)
    ↓
Agent sends SBOM → NATS JetStream (ksam.sbom.created)
    ↓
Core SBOM worker → persist SBOM + components → CVE matching
    ↓
CVE Matcher (OSV.dev PostgreSQL) → cve_matches table
    ↓
Insight generator → insights table (vulnerability type)
    ↓
Risk scorer (v2) → risk_scores table
    ↓
Dashboard displays risks, SBOM, CVEs
```

### 2. Pod Sync → Capability Analysis

```
Agent syncs pod spec → Core (gRPC/HTTP)
    ↓
Core: upsert pod, check specHash
    ↓
PCE Evaluator: static rule analysis → pod_capabilities
    ↓
CSC: runtime signals → capability state promotion
    ↓
Risk scorer includes capability data
```

### 3. Runtime Monitoring

```
Agent: host /proc scanning, Falco JSONL, eBPF (future)
    ↓
Core: POST /api/v1/runtime/events
    ↓
runtime_events table → CSC promotion → risk re-evaluation
    ↓
Dashboard: Pod Detail events, Risk Center runtime signals
```

## API Architecture

### Route Organization (Domain-Based)

| Domain | Prefix | Purpose |
|--------|--------|---------|
| Auth | `/api/v1/auth/*` | Login, register, refresh token |
| Inventory | `/api/v1/inventory/*` | Pod capabilities, resources |
| Risk | `/api/v1/risk/*` | Insights, risk rules, risk scores |
| Runtime | `/api/v1/runtime/*` | Signals, events, network, metrics |
| Policy | `/api/v1/policy/*` | Policy rules, templates, instances |
| Dashboard | `/api/v1/dashboard/*` | Aggregated stats, pre-computed |
| Agent | `/api/v1/agent/*` | Agent ingest (sync, pod detail) |
| Cluster | `/api/v1/clusters/*` | Cluster management |
| SBOM | `/api/v1/sbom/*` | SBOM data and CVE matches |

### Route Classification

- **Group A (Stable):** Auth, health, identity, WebSocket — no changes needed
- **Group B (Accepted):** Agent ingest, dashboard aggregates — may evolve to `/ingest/v1/*`
- **Group C (Migrated):** Legacy routes moved to domain paths (pod-capabilities → inventory, insights → risk, rules → policy)

### Risk Rules vs Policy Rules

| Aspect | Risk Rules | Policy Rules |
|--------|-----------|--------------|
| Storage | Database (`risk_rules` table) | YAML files (`FORTUNA_RULES_DIR`) |
| API | `/api/v1/risk/rules` | `/api/v1/policy/rules` |
| UI | Settings → Risk Rules | Rules page |
| Purpose | Operational risk evaluation → creates insights | Rule catalog/reference, test, metrics |
| Engine | RiskWorker: DB priority, then YAML | RulesManager/YAMLEngine |

## Repository Structure

```
/
├── agent/              # Go agent (DaemonSet)
│   ├── cmd/            # Entry points
│   └── internal/       # Syncer, poddetail, sbom
├── core/               # Go core server
│   ├── cmd/            # Entry points (server, cve-loader)
│   ├── internal/       # API handlers, services, scheduler
│   ├── pkg/            # Shared: models, risk, capability
│   └── migrations/     # 96+ database migrations
├── dashboard/          # React SPA
│   ├── pages/          # Page components
│   ├── components/     # Reusable UI components
│   └── lib/            # API client, stores
├── deploy/             # K8s manifests
├── scripts/            # Build, deploy, verify scripts
├── docs/               # Documentation
└── e2e/                # E2E test suites
```

## Key Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| CVE source | OSV.dev (not Trivy) | Zero-dependency, PostgreSQL-native, full control |
| SBOM extraction | Custom parsers (not Syft) | Smaller footprint, containerd-native |
| Event bus | NATS JetStream | Lightweight, built-in persistence, K8s-native |
| Risk scoring | Rule-based v2 (not ML) | Deterministic, explainable, auditable |
| Capability model | MITRE ATT&CK mapping | Industry standard, enables attack path analysis |
| Auth | JWT + optional mTLS | Simple, stateless, K8s-compatible |

## Related Documentation

- **Components:** [Agent](../03-components/agent/README.md) | [SBOM](../03-components/sbom/README.md) | [CVE Scanner](../03-components/cve-scanner/README.md) | [Risk Center](../03-components/risk-center/README.md) | [PCE](../03-components/podCapabilityEngine/README.md) | [Pod Detail](../03-components/podDetail/README.md) | [Dashboard](../03-components/dashboard/README.md) | [Network Activity](../03-components/networkActivity/README.md)
- **ADRs:** [docs/adr/](../adr/README.md)
- **Operations:** [Deployment](../05-operations/DEPLOYMENT.md) | [GAP Resolution](../05-operations/FORTUNA_GAP_RESOLUTION_PLAN.md)
