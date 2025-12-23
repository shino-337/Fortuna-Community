# Core Component

The KSAM Core service is the central hub, coordinating all components and providing APIs, worker pipelines, and data processing.

---

## Overview

**Status**: ✅ Production Ready (MVP2)
**Deployment**: Kubernetes Deployment (3 replicas recommended)
**Database**: PostgreSQL with Apache AGE extension
**Message Queue**: NATS for async processing

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    KSAM Core Service                    │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │   REST API   │  │  gRPC Server │  │   Webhook    │ │
│  │   :8080      │  │    :50051    │  │   :8443      │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘ │
│         │                 │                  │         │
│         ▼                 ▼                  ▼         │
│  ┌─────────────────────────────────────────────────┐  │
│  │           Service Layer / Managers              │  │
│  └──────────────────────┬──────────────────────────┘  │
│                         │                             │
│         ┌───────────────┼───────────────┐            │
│         ▼               ▼               ▼            │
│  ┌──────────┐  ┌──────────────┐  ┌──────────┐      │
│  │   NATS   │  │ Worker Pool  │  │PostgreSQL│      │
│  │  Queue   │  │              │  │ + AGE    │      │
│  └────┬─────┘  └──────────────┘  └──────────┘      │
│       │                                              │
│       └──▶ Normalizer Worker                        │
│       └──▶ Correlator Worker                        │
│       └──▶ Risk Worker                               │
│       └──▶ CVE Matcher Worker                        │
└─────────────────────────────────────────────────────────┘
```

---

## Key Components

### 1. API Server (Port 8080)

**REST Endpoints**:
```
GET  /api/v1/serviceaccounts
GET  /api/v1/pods
GET  /api/v1/insights
GET  /api/v1/risk-trends
GET  /api/v1/attack-paths
POST /api/v1/policies
```

**Authentication**: JWT-based (optional: AUTH_ENABLED)

### 2. gRPC Server (Port 50051)

**Services**:
- `AgentService.SyncServiceAccounts`
- `AgentService.SyncPods`
- `AgentService.GetCertificate`

**Security**: mTLS enabled

### 3. Admission Webhook (Port 8443)

**Validating Webhooks**:
- Pod creation validation
- Policy enforcement
- CVE-based blocking

**Configuration**:
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: ksam-webhook
webhooks:
- name: validate.pods.ksam.io
  clientConfig:
    service:
      name: ksam-core
      namespace: ksam
      path: /validate-pods
```

### 4. Worker Pool

**Workers**:
- **Normalizer**: Cleans and standardizes incoming data
- **Correlator**: Builds relationships in graph database
- **Risk Worker**: Evaluates risk scores and creates insights
- **CVE Matcher**: Matches SBOMs against CVE database

**Processing**:
- NATS-based message queue
- Concurrent processing (10-50 workers)
- DLQ for failed messages
- Retry with exponential backoff

---

## Configuration

### Environment Variables

```yaml
# Database
DATABASE_URL: "postgresql://user:pass@postgres:5432/ksam"
POSTGRES_SSLMODE: "disable"

# NATS
NATS_URL: "nats://nats:4222"
NATS_CLUSTER_ID: "ksam-cluster"

# API
PORT: "8080"
AUTH_ENABLED: "true"
JWT_SECRET: "change-me-in-production"

# gRPC
GRPC_PORT: "50051"
TLS_ENABLED: "true"
MTLS_ENABLED: "true"

# Webhook
WEBHOOK_PORT: "8443"
WEBHOOK_TLS_CERT: "/etc/webhook/certs/tls.crt"
WEBHOOK_TLS_KEY: "/etc/webhook/certs/tls.key"

# Workers
WORKER_POOL_SIZE: "20"
WORKER_BATCH_SIZE: "100"
NORMALIZER_ENABLED: "true"
CORRELATOR_ENABLED: "true"
RISK_WORKER_ENABLED: "true"
CVE_MATCHER_ENABLED: "true"

# Features
CVE_SCANNING_ENABLED: "true"
POLICY_ENFORCEMENT_ENABLED: "true"
GRAPH_ANALYSIS_ENABLED: "true"
```

---

## API Reference

### ServiceAccounts

```bash
# List all service accounts
curl http://localhost:8080/api/v1/serviceaccounts

# Get specific service account
curl http://localhost:8080/api/v1/serviceaccounts/{namespace}/{name}

# Response:
{
  "name": "default",
  "namespace": "kube-system",
  "uid": "abc-123",
  "cluster_name": "production",
  "created_at": "2025-01-15T10:00:00Z",
  "risk_score": 7.5,
  "roles_count": 2,
  "pods_count": 5
}
```

### Insights

```bash
# List insights
curl http://localhost:8080/api/v1/insights?severity=CRITICAL&status=active

# Response:
{
  "insights": [
    {
      "id": "uuid-123",
      "title": "Cluster Admin Binding",
      "description": "ServiceAccount has cluster-admin role",
      "severity": "CRITICAL",
      "risk_score": 10.0,
      "status": "active",
      "created_at": "2025-01-15T10:30:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

### Risk Trends

```bash
# Get risk trends over time
curl http://localhost:8080/api/v1/risk-trends?days=30

