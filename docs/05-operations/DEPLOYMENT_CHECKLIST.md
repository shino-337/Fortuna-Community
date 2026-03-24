# Fortuna Deployment Checklist

Follow these steps in order for a successful deployment.

## Prerequisites

- [ ] Kubernetes cluster (v1.25+) running
- [ ] containerd installed and running
- [ ] kubectl configured and has cluster access
- [ ] nerdctl and ctr available
- [ ] openssl installed
- [ ] Network connectivity between nodes (for multi-node clusters)
- [ ] **Multi-node only:** Pod network (Flannel/CNI) working so pods on worker can reach pods on master (DNS and Core). If Agent on worker cannot reach Core, run: `./scripts/deploy/fix-flannel-vxlan.sh`

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
bash scripts/utils/create_mtls_secret.sh
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
bash scripts/build/build-and-load-containerd.sh
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
PUSH_TO_WORKERS=true bash scripts/build/build-and-load-containerd.sh

# Option 2: Push separately after build
bash scripts/utils/push-images-to-workers.sh
```

**Lưu ý quan trọng (mặc định mới):**
- Script push hiện bật `VERIFY_REMOTE_DIGEST=true` theo mặc định.
- Nếu digest image trên node không khớp digest local, script sẽ fail-fast để tránh deploy lệch phiên bản giữa master/worker.

**Configuration** (if needed):
```bash
export WORKER_NODES="k8s-worker01 k8s-worker02"
export SSH_USER="k8s"
export SSH_PASS="k8s"  # Or use SSH keys
bash scripts/utils/push-images-to-workers.sh
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

**Verify digest after push (recommended):**
```bash
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent \
  -o jsonpath='{range .items[*]}{.metadata.name}{"|"}{.spec.nodeName}{"|"}{.status.containerStatuses[0].imageID}{"\n"}{end}'
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
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s
```

**Core defaults now include R5 anomaly tuning envs** (auto-applied from manifest on each deploy):
- `POD_DETAIL_NET_SPIKE_WINDOW_MINUTES=30`
- `POD_DETAIL_NET_SPIKE_MIN_SAMPLES=5`
- `POD_DETAIL_NET_SPIKE_MULTIPLIER=4.0`
- `POD_DETAIL_NET_SPIKE_MIN_QUEUE_BYTES=4096`

**Verify env in running core pod**:
```bash
kubectl -n fortuna exec deploy/fortuna-core -- env | grep POD_DETAIL_NET_SPIKE
```
```bash
kubectl -n fortuna exec deploy/fortuna-core -- env | grep -E "ADMISSION_RISK_GATE_ENABLED|ADMISSION_RISK_BLOCK_THRESHOLD|ADMISSION_RISK_SENSITIVE_NAMESPACES"
```

### Step 9.1: Apply Admission Webhook Configuration (R10 hybrid)

```bash
kubectl apply -f deploy/webhook-service.yaml
kubectl apply -f deploy/webhook-config.yaml
```

**Verify**:
```bash
kubectl get validatingwebhookconfiguration fortuna-policy-webhook
kubectl get svc -n fortuna fortuna-webhook
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
bash scripts/verify/verify-database-schema.sh
```

**Verify**: Script should report all tables and columns exist.

**Expected output**:
- ✅ All required tables exist
- ✅ All critical columns exist
- ✅ Schema verification completed successfully

### Step 11: Load CVE Data (Optional)

If CVE matching is required, load CVE data:

```bash
bash scripts/utils/load-cve-data.sh
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

### Step 12: Configure DNS (Required for Multi-Node Clusters)

**Important**: DNS configuration is critical for Agent-Core communication. Run the automated fix:

```bash
bash scripts/deploy/fix-dns-config.sh
```

**Verify**: Script should report:
- ✅ CoreDNS ConfigMap updated
- ✅ Agent DNS config verified
- ✅ CoreDNS pods restarted
- ✅ Agent pods restarted
- ✅ DNS resolution test passed

**Manual verification** (if script fails):
```bash
# Test DNS resolution
kubectl run dns-test --image=busybox:1.35 --restart=Never -n fortuna --rm -i -- nslookup fortuna-core.fortuna.svc.cluster.local

# Check Agent DNS config
kubectl get daemonset -n fortuna fortuna-agent -o jsonpath='{.spec.template.spec.dnsConfig}' | grep -E "timeout|attempts"
```

**Expected**: 
- DNS resolves to Core service IP
- Agent DNS config: timeout >= 5s, attempts >= 5

### Step 13: Fix Flannel VXLAN (Required for Multi-Node Clusters)

**Critical**: For multi-node clusters, Flannel VXLAN tunnel must be properly configured to enable pod-to-pod communication between nodes.

```bash
bash scripts/deploy/fix-flannel-vxlan.sh
```

**Verify**: Script should report:
- ✅ Flannel ConfigMap verified (Network: 10.244.0.0/16, Backend: vxlan)
- ✅ Node PodCIDR assignments verified
- ✅ Flannel DaemonSet restarted
- ✅ Connectivity test passed (or warning if test inconclusive)

**Manual verification** (if script fails):
```bash
# Check Flannel ConfigMap
kubectl get configmap kube-flannel-cfg -n kube-flannel -o jsonpath='{.data.net-conf\.json}' | jq .

# Check Node PodCIDR
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.podCIDR}{"\n"}{end}'

# Check Flannel pods
kubectl get pods -n kube-flannel -l app=flannel

# On each node, verify VXLAN interface (requires SSH)
ip addr show flannel.1
# Should show: inet 10.244.x.0/32 (not just inet6)

# On each node, verify routes (requires SSH)
ip route | grep 10.244
# Master should show: 10.244.1.0/24 via 10.244.1.0 dev flannel.1
# Worker should show: 10.244.0.0/24 via 10.244.0.0 dev flannel.1
```

**Expected**: 
- Flannel ConfigMap has correct Network and Backend
- All nodes have PodCIDR assigned
- VXLAN interfaces have IPv4 addresses
- Routes between subnets exist via `flannel.1`

**Note**: Wait 30-60 seconds after Flannel restart for VXLAN to fully initialize.

### Step 14: Deploy Agent

```bash
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=agent -n fortuna --timeout=300s
```

**Verify**:
```bash
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent
# Should see 1 pod per node, all Running
```

**Verify Agent Connection to Core**:
```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=20 | grep -E "Connected|Heartbeat"
```

**Expected**: Should see "✅ Connected to Core" or "Heartbeat successful" messages.

**R9 eBPF (phase-1 scaffold)**
- Default in manifest: `EBPF_ENABLED=false` (safe rollout).
- To canary on selected node pool, set `EBPF_ENABLED=true` and verify agent log:
```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=80 | grep -i ebpf
```

**If Agent on worker node cannot connect**:
- Wait 30-60 seconds after Flannel restart
- Check Flannel VXLAN is working: `bash scripts/deploy/fix-flannel-vxlan.sh`
- Check Agent logs: `kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50`
- Verify routes on nodes: `ip route | grep 10.244`

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
bash scripts/verify/verify-database-schema.sh
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
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50
```

**Look for**:
- ✅ "Heartbeat successful" or "Connected to Core"
- ✅ "Registered with Core"
- ✅ No connection errors
- ❌ If you see DNS timeout errors, run: `bash scripts/deploy/fix-dns-config.sh`

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

