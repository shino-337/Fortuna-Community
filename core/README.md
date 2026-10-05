# Fortuna Core

Fortuna Core is the central processing and storage component of **Fortuna**. It provides SBOM management, CVE matching, security insights generation, policy evaluation, and comprehensive REST/gRPC APIs.

---

## Overview

Fortuna Core orchestrates security analysis across Kubernetes clusters. It receives SBOM data from Agents, performs CVE matching, generates security insights, and exposes data via REST and gRPC APIs.

### Key Responsibilities

- **SBOM Storage & Management**: Store and manage Software Bill of Materials from container images
- **CVE Matching**: Real-time vulnerability detection by matching SBOM components against CVE database
- **Insight Generation**: Automated security insights with severity classification (CRITICAL/HIGH/MEDIUM)
- **Policy Evaluation**: Configurable security policies with CEL-based evaluation
- **Event-Driven Architecture**: NATS JetStream for asynchronous processing
- **API Services**: REST API for dashboard and external tools, gRPC for Agent communication
- **Database Management**: PostgreSQL with automatic migrations

---

## Architecture

### Layout

```
core/
├── cmd/
│   ├── main.go               # Core server
│   ├── cve-loader/           # Loads an OSV/CVE catalog into PostgreSQL
│   └── migration-rehearsal/  # Runs migrations against a restored backup
├── internal/
│   ├── api/                  # REST handlers; routes*.go register them and
│   │                         # route_security_inventory.go declares every route
│   ├── grpc/                 # Agent gRPC server and per-RPC authorization
│   ├── auth/, sessions/      # JWT login and server-side sessions
│   ├── middleware/           # CORS, auth, cluster scope, metrics
│   ├── ingest/               # Agent HTTP ingest
│   ├── webhook/              # Admission webhook
│   ├── scheduler/, service/  # Background jobs and services
│   └── config/, health/, storage/, metrics/, k8s/, repository/
├── pkg/
│   ├── agentidentity/        # Per-Agent HTTP and gRPC credential registries
│   ├── authorization/        # Permissions and cluster scope
│   ├── messaging/            # NATS JetStream client and streams
│   ├── worker/               # SBOM, CVE matcher, correlator, risk and DLQ workers
│   ├── riskengine/, risk/    # Rule evaluation, scoring and resolution evidence
│   ├── rbacinventory/        # Shared RBAC grant resolver
│   ├── graph/                # Attack graph and cluster-scoped AGE queries
│   ├── mutations/            # Reviewed ServiceAccount revocation
│   ├── sourcehealth/         # Signed runtime source-health verification
│   ├── models/               # Database models
│   └── ...                   # cve, sbom, policy, malware, epss, kev and more
├── migrations/               # Schema migrations (see migrations/README.md)
└── Dockerfile
```

The gRPC protocol is defined in the shared [`api`](../api/README.md) module.

---

## Features

### 1. SBOM Processing

- Receives SBOMs from Agents via gRPC
- Stores SBOMs in PostgreSQL with full metadata
- Extracts components with PURL (Package URL) support
- Supports multiple package ecosystems (dpkg, apk, rpm, npm, pip, gomod)

### 2. CVE Matching

- **Ecosystem-Specific Version Comparison**:
  - Debian: `github.com/knqyf263/go-deb-version`
  - RPM/Alpine/npm/pypi/go: `hashicorp/go-version`
- **Bulk Processing**: Efficient batch queries by ecosystem
- **Deduplication**: Unique constraints prevent duplicate matches
- **Severity Filtering**: Configurable (CRITICAL/HIGH/MEDIUM)

### 3. Security Insights

- **Automatic Generation**: From CVE matches
- **Resource Context**: Links insights to pods, namespaces, clusters
- **Severity Classification**: CRITICAL, HIGH, MEDIUM, LOW
- **Status Management**: active, resolved, dismissed
- **Batch Upsert**: Efficient bulk operations

### 4. Event-Driven Architecture

**NATS JetStream streams** (file storage, 3 replicas, oldest messages discarded at the limit; `core/pkg/messaging/nats_client.go`):

- `fortuna-raw`: raw inventory events (work queue, 24h, 100K messages, 1 GiB)
- `fortuna-events`: runtime, SBOM and CVE events (work queue, 48h, 200K messages, 2 GiB)
- `fortuna-insights`: insight created/updated events (limits, 7 days, 50K messages, 512 MiB)
- `fortuna-normalized`: normalized events (work queue, 24h, 100K messages, 1 GiB)
- `fortuna-siem`: security audit events for SIEM export (limits, 7 days, 100K messages, 1 GiB)

**NATS Subjects**:

- `fortuna.raw.*`: Raw inventory events
- `fortuna.events.runtime`: Runtime events
- `fortuna.sbom.created`: SBOM creation events
- `fortuna.cve.*`: CVE-related events
- `fortuna.insights.created`, `fortuna.insights.updated`: Insight events
- `fortuna.normalized.*`: Normalized events
- `fortuna.siem.events`: Security audit events

**Workers**:

- **SBOM Worker**: Processes SBOM creation events
- **CVE Matcher Worker**: Matches CVEs and generates insights
- **Correlator Worker**: Correlates events across resources
- **Risk Worker**: Calculates risk scores

### 5. REST API

