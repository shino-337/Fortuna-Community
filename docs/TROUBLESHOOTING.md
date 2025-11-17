# Troubleshooting Guide

Hướng dẫn xử lý các lỗi thường gặp khi setup và chạy KSAM.

## Docker Build Issues

### Error: `go: updates to go.mod needed; to update it: go mod tidy`

**Nguyên nhân**: `go.mod` chưa được sync với dependencies hoặc `go.sum` thiếu.

**Giải pháp**:

1. **Chạy `go mod tidy` trước khi build**:
```bash
cd agent
go mod tidy
cd ../core
go mod tidy
```

2. **Verify `go.sum` tồn tại**:
```bash
ls -la agent/go.sum core/go.sum
```

3. **Rebuild images**:
```bash
# Agent
cd agent
docker build -t ksam/agent:latest .

# Core
cd ../core
docker build -t ksam/core:latest .
```

4. **Nếu vẫn lỗi, force rebuild**:
```bash
docker build --no-cache -t ksam/agent:latest ./agent
docker build --no-cache -t ksam/core:latest ./core
```

### Error: `go.mod file indicates go 1.21, but maximum version supported by tidy is 1.20`

**Nguyên nhân**: Go version trong Dockerfile không khớp với `go.mod`.

**Giải pháp**:

1. **Update Dockerfile** để sử dụng Go 1.20:
```dockerfile
FROM golang:1.20-alpine AS builder
```

2. **Hoặc update go.mod**:
```bash
cd agent
sed -i '' 's/go 1.21/go 1.20/g' go.mod
go mod tidy
```

### Error: `failed to solve: process did not complete successfully`

**Nguyên nhân**: Build process failed.

**Giải pháp**:

1. **Check logs chi tiết**:
```bash
docker build --progress=plain -t ksam/agent:latest ./agent
```

2. **Verify source code**:
```bash
cd agent
go build ./cmd
```

3. **Check dependencies**:
```bash
go mod verify
```

## Minikube Issues

### Error: `minikube start` fails

**Giải pháp**:

1. **Check Minikube status**:
```bash
minikube status
```

2. **Delete và recreate**:
```bash
minikube delete
minikube start --memory=4096 --cpus=2
```

3. **Check Docker**:
```bash
docker ps
eval $(minikube docker-env)
docker ps
```

### Error: Images not found in Minikube

**Nguyên nhân**: Images được build trong local Docker, không phải Minikube Docker.

**Giải pháp**:

1. **Set Docker environment**:
```bash
eval $(minikube docker-env)
```

2. **Rebuild images**:
```bash
docker build -t ksam/agent:latest ./agent
docker build -t ksam/core:latest ./core
docker build -t ksam/dashboard:latest ./dashboard
```

3. **Verify images**:
```bash
docker images | grep ksam
```

### Error: `ImagePullBackOff` hoặc `ErrImagePull`

**Nguyên nhân**: Pod không tìm thấy image.

**Giải pháp**:

1. **Check imagePullPolicy**:
```yaml
imagePullPolicy: Never  # For local images
```

2. **Verify image exists**:
```bash
kubectl describe pod <pod-name> -n ksam
```

3. **Check image name**:
```bash
kubectl get pod <pod-name> -n ksam -o yaml | grep image
```

## Agent Issues

### Agent không connect được Core

**Giải pháp**:

1. **Check Core service**:
```bash
kubectl get svc ksam-core -n ksam
```

2. **Test connectivity từ Agent pod**:
```bash
kubectl exec -it <agent-pod> -n kube-system -- \
  wget -O- http://ksam-core.ksam.svc.cluster.local:8080/health
```

3. **Check DNS**:
```bash
kubectl exec -it <agent-pod> -n kube-system -- \
  nslookup ksam-core.ksam.svc.cluster.local
```

4. **Check Agent logs**:
```bash
kubectl logs -l app=ksam-agent -n kube-system
```

### Agent không collect data

**Giải pháp**:

1. **Check RBAC**:
```bash
kubectl get clusterrole ksam-agent-reader
kubectl get clusterrolebinding ksam-agent-reader
```

2. **Check ServiceAccount**:
```bash
kubectl get serviceaccount ksam-agent -n kube-system
```

3. **Test permissions**:
```bash
kubectl auth can-i list serviceaccounts --as=system:serviceaccount:kube-system:ksam-agent
```

## Core Controller Issues

### Database connection failed

**Giải pháp**:

1. **Check PostgreSQL**:
```bash
kubectl get pods -l app=postgres -n ksam
kubectl logs -l app=postgres -n ksam
```

