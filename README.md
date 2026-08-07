# Fortuna

### Kubernetes Security. From Misconfiguration to Attack Path.

Fortuna is an open-source Kubernetes security platform that connects **Kubernetes posture, RBAC exposure, vulnerabilities, runtime signals, and attack paths** into one risk model.

Instead of asking only *"Is this pod vulnerable?"*, Fortuna asks:

> **"Can an attacker reach this workload, what can they do from it, and how far can the compromise spread?"**

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.28%2B-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io/)
[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Runtime Security](https://img.shields.io/badge/runtime-Falco%20%2B%20eBPF-orange)](docs/03-components/README.md)
[![GHCR](https://img.shields.io/badge/images-GHCR-blue?logo=github)](https://github.com/shino-337/Fortuna-Community/pkgs/container/fortuna-community)

**Website:** https://fortunahub.dev  ·  **Docs:** [Documentation](docs/README.md)  ·  **Release:** [v1.0.0](https://github.com/shino-337/Fortuna-Community/releases/tag/v1.0.0)

---

## Why Fortuna?

Kubernetes security tools often answer different questions in isolation:

- CVE scanners tell you **what is vulnerable**.
- Kubernetes posture tools tell you **what is misconfigured**.
- RBAC analyzers tell you **what identities can access**.
- Runtime tools tell you **what actually happened**.

Fortuna correlates these signals to answer the security question that matters most: **what is the realistic attack path and business-relevant risk?**

```text
Kubernetes Inventory
        │
        ├── RBAC / Service Accounts
        ├── Pod Security / Capabilities
        ├── SBOM / CVEs
        ├── Runtime Signals
        │
        ▼
   Attack Path Analysis
        │
        ▼
   Unified Risk Scoring
        │
        ▼
 Prioritized Findings
```

## What Fortuna Finds

| Security question | Fortuna capability |
|---|---|
| Which workloads are exposed? | Kubernetes inventory + posture analysis |
| Which pods have dangerous privileges? | Pod Capability Engine + RBAC analysis |
| Which vulnerabilities actually matter? | SBOM/CVE correlation + exploit context |
| Can a compromised pod escalate? | Attack Path Analysis |
| Is suspicious behavior occurring now? | Falco + eBPF runtime signals |
| How severe is the combined risk? | Unified multi-factor risk scoring |
| Why did a workload receive this score? | Explainable risk factors and evidence |

## Core Capabilities

### 🔐 Kubernetes & RBAC Security

- Service Account and RBAC inventory
- Role/ClusterRole and binding analysis
- Privilege-escalation path detection
- Dangerous verbs/resources and wildcard permission analysis
- Pod security posture and escape-surface analysis
- Blast-radius analysis

### 🧬 SBOM & Vulnerability Management

- Per-workload SBOM extraction
- PURL normalization and CVE matching
- OSV-backed vulnerability catalog
- Malware/package intelligence integration
- Trust-aware matching and explainable findings
- Historical and replay-safe processing

### ⚔️ Attack Path Analysis

Fortuna builds paths such as:

```text
Pod
 │
 ├── ServiceAccount
 │       │
 │       └── RoleBinding
 │               │
 │               └── ClusterRole
 │                       │
 │                       └── cluster-admin
 │
 └── HostPath / hostNetwork / runtime capability
```

Built-in scenarios cover privilege escalation, Service Account token reuse, host-path escape, lateral movement, and broken attack chains.

### 🛰️ Runtime Security

- Falco event ingestion
- eBPF syscall telemetry
- Process snapshot diffing
- Network connection tracking
- Runtime signal promotion: `observed → confirmed → exploited`
- Correlation between runtime evidence and static security posture

### 📊 Unified Risk

Risk is not treated as a single CVSS number. Fortuna combines vulnerability, capability, RBAC, runtime, attack-path, and contextual evidence into a prioritized workload risk model with contributing-factor explainability.

---

## See It in Action

The dashboard provides dedicated workspaces for:

- Platform Integrity
- Security Operations
- Findings
- Attack Paths
- Runtime Network
- Kubernetes Inventory
- Policy Rules
- Runtime / Pipeline Health
- Reports

More screenshots: [Dashboard User Guide](docs/04-user-guide/README.md)

---

## Quick Start

### Option A — Run the released images

Fortuna v1.0.0 publishes Core, Agent, and Dashboard images to GHCR:

```text
ghcr.io/shino-337/fortuna-community/fortuna-core:v1.0.0
ghcr.io/shino-337/fortuna-community/fortuna-agent:v1.0.0
ghcr.io/shino-337/fortuna-community/fortuna-dashboard:v1.0.0
```

Prerequisites:

- Kubernetes 1.28+
- `kubectl`
- A working StorageClass
- Internet access from cluster nodes, unless images are mirrored locally

Then follow the maintained [Quickstart](docs/01-getting-started/QUICKSTART.md).

### Option B — Build from source

```bash
git clone https://github.com/shino-337/Fortuna-Community.git
cd Fortuna-Community

go work use ./core ./agent ./api
```

For a complete local deployment:

```bash
export FORTUNA_ADMIN_PASSWORD='<strong-password>'
export FORTUNA_JWT_SECRET="$(openssl rand -base64 32)"
export FORTUNA_POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '=+/ ' | cut -c1-24)"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
export FORTUNA_PACKAGE_SOURCE=local

./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-runtime
```

Open the dashboard through a port-forward:

```bash
kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80
```

Then open `http://127.0.0.1:8081/`.

For installation, environment requirements, verification, and troubleshooting, start with [Getting Started](docs/01-getting-started/README.md).

---

## Architecture

```mermaid
flowchart LR
    K8S[Kubernetes Clusters] --> Agent[Fortuna Agent DaemonSet]
    Agent -->|gRPC / mTLS| Core[Fortuna Core]
    Falco[Falco / eBPF] --> Agent
    Core --> NATS[NATS JetStream]
    Core --> DB[(PostgreSQL)]
    NATS --> Workers[Security Workers]
    Workers --> DB
    Workers --> Risk[Risk + Attack Path Engine]
    Risk --> Core
    Core --> Dashboard[Fortuna Dashboard]
```

### Multi-cluster

A management cluster runs Core, Dashboard, PostgreSQL, and NATS. Additional Kubernetes clusters run the Fortuna Agent and optionally Falco. Agents send telemetry to the management Core over authenticated gRPC/mTLS.

See the [Architecture Guide](docs/02-architecture/ARCHITECTURE.md).

---

## Repository Layout

```text
Fortuna-Community/
├── agent/       # Kubernetes/runtime collection agent
├── core/        # API, workers, risk engine, attack paths, persistence
├── api/         # Shared protobuf/API contracts
├── dashboard/   # React/Vite security dashboard
├── deploy/      # Kubernetes deployment manifests
├── scenarios/   # Attack-path validation scenarios
├── scripts/     # Build, deploy and verification automation
└── docs/        # User-facing documentation
```

---

## Validation & Security Scenarios

Fortuna includes reproducible attack-path scenarios rather than relying only on static screenshots or synthetic scores.

Examples include:

- RBAC privilege escalation
- Service Account token abuse
- HostPath-based escape surfaces
- Noisy discovery / reconnaissance
- Lateral movement
- Broken/incomplete attack chains

Scenario definitions live under [`scenarios/`](scenarios/), with verification tooling under [`scripts/verify/`](scripts/verify/).

---

## Documentation

| Need | Start here |
|---|---|
| Install Fortuna | [Getting Started](docs/01-getting-started/README.md) |
| Fast deployment | [Quickstart](docs/01-getting-started/QUICKSTART.md) |
| Understand the architecture | [Architecture](docs/02-architecture/ARCHITECTURE.md) |
| Explore components | [Component Catalog](docs/03-components/README.md) |
| Use the dashboard | [User Guide](docs/04-user-guide/README.md) |
| Understand operational workflows | [Use Cases](docs/04-user-guide/USE_CASES.md) |
| Production deployment | [Production Deployment](docs/05-operations/PRODUCTION_DEPLOYMENT.md) |
| Security / credentials | [Security Reference](docs/06-reference/SECURITY.md) |

---

## Project Status

Fortuna is an **active community project**. The first public release is `v1.0.0` and the repository continues to evolve across risk scoring, attack-path analysis, runtime detection, and operational hardening.

The project is deliberately security-engineering focused. Expect the codebase and detection model to evolve as new Kubernetes attack techniques and operational requirements are validated.

See the [latest release](https://github.com/shino-337/Fortuna-Community/releases) and repository [Pull Requests](https://github.com/shino-337/Fortuna-Community/pulls) for current implementation work.

---

## Contributing

Contributions are welcome, especially in:

- Kubernetes attack-path research
- RBAC / privilege-escalation detection
- Falco and eBPF runtime detection
- SBOM / vulnerability intelligence
- Risk scoring and security analytics
- Dashboard UX and visualization
- Reproducible security scenarios and tests

Before contributing, read [CONTRIBUTING.md](CONTRIBUTING.md).

Good first contributions should be reproducible, security-focused, and include tests or verification evidence where practical.

---

## Security

Please do **not** disclose security vulnerabilities through public issues. Follow the process in [SECURITY.md](SECURITY.md).

Fortuna is intended for authorized security testing and defensive Kubernetes security operations.

---

## License

Fortuna is released under the [Apache License 2.0](LICENSE).

---

<p align="center">
  <strong>Fortuna — understand the attack path, not just the vulnerability.</strong>
</p>
