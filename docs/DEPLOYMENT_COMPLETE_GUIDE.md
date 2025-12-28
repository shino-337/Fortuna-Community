# Complete Deployment Guide - KSAM Platform

**Version**: 1.0  
**Date**: $(date)

---

## Overview

Hướng dẫn đầy đủ để deploy KSAM platform từ đầu, bao gồm:
1. Prerequisites và setup
2. Build Docker images
3. Deploy infrastructure (PostgreSQL, NATS)
4. Deploy Core và Agent
5. Load CVE database
6. Verify deployment
7. Run E2E tests

---

## Prerequisites

### 1. System Requirements
- Kubernetes cluster (minikube recommended for local)
- Docker installed
- kubectl configured
- Go 1.24+ (for building)
- PostgreSQL client tools (optional, for debugging)

### 2. Verify Prerequisites

```bash
# Check minikube
minikube status

# Check kubectl
kubectl version --client

# Check Docker
docker version

# Check Go
go version
```

---

## Step 1: Cleanup Existing Deployment

### 1.1 Delete All Resources

```bash
# Set namespace
export NAMESPACE="fortuna"

# Delete deployments
kubectl delete deployment -n $NAMESPACE --all

# Delete daemonsets
kubectl delete daemonset -n $NAMESPACE --all

# Delete services (keep infrastructure services)
kubectl delete service -n $NAMESPACE fortuna-core fortuna-agent 2>/dev/null || true

# Delete test pods
kubectl delete pod -n $NAMESPACE -l app=test-pod-e2e 2>/dev/null || true

# Clean up Docker images in minikube
eval $(minikube docker-env)
docker system prune -f
```

### 1.2 Clean Database (Optional - Only if starting fresh)

```bash
# WARNING: This will delete all data!
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POSTGRES_POD" ]; then
  echo "⚠️  WARNING: This will delete all database data!"
  read -p "Continue? (y/N): " confirm
  if [ "$confirm" = "y" ]; then
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \
      "TRUNCATE TABLE cves, package_vulnerabilities, cve_matches, insights, sbom_components, sboms CASCADE;"
  fi
fi
```

---

## Step 2: Setup Minikube Environment

### 2.1 Start Minikube

```bash
# Start minikube if not running
minikube status || minikube start

# Set Docker environment to use minikube's Docker daemon
eval $(minikube docker-env)

# Verify
docker ps
```

### 2.2 Create Namespace

```bash
kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -
```

---

## Step 3: Deploy Infrastructure

### 3.1 Deploy PostgreSQL

```bash
# Apply PostgreSQL deployment
kubectl apply -f KSAM/deploy/infrastructure/postgres.yaml

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -n fortuna -l app=postgres --timeout=300s

# Verify PostgreSQL
kubectl get pods -n fortuna -l app=postgres
```

### 3.2 Deploy NATS

```bash
# Apply NATS deployment
kubectl apply -f KSAM/deploy/infrastructure/nats.yaml

# Wait for NATS to be ready
kubectl wait --for=condition=ready pod -n fortuna -l app=nats --timeout=300s

# Verify NATS
kubectl get pods -n fortuna -l app=nats
```

### 3.3 Verify Infrastructure

```bash
# Check all infrastructure pods
kubectl get pods -n fortuna

# Expected output:
# - postgres-* (1/1 Running)
# - nats-* (3/3 Running)
```

---

## Step 4: Build Docker Images

### 4.1 Build Core Image

```bash
# Set Docker environment
eval $(minikube docker-env)

# Navigate to project root
cd "/Users/tuatnh/Desktop/Learn/K8s Service Account Management Platform/KSAM"

# Build Core image
docker build -f core/Dockerfile -t fortuna-core:latest .

# Verify image
docker images | grep fortuna-core
```

### 4.2 Build Agent Image

```bash
# Build Agent image
docker build -f agent/Dockerfile -t fortuna-agent:latest .

# Verify image
docker images | grep fortuna-agent
```