# Response:
{
  "trends": [
    {
      "date": "2025-01-01",
      "avg_risk_score": 5.2,
      "critical_count": 12,
      "high_count": 45,
      "medium_count": 123,
      "low_count": 456
    }
  ]
}
```

---

## Worker Pipeline Details

### Normalizer Worker

**Purpose**: Clean and standardize incoming data

**Processing**:
```
Agent Data → Validate → Normalize → Enrich → Database
```

**Metrics**:
- `ksam_normalizer_processed_total`
- `ksam_normalizer_errors_total`
- `ksam_normalizer_duration_seconds`

### Correlator Worker

**Purpose**: Build relationships in graph database

**Processing**:
```
ServiceAccount → Find Pods → Find Roles → Build Graph Edges
```

**Graph Queries**:
- Who can access what
- What can access who
- Attack path analysis

### Risk Worker

**Purpose**: Evaluate risk scores and create insights

**Processing**:
```
Resource → Apply Risk Rules → Calculate Score → Create Insights
```

**Risk Sources**:
- Policy violations (CEL rules)
- CVE matches (vulnerability scanner)
- RBAC analysis (excessive permissions)
- Behavioral anomalies

### CVE Matcher Worker

**Purpose**: Match SBOMs against CVE database

**Processing**:
```
SBOM Created → Load Components → Query CVE DB → Create Matches → Create Insights
```

**Performance**:
- 200ms per SBOM (uncached)
- 50ms per SBOM (cached)

---

## Monitoring

### Prometheus Metrics

```
# API metrics
ksam_http_requests_total{method="GET",endpoint="/api/v1/insights"}
ksam_http_request_duration_seconds

# Worker metrics
ksam_worker_queue_length{worker="normalizer"}
ksam_worker_processed_total{worker="risk"}
ksam_worker_errors_total{worker="correlator"}

# Database metrics
ksam_db_connections_active
ksam_db_query_duration_seconds

# NATS metrics
ksam_nats_messages_published_total
ksam_nats_messages_consumed_total
```

### Health Endpoints

```bash
# Liveness probe
curl http://localhost:8080/healthz

# Readiness probe
curl http://localhost:8080/readyz

# Detailed health
curl http://localhost:8080/health
# Returns:
{
  "status": "healthy",
  "database": "ok",
  "nats": "ok",
  "workers": {
    "normalizer": "running",
    "correlator": "running",
    "risk": "running"
  }
}
```

---

## Troubleshooting

### Core Pod Not Starting

```bash
# Check logs
kubectl logs -n ksam ksam-core-*

# Common issues:
# 1. Database connection failed
kubectl exec -n ksam postgres-* -- pg_isready

# 2. NATS connection failed
kubectl get pods -n ksam -l app=nats

# 3. Certificate issues
kubectl describe secret -n ksam ksam-core-certs
```

### Workers Not Processing

```bash
# Check NATS queue
kubectl exec -n ksam nats-* -- nats stream info

# Check worker logs
kubectl logs -n ksam ksam-core-* | grep -i worker

# Check dead letter queue
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) FROM dead_letter_queue;"
```

### API Returning 500 Errors

```bash
# Check database connection
kubectl logs -n ksam ksam-core-* | grep -i "database\\|postgres"

# Check query performance
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT query, calls, mean_exec_time FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;"

# Enable SQL logging
kubectl set env deployment/ksam-core -n ksam LOG_LEVEL=debug
```

---

## Development

### Local Development

```bash
# Start dependencies
docker-compose up -d postgres nats

# Set environment
export DATABASE_URL="postgresql://postgres:postgres@localhost:5432/ksam"
export NATS_URL="nats://localhost:4222"

# Run migrations
cd core
go run cmd/migrate/main.go

# Run core
go run cmd/main.go
```

### Running Tests

```bash
cd core

# Unit tests
go test ./... -v

# Integration tests
go test ./tests/integration/... -v

# E2E tests
./scripts/run_e2e_tests.sh
```

---

## Related Components

- [Agent](../agent/) - Collects data, sends to Core
- [CVE Scanner](../cve-scanner/) - CVE database used by Core
- [SBOM Generator](../sbom/) - SBOM data processed by Core
- [Policy Engine](../policy-engine/) - Policy evaluation in Core
- [Risk Engine](../risk-engine/) - Risk calculation in Core
- [Graph Engine](../graph-engine/) - Graph queries from Core
- [Dashboard](../dashboard/) - Consumes Core APIs

---

## Performance Tuning

### Database Optimization

```sql
-- Create indexes
CREATE INDEX CONCURRENTLY idx_insights_severity_status
ON insights(severity, status) WHERE deleted_at IS NULL;

-- Vacuum
VACUUM ANALYZE;

-- Connection pooling
-- Set in DATABASE_URL: ?pool_max_conns=25
```

### Worker Scaling

```bash
# Increase worker count
kubectl set env deployment/ksam-core -n ksam WORKER_POOL_SIZE=50

# Increase batch size
kubectl set env deployment/ksam-core -n ksam WORKER_BATCH_SIZE=200
```

### API Rate Limiting

```yaml
# Enable rate limiting
RATE_LIMIT_ENABLED: "true"
RATE_LIMIT_REQUESTS: "100"
RATE_LIMIT_WINDOW: "60s"
```

---

**Last Updated**: December 16, 2025
**Status**: ✅ Production Ready
