# Fortuna Deployment Checklist

Follow these steps in order for a successful deployment.

## Prerequisites

- [ ] Kubernetes cluster (v1.25+) running
- [ ] containerd installed and running
- [ ] kubectl configured and has cluster access
- [ ] nerdctl and ctr available
- [ ] openssl installed
- [ ] Network connectivity between nodes (for multi-node clusters)

## Step-by-Step Deployment

### Step 1: Create Namespace

```bash
kubectl create namespace fortuna
```

**Verify**:
```bash
kubectl get namespace fortuna
```

### Step 2: Generate mTLS Certificates

```bash
bash scripts/create_mtls_secret.sh
```

**Verify**: Check secrets created:
```bash
kubectl get secrets -n fortuna | grep fortuna
```

**Expected**: 
- `fortuna-ca-cert`
- `fortuna-core-tls`
- `fortuna-agent-tls`
- `fortuna-webhook-tls`

### Step 3: Create Application Secrets

```bash
kubectl create secret generic fortuna-secrets \
  --from-literal=database-url="postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable" \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --namespace=fortuna
```

**Verify**:
```bash
kubectl get secret fortuna-secrets -n fortuna
```

### Step 4: Deploy PostgreSQL

```bash
kubectl apply -f deploy/infrastructure/postgresql.yaml
kubectl wait --for=condition=ready pod -l app=postgres -n fortuna --timeout=300s
```

**Verify**:
```bash
kubectl get pods -n fortuna -l app=postgres
kubectl get svc -n fortuna -l app=postgres
```

### Step 5: Deploy NATS

```bash
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl wait --for=condition=ready pod -l app=nats -n fortuna --timeout=300s
```

**Verify**:
```bash
kubectl get pods -n fortuna -l app=nats
# Should see nats-0, nats-1, nats-2 all Running
kubectl get svc -n fortuna | grep nats
```

### Step 6: Build and Load Images

```bash
bash scripts/build-and-load-containerd.sh
```

**Verify**: Check images loaded:
```bash
ctr -n k8s.io images ls | grep fortuna
```

**Expected**: 
- `fortuna-core:latest`
- `fortuna/agent:latest`

**For Multi-Node Clusters**: Automatically push images to worker nodes:
```bash
# Option 1: Build and push in one step
PUSH_TO_WORKERS=true bash scripts/build-and-load-containerd.sh

# Option 2: Push separately after build
bash scripts/push-images-to-workers.sh
```

**Configuration** (if needed):
```bash
export WORKER_NODES="k8s-worker01 k8s-worker02"
export SSH_USER="k8s"
export SSH_PASS="k8s"  # Or use SSH keys
bash scripts/push-images-to-workers.sh
```

**Manual method** (if script fails):
```bash
# Export images
nerdctl --namespace k8s.io save -o fortuna-core.tar fortuna-core:latest
nerdctl --namespace k8s.io save -o fortuna-agent.tar fortuna/agent:latest

# Copy to worker nodes and import
scp fortuna-*.tar user@worker:/tmp/
ssh user@worker "sudo ctr -n k8s.io images import /tmp/fortuna-core.tar"
ssh user@worker "sudo ctr -n k8s.io images import /tmp/fortuna-agent.tar"
```

### Step 7: Deploy RBAC

```bash
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/agent-rbac.yaml
```

**Verify**:
```bash
kubectl get serviceaccount -n fortuna
kubectl get clusterrole | grep fortuna
```

### Step 8: Deploy Core Service

```bash
kubectl apply -f deploy/core-service.yaml
```

**Verify**:
```bash
kubectl get svc fortuna-core -n fortuna
```

### Step 9: Deploy Core

```bash
kubectl apply -f deploy/core-deployment.yaml
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s
```

**Note**: Core will automatically run database migrations on startup. This may take 1-2 minutes.

**Monitor migrations**:
```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core -f
```

