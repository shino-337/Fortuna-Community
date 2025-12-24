# Deployment Guide - Minikube

Hướng dẫn build Docker images và deploy vào minikube.

## 📋 Prerequisites

1. **Minikube** đã cài đặt và chạy
2. **Docker** đã cài đặt
3. **kubectl** đã cài đặt

## 🚀 Quick Deploy

### Cách 1: Sử dụng script tự động

```bash
cd KSAM
./scripts/quick-deploy.sh
```

Script này sẽ:
1. Kiểm tra và start minikube nếu cần
2. Build Docker images cho Core và Agent
3. Load images vào minikube
4. Deploy Core và Agent vào Kubernetes
5. Hiển thị status

### Cách 2: Sử dụng script chi tiết

```bash
cd KSAM
./scripts/build-and-deploy.sh
```

## 📝 Manual Deployment

### Bước 1: Start Minikube

```bash
minikube start
```

### Bước 2: Configure Docker cho Minikube

```bash
eval $(minikube docker-env)
```

**Lưu ý**: Lệnh này cần chạy trong mỗi terminal mới. Nó configures Docker để build images trực tiếp vào minikube's Docker daemon.

### Bước 3: Build Core Image

```bash
cd core
docker build -t fortuna-core:latest .
cd ..
```

### Bước 4: Build Agent Image

```bash
cd agent
docker build -t fortuna-agent:latest .
cd ..
```

### Bước 5: Verify Images

```bash
docker images | grep fortuna
```

Bạn sẽ thấy:
```
fortuna-core    latest    ...    ...    ...
fortuna-agent   latest    ...    ...    ...
```

### Bước 6: Deploy vào Kubernetes

```bash
cd deploy

# Apply RBAC (nếu có)
kubectl apply -f fortuna-rbac.yaml

# Deploy Core
kubectl apply -f fortuna-core-deployment.yaml

# Deploy Agent
kubectl apply -f fortuna-agent-daemonset.yaml
```

### Bước 7: Kiểm tra Status

```bash
# Check pods
kubectl get pods

# Check deployments
kubectl get deployments

# Check daemonsets
kubectl get daemonsets

# Check services
kubectl get services
```

## 🔍 Verify Deployment

### Check Pods

```bash
kubectl get pods -l app=fortuna-core
kubectl get pods -l app=fortuna-agent
```

### Check Logs

```bash
# Core logs
kubectl logs -l app=fortuna-core --tail=50

# Agent logs
kubectl logs -l app=fortuna-agent --tail=50
```

### Check Images trong Minikube

```bash
# Set docker environment
eval $(minikube docker-env)

# List images
docker images | grep fortuna
```

## 🐛 Troubleshooting

### Minikube không chạy

```bash
minikube status
minikube start
```

### Docker images không thấy trong minikube

**Vấn đề**: Images được build trong local Docker, không phải minikube's Docker.

**Giải pháp**:
```bash
# Set docker environment cho minikube
eval $(minikube docker-env)

# Build lại images
cd core && docker build -t fortuna-core:latest .
cd ../agent && docker build -t fortuna-agent:latest .
```

### Pods không start

```bash
# Check pod status
kubectl describe pod <pod-name>

# Check events
kubectl get events --sort-by='.lastTimestamp'

# Check logs
kubectl logs <pod-name>
```

### ImagePullBackOff Error

**Vấn đề**: Kubernetes không tìm thấy image.

**Giải pháp**: Đảm bảo đã set `imagePullPolicy: Never` trong deployment files, hoặc build images trong minikube's Docker daemon.

## 📝 Deployment Files

Các file deployment nằm trong `deploy/`:

- `fortuna-rbac.yaml` - RBAC permissions
- `fortuna-core-deployment.yaml` - Core deployment
- `fortuna-agent-daemonset.yaml` - Agent DaemonSet
- `core-service.yaml` - Core service (nếu có)
- `core-secrets.yaml` - Secrets (nếu có)

## 🔧 Configuration

### Update Image Tag

Nếu muốn dùng tag khác:

```bash
export IMAGE_TAG="v1.0.0"
./scripts/build-and-deploy.sh
```

### Update Deployment

Sau khi update code, rebuild và redeploy:

```bash
# Rebuild images
eval $(minikube docker-env)
cd core && docker build -t fortuna-core:latest .
cd ../agent && docker build -t fortuna-agent:latest .

# Restart deployments
kubectl rollout restart deployment/fortuna-core
kubectl rollout restart daemonset/fortuna-agent
```

## 📊 Status Commands

```bash
# All resources
kubectl get all -l app=fortuna-core
kubectl get all -l app=fortuna-agent

# Pod details
kubectl describe pod <pod-name>

# Service endpoints
kubectl get endpoints

# Resource usage
kubectl top pods
```

## ✅ Success Indicators

Deployment thành công khi:

1. ✅ Pods đang chạy: `kubectl get pods` shows `Running`
2. ✅ Images có trong minikube: `docker images | grep fortuna`
3. ✅ Deployments ready: `kubectl get deployments` shows `AVAILABLE`
4. ✅ Logs không có errors: `kubectl logs` shows normal startup

## 🎯 Next Steps

Sau khi deploy thành công:

1. Verify services đang chạy
2. Check logs để đảm bảo không có errors
3. Test API endpoints (nếu có service exposed)
4. Run test suite: `cd tests && ./run-all-tests.sh`

---

**Lưu ý**: Đảm bảo đã set `imagePullPolicy: Never` trong deployment files để Kubernetes sử dụng local images thay vì pull từ registry.

