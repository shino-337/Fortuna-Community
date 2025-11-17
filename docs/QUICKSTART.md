# Quick Start Guide - KSAM với Minikube

Hướng dẫn nhanh để chạy thử KSAM trên Minikube.

## 🚀 Quick Start (3 bước)

### Bước 1: Setup Minikube và Deploy

```bash
# Automated setup (recommended)
./scripts/setup-minikube.sh
```

Script này sẽ:
- ✅ Start Minikube
- ✅ Build Docker images
- ✅ Deploy PostgreSQL
- ✅ Deploy Core Controller
- ✅ Deploy Agent
- ✅ Deploy Dashboard

### Bước 2: Tạo Test Data

```bash
# Tạo test ServiceAccounts, RoleBindings
./scripts/create-test-data.sh
```

### Bước 3: Access Dashboard

```bash
# Port-forward Dashboard
kubectl port-forward -n ksam svc/ksam-dashboard 3000:80

# Hoặc sử dụng script
./scripts/port-forward.sh
```

**Access**: http://localhost:3000  
**Login**: `admin` / `admin123`

## 📋 Manual Setup (Chi tiết)

Nếu muốn setup manual, xem [MINIKUBE_GUIDE.md](MINIKUBE_GUIDE.md).

## 🧪 Testing

### Test API

```bash
# Run test script
./scripts/test-api.sh

# Hoặc manual
kubectl port-forward -n ksam svc/ksam-core 8080:8080

# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | jq -r '.token')

# Test endpoints
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/clusters
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/serviceaccounts
curl http://localhost:8080/metrics
```

### Test Dashboard

1. Access http://localhost:3000
2. Login với `admin` / `admin123`
3. Navigate các pages:
   - Dashboard: Overview statistics
   - Graph View: Interactive graph
   - ServiceAccounts: List và filters
   - Audit Logs: Audit history

## 🔍 Verify

### Check Pods

```bash
kubectl get pods -n ksam
kubectl get pods -n kube-system | grep ksam
```

### Check Logs

```bash
# Core logs
kubectl logs -l app=ksam-core -n ksam -f

# Agent logs
kubectl logs -l app=ksam-agent -n kube-system -f
```

### Check Database

```bash
# Port-forward PostgreSQL
kubectl port-forward -n ksam svc/postgres 5432:5432

# Connect
psql -h localhost -U postgres -d ksam

# Check data
SELECT COUNT(*) FROM service_accounts;
SELECT * FROM clusters;
```

## 🧹 Cleanup

```bash
# Cleanup script
./scripts/cleanup.sh

# Hoặc manual
kubectl delete namespace ksam
kubectl delete clusterrole ksam-agent-reader
kubectl delete clusterrolebinding ksam-agent-reader
```

## 📚 Next Steps

- [Testing Guide](TESTING.md) - Chi tiết testing
- [Minikube Guide](MINIKUBE_GUIDE.md) - Setup chi tiết
- [Architecture](ARCHITECTURE.md) - Kiến trúc hệ thống

## 💡 Tips

1. **Use Makefile**: `make quickstart` để setup nhanh
2. **Port Forwarding**: Sử dụng `./scripts/port-forward.sh`
3. **Logs**: Sử dụng `kubectl logs -f` để follow logs
4. **Minikube Dashboard**: `minikube dashboard` để xem resources
