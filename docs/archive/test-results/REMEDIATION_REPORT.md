# Báo cáo khắc phục cluster và deploy Fortuna

**Ngày**: 2026-02-22  
**Tham chiếu**: ENVIRONMENT_PREPARATION.md, DEPLOYMENT_CHECKLIST.md, PRODUCTION_DEPLOYMENT.md, POD_STATUS_AND_LOGS_REPORT.md

---

## 1. Tóm tắt vấn đề (đã xác định trước đó)

- **Thiếu kube-proxy** → ClusterIP (10.96.0.1) không hoạt động → Flannel và local-path-provisioner không gọi được API server → CrashLoopBackOff.
- **Thiếu CoreDNS** → DNS trong cluster không hoạt động.
- **Postgres Pending** → PVC không bind (provisioner không chạy).
- **Core không start** → Thiếu secret mTLS (fortuna-ca-cert, fortuna-core-tls, fortuna-webhook-tls).

---

## 2. Các bước khắc phục đã thực hiện

### 2.1 Cài kube-proxy (theo kubeadm, ENVIRONMENT_PREPARATION / cluster addon)

```bash
kubeadm init phase addon kube-proxy --kubeconfig /etc/kubernetes/admin.conf
```

- **Kết quả**: `[addons] Applied essential addon: kube-proxy`
- **Kiểm tra**: `kubectl get pods -n kube-system -l k8s-app=kube-proxy` → 2/2 Running (master + worker).

### 2.2 Cài CoreDNS (theo kubeadm addon)

```bash
kubeadm init phase addon coredns --kubeconfig /etc/kubernetes/admin.conf
```

- **Kết quả**: `[addons] Applied essential addon: CoreDNS`
- **Kiểm tra**: `kubectl get pods -n kube-system -l k8s-app=kube-dns` → 2/2 Running.

### 2.3 Khởi động lại Flannel và local-path-provisioner

- Sau khi kube-proxy Running, ClusterIP 10.96.0.1 hoạt động.
- Đã xóa pod Flannel và local-path-provisioner để tạo lại:
  - `kubectl delete pod -n kube-flannel --all --force --grace-period=0`
  - `kubectl delete pod -n local-path-storage --all --force --grace-period=0`
- **Kết quả**: Flannel 2/2 Running, local-path-provisioner 1/1 Running; PVC postgres-pvc **Bound**.

### 2.4 Deploy Fortuna (theo DEPLOYMENT_CHECKLIST / deploy-fortuna-robust.sh)

- Chạy `./scripts/deploy/deploy-fortuna-robust.sh` (NATS, RBAC, Core, Flannel fix).
- **NATS**: StatefulSet 3/3 Running.
- **Core**: Ban đầu ContainerCreating do thiếu secret.

### 2.5 Tạo mTLS và application secrets (theo DEPLOYMENT_CHECKLIST Step 2–3)

```bash
bash scripts/utils/create_mtls_secret.sh
kubectl create secret generic fortuna-secrets \
  --from-literal=database-url="postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable" \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --namespace=fortuna
```

- **Kết quả**: fortuna-ca-cert, fortuna-core-tls, fortuna-agent-tls, fortuna-webhook-tls, fortuna-secrets đã tạo.
- Xóa pod Core để tạo lại với secret → Core 1/1 Running.

### 2.6 Deploy Agent và Dashboard

- Apply thủ công (do script deploy timeout):
  - `kubectl apply -f deploy/fortuna-agent-daemonset.yaml`
  - `kubectl apply -f deploy/dashboard-nginx-configmap.yaml`
  - `kubectl apply -f deploy/dashboard-deployment.yaml`
- **Kết quả**: Agent 2/2 Running (master + worker), Dashboard 1/1 Running.

### 2.7 Dọn replicaset Postgres trùng

- Scale replicaset postgres-5898b788f6 (bản có nodeSelector control-plane) về 0 để chỉ còn 1 Postgres chạy từ replicaset cũ (pod đã bind PVC trên worker).

---

## 3. Trạng thái sau khắc phục

| Thành phần | Namespace | Trạng thái |
|------------|-----------|------------|
| kube-proxy | kube-system | 2/2 Running |
| CoreDNS | kube-system | 2/2 Running |
| Flannel | kube-flannel | 2/2 Running |
| local-path-provisioner | local-path-storage | 1/1 Running |
| postgres-pvc | fortuna | Bound |
| Postgres | fortuna | 1/1 Running |
| NATS | fortuna | 3/3 Running |
| fortuna-core | fortuna | 1/1 Running |
| fortuna-agent | fortuna | 2/2 Running (DaemonSet) |
| fortuna-dashboard | fortuna | 1/1 Running |

---

## 4. Tài liệu đã tham chiếu

- **docs/ENVIRONMENT_PREPARATION.md**: StorageClass, CNI, CoreDNS kiểm tra.
- **docs/DEPLOYMENT_CHECKLIST.md**: Step 2 (mTLS), Step 3 (fortuna-secrets), Step 4–6 (Postgres, NATS, build).
- **docs/PRODUCTION_DEPLOYMENT.md**: DNS (fix-dns-config.sh), Flannel (fix-flannel-vxlan.sh).
- **docs/test-results/POD_STATUS_AND_LOGS_REPORT.md**: Nguyên nhân gốc (thiếu kube-proxy), log Flannel/provisioner.
- **Kubernetes / kubeadm**: `kubeadm init phase addon kube-proxy`, `kubeadm init phase addon coredns`.

---

## 5. Khuyến nghị tiếp theo

1. **Core migrations**: Core chạy migrations khi start; kiểm tra log Core nếu có lỗi DB (ví dụ migration 062).
2. **Agent ↔ Core**: Sau vài phút kiểm tra `kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=20` (Heartbeat, Sync).
3. **Dashboard**: Port-forward `kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80` và mở http://localhost:8081.
4. **Postgres duplicate**: Nếu muốn Postgres chạy trên control-plane, cần dùng PVC mới hoặc local-path trên master; hiện tại 1 Postgres trên worker là đủ.

---

*Báo cáo được tạo sau khi hoàn tất các bước khắc phục theo tài liệu deploy và troubleshooting.*
