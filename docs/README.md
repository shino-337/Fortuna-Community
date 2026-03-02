# Fortuna Platform Documentation

**Version**: 2.1  
**Last Updated**: 2026-03-02

---

## Welcome to Fortuna

Fortuna is a security and risk management platform for Kubernetes. It provides SBOM extraction, CVE matching, security insights, Pod Capability Engine (PCE), runtime signals, and a web dashboard.

**Production:** [docs-prod/](../docs-prod/README.md) (overview, architecture, operations) · [deploy/README.md](../deploy/README.md) (manifests, Helm, deploy checklist).

---

## Quick Start

1. **Prepare Environment**: [Environment Preparation Guide](01-getting-started/ENVIRONMENT_PREPARATION.md)
2. **Build Images**: [Build Guide](01-getting-started/BUILD_GUIDE.md)
3. **Deploy**: [Production Deployment Guide](05-operations/PRODUCTION_DEPLOYMENT.md)
4. **Verify**: Check deployment status and run end-to-end tests

---

## Documentation Index

### Getting Started (01-getting-started/)

- [Environment Preparation](01-getting-started/ENVIRONMENT_PREPARATION.md) - Prepare your Kubernetes cluster
- [Build Guide](01-getting-started/BUILD_GUIDE.md) - Build Core and Agent components
- [Deployment (quick)](05-operations/DEPLOYMENT.md) - Quick deployment reference
- [Production Deployment Guide](05-operations/PRODUCTION_DEPLOYMENT.md) - Complete deployment instructions

### Architecture & Design (02-architecture/)

