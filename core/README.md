# Fortuna Core

**Version**: 1.0.0  
**Status**: Production Ready

Fortuna Core is the central processing and storage component of the Fortuna security platform. It provides SBOM management, CVE matching, security insights generation, policy evaluation, and comprehensive REST/gRPC APIs.

---

## Overview

Fortuna Core serves as the brain of the Fortuna platform, orchestrating security analysis across Kubernetes clusters. It receives SBOM data from Agents, performs CVE matching, generates security insights, and exposes data via REST and gRPC APIs.

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

### Components

```
core/
├── cmd/
│   └── main.go                    # Application entry point
├── internal/
│   ├── api/                       # REST API handlers
│   │   ├── routes.go              # Route definitions
│   │   ├── insights_handlers.go  # Insights API
│   │   ├── sbom_handlers.go      # SBOM API
│   │   └── ...
│   ├── grpc/                      # gRPC server
│   │   ├── handler_sbom.go        # SBOM gRPC handlers
│   │   └── ...
│   ├── storage/                   # Database layer
│   │   └── storage.go            # GORM database connection
│   ├── config/                    # Configuration management
│   ├── health/                    # Health check endpoints
│   ├── middleware/                # HTTP middleware (CORS, auth, metrics)
│   ├── scheduler/                 # Background jobs
│   ├── webhook/                   # Admission webhook
│   └── service/                   # Business logic services
├── pkg/
│   ├── messaging/                 # NATS JetStream client
│   │   ├── nats_client.go        # NATS connection & streams
│   │   └── publisher.go          # Event publishing
│   ├── worker/                    # Background workers
│   │   ├── sbom_worker.go        # SBOM processing worker
│   │   ├── cve_matcher_worker.go # CVE matching worker
│   │   ├── correlator_worker.go  # Event correlation worker
│   │   ├── risk_worker.go        # Risk scoring worker
│   │   └── pool.go               # Worker pool management
│   ├── cve/                       # CVE matching logic
│   ├── models/                    # Database models
│   ├── policy/                    # Policy engine
│   ├── reconciler/                # State reconciliation
│   └── metrics/                   # Prometheus metrics
├── migrations/                    # Database migrations
│   ├── migrations.go              # Migration orchestrator
│   └── mvp2/                      # MVP2 migrations
├── proto/                         # gRPC proto definitions
├── Dockerfile
├── go.mod
└── go.sum
```

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

**NATS JetStream Streams**:

- `fortuna-raw`: Raw events from agents (WorkQueuePolicy, 24h, 100K msgs, 1GB)
- `fortuna-events`: Normalized events and SBOM/CVE processing (WorkQueuePolicy, 48h, 200K msgs, 2GB)
- `fortuna-insights`: Insight generation events (LimitsPolicy, 48h, 50K msgs, 512MB)
- `fortuna-normalized`: Normalized event processing (WorkQueuePolicy, 24h, 100K msgs, 1GB)

**NATS Subjects**:

- `fortuna.raw.*`: Raw inventory events
- `fortuna.events.runtime`: Runtime events
- `fortuna.sbom.created`: SBOM creation events
- `fortuna.cve.*`: CVE-related events
- `fortuna.insights.created`: Insight creation events
- `fortuna.normalized.*`: Normalized events

**Workers**:

- **SBOM Worker**: Processes SBOM creation events
- **CVE Matcher Worker**: Matches CVEs and generates insights
- **Correlator Worker**: Correlates events across resources
- **Risk Worker**: Calculates risk scores

### 5. REST API

Comprehensive REST API with 50+ endpoints:

- **Health**: `/health`, `/ready`, `/live`
- **Insights**: `/api/v1/insights` (with filters, summary, actions)
- **SBOMs**: `/api/v1/sboms` (list, get, components)
- **CVEs**: `/api/v1/cves` (list, get, matches)
- **Risk**: `/api/v1/risk` (scores, trends, analytics)
- **Clusters**: `/api/v1/clusters`
- **Pods**: `/api/v1/pods`
- **Service Accounts**: `/api/v1/serviceaccounts`
- **Graph**: `/api/v1/graph` (blast radius, attack paths)
- **Audit**: `/api/v1/audit`
- **Policy**: `/api/v1/policy`
- **Metrics**: `/metrics` (Prometheus)

See [API Reference](../../docs/API_REFERENCE.md) for complete documentation.

### 6. gRPC API

- `RegisterAgent`: Agent registration
- `Ping`: Health check
- `SubmitSBOM`: SBOM submission from Agents
- `Heartbeat`: Agent heartbeat

---

## Configuration

### Environment Variables

**Database**:
- `DATABASE_URL`: PostgreSQL connection string (required)
  - Example: `postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable`

**NATS**:
- `NATS_ENDPOINT`: NATS server endpoint (required)
  - Example: `nats://nats-client.fortuna.svc.cluster.local:4222`

**Server**:
- `HTTP_PORT`: HTTP server port (default: `8080`)
- `GRPC_PORT`: gRPC server port (default: `9090`)
- `LOG_LEVEL`: Log level (default: `info`)

**TLS/mTLS**:
- `TLS_ENABLED`: Enable TLS (default: `false`)
- `TLS_CERT_PATH`: Server certificate path
- `TLS_KEY_PATH`: Server private key path
- `TLS_CA_CERT_PATH`: CA certificate path

**Authentication**:
- `AUTH_ENABLED`: Enable JWT authentication (default: `false`)
- `JWT_SECRET`: JWT signing secret

**NATS Durables**:
- `FORTUNA_JS_DURABLES`: Enable durable consumers (default: `false`)

---

## Build

### Prerequisites

- Go 1.21+
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

See [Production Deployment Guide](../../docs/PRODUCTION_DEPLOYMENT.md) for complete deployment instructions.

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

See [Migrations Guide](../../docs/MIGRATIONS.md) for complete schema documentation.

---

## Migrations

Core automatically runs database migrations on startup. Migrations are located in `migrations/` and are executed in order.

To verify migrations:
```bash
psql $DATABASE_URL -c "SELECT * FROM schema_migrations ORDER BY version;"
```

---

## Monitoring

### Health Endpoints

- `GET /health`: Basic health check
- `GET /ready`: Readiness check (database + NATS)
- `GET /live`: Liveness check

### Metrics

Prometheus metrics available at `/metrics`:

- Database connection pool metrics
- Worker queue depth
- CVE matching performance
- API request metrics
- Admission webhook metrics

---

## Development

### Generate gRPC Code

```bash
# Install protoc and plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Go code from proto
protoc --go_out=. --go-grpc_out=. proto/fortuna.proto
```

### Run Tests

```bash
go test ./...
```

### Code Structure

- **`internal/`**: Internal packages (not exported)
- **`pkg/`**: Public packages (exported)
- **`migrations/`**: Database migrations
- **`proto/`**: gRPC proto definitions

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

- [Architecture](../../docs/ARCHITECTURE.md)
- [API Reference](../../docs/API_REFERENCE.md)
- [Production Deployment](../../docs/PRODUCTION_DEPLOYMENT.md)
- [Migrations](../../docs/MIGRATIONS.md)

---

**Version**: 1.0.0  
**Last Updated**: 2026-01-06
