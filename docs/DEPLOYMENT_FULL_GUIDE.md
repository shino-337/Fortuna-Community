# Complete Deployment Guide - Full Version

**Version**: 1.0  
**Date**: $(date)

---

## Mục đích

Hướng dẫn đầy đủ, chi tiết từng bước để deploy KSAM platform từ đầu, bao gồm tất cả các bước cần thiết để có thể test ngay sau khi deploy.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Step 1: Cleanup](#step-1-cleanup)
3. [Step 2: Setup Minikube](#step-2-setup-minikube)
4. [Step 3: Deploy Infrastructure](#step-3-deploy-infrastructure)
5. [Step 4: Build Docker Images](#step-4-build-docker-images)
6. [Step 5: Deploy Core Service](#step-5-deploy-core-service)
7. [Step 6: Deploy Agent Service](#step-6-deploy-agent-service)
8. [Step 7: Load CVE Database](#step-7-load-cve-database)
9. [Step 8: Verify Deployment](#step-8-verify-deployment)
10. [Step 9: Run E2E Test](#step-9-run-e2e-test)
11. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### 1.1 System Requirements

- **Kubernetes**: minikube (recommended) hoặc cluster khác
- **Docker**: Installed và running
- **kubectl**: Configured và connected to cluster
- **Go**: 1.24+ (optional, chỉ cần nếu build local)
- **PostgreSQL client**: (optional, để debug)

### 1.2 Verify Prerequisites

```bash
# Kiểm tra minikube
minikube status
# Expected: minikube đang chạy

# Kiểm tra kubectl
kubectl version --client
# Expected: Client version hiển thị

# Kiểm tra Docker
docker version
# Expected: Client và Server version hiển thị

# Kiểm tra Go (optional)
go version
# Expected: go version 1.24.x hoặc cao hơn
```

### 1.3 Cấu hình

```bash
# Set namespace
export NAMESPACE="fortuna"

# Set project root
export PROJECT_ROOT="/Users/tuatnh/Desktop/Learn/K8s Service Account Management Platform/KSAM"

# Verify project structure
ls -d $PROJECT_ROOT/{core,agent,deploy,cve-data}
```

---

## Step 1: Cleanup

### 1.1 Xóa Deployments và DaemonSets

**Mô tả**: Xóa tất cả deployments và daemonsets cũ để bắt đầu từ đầu.

```bash
# Xóa Core deployment
kubectl delete deployment -n $NAMESPACE fortuna-core 2>/dev/null || echo "Core deployment không tồn tại"

# Xóa Agent DaemonSet
kubectl delete daemonset -n $NAMESPACE fortuna-agent 2>/dev/null || echo "Agent DaemonSet không tồn tại"

# Xóa Services
kubectl delete service -n $NAMESPACE fortuna-core fortuna-agent 2>/dev/null || echo "Services không tồn tại"

# Xóa test pods
kubectl delete pod -n $NAMESPACE -l app=test-pod-e2e 2>/dev/null || echo "Test pods không tồn tại"

# Xóa CVE loader job (nếu có)
kubectl delete job -n $NAMESPACE cve-loader 2>/dev/null || echo "CVE loader job không tồn tại"
```

**Expected Output**:
```
deployment.apps "fortuna-core" deleted
daemonset.apps "fortuna-agent" deleted
service "fortuna-core" deleted
```

### 1.2 Cleanup Docker Images (Optional)

**Mô tả**: Xóa old Docker images để đảm bảo build images mới.

```bash
# Set Docker environment cho minikube
eval $(minikube docker-env)

# Xóa old images
docker rmi fortuna-core:latest fortuna-agent:latest 2>/dev/null || true

# Cleanup Docker system
docker system prune -f
```

**Expected Output**:
```
Deleted: sha256:...
```

### 1.3 Cleanup Database (Optional - Chỉ khi muốn bắt đầu từ đầu)

**⚠️  WARNING**: Lệnh này sẽ xóa toàn bộ dữ liệu trong database!

```bash
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POSTGRES_POD" ]; then
  echo "⚠️  WARNING: This will delete all database data!"
  read -p "Continue? (y/N): " confirm
  if [ "$confirm" = "y" ]; then
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \
      "TRUNCATE TABLE cves, package_vulnerabilities, cve_matches, insights, sbom_components, sboms CASCADE;"
    echo "✅ Database cleaned"
  fi
fi
```

---

## Step 2: Setup Minikube

### 2.1 Start Minikube

**Mô tả**: Khởi động minikube và cấu hình Docker environment.

```bash
# Kiểm tra và start minikube
if ! minikube status >/dev/null 2>&1; then
  echo "Starting minikube..."
  minikube start
else
  echo "✅ Minikube đang chạy"
fi

# Set Docker environment để build images vào minikube
eval $(minikube docker-env)

# Verify
docker ps
```

**Expected Output**:
```
minikube
type: Control Plane
host: Running
kubelet: Running
apiserver: Running
kubeconfig: Configured
docker-env: in-use
```

### 2.2 Tạo Namespace

**Mô tả**: Tạo namespace `fortuna` nếu chưa có.

```bash
# Tạo namespace nếu chưa có
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Verify
kubectl get namespace $NAMESPACE
```

**Expected Output**:
```
NAME     STATUS   AGE
fortuna  Active   1d
```

---

## Step 3: Deploy Infrastructure

### 3.1 Deploy PostgreSQL

**Mô tả**: Deploy PostgreSQL database để lưu trữ SBOMs, CVEs, insights, và các dữ liệu khác.

```bash
# Apply PostgreSQL deployment
kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/postgres.yaml

# Đợi PostgreSQL ready
echo "Đợi PostgreSQL khởi động..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=postgres --timeout=300s

# Verify
kubectl get pods -n $NAMESPACE -l app=postgres
```

**Expected Output**:
```
NAME                        READY   STATUS    RESTARTS   AGE
postgres-747fc6cdfb-dh64f   1/1     Running   0          1m
```

**Mô tả chi tiết**:
- PostgreSQL là database chính cho KSAM platform
- Lưu trữ: SBOMs, components, CVEs, package_vulnerabilities, insights, cve_matches
- Port: 5432 (internal)
- Database name: `ksam`
- User: `postgres`

### 3.2 Deploy NATS

**Mô tả**: Deploy NATS JetStream message queue system.

```bash
# Apply NATS deployment
kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/nats.yaml

# Đợi NATS ready
echo "Đợi NATS khởi động..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=nats --timeout=300s

# Verify
kubectl get pods -n $NAMESPACE -l app=nats
```

**Expected Output**:
```
NAME      READY   STATUS    RESTARTS   AGE
nats-0    1/1     Running   0          1m
nats-1    1/1     Running   0          1m
nats-2    1/1     Running   0          1m
```

**Mô tả chi tiết**:
- NATS JetStream là message queue system
- Dùng để giao tiếp giữa Agent và Core
- Xử lý các events: SBOM_CREATED, CVE_MATCHED, INSIGHT_CREATED
- Cluster mode: 3 replicas (StatefulSet)
- Port: 4222 (client), 6222 (cluster), 8222 (monitoring)

### 3.3 Verify Infrastructure

**Mô tả**: Kiểm tra tất cả infrastructure pods đang chạy.

```bash
# Kiểm tra tất cả infrastructure pods
kubectl get pods -n $NAMESPACE

# Expected output:
# - postgres-* (1/1 Running)
# - nats-* (3/3 Running)
```

---

## Step 4: Build Docker Images

### 4.1 Build Core Image

**Mô tả**: Build Core Docker image bao gồm Core binary và CVE loader binary.

```bash
# Navigate to project root
cd "$PROJECT_ROOT"

# Set Docker environment
eval $(minikube docker-env)

# Build Core image
echo "Building Core image..."
docker build -f core/Dockerfile -t fortuna-core:latest .

# Verify
docker images | grep fortuna-core
```

**Expected Output**:
```
fortuna-core   latest   sha256:...   2 minutes ago   65MB
```

**Mô tả chi tiết**:
- Core image bao gồm:
  - `/app/fortuna-core` - Core service binary
  - `/app/cve-loader` - CVE loader binary
  - `/app/migrations/` - Database migration files
  - `/app/rules/` - Policy rules files
- Base image: `alpine:3.20`
- Exposed ports: 8080 (HTTP API), 9090 (gRPC)

### 4.2 Build Agent Image

**Mô tả**: Build Agent Docker image.

```bash
# Build Agent image
echo "Building Agent image..."
docker build -f agent/Dockerfile -t fortuna-agent:latest .

# Verify
docker images | grep fortuna-agent
```

**Expected Output**:
```
fortuna-agent   latest   sha256:...   1 minute ago   45MB
```

**Mô tả chi tiết**:
- Agent image bao gồm:
  - `/app/fortuna-agent` - Agent binary
  - Docker CLI (để extract SBOM từ containers)
- Base image: `alpine:3.20`
- Runs as DaemonSet trên mỗi node

---

## Step 5: Deploy Core Service

### 5.1 Apply Core Deployment

**Mô tả**: Deploy Core service để xử lý SBOMs, match CVEs, và tạo insights.

```bash
# Apply Core deployment
kubectl apply -f $PROJECT_ROOT/deploy/fortuna-core-deployment.yaml

# Đợi Core ready
echo "Đợi Core khởi động..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' --timeout=300s

# Verify
kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core'
```

**Expected Output**:
```
NAME                            READY   STATUS    RESTARTS   AGE
fortuna-core-867f95d8f6-nwvqz   1/1     Running   0          1m
```

**Mô tả chi tiết**:
- Core service xử lý:
  - Nhận SBOM từ Agent qua gRPC
  - Match CVEs với packages
  - Tạo insights
  - Expose API endpoints (HTTP)
  - Expose gRPC endpoints
- Replicas: 1 (có thể scale)
- Resources: CPU/Memory limits tùy cấu hình

### 5.2 Kiểm tra Core Logs

**Mô tả**: Kiểm tra Core logs để đảm bảo startup thành công.

```bash
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

# Kiểm tra startup logs
kubectl logs -n $NAMESPACE $CORE_POD --tail=50

# Kiểm tra migrations
kubectl logs -n $NAMESPACE $CORE_POD | grep -i "migration\|Running migration"

# Kiểm tra lỗi
kubectl logs -n $NAMESPACE $CORE_POD | grep -i "error\|fatal" | tail -20
```

**Expected Logs**:
```
[MAIN] Starting KSAM Core...
Running database migrations...
[Storage] Started connection pool monitoring
✅ Core service started
```

---

## Step 6: Deploy Agent Service

### 6.1 Apply Agent DaemonSet

**Mô tả**: Deploy Agent DaemonSet để chạy trên mỗi node, detect pods và extract SBOMs.

```bash
# Apply Agent DaemonSet
kubectl apply -f $PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml

# Đợi Agent ready
echo "Đợi Agent khởi động..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent' --timeout=300s

# Verify
kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=agent'
```

**Expected Output**:
```
NAME                 READY   STATUS    RESTARTS   AGE
fortuna-agent-x47kq   1/1     Running   0          1m
```

**Mô tả chi tiết**:
- Agent DaemonSet chạy trên mỗi node
- Nhiệm vụ:
  - Detect pods mới (via Kubernetes informer)
  - Extract SBOM từ container images
  - Gửi SBOM đến Core qua gRPC
  - Heartbeat với Core
- Resources: Memory limit thường 2-4Gi

### 6.2 Kiểm tra Agent Logs

**Mô tả**: Kiểm tra Agent logs để đảm bảo registration với Core thành công.

```bash
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=agent' -o jsonpath='{.items[0].metadata.name}')

# Kiểm tra startup logs
kubectl logs -n $NAMESPACE $AGENT_POD --tail=50

# Kiểm tra registration với Core
kubectl logs -n $NAMESPACE $AGENT_POD | grep -i "registered\|heartbeat"

# Kiểm tra lỗi
kubectl logs -n $NAMESPACE $AGENT_POD | grep -i "error\|fatal" | tail -20
```

**Expected Logs**:
```
[Agent] Starting KSAM Agent...
[gRPCClient] Connecting to Core...
✅ Agent registered with Core
✅ Heartbeat successful
```

---

## Step 7: Load CVE Database

### 7.1 Kiểm tra CVE Data

**Mô tả**: Verify CVE data files tồn tại.

```bash
# Kiểm tra CVE data files
CVE_FILE_COUNT=$(find "$PROJECT_ROOT/cve-data/all" -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
echo "CVE files found: $CVE_FILE_COUNT"

if [ "$CVE_FILE_COUNT" -eq 0 ]; then
  echo "⚠️  WARNING: No CVE data files found!"
  echo "CVE data should be in: $PROJECT_ROOT/cve-data/all/"
  exit 1
fi
```

**Expected Output**:
```
CVE files found: 74561
```

**Mô tả chi tiết**:
- CVE data là các file JSON từ OSV.dev
- Format: `CVE-YYYY-NNNNN.json`
- Chứa thông tin về vulnerabilities cho các packages
- Location: `KSAM/cve-data/all/`

### 7.2 Option A: Load CVE Data via Kubernetes Job (Recommended)

**Mô tả**: Sử dụng Kubernetes Job để load CVE data. Method này recommended vì:
- Job có thể retry nếu fail
- Dễ monitor progress
- Không ảnh hưởng đến Core pod

```bash
# Tạo CVE loader job manifest
cat > /tmp/cve-loader-job.yaml << EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: cve-loader
  namespace: $NAMESPACE
spec:
  template:
    spec:
      containers:
      - name: cve-loader
        image: fortuna-core:latest
        command: ["/app/cve-loader"]
        args:
        - "--source"
        - "/cve-data/all"
        - "--workers"
        - "20"
        - "--batch-size"
        - "100"
        - "--skip-errors"
        - "true"
        env:
        - name: DATABASE_URL
          value: "postgres://postgres:postgres@postgres.$NAMESPACE.svc.cluster.local:5432/ksam?sslmode=disable"
        volumeMounts:
        - name: cve-data
          mountPath: /cve-data
      volumes:
      - name: cve-data
        hostPath:
          path: $PROJECT_ROOT/cve-data
          type: Directory
      restartPolicy: Never
  backoffLimit: 3
EOF

# Apply job
kubectl apply -f /tmp/cve-loader-job.yaml

# Monitor job
kubectl get job -n $NAMESPACE cve-loader -w
```

**Mô tả chi tiết**:
- Job sẽ:
  1. Mount CVE data từ hostPath
  2. Parse các JSON files
  3. Insert vào `cves` và `package_vulnerabilities` tables
  4. Process với 20 workers, batch size 100
- Thời gian: 10-30 phút tùy dataset size
- Monitor: `kubectl logs -n $NAMESPACE -l job-name=cve-loader -f`

### 7.3 Option B: Load CVE Data via Core Pod

**Mô tả**: Copy CVE data vào Core pod và chạy loader trực tiếp.

```bash
# Get Core pod
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

# Tạo directory
kubectl exec -n $NAMESPACE $CORE_POD -- mkdir -p /cve-data

# Copy CVE data vào Core pod
echo "Copying CVE data to Core pod..."
echo "This may take several minutes for large datasets..."
kubectl cp "$PROJECT_ROOT/cve-data/all" $NAMESPACE/$CORE_POD:/cve-data/all

# Verify data copied
kubectl exec -n $NAMESPACE $CORE_POD -- ls -lh /cve-data/all | head -10

# Run CVE loader
echo "Starting CVE loader..."
echo "This will take 10-30 minutes depending on dataset size..."
kubectl exec -n $NAMESPACE $CORE_POD -- /app/cve-loader \
  --source /cve-data/all \
  --workers 20 \
  --batch-size 100 \
  --skip-errors true

# Monitor progress (trong terminal khác)
# kubectl logs -n $NAMESPACE $CORE_POD -f | grep -i "cve\|loader"
```

**Mô tả chi tiết**:
- Method này copy data vào Core pod
- Chạy loader trực tiếp trong Core pod
- Có thể monitor qua Core logs
- ⚠️  Lưu ý: Nếu Core pod restart, data sẽ mất (cần copy lại)

### 7.4 Verify CVE Database Loaded

**Mô tả**: Verify CVE data đã được load vào database.

```bash
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Kiểm tra CVE count
echo "Checking CVE count..."
CVE_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM cves;" 2>/dev/null | tr -d ' ')
echo "Total CVEs: $CVE_COUNT"

# Kiểm tra package_vulnerabilities count
echo "Checking package_vulnerabilities count..."
PKG_VULN_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM package_vulnerabilities;" 2>/dev/null | tr -d ' ')
echo "Total package_vulnerabilities: $PKG_VULN_COUNT"

# Sample data
if [ "$CVE_COUNT" -gt 0 ]; then
  echo ""
  echo "Sample CVEs:"
  kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \
    "SELECT cve_id, severity, title FROM cves LIMIT 5;"
fi

if [ "$PKG_VULN_COUNT" -gt 0 ]; then
  echo ""
  echo "Sample package_vulnerabilities:"
  kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \
    "SELECT cve_id, package_name, ecosystem FROM package_vulnerabilities LIMIT 5;"
fi
```

**Expected Output**:
```
Total CVEs: 50000
Total package_vulnerabilities: 200000

Sample CVEs:
 cve_id      | severity | title
-------------+----------+-------
 CVE-2023-...| HIGH     | ...
```

**Mô tả chi tiết**:
- Sau khi load thành công:
  - CVEs: Thousands of records (50k-100k+)
  - package_vulnerabilities: Hundreds of thousands (200k-500k+)
- Nếu counts = 0, loader chưa hoàn thành hoặc có lỗi

---

## Step 8: Verify Deployment

### 8.1 Kiểm tra Pods

**Mô tả**: Kiểm tra tất cả pods đang chạy.

```bash
# Kiểm tra tất cả pods
kubectl get pods -n $NAMESPACE

# Expected:
# - postgres-* (1/1 Running)
# - nats-* (3/3 Running)
# - fortuna-core-* (1/1 Running)
# - fortuna-agent-* (1/1 Running)
```

### 8.2 Health Check

**Mô tả**: Kiểm tra Core API health endpoint.

```bash
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

# Port forward
kubectl port-forward -n $NAMESPACE $CORE_POD 8080:8080 > /tmp/port-forward.log 2>&1 &
PORT_FORWARD_PID=$!
sleep 3

# Health check
HEALTH_RESPONSE=$(curl -s http://localhost:8080/health 2>/dev/null)
echo "$HEALTH_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$HEALTH_RESPONSE"

# Stop port forward
kill $PORT_FORWARD_PID 2>/dev/null || true
```

**Expected Output**:
```json
{
    "status": "healthy",
    "timestamp": "2025-12-28T05:40:54Z",
    "checks": {
        "database": "ok"
    }
}
```

---

## Step 9: Run E2E Test

### 9.1 Tạo Test Pod

**Mô tả**: Tạo test pod để trigger SBOM extraction và CVE matching.

```bash
# Tạo test pod
TEST_POD_NAME="test-pod-$(date +%s)"
cat > /tmp/test-pod.yaml << EOF
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD_NAME
  namespace: $NAMESPACE
  labels:
    app: test-pod-e2e
spec:
  nodeSelector:
    kubernetes.io/hostname: minikube
  containers:
  - name: test-container
    image: nginx:1.25-alpine
    ports:
    - containerPort: 80
  restartPolicy: Never
EOF

kubectl apply -f /tmp/test-pod.yaml

# Đợi pod ready
kubectl wait --for=condition=Ready pod/$TEST_POD_NAME -n $NAMESPACE --timeout=60s

# Get pod UID
POD_UID=$(kubectl get pod -n $NAMESPACE $TEST_POD_NAME -o jsonpath='{.metadata.uid}')
echo "Test Pod: $TEST_POD_NAME"
echo "Pod UID: $POD_UID"
```

### 9.2 Monitor Processing

**Mô tả**: Monitor logs để xem quá trình xử lý.

```bash
# Monitor Agent logs
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=agent' -o jsonpath='{.items[0].metadata.name}')
echo "Monitoring Agent logs for test pod..."
kubectl logs -n $NAMESPACE $AGENT_POD -f | grep -E "(test-pod|$TEST_POD_NAME|SBOM|Extracting)" &
AGENT_LOG_PID=$!

# Monitor Core logs
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
echo "Monitoring Core logs for processing..."
kubectl logs -n $NAMESPACE $CORE_POD -f | grep -E "(SBOM|CVE|insight|$POD_UID)" &
CORE_LOG_PID=$!

# Đợi processing (5-10 phút)
echo "Đợi processing hoàn thành (5-10 phút)..."
sleep 600

# Stop monitoring
kill $AGENT_LOG_PID $CORE_LOG_PID 2>/dev/null || true
```

### 9.3 Verify Results

**Mô tả**: Verify kết quả bằng verification script.

```bash
# Sử dụng verification script
cd "$PROJECT_ROOT/tests/e2e/scripts"
export TEST_POD_NAME="$TEST_POD_NAME"
export POD_UID="$POD_UID"
./verify-e2e-flow.sh
```

---

## Troubleshooting

### Issue: Core Pod CrashLoopBackOff

**Nguyên nhân**: Core không thể khởi động.

**Giải pháp**:
```bash
# Check logs
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n $NAMESPACE $CORE_POD --previous

# Common issues:
# - Database connection failed → Check PostgreSQL
# - NATS connection failed → Check NATS
# - Migration errors → Check migration logs
```

### Issue: Agent Pod Not Processing Pods

**Nguyên nhân**: Agent không detect hoặc không xử lý pods.

**Giải pháp**:
```bash
# Check Agent logs
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=agent' -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n $NAMESPACE $AGENT_POD

# Verify Agent can connect to Core
kubectl logs -n $NAMESPACE $AGENT_POD | grep -i "heartbeat\|registered"
```

### Issue: CVE Database Empty After Loader

**Nguyên nhân**: CVE loader không chạy hoặc fail.

**Giải pháp**:
```bash
# Check loader logs
kubectl logs -n $NAMESPACE -l job-name=cve-loader

# Verify database connection
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM cves;"
```

---

## Quick Reference Commands

```bash
# Check all pods
kubectl get pods -n fortuna

# Check Core logs
kubectl logs -n fortuna -l 'app.kubernetes.io/component=core' --tail=100

# Check Agent logs
kubectl logs -n fortuna -l 'app.kubernetes.io/component=agent' --tail=100

# Check database
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM cves;"

# Port forward for API
kubectl port-forward -n fortuna <core-pod> 8080:8080
```

---

**Last Updated**: $(date)

