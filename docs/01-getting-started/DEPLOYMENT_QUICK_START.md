# Quick Start Deployment Guide

**Version**: 1.0  
**Date**: $(date)

---

## Tổng quan

Hướng dẫn nhanh để deploy KSAM platform từ đầu, bao gồm tất cả các bước cần thiết.

---

## Prerequisites

```bash
# Kiểm tra prerequisites
minikube status
kubectl version --client
docker version
```

---

## Quick Deploy (All-in-One Script)

```bash
cd "/Users/tuatnh/Desktop/Learn/K8s Service Account Management Platform/KSAM"

# Chạy script deploy đầy đủ
./scripts/DEPLOY_ALL.sh
```

Script này sẽ tự động:
1. ✅ Cleanup deployment cũ
2. ✅ Setup Minikube
3. ✅ Deploy infrastructure (PostgreSQL, NATS)
4. ✅ Build Docker images
5. ✅ Deploy Core và Agent
6. ✅ Load CVE database
7. ✅ Verify deployment

---

## Manual Deployment (Step by Step)

### 1. Cleanup

```bash
export NAMESPACE="fortuna"
kubectl delete deployment,daemonset,service -n $NAMESPACE fortuna-core fortuna-agent 2>/dev/null || true
kubectl delete pod -n $NAMESPACE -l app=test-pod-e2e 2>/dev/null || true
kubectl delete job -n $NAMESPACE cve-loader 2>/dev/null || true
```

### 2. Setup

```bash
minikube status || minikube start
eval $(minikube docker-env)
kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -
```

### 3. Deploy Infrastructure

```bash
export PROJECT_ROOT="/Users/tuatnh/Desktop/Learn/K8s Service Account Management Platform/KSAM"
kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/postgres.yaml
kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/nats.yaml
kubectl wait --for=condition=ready pod -n fortuna -l app=postgres --timeout=300s
kubectl wait --for=condition=ready pod -n fortuna -l app=nats --timeout=300s
```

### 4. Build Images

```bash
cd "$PROJECT_ROOT"
eval $(minikube docker-env)
docker build -f core/Dockerfile -t fortuna-core:latest .
docker build -f agent/Dockerfile -t fortuna-agent:latest .
```

### 5. Deploy Services

```bash
kubectl apply -f $PROJECT_ROOT/deploy/fortuna-core-deployment.yaml
kubectl apply -f $PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml
kubectl wait --for=condition=ready pod -n fortuna -l 'app.kubernetes.io/component=core' --timeout=300s
kubectl wait --for=condition=ready pod -n fortuna -l 'app.kubernetes.io/component=agent' --timeout=300s
```

### 6. Load CVE Database

```bash
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
kubectl cp "$PROJECT_ROOT/cve-data/all" fortuna/$CORE_POD:/cve-data/all
kubectl exec -n fortuna $CORE_POD -- /app/cve-loader \
  --source /cve-data/all \
  --workers 20 \
  --batch-size 100 \
  --skip-errors true
```

### 7. Verify

```bash
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM cves;"
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM package_vulnerabilities;"
```

---

## Verify Deployment

```bash
# Check pods
kubectl get pods -n fortuna

# Check Core health
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
kubectl port-forward -n fortuna $CORE_POD 8080:8080 &
sleep 3
curl -s http://localhost:8080/health | python3 -m json.tool
pkill -f "kubectl port-forward"
```

---

## Run E2E Test

```bash
cd "$PROJECT_ROOT/tests/e2e/scripts"
export TEST_POD_NAME="test-pod-$(date +%s)"
export POD_UID="<pod-uid-from-test-pod>"
./verify-e2e-flow.sh
```

---

**Last Updated**: $(date)