2. **Test connection**:
```bash
kubectl exec -it <postgres-pod> -n ksam -- \
  psql -U postgres -d ksam -c "SELECT 1;"
```

3. **Check secret**:
```bash
kubectl get secret ksam-secrets -n ksam
kubectl get secret ksam-secrets -n ksam -o jsonpath='{.data.database-url}' | base64 -d
```

4. **Check Core logs**:
```bash
kubectl logs -l app=ksam-core -n ksam
```

### API không response

**Giải pháp**:

1. **Check pod status**:
```bash
kubectl get pods -l app=ksam-core -n ksam
kubectl describe pod <pod-name> -n ksam
```

2. **Check logs**:
```bash
kubectl logs -l app=ksam-core -n ksam --tail=50
```

3. **Test health endpoint**:
```bash
kubectl port-forward -n ksam svc/ksam-core 8080:8080
curl http://localhost:8080/health
```

4. **Check resources**:
```bash
kubectl top pod -l app=ksam-core -n ksam
```

## Dashboard Issues

### Dashboard không load

**Giải pháp**:

1. **Check pod status**:
```bash
kubectl get pods -l app=ksam-dashboard -n ksam
```

2. **Check logs**:
```bash
kubectl logs -l app=ksam-dashboard -n ksam
```

3. **Test API connectivity**:
```bash
kubectl exec -it <dashboard-pod> -n ksam -- \
  wget -O- http://ksam-core:8080/health
```

4. **Check environment variables**:
```bash
kubectl get pod <dashboard-pod> -n ksam -o yaml | grep -A 5 env
```

### Dashboard không connect được API

**Giải pháp**:

1. **Check API URL**:
```bash
kubectl get pod <dashboard-pod> -n ksam -o yaml | grep VITE_API_URL
```

2. **Test API từ Dashboard pod**:
```bash
kubectl exec -it <dashboard-pod> -n ksam -- \
  wget -O- http://ksam-core:8080/health
```

3. **Check Core service**:
```bash
kubectl get svc ksam-core -n ksam
```

## Authentication Issues

### Login failed

**Giải pháp**:

1. **Check default admin user**:
```bash
kubectl logs -l app=ksam-core -n ksam | grep admin
```

2. **Verify credentials**:
```bash
# Default: admin / admin123
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

3. **Check JWT secret**:
```bash
kubectl get secret ksam-secrets -n ksam -o jsonpath='{.data.jwt-secret}' | base64 -d
```

### Token expired

**Giải pháp**:

1. **Login lại để lấy token mới**:
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | jq -r '.token')
```

2. **Check token expiration**:
```bash
# Token expires sau 24h (default)
```

## Performance Issues

### Slow API responses

**Giải pháp**:

1. **Check database connection pool**:
```bash
kubectl logs -l app=ksam-core -n ksam | grep connection
```

2. **Check metrics**:
```bash
curl http://localhost:8080/metrics | grep ksam_http_request_duration
```

3. **Check resource usage**:
```bash
kubectl top pod -l app=ksam-core -n ksam
```

### High memory usage

**Giải pháp**:

1. **Check resource limits**:
```bash
kubectl get pod <pod-name> -n ksam -o yaml | grep -A 5 resources
```

2. **Increase limits nếu cần**:
```yaml
resources:
  limits:
    memory: "1Gi"
    cpu: "1000m"
```

## Common Commands

### Debug Commands

```bash
# Check all pods
kubectl get pods -A | grep ksam

# Check logs
kubectl logs -l app=ksam-core -n ksam -f
kubectl logs -l app=ksam-agent -n kube-system -f

# Describe pod
kubectl describe pod <pod-name> -n ksam

# Check events
kubectl get events -n ksam --sort-by='.lastTimestamp'

# Check services
kubectl get svc -n ksam

# Check secrets
kubectl get secrets -n ksam
```

### Cleanup Commands

```bash
# Delete namespace
kubectl delete namespace ksam

# Delete Agent resources
kubectl delete daemonset ksam-agent -n kube-system
kubectl delete clusterrole ksam-agent-reader
kubectl delete clusterrolebinding ksam-agent-reader

# Clean Docker images
docker rmi ksam/agent:latest ksam/core:latest ksam/dashboard:latest
```

## Getting Help

1. **Check logs**: `kubectl logs -l app=ksam-core -n ksam`
2. **Check events**: `kubectl get events -n ksam`
3. **Describe resources**: `kubectl describe pod <pod-name> -n ksam`
4. **Check documentation**: Xem [docs/](docs/) để biết thêm chi tiết

