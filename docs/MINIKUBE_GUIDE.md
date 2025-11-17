# KSAM trên Minikube - Hướng dẫn Chi tiết

Hướng dẫn đầy đủ để chạy thử KSAM trên Minikube.

## 📋 Prerequisites

### Cài đặt Tools

```bash
# macOS
brew install minikube kubectl helm docker

# Linux
# Minikube
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube

# kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

### Kiểm tra

```bash
minikube version
kubectl version --client
helm version
docker --version
```

## 🚀 Quick Start (Automated)

Sử dụng script tự động:

```bash
# Setup toàn bộ
./scripts/setup-minikube.sh

# Tạo test data
./scripts/create-test-data.sh

# Test API
./scripts/test-api.sh

# Cleanup
./scripts/cleanup.sh
```

## 📝 Manual Setup

### Bước 1: Start Minikube

```bash
# Start với đủ resources
minikube start --memory=4096 --cpus=2

# Verify
kubectl cluster-info
kubectl get nodes
```

### Bước 2: Setup Docker Environment

```bash
# Set Docker environment để build trong Minikube
eval $(minikube docker-env)

# Verify
docker ps
```

### Bước 3: Build Images

```bash
# Build Agent
cd agent
docker build -t ksam/agent:latest .
cd ..

# Build Core
cd core
docker build -t ksam/core:latest .
cd ..

# Build Dashboard
cd dashboard
docker build -t ksam/dashboard:latest .
cd ..

# Verify images
docker images | grep ksam
```

### Bước 4: Deploy PostgreSQL

```bash
# Tạo namespace
kubectl create namespace ksam

# Deploy PostgreSQL (xem QUICKSTART.md cho full YAML)
kubectl apply -f - <<EOF
# PostgreSQL YAML (xem QUICKSTART.md)
EOF

# Wait
kubectl wait --for=condition=ready pod -l app=postgres -n ksam --timeout=120s
```

### Bước 5: Deploy Core Controller

```bash
# Tạo secrets
kubectl create secret generic ksam-secrets -n ksam \
  --from-literal=database-url="postgres://postgres:postgres@postgres:5432/ksam?sslmode=disable" \
  --from-literal=jwt-secret="minikube-test-secret"

# Deploy Core (xem QUICKSTART.md cho full YAML)
kubectl apply -f - <<EOF
# Core YAML (xem QUICKSTART.md)
EOF

# Wait
kubectl wait --for=condition=ready pod -l app=ksam-core -n ksam --timeout=120s

# Check logs
kubectl logs -l app=ksam-core -n ksam
```

### Bước 6: Deploy Agent

```bash
# Deploy RBAC
kubectl apply -f agent/deploy/rbac.yaml

# Deploy DaemonSet (xem QUICKSTART.md cho full YAML)
kubectl apply -f - <<EOF
# Agent YAML (xem QUICKSTART.md)
EOF

# Wait
kubectl wait --for=condition=ready pod -l app=ksam-agent -n kube-system --timeout=120s

# Check logs
kubectl logs -l app=ksam-agent -n kube-system
```

### Bước 7: Deploy Dashboard

```bash
# Deploy Dashboard (xem QUICKSTART.md cho full YAML)
kubectl apply -f - <<EOF
# Dashboard YAML (xem QUICKSTART.md)
EOF

# Wait
kubectl wait --for=condition=ready pod -l app=ksam-dashboard -n ksam --timeout=120s
```

## 🧪 Testing

### 1. Test API

```bash
# Port-forward Core
kubectl port-forward -n ksam svc/ksam-core 8080:8080

# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | jq -r '.token')

# Test endpoints
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/clusters
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/serviceaccounts
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/graph
curl http://localhost:8080/metrics
```

### 2. Test Dashboard

```bash
# Port-forward Dashboard
kubectl port-forward -n ksam svc/ksam-dashboard 3000:80

# Open browser
open http://localhost:3000  # macOS
```

### 3. Create Test Data

```bash
# Run test data script
./scripts/create-test-data.sh

# Hoặc manual
kubectl create namespace test-ksam
kubectl create serviceaccount test-sa -n test-ksam
kubectl create rolebinding test-rb -n test-ksam \
  --role=edit \
  --serviceaccount=test-ksam:test-sa