### 4.3 Build CVE Loader Image (Optional - if separate image needed)

```bash
# If CVE loader has separate Dockerfile
if [ -f "core/Dockerfile.cve-loader" ]; then
  docker build -f core/Dockerfile.cve-loader -t fortuna-cve-loader:latest .
fi
```

---

## Step 5: Deploy Core Service

### 5.1 Apply Core Deployment

```bash
# Apply Core deployment
kubectl apply -f KSAM/deploy/fortuna-core-deployment.yaml

# Wait for Core to be ready
kubectl wait --for=condition=ready pod -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' --timeout=300s

# Verify Core
kubectl get pods -n fortuna -l 'app.kubernetes.io/component=core'
```

### 5.2 Check Core Logs

```bash
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

# Check startup logs
kubectl logs -n fortuna $CORE_POD --tail=50

# Verify migrations completed
kubectl logs -n fortuna $CORE_POD | grep -i "migration\|Running migration"
```

---

## Step 6: Deploy Agent Service

### 6.1 Apply Agent DaemonSet

```bash
# Apply Agent DaemonSet
kubectl apply -f KSAM/deploy/fortuna-agent-daemonset.yaml

# Wait for Agent to be ready
kubectl wait --for=condition=ready pod -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent' --timeout=300s

# Verify Agent
kubectl get pods -n fortuna -l 'app.kubernetes.io/component=agent'
```

### 6.2 Check Agent Logs

```bash
AGENT_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent' -o jsonpath='{.items[0].metadata.name}')

# Check startup logs
kubectl logs -n fortuna $AGENT_POD --tail=50

# Verify Agent registered with Core
kubectl logs -n fortuna $AGENT_POD | grep -i "registered\|heartbeat"
```

---

## Step 7: Load CVE Database

### 7.1 Prepare CVE Data

```bash
# Verify CVE data exists
ls -lh KSAM/cve-data/all/*.json | wc -l

# Expected: Thousands of JSON files
```

### 7.2 Option A: Load CVE Data via Core Pod (Recommended)

```bash
# Get Core pod
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

# Copy CVE data to Core pod
echo "Copying CVE data to Core pod..."
kubectl cp KSAM/cve-data/all fortuna/$CORE_POD:/cve-data/all

# Verify data copied
kubectl exec -n fortuna $CORE_POD -- ls -lh /cve-data/all | head -10

# Run CVE loader
echo "Starting CVE loader..."
kubectl exec -n fortuna $CORE_POD -- /app/cve-loader \
  --source /cve-data/all \
  --workers 20 \
  --batch-size 100 \
  --skip-errors true

# Monitor progress
kubectl logs -n fortuna $CORE_POD -f | grep -i "cve\|loader" &
LOADER_PID=$!

# Wait for completion (check manually)
# Press Ctrl+C to stop monitoring when done
```

### 7.3 Option B: Create CVE Loader Job

```bash
# Create CVE loader job manifest
cat > /tmp/cve-loader-job.yaml << 'EOF'
apiVersion: batch/v1
kind: Job
metadata:
  name: cve-loader
  namespace: fortuna
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
          value: "postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/ksam?sslmode=disable"
        volumeMounts:
        - name: cve-data
          mountPath: /cve-data
      volumes:
      - name: cve-data
        hostPath:
          path: /Users/tuatnh/Desktop/Learn/K8s\ Service\ Account\ Management\ Platform/KSAM/cve-data
          type: Directory
      restartPolicy: Never
  backoffLimit: 3
EOF

# Apply job
kubectl apply -f /tmp/cve-loader-job.yaml

# Monitor job
kubectl get job -n fortuna cve-loader -w

# Check job logs
kubectl logs -n fortuna -l job-name=cve-loader -f
```

### 7.4 Verify CVE Database Loaded

