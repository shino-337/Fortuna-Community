# Deployment Guide - Step by Step

**Version**: 1.0  
**Date**: $(date)

---

## Mục đích

Hướng dẫn chi tiết từng bước để deploy KSAM platform từ đầu, bao gồm:
- Cleanup deployment cũ
- Build Docker images
- Deploy infrastructure
- Deploy Core và Agent
- Load CVE database
- Verify và test

---

## Prerequisites

### 1. Kiểm tra Prerequisites

```bash
# Kiểm tra minikube
minikube status

# Kiểm tra kubectl
kubectl version --client

# Kiểm tra Docker
docker version

# Kiểm tra Go (nếu build local)
go version
```

### 2. Cấu hình

```bash
# Set namespace
export NAMESPACE="fortuna"

# Set project root
export PROJECT_ROOT="/Users/tuatnh/Desktop/Learn/K8s Service Account Management Platform/KSAM"
```

---

## Bước 1: Cleanup Deployment Cũ

### 1.1 Xóa Deployments và DaemonSets

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

### 1.2 Cleanup Docker Images (Optional)

```bash
# Set Docker environment cho minikube
eval $(minikube docker-env)

# Xóa old images
docker rmi fortuna-core:latest fortuna-agent:latest 2>/dev/null || true

# Cleanup Docker system
docker system prune -f
```

### 1.3 Cleanup Database (Optional - Chỉ khi muốn bắt đầu từ đầu)

```bash
# ⚠️  WARNING: Lệnh này sẽ xóa toàn bộ dữ liệu!
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

## Bước 2: Setup Minikube

### 2.1 Start Minikube

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

### 2.2 Tạo Namespace

```bash
# Tạo namespace nếu chưa có
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Verify
kubectl get namespace $NAMESPACE
```

---

## Bước 3: Deploy Infrastructure

### 3.1 Deploy PostgreSQL

```bash
# Apply PostgreSQL deployment
kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/postgres.yaml

# Đợi PostgreSQL ready
echo "Đợi PostgreSQL khởi động..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=postgres --timeout=300s

# Verify
kubectl get pods -n $NAMESPACE -l app=postgres
```

**Mô tả**: PostgreSQL là database chính cho KSAM platform, lưu trữ SBOMs, CVEs, insights, và các dữ liệu khác.

### 3.2 Deploy NATS

```bash
# Apply NATS deployment
kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/nats.yaml

# Đợi NATS ready
echo "Đợi NATS khởi động..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=nats --timeout=300s

# Verify
kubectl get pods -n $NAMESPACE -l app=nats
```

**Mô tả**: NATS JetStream là message queue system, dùng để giao tiếp giữa Agent và Core, và xử lý các events (SBOM_CREATED, CVE_MATCHED, etc.).

### 3.3 Verify Infrastructure

```bash
# Kiểm tra tất cả infrastructure pods
kubectl get pods -n $NAMESPACE

# Expected output:
# - postgres-* (1/1 Running)
# - nats-* (3/3 Running)
```

---

## Bước 4: Build Docker Images

### 4.1 Build Core Image

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

**Mô tả**: Build Core image bao gồm:
- Core binary (`/app/fortuna-core`)
- CVE loader binary (`/app/cve-loader`)
- Migration files
- Rules files

### 4.2 Build Agent Image

```bash
# Build Agent image
echo "Building Agent image..."
docker build -f agent/Dockerfile -t fortuna-agent:latest .

# Verify
docker images | grep fortuna-agent
```

**Mô tả**: Build Agent image bao gồm:
- Agent binary (`/app/fortuna-agent`)
- Docker CLI (để extract SBOM từ containers)

---

## Bước 5: Deploy Core Service

### 5.1 Apply Core Deployment

```bash
# Apply Core deployment
kubectl apply -f $PROJECT_ROOT/deploy/fortuna-core-deployment.yaml

# Đợi Core ready
echo "Đợi Core khởi động..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' --timeout=300s

# Verify
kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core'
```

**Mô tả**: Core service xử lý:
- Nhận SBOM từ Agent
- Match CVEs
- Tạo insights
- Expose API endpoints

### 5.2 Kiểm tra Core Logs

```bash
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

# Kiểm tra startup logs
kubectl logs -n $NAMESPACE $CORE_POD --tail=50