- [Architecture Documentation](02-architecture/ARCHITECTURE.md) - System architecture and components
- [Components Documentation](02-architecture/COMPONENTS.md) - Detailed component breakdown
- [Repository Structure](02-architecture/REPOSITORY_STRUCTURE.md) - Repo layout
- [Database Schema](04-development/MIGRATIONS.md#database-schema) - Complete database schema reference
- [Migration Guide](04-development/MIGRATIONS.md) - Database migrations and schema management

### Operations (05-operations/)

- [Deployment Checklist](05-operations/DEPLOYMENT_CHECKLIST.md) - Step-by-step deployment
- [Clean Rebuild & Verify](05-operations/CLEAN_REBUILD_REDEPLOY_AND_VERIFY.md) - Clean rebuild and E2E verify
- [Troubleshooting](05-operations/PRODUCTION_DEPLOYMENT.md#troubleshooting) - Common issues and solutions
- **[Agent/Core: Monitor & troubleshooting](AGENT_CORE_ERRORS_MONITOR.md)** – Monitor commands (monitor-agent-core-errors.sh), error analysis (OOM, secrets, containerd digest, Sync 500, DNS)

### Components (03-components/)

- [Core Components Analysis](03-components/CORE_COMPONENTS_ANALYSIS.md) - Core component breakdown
- [Agent Components Analysis](03-components/AGENT_COMPONENTS_ANALYSIS.md) - Agent component breakdown
- [Agent-Core Flow](03-components/AGENT_CORE_FLOW.md) - Data flow between Agent and Core

### Reference (06-reference/)

- [API Reference](06-reference/API_REFERENCE.md) - REST API documentation

### Testing

- [Test Results](test-results/) – Latest test results (see [TESTCASE_MONITOR.md](TESTCASE_MONITOR.md) for script list)

---

## System Components

### Core

Central processing component that handles:
- SBOM storage and management
- CVE matching and vulnerability detection
- Insight generation
- Policy evaluation
- API services

**Deployment**: Kubernetes Deployment (control-plane node)

### Agent

Node-level component that:
- Monitors pods on each node
- Extracts SBOMs from container images
- Communicates with Core via gRPC

**Deployment**: Kubernetes DaemonSet (runs on all nodes)

### Dashboard

- **React/Vite** web UI: Risk Center, SBOM browser, threat velocity, pod capabilities, runtime signals. Proxies `/api` to Core.

### Infrastructure

- **PostgreSQL**: Database for SBOMs, CVEs, insights, capabilities, attack steps
- **NATS JetStream**: Message queue for event processing (3-replica cluster)
- **Metrics**: Core exposes `/metrics` (Prometheus)

---

## Architecture Overview

```
┌─────────────┐
│   Agent     │ (DaemonSet - one per node)
│  - Pod Watch│
│  - SBOM Ext │
└──────┬──────┘
       │ gRPC (mTLS)
       ▼
┌─────────────┐
│    Core     │ (Deployment)
│  - Storage  │
│  - CVE Match│
│  - Insights │
└──────┬──────┘
       │
       ├──► PostgreSQL (Database)
       └──► NATS JetStream (Events)
```

---

## Key Features

- ✅ **Automatic SBOM Extraction**: Extracts SBOMs from container images using multiple parsers
- ✅ **Real-time CVE Matching**: Matches CVEs against extracted packages
- ✅ **Security Insights**: Generates actionable security insights
- ✅ **Policy Engine**: Configurable security policies with CEL-based evaluation
- ✅ **Pod Capability Engine (PCE)**: Runtime capability detection and attack step inference
- ✅ **Attack Path Analysis**: Graph-based attack path visualization
- ✅ **Runtime Signals**: Real-time security signal detection and correlation
- ✅ **High Availability**: NATS cluster with 3 replicas
- ✅ **Scalable**: Horizontal scaling support
- ✅ **Secure**: mTLS for all inter-component communication
- ✅ **Observability**: Metrics endpoint (`/metrics`) for Prometheus scraping

---

## Production Readiness

Fortuna is production-ready with:
- ✅ Test coverage and E2E verification (see [TESTCASE_MONITOR.md](TESTCASE_MONITOR.md))
- ✅ High availability (NATS cluster, multiple replicas)
- ✅ Security (mTLS, RBAC, secure defaults)
- ✅ Observability (Prometheus metrics at `/metrics`)
- ✅ Documentation and operational guides

---

## Support

For issues and questions:
1. **[Agent/Core errors & monitoring](AGENT_CORE_ERRORS_MONITOR.md)** – Monitor commands and error analysis (OOM, secrets, Sync 500, DNS)
2. [Troubleshooting](05-operations/PRODUCTION_DEPLOYMENT.md#troubleshooting)
3. Monitor errors: `./scripts/monitor/monitor-agent-core-errors.sh` or `--follow`
4. Logs: `kubectl logs -n fortuna -l app.kubernetes.io/component=core` / `component=agent`
5. [Architecture](02-architecture/ARCHITECTURE.md)

---

## License

See repository root for license information.

---

**Last Updated**: 2026-03-02

---

## Documentation Structure

The documentation is organized into the following structure:

- **Root**: README.md, [DOCS_STRUCTURE.md](DOCS_STRUCTURE.md)
- **01-getting-started/**: Environment, build, quickstart
- **02-architecture/**: Architecture, components, repository structure
- **03-components/**: Core, Agent, PCE, SBOM, runtime signals
- **04-development/**: Migrations, seed data, development logic
- **05-operations/**: Deployment, checklist, clean rebuild, port-forward, network
- **06-reference/**: API reference, script paths
- **07-guides/**: UI/UX, dashboard, risk center — [Risk Center Total findings logic](07-guides/RISK_CENTER_TOTAL_FINDINGS_LOGIC.md); [Threat Velocity](07-guides/THREAT_VELOCITY.md); [Dashboard charts & E2E](07-guides/DASHBOARD_CHARTS_AND_E2E.md) (why velocity/risk charts may not change after E2E)
- **08-tutorials/**: Tutorials
- **test-results/**: Current test results (README)
- **archive/**: Outdated or one-off docs

For detailed structure, see [DOCS_STRUCTURE.md](DOCS_STRUCTURE.md).
