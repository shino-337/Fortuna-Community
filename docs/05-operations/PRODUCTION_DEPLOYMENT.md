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
bash scripts/utils/create_mtls_secret.sh

# 3. Create secrets
kubectl create secret generic fortuna-secrets \
  --from-literal=database-url="postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable" \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --namespace=fortuna

# 4. Deploy infrastructure
kubectl apply -f deploy/infrastructure/postgresql.yaml
kubectl apply -f deploy/infrastructure/nats.yaml

# 5. Build and load images
bash scripts/build/build-and-load-containerd.sh

# 6. Deploy RBAC
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/agent-rbac.yaml

# 7. Deploy Core
kubectl apply -f deploy/core-service.yaml
kubectl apply -f deploy/fortuna-core-deployment.yaml

# 8. Configure DNS (required for multi-node clusters)
bash scripts/deploy/fix-dns-config.sh

# 9. Fix Flannel VXLAN (required for multi-node clusters)
bash scripts/deploy/fix-flannel-vxlan.sh

# 10. Deploy Agent
kubectl apply -f deploy/fortuna-agent-daemonset.yaml

# 11. Wait for readiness
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=agent -n fortuna --timeout=300s
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
bash scripts/utils/create_mtls_secret.sh
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
kubectl apply -f deploy/fortuna-core-deployment.yaml
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
bash scripts/verify/verify-database-schema.sh
```

This script verifies:
- All required tables exist (core, CVE, SBOM tables)
- All critical columns exist with correct types
- Schema integrity

If verification fails, check Core logs for migration errors.

### 4. Verify Database Schema

After Core migrations complete, verify the database schema:

```bash
bash scripts/verify/verify-database-schema.sh
```

This script automatically checks:
- All required tables exist
- All critical columns exist
- Schema integrity

**Expected output**: All tables and columns verified successfully.

### 5. Load CVE Data

Load CVE data from OSV.dev JSON files for CVE matching:

```bash
bash scripts/utils/load-cve-data.sh
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

### 6. Configure DNS (Required for Multi-Node Clusters)

**Important**: DNS configuration is critical for Agent-Core communication, especially in multi-node clusters. Run the automated DNS fix script:

```bash
bash scripts/deploy/fix-dns-config.sh
```

### 7. Fix Flannel VXLAN (Required for Multi-Node Clusters)

**Critical**: For multi-node clusters, Flannel VXLAN tunnel must be properly configured to enable pod-to-pod communication between nodes. Run the automated fix script:

```bash
bash scripts/deploy/fix-flannel-vxlan.sh
```

This script automatically:
- Verifies Flannel ConfigMap (Network: 10.244.0.0/16, Backend: vxlan)
- Checks Node PodCIDR assignments
- Restarts Flannel DaemonSet to reinitialize VXLAN
- Verifies VXLAN interfaces have IPv4 addresses
- Verifies routes between subnets are created
- Tests pod-to-pod connectivity

**Manual Fix** (if script fails):

1. **Verify Flannel ConfigMap**:
```bash
kubectl get configmap kube-flannel-cfg -n kube-flannel -o yaml | grep -A 5 "net-conf.json"
```

Must have:
- `Network: "10.244.0.0/16"`
- `Backend.Type: "vxlan"`

2. **Verify Node PodCIDR**:
```bash
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.podCIDR}{"\n"}{end}'
```

Each node must have a PodCIDR assigned (e.g., `10.244.0.0/24`, `10.244.1.0/24`).

3. **Restart Flannel**:
```bash
kubectl rollout restart daemonset kube-flannel-ds -n kube-flannel
kubectl wait --for=condition=ready pod -l app=flannel -n kube-flannel --timeout=60s
```

4. **Verify VXLAN Interfaces** (on each node):
```bash
# On master node
ip addr show flannel.1
# Should show: inet 10.244.0.0/32 (not just inet6)

# On worker node
ip addr show flannel.1
# Should show: inet 10.244.1.0/32 (not just inet6)
```