# Kiểm tra migrations
kubectl logs -n $NAMESPACE $CORE_POD | grep -i "migration\|Running migration"

# Kiểm tra lỗi
kubectl logs -n $NAMESPACE $CORE_POD | grep -i "error\|fatal" | tail -20
```

---

## Bước 6: Deploy Agent Service

### 6.1 Apply Agent DaemonSet

```bash
# Apply Agent DaemonSet
kubectl apply -f $PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml

# Đợi Agent ready
echo "Đợi Agent khởi động..."
kubectl wait --for=condition=ready pod -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent' --timeout=300s

# Verify
kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=agent'
```

**Mô tả**: Agent DaemonSet chạy trên mỗi node, có nhiệm vụ:
- Detect pods mới
- Extract SBOM từ container images
- Gửi SBOM đến Core qua gRPC

### 6.2 Kiểm tra Agent Logs

```bash
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=agent' -o jsonpath='{.items[0].metadata.name}')

# Kiểm tra startup logs
kubectl logs -n $NAMESPACE $AGENT_POD --tail=50

# Kiểm tra registration với Core
kubectl logs -n $NAMESPACE $AGENT_POD | grep -i "registered\|heartbeat"

# Kiểm tra lỗi
kubectl logs -n $NAMESPACE $AGENT_POD | grep -i "error\|fatal" | tail -20
```

---

## Bước 7: Load CVE Database

### 7.1 Kiểm tra CVE Data

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

**Mô tả**: CVE data là các file JSON từ OSV.dev, chứa thông tin về vulnerabilities cho các packages.

### 7.2 Option A: Load CVE Data via Core Pod (Recommended)

```bash
# Get Core pod
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

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

**Mô tả**: 
- Copy CVE data files vào Core pod
- Chạy CVE loader để parse và insert vào database
- Process có thể mất 10-30 phút tùy vào số lượng files

### 7.3 Option B: Create CVE Loader Job

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

# Check job logs (trong terminal khác)
# kubectl logs -n $NAMESPACE -l job-name=cve-loader -f
```

**Mô tả**: Tạo Kubernetes Job để chạy CVE loader, sử dụng hostPath volume để mount CVE data.

### 7.4 Verify CVE Database Loaded

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

**Mô tả**: Verify rằng CVE data đã được load vào database. Expected:
- CVEs: Thousands of records
- package_vulnerabilities: Hundreds of thousands of records

---

## Bước 8: Verify Deployment

### 8.1 Kiểm tra Pods

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

**Mô tả**: Verify Core API đang hoạt động và database connection OK.

---

## Bước 9: Run E2E Test

### 9.1 Tạo Test Pod

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

**Mô tả**: Tạo test pod để trigger SBOM extraction và CVE matching.

### 9.2 Monitor Processing

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

**Mô tả**: Monitor logs để xem quá trình xử lý:
1. Agent detect pod
2. Agent extract SBOM
3. Agent gửi SBOM đến Core
4. Core match CVEs
5. Core tạo insights

### 9.3 Verify Results

```bash
# Sử dụng verification script
cd "$PROJECT_ROOT/tests/e2e/scripts"
export TEST_POD_NAME="$TEST_POD_NAME"
export POD_UID="$POD_UID"
./verify-e2e-flow.sh
```

**Mô tả**: Script sẽ verify:
- SBOM trong database
- Components
- CVE matches
- Insights
- API response

---

## Bước 10: Cleanup (Optional)

```bash
# Xóa test pod
kubectl delete pod -n $NAMESPACE $TEST_POD_NAME

# Xóa CVE loader job
kubectl delete job -n $NAMESPACE cve-loader 2>/dev/null || true
```

---

## Troubleshooting

### Issue: Core Pod CrashLoopBackOff

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

```bash
# Check Agent logs
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/component=agent' -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n $NAMESPACE $AGENT_POD

# Verify Agent can connect to Core
kubectl logs -n $NAMESPACE $AGENT_POD | grep -i "heartbeat\|registered"
```

### Issue: CVE Database Empty After Loader

```bash
# Check loader logs
kubectl logs -n $NAMESPACE -l job-name=cve-loader

# Verify database connection
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM cves;"
```

---

## Quick Reference

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

