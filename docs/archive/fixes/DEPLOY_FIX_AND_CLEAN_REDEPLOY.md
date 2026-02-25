# Tổng kết kiểm tra, sửa lỗi và clean + deploy lại

## 1. Các vấn đề đã phát hiện và xử lý

### 1.1 Postgres & NATS – nodeSelector master vs PVC trên worker01

- **Vấn đề:** Manifest đã gắn `nodeSelector: k8s-master` để tránh DiskPressure trên worker01, nhưng PVC (postgres-pvc, data-nats-*) đã bind PV trên **worker01** (local-path RWO). Pod với nodeSelector master không thể mount volume trên worker01 → **volume node affinity conflict**, NATS/Postgres Pending.
- **Xử lý:** Bỏ `nodeSelector` và `tolerations` trong `deploy/infrastructure/postgresql-with-age.yaml` và `deploy/infrastructure/nats.yaml`. Postgres và NATS schedule trên worker01 (nơi có PVC); worker01 đã tăng disk nên đủ dung lượng.

### 1.2 Agent – ErrImageNeverPull trên worker01

- **Vấn đề:** DaemonSet dùng `image: fortuna-agent:v1.0.0-33-g3fcca458d-dirty` và `imagePullPolicy: Never`. Image chỉ có trên master (build/load tại đó) → trên worker01 không có image → **ErrImageNeverPull**.
- **Xử lý:** Đổi sang `image: fortuna-agent:latest` và `imagePullPolicy: IfNotPresent`. Build với `BUILD_TAG=latest` và load trên master. Trên **worker01** cần load image thủ công (xem mục 3).

### 1.3 Core & Dashboard – image tag và pull policy

- **Xử lý:** Đổi sang `image: fortuna-core:latest` / `fortuna-dashboard:latest` và `imagePullPolicy: IfNotPresent` để thống nhất với build `BUILD_TAG=latest` và chỉ dùng image local.

### 1.4 Deploy script – infra luôn được apply

- **Vấn đề:** Script chỉ deploy Postgres/NATS khi “chưa tồn tại”, nên khi đổi spec (bỏ nodeSelector) không được apply lại.
- **Xử lý:** Bước 4 luôn chạy `kubectl apply -f` cho postgres và nats, rồi đợi ready.

### 1.5 NATS Pending sau khi sửa spec

- **Vấn đề:** Sau khi bỏ nodeSelector, các pod nats-0, nats-2 vẫn Pending (spec cũ đã được tạo trước đó).
- **Xử lý:** Xóa pod Pending (`kubectl delete pod nats-0 nats-2 --force --grace-period=0`) để StatefulSet tạo lại; pod mới schedule trên worker01 và chạy bình thường.

---

## 2. Đã thực hiện: clean và deploy lại

1. **Clean:** Chạy `scripts/clean/clean-evicted-completed-pods.sh` (xóa pod evicted/completed).
2. **Build:** `BUILD_TAG=latest scripts/build/build-and-load-containerd.sh` → fortuna-core:latest, fortuna-agent:latest, fortuna-dashboard:latest (load vào containerd trên **master**).
3. **Deploy:** Chạy `scripts/deploy/deploy-fortuna-robust.sh` (bị timeout ở bước đợi NATS); sau đó:
   - Xóa pod nats-0, nats-2 Pending để StatefulSet tạo lại trên worker01.
   - Apply RBAC, Core, Agent, Dashboard.

**Kết quả hiện tại:**

- **fortuna-core:** 1/1 Running (master)
- **fortuna-dashboard:** 1/1 Running (master)
- **postgres:** 1/1 Running (worker01)
- **nats:** 3/3 Running (worker01)
- **fortuna-agent:** 1/2 (master Running; worker01 ErrImagePull cho đến khi load image trên worker01)

---

## 3. Push image sang worker (multi-node) – tự động và thủ công

**Tự động:** Các lần chạy clean/build/deploy sau sẽ tự gọi push:
- **full-clean-database-rebuild-deploy.sh** – Phase 2b (sau rebuild, trước deploy) chạy `scripts/utils/push-images-to-workers.sh`.
- **deploy-fortuna-robust.sh** – Step 5b (khi cluster có > 1 node) chạy cùng script push trước khi deploy Core/Agent.

**Thủ công (khi push tự động thất bại, thường do SSH):**

```bash
# Cấu hình SSH (chọn user có quyền trên worker)
export SSH_USER=root
export SSH_PASS=yourpassword   # hoặc dùng SSH key, không cần SSH_PASS
export WORKER_NODES="192.168.56.101"   # hoặc bỏ qua để dùng tất cả node từ kubectl

bash scripts/utils/push-images-to-workers.sh
```

Nếu không set `SSH_USER`, script sẽ thử lần lượt: `$USER`, `root`, `k8s`. Khi copy/import thất bại, script in ra lỗi SSH và gợi ý lệnh.

Sau khi push xong, xóa pod agent trên worker để DaemonSet tạo lại với image mới:
`kubectl delete pod -n fortuna -l app.kubernetes.io/component=agent --field-selector spec.nodeName=k8s-worker01`

---

## 4. File đã sửa / thêm

| File | Thay đổi |
|------|----------|
| `deploy/infrastructure/postgresql-with-age.yaml` | Bỏ nodeSelector + tolerations |
| `deploy/infrastructure/nats.yaml` | Bỏ nodeSelector + tolerations |
| `deploy/fortuna-agent-daemonset.yaml` | image fortuna-agent:latest, imagePullPolicy IfNotPresent |
| `deploy/fortuna-core-deployment.yaml` | image fortuna-core:latest, imagePullPolicy IfNotPresent |
| `deploy/dashboard-deployment.yaml` | image fortuna-dashboard:latest, imagePullPolicy IfNotPresent |
| `scripts/deploy/deploy-fortuna-robust.sh` | Bước 4 luôn apply postgres + nats |
| `scripts/build/export-agent-image-for-workers.sh` | Script export image agent ra tar để copy sang worker |

---

## 5. Lệnh clean + deploy đầy đủ (lần sau)

```bash
# Clean
bash scripts/clean/clean-evicted-completed-pods.sh
# Optional: bash scripts/clean/cleanup-environment.sh --aggressive

# Build (trên master)
BUILD_TAG=latest bash scripts/build/build-and-load-containerd.sh

# Deploy
AUTO_LOAD_CVE_ON_DEPLOY=false bash scripts/deploy/deploy-fortuna-robust.sh
# Nếu NATS vẫn Pending: xóa pod nats-0 nats-2 để StatefulSet tạo lại

# Multi-node: load agent image trên worker01 (xem mục 3)
```