5. **Verify Routes** (on each node):
```bash
# On master node
ip route | grep 10.244
# Should show: 10.244.1.0/24 via 10.244.1.0 dev flannel.1

# On worker node
ip route | grep 10.244
# Should show: 10.244.0.0/24 via 10.244.0.0 dev flannel.1
```

**Expected Result**:
- VXLAN interfaces have IPv4 addresses
- Routes between subnets exist via `flannel.1`
- Pod-to-pod connectivity works between nodes
- Agent on worker node can connect to Core on master node

**Troubleshooting**: See [Troubleshooting - Agent Cannot Connect to Core](#agent-cannot-connect-to-core) section.

This script automatically:
- Updates CoreDNS ConfigMap with `except` clauses for internal domains
- Verifies Agent DNS configuration (timeout: 5s, attempts: 5)
- Restarts CoreDNS pods to apply changes
- Restarts Agent pods to apply DNS config
- Tests DNS resolution

**Manual DNS Configuration** (if script fails):

1. **Update CoreDNS ConfigMap**:
```bash
kubectl get configmap coredns -n kube-system -o yaml > /tmp/coredns.yaml
# Edit /tmp/coredns.yaml to add except clauses in forward section:
# forward . /etc/resolv.conf {
#    except cluster.local
#    except svc.cluster.local
#    except fortuna.svc.cluster.local
#    max_concurrent 1000
# }
kubectl apply -f /tmp/coredns.yaml
kubectl rollout restart deployment coredns -n kube-system
```

2. **Verify Agent DNS Config** in `deploy/fortuna-agent-daemonset.yaml`:
   - `timeout`: Should be `5` (not `2`)
   - `attempts`: Should be `5` (not `3`)

3. **Restart Agent**:
```bash
kubectl rollout restart daemonset -n fortuna fortuna-agent
```

**Verify DNS Resolution**:
```bash
# Test from Agent pod
kubectl run dns-test --image=busybox:1.35 --restart=Never -n fortuna --rm -i -- nslookup fortuna-core.fortuna.svc.cluster.local
```

**Expected**: Should resolve to Core service IP (e.g., `10.100.74.197`).

### 8. Deploy Agent

```bash
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

Wait for Agents to be ready (one per node):

```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=agent -n fortuna --timeout=300s
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
kubectl logs -n fortuna -l app.kubernetes.io/component=agent | grep -E "(Heartbeat|Connected|Registered)"
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
bash scripts/verify/verify-database-schema.sh
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
bash scripts/utils/load-cve-data.sh
```

### 6. Test End-to-End Flow

Create a test pod:

```bash
kubectl run test-pod --image=nginx:alpine --namespace=default --restart=Never
```

Monitor Agent logs:

```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=agent -f | grep -E "(test-pod|SBOM|Extracted)"
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

Key environment variables in `deploy/fortuna-core-deployment.yaml`:

- `DATABASE_URL`: PostgreSQL connection string
- `NATS_ENDPOINT`: NATS JetStream endpoint
- `TLS_ENABLED`: Enable mTLS (should be `true` in production)
- `AUTH_ENABLED`: Enable API authentication
- `FORTUNA_ADMIN_USERNAME`: Admin username (change in production)
- `FORTUNA_ADMIN_PASSWORD`: Admin password (change in production)

### Agent Configuration

Key environment variables in `deploy/fortuna-agent-daemonset.yaml`:

- `CORE_GRPC_ENDPOINT`: Core gRPC endpoint
- `SYNC_INTERVAL`: Full-sync interval (default configured by daemonset)
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
5. **Monitoring**: Core service exposes `/metrics` endpoint (Prometheus format). Deploy Prometheus if metrics collection is needed.
6. **Logging**: Configure centralized logging (e.g., ELK, Loki)

---

## Troubleshooting

### Agent Cannot Connect to Core

#### Error 1: Certificate Error

**Error**: `x509: certificate relies on legacy Common Name field, use SANs instead`

**Solution**: Regenerate certificates with SANs:

```bash
bash scripts/utils/create_mtls_secret.sh
kubectl delete pod -n fortuna -l app.kubernetes.io/component=core
kubectl delete pod -n fortuna -l app.kubernetes.io/component=agent
```

#### Error 2: DNS Resolution Failure

**Error**: `dial tcp: lookup fortuna-core.fortuna.svc.cluster.local: i/o timeout`

**Solution**: Run DNS fix script:

```bash
bash scripts/deploy/fix-dns-config.sh
```

**Manual Fix**:
1. Update CoreDNS ConfigMap (see [Configure DNS](#6-configure-dns-required-for-multi-node-clusters))
2. Verify Agent DNS config in `deploy/fortuna-agent-daemonset.yaml`:
   - `timeout: "5"` (not `2`)
   - `attempts: "5"` (not `3`)
3. Restart CoreDNS and Agent pods

#### Error 3: Connection Timeout After DNS Resolution

**Error**: `dial tcp 10.100.74.197:9090: i/o timeout` or `connect: connection timed out` (DNS resolves but connection fails)

**Root Cause**: Flannel VXLAN tunnel not properly configured, causing network routing issues between worker and master nodes.

**Solution**: Run the automated Flannel VXLAN fix script:

```bash
bash scripts/deploy/fix-flannel-vxlan.sh
```

**Manual Fix**:

1. **Verify Flannel ConfigMap**:
```bash
kubectl get configmap kube-flannel-cfg -n kube-flannel -o jsonpath='{.data.net-conf\.json}' | jq .
```

Must have:
- `Network: "10.244.0.0/16"`
- `Backend.Type: "vxlan"`

2. **Verify Node PodCIDR**:
```bash
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.podCIDR}{"\n"}{end}'
```

3. **Restart Flannel**:
```bash
kubectl rollout restart daemonset kube-flannel-ds -n kube-flannel
```

4. **Wait 30-60 seconds**, then verify:
   - VXLAN interfaces have IPv4 addresses: `ip addr show flannel.1`
   - Routes exist: `ip route | grep 10.244`
   - Routes use `flannel.1`, not physical network

**Expected Routes**:
- Master: `10.244.1.0/24 via 10.244.1.0 dev flannel.1`
- Worker: `10.244.0.0/24 via 10.244.0.0 dev flannel.1`

**Alternative Solutions** (if Flannel fix doesn't work):
- **Option 1**: Allow Core to run on worker nodes by removing `nodeSelector` from Core deployment
- **Option 2**: Use Core pod IP directly (workaround, less reliable)

### SBOM Tables Missing

**Error**: `ERROR: relation "sboms" does not exist`

**Solution**: Run database migrations. See [Migration Guide](../04-development/MIGRATIONS.md).

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

**Solution**: Increase memory limits in `deploy/fortuna-agent-daemonset.yaml`:

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
- [ ] Database schema verified (`scripts/verify/verify-database-schema.sh`)
- [ ] CVE data loaded (if required) (`scripts/utils/load-cve-data.sh`)
- [ ] Core deployed and healthy
- [ ] Agent deployed on all nodes
- [ ] End-to-end test successful
- [ ] Monitoring configured
- [ ] Logging configured
- [ ] Backup strategy in place
- [ ] Disaster recovery plan documented

---

## Next Steps

- [Architecture Documentation](../02-architecture/ARCHITECTURE.md)
- [Migration Guide](../04-development/MIGRATIONS.md)
- [API Reference](../06-reference/API_REFERENCE.md)
- [Operations Guide](OPERATIONS.md)
- [DNS Configuration Guide](DNS_CONFIGURATION.md) - DNS setup and troubleshooting
- [Flannel VXLAN Fix Guide](FLANNEL_VXLAN_FIX.md) - **NEW**: Flannel VXLAN configuration and troubleshooting

---

**For support**: See [Troubleshooting](#troubleshooting) section or check logs:

```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100
```

