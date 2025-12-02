# Minikube Setup Guide for KSAM

**Purpose**: Hướng dẫn khởi động Minikube với tài nguyên phù hợp cho KSAM

---

## Yêu cầu Tài nguyên

### Minimum Requirements
- **Memory**: 4GB (4096MB)
- **CPUs**: 2 cores
- **Disk**: 20GB

### Recommended (for better performance)
- **Memory**: 6GB (6144MB)
- **CPUs**: 3-4 cores
- **Disk**: 30GB

---

## Cách 1: Sử dụng Script (Khuyến nghị)

### Khởi động Minikube với cấu hình mặc định (4GB RAM, 2 CPUs, 20GB disk)

```bash
cd KSAM
bash scripts/start_minikube.sh
```

### Khởi động với cấu hình tùy chỉnh

```bash
# Set environment variables
export MINIKUBE_MEMORY=6144      # 6GB RAM
export MINIKUBE_CPUS=3           # 3 CPUs
export MINIKUBE_DISK_SIZE=30g    # 30GB disk
export MINIKUBE_DRIVER=docker    # docker driver

# Run script
bash scripts/start_minikube.sh
```

---

## Cách 2: Sử dụng Minikube Command Trực tiếp

### Khởi động với cấu hình cơ bản

```bash
minikube start \
    --memory=4096 \
    --cpus=2 \
    --disk-size=20g \
    --driver=docker \
    --addons=ingress \
    --addons=metrics-server
```

### Khởi động với cấu hình khuyến nghị

```bash
minikube start \
    --memory=6144 \
    --cpus=3 \
    --disk-size=30g \
    --driver=docker \
    --addons=ingress \
    --addons=metrics-server
```

### Khởi động với cấu hình cao (nếu có đủ tài nguyên)

```bash
minikube start \
    --memory=8192 \
    --cpus=4 \
    --disk-size=50g \
    --driver=docker \
    --addons=ingress \
    --addons=metrics-server \
    --addons=dashboard
```

---

## Kiểm tra Tài nguyên

### Kiểm tra cấu hình hiện tại

```bash
bash scripts/check_minikube_resources.sh
```

Hoặc:

```bash
minikube config get memory
minikube config get cpus
minikube config get disk-size
```

### Kiểm tra sử dụng tài nguyên

```bash
# Check node resources
kubectl top nodes

# Check pod resources
kubectl top pods -n ksam

# Check disk usage
minikube ssh -- df -h /
```

---

## Cập nhật Cấu hình

### Nếu Minikube đã chạy và muốn thay đổi cấu hình

```bash
# 1. Stop Minikube
minikube stop

# 2. Delete cluster (optional, để reset)
minikube delete

# 3. Start với cấu hình mới
minikube start \
    --memory=6144 \
    --cpus=3 \
    --disk-size=30g \
    --driver=docker
```

### Cập nhật cấu hình mà không xóa cluster

```bash
# Set new values
minikube config set memory 6144
minikube config set cpus 3
minikube config set disk-size 30g

# Restart Minikube
minikube stop
minikube start
```

---

## Troubleshooting

### Lỗi: "Insufficient memory"

```bash
# Kiểm tra memory available
minikube ssh -- free -h

# Giảm memory requirement
minikube start --memory=3072 --cpus=2
```

### Lỗi: "Disk space full"

```bash
# Kiểm tra disk usage
minikube ssh -- df -h /

# Clean up unused images
minikube ssh -- docker system prune -a -f

# Hoặc tăng disk size
minikube stop
minikube delete
minikube start --disk-size=50g
```

### Lỗi: "Docker driver not available"

```bash
# Kiểm tra Docker Desktop
docker ps

# Nếu Docker không chạy, start Docker Desktop
# Hoặc sử dụng driver khác
minikube start --driver=virtualbox
```

### Lỗi: "Out of memory" khi chạy pods

```bash
# Tăng memory cho Minikube
minikube stop
minikube delete
minikube start --memory=8192 --cpus=4

# Hoặc giảm resource requests trong deployments
kubectl edit deployment ksam-core -n ksam
# Giảm memory requests/limits
```

---

## Quick Start Commands

### Khởi động nhanh với cấu hình khuyến nghị

```bash
# One-liner command
minikube start --memory=6144 --cpus=3 --disk-size=30g --driver=docker --addons=ingress --addons=metrics-server
```

### Khởi động và setup namespace

```bash
# Start Minikube
bash scripts/start_minikube.sh

# Create namespace
kubectl create namespace ksam

# Verify
kubectl get nodes
kubectl get namespaces
```

---

## Verification

Sau khi khởi động, kiểm tra:

```bash
# 1. Check Minikube status
minikube status

# Expected output:
# minikube
# type: Control Plane
# host: Running
# kubelet: Running
# apiserver: Running
# kubeconfig: Configured

# 2. Check node resources
kubectl describe node minikube | grep -A 5 "Allocated resources"

# 3. Check kubectl context
kubectl config current-context
# Expected: minikube

# 4. Test cluster
kubectl get nodes
# Expected: minikube   Ready
```

---

## Environment Variables

Có thể set các biến môi trường để tự động cấu hình:

```bash
export MINIKUBE_MEMORY=6144
export MINIKUBE_CPUS=3
export MINIKUBE_DISK_SIZE=30g
export MINIKUBE_DRIVER=docker
```

Sau đó chạy script sẽ tự động sử dụng các giá trị này.

---

## Best Practices

1. **Luôn kiểm tra tài nguyên trước khi deploy**
   ```bash
   bash scripts/check_minikube_resources.sh
   ```

2. **Monitor resource usage sau khi deploy**
   ```bash
   kubectl top nodes
   kubectl top pods -n ksam
   ```

3. **Clean up unused resources định kỳ**
   ```bash
   minikube ssh -- docker system prune -a -f
   ```

4. **Backup important data trước khi delete cluster**
   ```bash
   # Export data if needed
   kubectl get all -n ksam -o yaml > backup.yaml
   ```

---

## References

- [Minikube Documentation](https://minikube.sigs.k8s.io/docs/)
- [Minikube Drivers](https://minikube.sigs.k8s.io/docs/drivers/)
- [Resource Management](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

