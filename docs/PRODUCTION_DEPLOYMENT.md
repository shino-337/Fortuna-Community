# Fortuna Production Deployment Guide

**Version**: 1.1  
**Last Updated**: 2026-01-05  
**Status**: Production Ready  
**Changes**: Policy evaluator made optional, Agent imagePullPolicy set to IfNotPresent

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start](#quick-start)
3. [Environment Preparation](#environment-preparation)
4. [Build and Package](#build-and-package)
5. [Infrastructure Deployment](#infrastructure-deployment)
6. [Application Deployment](#application-deployment)
7. [Post-Deployment Verification](#post-deployment-verification)
8. [Configuration](#configuration)
9. [Troubleshooting](#troubleshooting)

---

## Quick Start

For a step-by-step checklist, see [DEPLOYMENT_CHECKLIST.md](DEPLOYMENT_CHECKLIST.md).

**Quick deployment commands** (assumes prerequisites are met):

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Generate certificates
bash scripts/create_mtls_secret.sh

# 3. Create secrets
kubectl create secret generic fortuna-secrets \
  --from-literal=database-url="postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable" \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --namespace=fortuna

# 4. Deploy infrastructure
kubectl apply -f deploy/infrastructure/postgresql.yaml
kubectl apply -f deploy/infrastructure/nats.yaml

# 5. Build and load images
bash scripts/build-and-load-containerd.sh

# 6. Deploy RBAC
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/agent-rbac.yaml

# 7. Deploy Core
kubectl apply -f deploy/core-service.yaml
kubectl apply -f deploy/core-deployment.yaml

# 8. Deploy Agent
kubectl apply -f deploy/agent-daemonset.yaml

# 9. Wait for readiness
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s
kubectl wait --for=condition=ready pod -l app=fortuna-agent -n fortuna --timeout=300s
```

---

## Prerequisites

### System Requirements

- **Kubernetes**: v1.25+ (tested with v1.28)
- **Container Runtime**: containerd 1.6+ or Docker 20.10+
- **Storage**: Local storage provisioner (local-path-provisioner) or equivalent
- **Network**: CNI plugin (Flannel, Calico, etc.)
- **Resources**:
  - Master node: 2 CPU, 4GB RAM minimum
  - Worker nodes: 2 CPU, 4GB RAM minimum per node
  - Storage: 50GB+ available for PVCs

### Required Tools

- `kubectl` (v1.25+)
- `nerdctl` or `docker` for building images
- `ctr` (containerd CLI) for image management
- `openssl` for certificate generation
- `psql` (PostgreSQL client) for database operations

### Access Requirements

- Kubernetes cluster admin access
- Ability to create namespaces, secrets, and RBAC resources
- Network access to pull base images (if not using local images)

---

## Environment Preparation

### 1. Create Namespace

```bash
kubectl create namespace fortuna
```

### 2. Generate mTLS Certificates

Fortuna uses mutual TLS (mTLS) for secure communication between Core and Agent components.

```bash
cd /path/to/fortuna
bash scripts/create_mtls_secret.sh
```

This script will:
- Generate CA certificate
- Generate server certificate with SANs for Core
- Generate client certificate with SANs for Agent
- Create Kubernetes secrets:
  - `fortuna-ca-cert`: CA certificate
  - `fortuna-core-tls`: Core server certificate
  - `fortuna-agent-tls`: Agent client certificate
  - `fortuna-webhook-tls`: Webhook server certificate

**Important**: Certificates include Subject Alternative Names (SANs) to avoid "x509: certificate relies on legacy Common Name field" errors.

### 3. Create Application Secrets

Create secrets for database connection and JWT:

```bash
kubectl create secret generic fortuna-secrets \
  --from-literal=database-url="postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable" \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --namespace=fortuna
```

**Production Note**: Use strong, randomly generated secrets. Consider using external secret management (e.g., HashiCorp Vault, AWS Secrets Manager).

---

## Build and Package

### Build Core Image

```bash
cd core
go build -o ../bin/fortuna-core ./cmd
cd ..
nerdctl build -f core/Dockerfile -t fortuna-core:latest --namespace k8s.io .
```

### Build Agent Image

```bash
cd agent
go build -o ../bin/fortuna-agent ./cmd
cd ..
nerdctl build -f agent/Dockerfile -t fortuna/agent:latest --namespace k8s.io .
```

### Load Images to containerd

If using containerd:

```bash
# Verify images are loaded
ctr -n k8s.io images list | grep fortuna
```

### Production Image Tags

For production, use versioned tags:

```bash
VERSION="v1.0.0"
nerdctl tag fortuna-core:latest fortuna-core:${VERSION}
nerdctl tag fortuna/agent:latest fortuna/agent:${VERSION}
```

Update deployment files to use versioned tags and set `imagePullPolicy: IfNotPresent` or `Always`.

---

## Infrastructure Deployment

### 1. Deploy PostgreSQL

```bash
kubectl apply -f deploy/infrastructure/postgresql.yaml
```

Wait for PostgreSQL to be ready:

```bash
kubectl wait --for=condition=ready pod -l app=postgres -n fortuna --timeout=300s
```

### 2. Deploy NATS JetStream

```bash
kubectl apply -f deploy/infrastructure/nats.yaml
```

Wait for NATS cluster to be ready (3 replicas):

```bash
kubectl wait --for=condition=ready pod -l app=nats -n fortuna --timeout=300s
```

Verify NATS cluster status:

```bash
kubectl exec -n fortuna nats-0 -- nats server info
```

### 3. Deploy Redis (Optional)

```bash
kubectl apply -f deploy/infrastructure/redis.yaml
```

---

## Application Deployment

### 1. Deploy RBAC

```bash
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/agent-rbac.yaml
```

### 2. Deploy Core Service

```bash
kubectl apply -f deploy/core-service.yaml
```

**Verify**:
```bash
kubectl get svc fortuna-core -n fortuna
```

### 3. Deploy Core

```bash
kubectl apply -f deploy/core-deployment.yaml
```

Wait for Core to be ready:

```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s
```

**Note**: Core will automatically run database migrations on startup. This may take 1-2 minutes. Monitor migrations:

```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core -f
```

Look for messages like:
- "Running database migrations..."
- "Migration X completed successfully"
- "All migrations completed"

**Verify migrations completed**:
```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';"
```

Expected: At least 20+ tables including `sboms`, `sbom_components`, `cve_matches`, `insights`, etc.

**Automated schema verification**:
```bash
bash scripts/verify-database-schema.sh
```

This script verifies:
- All required tables exist (core, CVE, SBOM tables)
- All critical columns exist with correct types
- Schema integrity

If verification fails, check Core logs for migration errors.

### 4. Verify Database Schema

After Core migrations complete, verify the database schema:

```bash
bash scripts/verify-database-schema.sh
```

This script automatically checks:
- All required tables exist
- All critical columns exist
- Schema integrity

**Expected output**: All tables and columns verified successfully.

### 5. Load CVE Data

Load CVE data from OSV.dev JSON files for CVE matching:

```bash
bash scripts/load-cve-data.sh
```

This script:
- Checks if CVE data directory exists (`/home/k8s/fortuna/cve-data/all`)
- Verifies CVE tables exist
- Checks if data is already loaded (skips if present)
- Creates a Kubernetes Job to load CVE data
- Verifies loaded data

**Note**: CVE data loading may take 10-30 minutes depending on data size. The script will monitor progress automatically.

**Prerequisites**:
- CVE data directory must exist at `/home/k8s/fortuna/cve-data/all` (or set `CVE_DATA_DIR` environment variable to parent directory `/home/k8s/fortuna/cve-data`)
- Core image must be built and loaded (from step 3)

**Note**: The script mounts the parent directory (`/home/k8s/fortuna/cve-data`) to `/cve-data` in the container, allowing access to `/cve-data/all` subdirectory.

**Skip if**: CVE data is not available or CVE matching is not required immediately.

### 6. Deploy Agent

```bash
kubectl apply -f deploy/agent-daemonset.yaml
```

Wait for Agents to be ready (one per node):

```bash
kubectl wait --for=condition=ready pod -l app=fortuna-agent -n fortuna --timeout=300s
```

**Note**: Agent uses `imagePullPolicy: IfNotPresent`. For multi-node clusters:
- Ensure images are built and loaded on all nodes, OR
- Use a container registry accessible to all nodes

**For multi-node clusters**, export and import images:
```bash
# On master node: Export images
nerdctl --namespace k8s.io save -o fortuna-core.tar fortuna-core:latest
nerdctl --namespace k8s.io save -o fortuna-agent.tar fortuna/agent:latest

# Copy to worker nodes and import
scp fortuna-*.tar user@worker:/tmp/
ssh user@worker "ctr -n k8s.io images import /tmp/fortuna-core.tar"
ssh user@worker "ctr -n k8s.io images import /tmp/fortuna-agent.tar"
```

---

## Post-Deployment Verification

### 1. Check Pod Status

```bash
kubectl get pods -n fortuna
```

All pods should be in `Running` state with `READY 1/1`.

**Expected pods**:
- `fortuna-core-*`: 1 pod (Running)
- `fortuna-agent-*`: 1 pod per node (Running)
- `postgres-*`: 1 pod (Running)
- `nats-0`, `nats-1`, `nats-2`: 3 pods (Running)

### 2. Verify Agent-Core Connection

```bash
kubectl logs -n fortuna -l app=fortuna-agent | grep -E "(Heartbeat|Connected|Registered)"
```

You should see successful heartbeat messages.

### 3. Check Core Health

```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

### 4. Verify Database Schema

**Automated verification** (recommended):
```bash
bash scripts/verify-database-schema.sh
```

**Manual verification**:
```bash
# Check total tables (should be 20+)
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';"

# Check key tables exist
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('sboms', 'sbom_components', 'cve_matches', 'insights', 'cves', 'package_vulnerabilities');"
```

**Expected**: All key tables should exist. If `sboms` table is missing, migrations may still be running. Wait and check Core logs.

### 5. Verify CVE Data

Check if CVE data is loaded:

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM cves; SELECT COUNT(*) FROM package_vulnerabilities;"
```

**Expected**: 
- CVE count: > 0 (if CVE data was loaded)
- Package vulnerabilities count: > 0 (if CVE data was loaded)

If CVE data is not loaded and you need CVE matching:
```bash
bash scripts/load-cve-data.sh
```

### 6. Test End-to-End Flow

Create a test pod:

```bash
kubectl run test-pod --image=nginx:alpine --namespace=default --restart=Never
```

Monitor Agent logs:

```bash
kubectl logs -n fortuna -l app=fortuna-agent -f | grep -E "(test-pod|SBOM|Extracted)"
```

Monitor Core logs:

```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core -f | grep -E "(SBOM|Received|saved)"
```

Verify SBOM in database:

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT image_name, namespace, pod_name, package_count FROM sboms ORDER BY created_at DESC LIMIT 5;"
```

---

## Configuration

### Core Configuration

Key environment variables in `deploy/core-deployment.yaml`:

- `DATABASE_URL`: PostgreSQL connection string
- `NATS_ENDPOINT`: NATS JetStream endpoint
- `TLS_ENABLED`: Enable mTLS (should be `true` in production)
- `AUTH_ENABLED`: Enable API authentication
- `FORTUNA_ADMIN_USERNAME`: Admin username (change in production)
- `FORTUNA_ADMIN_PASSWORD`: Admin password (change in production)

### Agent Configuration

Key environment variables in `deploy/agent-daemonset.yaml`:

- `FORTUNA_CORE_ENDPOINT`: Core gRPC endpoint
- `FORTUNA_SYNC_INTERVAL`: Pod sync interval (default: 30s)
- `TLS_ENABLED`: Enable mTLS (should be `true` in production)

### Production Recommendations

1. **Secrets Management**: Use external secret management (Vault, AWS Secrets Manager, etc.)
2. **Resource Limits**: Adjust based on workload:
   - Core: 2Gi memory, 1000m CPU recommended
   - Agent: 2Gi memory, 1000m CPU per node
3. **Replicas**: 
   - Core: 2+ replicas for HA
   - NATS: 3 replicas (already configured)
   - PostgreSQL: Consider HA setup for production
4. **Storage**: Use production-grade storage (e.g., EBS, Azure Disk) instead of local-path
5. **Monitoring**: Deploy Prometheus and Grafana for metrics
6. **Logging**: Configure centralized logging (e.g., ELK, Loki)

---

## Troubleshooting

### Agent Cannot Connect to Core

**Error**: `x509: certificate relies on legacy Common Name field, use SANs instead`

**Solution**: Regenerate certificates with SANs:

```bash
bash scripts/create_mtls_secret.sh
kubectl delete pod -n fortuna -l app.kubernetes.io/component=core
kubectl delete pod -n fortuna -l app=fortuna-agent
```

### SBOM Tables Missing

**Error**: `ERROR: relation "sboms" does not exist`

**Solution**: Run database migrations. See [Migration Guide](MIGRATIONS.md).

### Core Pod CrashLoopBackOff

**Common Causes**:
1. Database connection failure
2. NATS connection failure
3. Missing secrets
4. Migration errors

**Debug Steps**:

```bash
# Check Core logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100

# Check database connectivity
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT 1;"

# Check NATS connectivity
kubectl exec -n fortuna nats-0 -- nats server info

# Verify secrets exist
kubectl get secrets -n fortuna
```

### Agent OOMKilled

**Solution**: Increase memory limits in `deploy/agent-daemonset.yaml`:

```yaml
resources:
  requests:
    memory: "512Mi"
  limits:
    memory: "2Gi"  # Increase if needed
```

### NATS Storage Issues

**Error**: `insufficient storage resources available`

**Solution**: 
1. Check NATS PVC size (should be 10Gi+)
2. Check available storage on nodes
3. Consider increasing PVC size or cleaning up old streams

---

## Production Checklist

- [ ] All prerequisites met
- [ ] Namespace created
- [ ] mTLS certificates generated
- [ ] Secrets created with strong passwords
- [ ] Infrastructure deployed (PostgreSQL, NATS)
- [ ] Database migrations completed
- [ ] Database schema verified (`scripts/verify-database-schema.sh`)
- [ ] CVE data loaded (if required) (`scripts/load-cve-data.sh`)
- [ ] Core deployed and healthy
- [ ] Agent deployed on all nodes
- [ ] End-to-end test successful
- [ ] Monitoring configured
- [ ] Logging configured
- [ ] Backup strategy in place
- [ ] Disaster recovery plan documented

---

## Next Steps

- [Architecture Documentation](ARCHITECTURE.md)
- [Migration Guide](MIGRATIONS.md)
- [API Reference](API_REFERENCE.md)
- [Operations Guide](OPERATIONS.md)

---

**For support**: See [Troubleshooting](#troubleshooting) section or check logs:

```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100
kubectl logs -n fortuna -l app=fortuna-agent --tail=100
```