REST API used by the dashboard and external tools. Health endpoints are listed under [Monitoring](#health-endpoints); the route groups are in the [API route overview](../docs/architecture/API_STANDARD.md).

Every `/api/*` route is declared in `internal/api/route_security_inventory.go`; Core refuses to start if a registered route is missing from that inventory.


### 6. gRPC API

Defined in `api/proto/agent/service.proto` and served on `GRPC_PORT` with TLS. Agent calls are authorized per RPC against the per-Agent certificate registry ([Agent identity](../docs/reference/AGENT_IDENTITY.md)).

- `RegisterAgent`, `Heartbeat`, `Ping`: Agent registration and liveness
- `SendSBOMFinding`: SBOM submission from an Agent
- `BatchSendSBOMFindings`: client-streamed SBOM submission
- `SendCVEFinding` and `SendCombinedFinding` are not served: Core derives CVE matches from SBOMs itself.

---

## Configuration

Core needs `DATABASE_URL`, `NATS_ENDPOINT` and a `JWT_SECRET` of at least 32 bytes (or `FORTUNA_DEV_MODE=1` for local development). Every other setting, with its default, is in the [configuration reference](../docs/reference/CONFIGURATION.md); Agent credentials are described in [Agent identity](../docs/reference/AGENT_IDENTITY.md).

---

## Build

### Prerequisites

- Go 1.26+ (see `go.mod`)
- PostgreSQL 15+
- NATS JetStream 2.10+

### Build Binary

```bash
go build -o bin/fortuna-core ./cmd
```

### Build Docker Image

```bash
docker build -t fortuna-core:latest -f Dockerfile .
```

Or using `nerdctl`:

```bash
nerdctl build -t fortuna-core:latest -f Dockerfile .
```

---

## Run

### Local Development

1. **Start PostgreSQL**:
```bash
docker run -d -p 5432:5432 \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=fortuna \
  postgres:15-alpine
```

2. **Start NATS**:
```bash
docker run -d -p 4222:4222 -p 8222:8222 \
  nats:latest -js
```

3. **Set Environment Variables**:
```bash
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/fortuna?sslmode=disable
export NATS_ENDPOINT=nats://localhost:4222
export HTTP_PORT=8080
export GRPC_PORT=9090
export FORTUNA_DEV_MODE=1   # generates an ephemeral JWT secret; set JWT_SECRET (32+ bytes) otherwise
```

4. **Run**:
```bash
go run cmd/main.go
```

Or using binary:
```bash
./bin/fortuna-core
```

### Kubernetes Deployment

Install with [Install on a cluster](../docs/getting-started/QUICKSTART.md), then harden with the [production deployment](../docs/operations/PRODUCTION_DEPLOYMENT.md) guide.

---

## Database Schema

### Key Tables

- **`sboms`**: SBOM metadata and content
- **`sbom_components`**: Individual packages from SBOMs
- **`cve_matches`**: CVE matches for packages
- **`cves`**: CVE metadata
- **`package_vulnerabilities`**: Package-to-CVE mappings
- **`insights`**: Security insights
- **`pods`**: Pod information
- **`clusters`**: Cluster information
- **`service_accounts`**: Service account data
- **`audit_logs`**: Audit log entries

See [migrations/README.md](migrations/README.md) for migration layout and conventions.

---

## Migrations

Core automatically runs database migrations on startup. Migrations are located in `migrations/` and are executed in order.

---

## Monitoring

### Health Endpoints

- `GET /healthz`, `GET /live`: liveness (process is up)
- `GET /ready`: readiness (HTTP and gRPC listeners are up; does not check PostgreSQL or NATS)
- `GET /health`: PostgreSQL connectivity
- `GET /status`: full dependency status, including PostgreSQL and NATS

### Metrics

Set `FORTUNA_METRICS_ADDR` (for example `:9091`, as the bundled manifest does) to serve Prometheus metrics at `/metrics` on a separate listener. It is unauthenticated, so keep that port off any public Service; it is disabled when the variable is unset. Metrics cover HTTP requests, ingest, workers, database connections, CVE matching, insights and the admission webhook (`core/pkg/metrics`, `core/internal/metrics`). Authenticated users can also read operational counters through `GET /api/v1/metrics/system` and `GET /api/v1/metrics/workers`.

---

## Development

### Generate gRPC Code

The protocol lives in the shared module; regenerate it with `make -C api/proto/agent` from the repository root.

### Run Tests

```bash
go test ./...
```

### Code Structure

- **`internal/`**: Internal packages (not exported)
- **`pkg/`**: Public packages (exported)
- **`migrations/`**: Database migrations

---

## Troubleshooting

### Database Connection Issues

- Check `DATABASE_URL` format
- Verify PostgreSQL is accessible
- Check network policies (if in Kubernetes)

### NATS Connection Issues

- Verify `NATS_ENDPOINT` is correct
- Check NATS cluster status
- Verify stream creation (check logs)

### Migration Issues

- Check migration logs in startup output
- Verify database permissions
- Check for schema conflicts

### Worker Issues

- Check NATS stream status
- Verify worker subscriptions
- Check queue depth metrics

---

## Related Documentation

- [Architecture](../docs/architecture/ARCHITECTURE.md)
- [API route overview](../docs/architecture/API_STANDARD.md)
- [Production Deployment](../docs/operations/PRODUCTION_DEPLOYMENT.md)
- [Configuration reference](../docs/reference/CONFIGURATION.md)
- [Agent identity](../docs/reference/AGENT_IDENTITY.md)
- [Migrations](migrations/README.md)