Look for messages like:
- "Running database migrations..."
- "Migration X completed successfully"
- "All migrations completed"

### Step 10: Verify Database Schema

After Core migrations complete, verify the database schema:

```bash
bash scripts/verify-database-schema.sh
```

**Verify**: Script should report all tables and columns exist.

**Expected output**:
- ✅ All required tables exist
- ✅ All critical columns exist
- ✅ Schema verification completed successfully

### Step 11: Load CVE Data (Optional)

If CVE matching is required, load CVE data:

```bash
bash scripts/load-cve-data.sh
```

**Note**: This step is optional. Skip if:
- CVE data is not available
- CVE matching is not required immediately

**Verify**: Script will:
- Check CVE data directory exists
- Verify CVE tables exist
- Check if data is already loaded (skips if present)
- Load CVE data via Kubernetes Job
- Verify loaded data

**Expected time**: 10-30 minutes depending on data size.

### Step 12: Deploy Agent

```bash
kubectl apply -f deploy/agent-daemonset.yaml
kubectl wait --for=condition=ready pod -l app=fortuna-agent -n fortuna --timeout=300s
```

**Verify**:
```bash
kubectl get pods -n fortuna -l app=fortuna-agent
# Should see 1 pod per node, all Running
```

## Post-Deployment Verification

### 1. Check All Pods

```bash
kubectl get pods -n fortuna
```

**Expected**:
- `fortuna-core-*`: 1 pod (Running)
- `fortuna-agent-*`: 1 pod per node (Running)
- `postgres-*`: 1 pod (Running)
- `nats-0`, `nats-1`, `nats-2`: 3 pods (Running)

### 2. Check Services

```bash
kubectl get svc -n fortuna
```

**Expected**:
- `fortuna-core`: ClusterIP (ports 8080, 9090)
- `postgres`: ClusterIP (port 5432)
- `nats`: Headless (ports 4222, 8222, 6222)
- `nats-client`: ClusterIP (ports 4222, 8222)

### 3. Verify Database Schema

**Automated verification** (recommended):
```bash
bash scripts/verify-database-schema.sh
```

**Manual verification**:
```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';"
```

**Expected**: At least 20+ tables including:
- `sboms`
- `sbom_components`
- `cve_matches`
- `insights`
- `cves`
- `package_vulnerabilities`
- `policy_templates`
- `schema_migrations`

### 4. Verify CVE Data (if loaded)

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM cves; SELECT COUNT(*) FROM package_vulnerabilities;"
```

**Expected**: 
- CVE count: > 0 (if CVE data was loaded)
- Package vulnerabilities count: > 0 (if CVE data was loaded)

### 5. Check Core Logs

```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50
```

**Look for**:
- ✅ "NATS client initialized successfully"
- ✅ "All migrations completed"
- ✅ "gRPC server started"
- ✅ "HTTP server started"
- ⚠️ Warnings about policy evaluator are OK if migrations not complete yet

### 6. Check Agent Logs

```bash
kubectl logs -n fortuna -l app=fortuna-agent --tail=50
```

**Look for**:
- ✅ "Heartbeat successful" or "Connected to Core"
- ✅ "Registered with Core"
- ✅ No connection errors

### 7. Test Core API

```bash
# Health check
kubectl exec -n fortuna deployment/fortuna-core -- curl -s http://localhost:8080/health

# Readiness check
kubectl exec -n fortuna deployment/fortuna-core -- curl -s http://localhost:8080/ready
```

## Troubleshooting

If any step fails, refer to the [Troubleshooting section](PRODUCTION_DEPLOYMENT.md#troubleshooting) in PRODUCTION_DEPLOYMENT.md.

## Next Steps

After successful deployment:
1. ✅ Database schema verified (Step 10)
2. ✅ CVE data loaded (Step 11, if required)
3. Configure monitoring and alerting
4. Set up backup for PostgreSQL
5. Review security settings
6. Run end-to-end tests

