# FortunaK8s

Kubernetes security and risk management platform with SBOM ingestion, CVE matching, risk insights, and dashboard visibility.

## Overview

FortunaK8s provides an end-to-end security pipeline:

- Agent collects runtime and SBOM data from cluster workloads.
- Core normalizes and stores SBOM/components, then matches vulnerabilities.
- Worker pipeline generates CVE matches and risk insights.
- Dashboard surfaces vulnerabilities, risk posture, and operational status.

The project is optimized for deterministic SBOM/CVE processing with replay protection, trust-aware matching, and clear observability.

## Features

- SBOM ingestion with PURL validation and canonicalization
- Trust-aware CVE matcher (high/medium/low trust handling)
- Deterministic resolver logic with stable ordering
- Replay guard for `sbom.created` event processing
- Snapshot-driven matching to reduce DB race/drift
- NATS JetStream event pipeline
- REST and gRPC APIs
- Web dashboard for security and risk operations

## Architecture

High-level flow:

1. Agent extracts findings and sends to Core over gRPC.
2. Core stores SBOM/components and publishes `fortuna.sbom.created`.
3. CVE matcher worker consumes events, applies replay/idempotency guards, resolves components, and matches CVEs.
4. Matches and insights are persisted for APIs and dashboard.

Core modules:

- `agent/`: in-cluster data collection and gRPC client
- `core/`: API, repository, matcher, workers, migrations
- `api/`: protobuf contracts shared by Agent and Core
- `dashboard/`: frontend UI
- `deploy/`: Kubernetes deployment manifests and configs
- `docs/`: technical and operational documentation

## Repository Structure

| Path | Purpose |
|---|---|
| `agent/` | Agent daemon and extraction pipeline |
| `core/` | Core backend, worker, CVE matcher, migrations |
| `api/` | Shared protobuf contracts (`github.com/fortuna/api`) |
| `dashboard/` | Web dashboard (React/Vite) |
| `deploy/` | Kubernetes manifests and deployment assets |
| `docs/` | System and component documentation |
| `scripts/` | Build/deploy/verify utilities |
| `tests/` | E2E and test assets |

## Configuration highlights (Core)

- **`NVD_API_KEY`** (optional, recommended): NIST NVD API key for Fortuna Core’s CVE matcher NVD fallback — higher rate limits. See [`docs/05-operations/NVD_API_KEY.md`](docs/05-operations/NVD_API_KEY.md) and `core/README.md` (Environment Variables).
- Disable NVD entirely: `FORTUNA_NVD_DISABLED=1`.

## Install

### Prerequisites

- Go 1.24+
- Docker or nerdctl/containerd toolchain
- Kubernetes cluster (local or remote)
- `kubectl`

### Clone

```bash
git clone https://github.com/shino-337/KSAM.git
cd KSAM
```

### Go workspace

This repository uses a multi-module workspace:

```bash
go work use ./core ./agent ./api
```

## Quick Start

### 1) Build images

```bash
./scripts/build/build-and-load-containerd.sh
```

### 2) Deploy stack

```bash
./scripts/deploy/deploy-fortuna-robust.sh
```

### 3) Verify deployment

```bash
./scripts/verify/check-full-deployment.sh
```

### 4) Access services

```bash
# Dashboard
kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80

# Core health
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
```

- Dashboard: [http://localhost:8081](http://localhost:8081)
- Core health: [http://localhost:8080/health](http://localhost:8080/health)

Default local login (change in production): `admin` / `admin123`

## Development

### Run tests

```bash
# Core
cd core && go test ./...

# Agent
cd ../agent && go test ./...
```

### Race tests (recommended for matcher/worker paths)

```bash
cd core
CGO_ENABLED=1 go test -race ./internal/repository ./pkg/worker ./pkg/cve/matcher
```

## Documentation

- Main docs index: `docs/README.md`
- SBOM component contract: `docs/03-components/sbom/SBOM_COMPONENT_SPEC.md`
- PURL policy: `docs/03-components/sbom/PURL_VALIDATION_POLICY.md`
- Event replay contract: `docs/03-components/sbom/SBOM_EVENT_REPLAY_CONTRACT.md`
- Deployment docs: `deploy/README.md`

## Roadmap (Short)

- Continue hardening replay/idempotency workflows
- Improve NVD strategy (cache/batch first)
- Expand matcher explainability and performance profiling

## Contributing

Contributions are welcome.

- Open an issue for bugs or feature proposals
- Keep changes scoped and covered by tests
- Update related docs/specs when contracts change

## License

This project is licensed under the Apache License 2.0.

See [LICENSE](LICENSE) for details.