```bash
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Check CVE count
echo "Checking CVE count..."
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cves;"

# Check package_vulnerabilities count
echo "Checking package_vulnerabilities count..."
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM package_vulnerabilities;"

# Sample data
echo "Sample CVEs:"
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT cve_id, severity, title FROM cves LIMIT 5;"

echo "Sample package_vulnerabilities:"
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT cve_id, package_name, ecosystem FROM package_vulnerabilities LIMIT 5;"
```

---

## Step 8: Verify Deployment

### 8.1 Check All Pods

```bash
# Check all pods status
kubectl get pods -n fortuna

# Expected:
# - postgres-* (1/1 Running)
# - nats-* (3/3 Running)
# - fortuna-core-* (1/1 Running)
# - fortuna-agent-* (1/1 Running)
```

### 8.2 Check Services

```bash
# Check services
kubectl get svc -n fortuna

# Verify Core service
kubectl get svc -n fortuna fortuna-core

# Verify Agent service (if exists)
kubectl get svc -n fortuna fortuna-agent 2>/dev/null || echo "Agent service not exposed (DaemonSet)"
```

### 8.3 Health Checks

```bash
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

# Port forward for API access
kubectl port-forward -n fortuna $CORE_POD 8080:8080 > /tmp/port-forward.log 2>&1 &
PORT_FORWARD_PID=$!
sleep 3

# Health check
curl -s http://localhost:8080/health | python3 -m json.tool

# Stop port forward
kill $PORT_FORWARD_PID 2>/dev/null || true
```

---

## Step 9: Run E2E Test

### 9.1 Create Test Pod

```bash
# Create test pod
cat > /tmp/test-pod.yaml << 'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-$(date +%s)
  namespace: fortuna
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

TEST_POD_NAME="test-pod-$(date +%s)"
sed "s/test-pod-\$(date +%s)/$TEST_POD_NAME/" /tmp/test-pod.yaml | kubectl apply -f -

# Wait for pod to be ready
kubectl wait --for=condition=Ready pod/$TEST_POD_NAME -n fortuna --timeout=60s
```

### 9.2 Monitor Processing

```bash
# Get pod UID
POD_UID=$(kubectl get pod -n fortuna $TEST_POD_NAME -o jsonpath='{.metadata.uid}')

# Monitor Agent logs
AGENT_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/component=agent' -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n fortuna $AGENT_POD -f | grep -E "(test-pod|SBOM|Extracting)" &
AGENT_LOG_PID=$!

# Monitor Core logs
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n fortuna $CORE_POD -f | grep -E "(SBOM|CVE|insight)" &
CORE_LOG_PID=$!

# Wait for processing (5-10 minutes)
echo "Monitoring processing... Press Ctrl+C when done"
sleep 600

# Stop monitoring
kill $AGENT_LOG_PID $CORE_LOG_PID 2>/dev/null || true
```

### 9.3 Verify Results

```bash
# Use verification script
cd KSAM/tests/e2e/scripts
export TEST_POD_NAME="$TEST_POD_NAME"
export POD_UID="$POD_UID"
./verify-e2e-flow.sh
```

---

## Step 10: Cleanup (Optional)

```bash
# Delete test pod
kubectl delete pod -n fortuna $TEST_POD_NAME

# Delete CVE loader job
kubectl delete job -n fortuna cve-loader 2>/dev/null || true
```

---

## Troubleshooting

### Issue: Core Pod CrashLoopBackOff

```bash
# Check logs
kubectl logs -n fortuna <core-pod> --previous

# Common issues:
# - Database connection failed → Check PostgreSQL
# - NATS connection failed → Check NATS
# - Migration errors → Check migration logs
```

### Issue: Agent Pod Not Processing Pods

```bash
# Check Agent logs
kubectl logs -n fortuna <agent-pod>

# Verify Agent can connect to Core
kubectl logs -n fortuna <agent-pod> | grep -i "heartbeat\|registered"
```

### Issue: CVE Database Empty After Loader

```bash
# Check loader logs
kubectl logs -n fortuna <cve-loader-pod>

# Verify database connection
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM cves;"
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