```

## 📊 Verify Data Collection

### Check Agent Logs

```bash
kubectl logs -l app=ksam-agent -n kube-system --tail=50
```

Expected output:
```
Starting data collection...
Collection complete: X SAs, Y RBs, Z CRBs...
```

### Check Core Logs

```bash
kubectl logs -l app=ksam-core -n ksam --tail=50
```

### Check Database

```bash
# Port-forward PostgreSQL
kubectl port-forward -n ksam svc/postgres 5432:5432

# Connect (in another terminal)
psql -h localhost -U postgres -d ksam

# Check data
SELECT COUNT(*) FROM service_accounts;
SELECT COUNT(*) FROM clusters;
SELECT * FROM clusters;
```

## 🔍 Monitoring

### Check Pod Status

```bash
# All pods
kubectl get pods -n ksam
kubectl get pods -n kube-system | grep ksam

# Describe nếu có issues
kubectl describe pod <pod-name> -n ksam
```

### Check Services

```bash
kubectl get svc -n ksam
```

### Check Metrics

```bash
# Port-forward Core
kubectl port-forward -n ksam svc/ksam-core 8080:8080

# Get metrics
curl http://localhost:8080/metrics | grep ksam
```

## 🐛 Troubleshooting

### Agent không connect được Core

```bash
# Check Core service
kubectl get svc ksam-core -n ksam

# Test từ Agent pod
kubectl exec -it <agent-pod> -n kube-system -- \
  wget -O- http://ksam-core.ksam.svc.cluster.local:8080/health

# Check DNS
kubectl exec -it <agent-pod> -n kube-system -- nslookup ksam-core.ksam.svc.cluster.local
```

### Database connection failed

```bash
# Check PostgreSQL
kubectl get pods -l app=postgres -n ksam
kubectl logs -l app=postgres -n ksam

# Test connection
kubectl exec -it <postgres-pod> -n ksam -- \
  psql -U postgres -d ksam -c "SELECT 1;"
```

### Dashboard không load

```bash
# Check logs
kubectl logs -l app=ksam-dashboard -n ksam

# Check API connectivity
kubectl exec -it <dashboard-pod> -n ksam -- \
  wget -O- http://ksam-core:8080/health
```

### Images không build được

```bash
# Verify Docker environment
eval $(minikube docker-env)
docker ps

# Rebuild
docker build --no-cache -t ksam/agent:latest ./agent
```

## 📈 Performance Testing

### Test với nhiều ServiceAccounts

```bash
# Tạo nhiều ServiceAccounts
for i in {1..100}; do
  kubectl create serviceaccount test-sa-$i -n test-ksam
done

# Monitor Agent sync
kubectl logs -l app=ksam-agent -n kube-system -f
```

### Monitor Metrics

```bash
# Watch metrics
watch -n 5 'curl -s http://localhost:8080/metrics | grep ksam_serviceaccounts_total'
```

## 🧹 Cleanup

```bash
# Run cleanup script
./scripts/cleanup.sh

# Hoặc manual
kubectl delete namespace ksam
kubectl delete clusterrole ksam-agent-reader
kubectl delete clusterrolebinding ksam-agent-reader
kubectl delete serviceaccount ksam-agent -n kube-system
```

## 📚 Next Steps

1. **Scale Testing**: Test với nhiều clusters
2. **Performance Testing**: Monitor metrics
3. **Security Testing**: Test authentication
4. **Integration Testing**: Test với external tools

## 💡 Tips

1. **Minikube Dashboard**: `minikube dashboard` để xem resources
2. **Port Forwarding**: Sử dụng port-forward để access services
3. **Logs**: `kubectl logs -f` để follow logs
4. **Debug**: `kubectl exec` để debug pods
5. **Metrics**: Monitor tại `/metrics` endpoint

## 🔗 Useful Links

- [Minikube Documentation](https://minikube.sigs.k8s.io/docs/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [KSAM Architecture](docs/ARCHITECTURE.md)
- [KSAM Deployment](docs/DEPLOYMENT.md)

