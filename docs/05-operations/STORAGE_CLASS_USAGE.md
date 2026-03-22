# StorageClass – Lịch sử và Hướng dẫn sử dụng

**Cập nhật**: 2026-02-21

---

## 1. Tổng quan

Fortuna dùng **PersistentVolumeClaim (PVC)** cho:

- **PostgreSQL**: `deploy/infrastructure/postgresql.yaml`, `postgresql-with-age.yaml` → `storageClassName: local-path`
- **NATS**: `deploy/infrastructure/nats.yaml` → `storageClassName: local-path`
- **Redis** (tùy chọn): `deploy/infrastructure/redis.yaml` → `storageClassName: local-path`

Tất cả đều yêu cầu cluster có **StorageClass** (mặc định dùng tên `local-path` từ Rancher local-path-provisioner).

---

## 2. Tài liệu hiện có

| Tài liệu | Nội dung liên quan StorageClass |
|---------|---------------------------------|
| **docs/ENVIRONMENT_PREPARATION.md** | Prerequisites: "Local storage provisioner (local-path-provisioner) or equivalent". Mục "Check Storage Class": kiểm tra `kubectl get storageclass`, kiểm tra pods `local-path-storage`, **lệnh cài thủ công** local-path-provisioner. Troubleshooting: "Storage Class Not Found" → cài local-path-provisioner. |
| **docs/PRODUCTION_DEPLOYMENT.md** | Prerequisites: "Local storage provisioner (local-path-provisioner) or equivalent". Production: dùng storage production (EBS, Azure Disk) thay local-path. |
| **docs/01-getting-started/ENVIRONMENT_REQUIREMENTS.md** | Fortuna cần StorageClass hỗ trợ ReadWriteOnce, SSD khuyến nghị, dynamic provisioning. |
| **deploy/infrastructure/postgresql-local-pv.yaml** | Dùng khi **không có StorageClass**: PV hostPath + PVC `storageClassName: local-storage`. Hướng dẫn: tạo thư mục trên node (`/mnt/postgres-data`), rồi apply file. |
| **deploy/README.md** | Quick Deploy dùng `postgresql.yaml` (cần local-path); không nêu bước cài StorageClass. |
| **docs/DEPLOYMENT_CHECKLIST.md** | Step 4: Deploy PostgreSQL bằng `postgresql.yaml`; không có bước kiểm tra/cài StorageClass trước đó. |

**Kết luận tài liệu**: Cách dùng StorageClass và lệnh cài **local-path-provisioner** được mô tả rõ trong **ENVIRONMENT_PREPARATION.md**. Không có tài liệu nào mô tả script tự động cài StorageClass; chỉ có hướng dẫn thủ công.

---

## 3. Script tự động

**Kiểm tra toàn bộ `scripts/`**:

- **Không có** script nào cài local-path-provisioner hoặc tạo StorageClass.
- **Không có** bước kiểm tra StorageClass trong `scripts/deploy/pre-deployment-checks.sh` (chỉ kiểm tra kubectl, nerdctl, cluster, namespace, postgres/nats service, CoreDNS, node labels).
- **deploy-fortuna-robust.sh** luôn dùng `postgresql-with-age.yaml` (PVC `storageClassName: local-path`), không tự cài provisioner.

**Script mới (2026-02-21)**:
- **`scripts/deploy/ensure-storage-class.sh`**: Kiểm tra StorageClass `local-path`; nếu chưa có thì cài Rancher local-path-provisioner và đợi pods Ready. Được gọi tự động trong pipeline (Phase 2c).
- **`scripts/pipeline/full-clean-database-rebuild-deploy.sh`**: Trước Phase 3 (Deploy) có **Phase 2c: Ensure StorageClass** — chạy `ensure-storage-class.sh` khi không skip deploy. Nhờ đó clean/rebuild/redeploy tự động đảm bảo StorageClass trước khi deploy PostgreSQL/NATS.

**Lưu ý**: Nếu cluster không có CNI hoạt động đầy đủ (ví dụ Flannel thiếu `/run/flannel/subnet.env` trên node), pod local-path-provisioner có thể không start được. Khi đó PVC sẽ ở trạng thái Pending. Cần sửa CNI trên cluster hoặc dùng `postgresql-local-pv.yaml` (hostPath) trên node đã chuẩn bị thư mục.

---

## 4. Cách dùng trong thực tế

### Option A: Cluster đã có StorageClass (local-path)

- Đảm bảo StorageClass tồn tại:
  ```bash
  kubectl get storageclass
  # Có tên local-path (hoặc default) là đủ.
  ```
- Deploy như bình thường:
  - Pipeline: `./scripts/pipeline/full-clean-database-rebuild-deploy.sh`
  - Deploy robust: `./scripts/deploy/deploy-fortuna-robust.sh`
  - Cả hai dùng `postgresql-with-age.yaml` (PVC `local-path`).

### Option B: Cluster không có StorageClass (ví dụ kubeadm mới init)

**Cách 1 – Cài local-path-provisioner (theo ENVIRONMENT_PREPARATION.md):**

```bash
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml
kubectl get pods -n local-path-storage   # Đợi Running
kubectl get storageclass                 # Có local-path
```

Sau đó chạy pipeline/deploy như Option A.

**Cách 2 – Dùng PostgreSQL local PV (không cần StorageClass):**

1. Trên node sẽ chạy Postgres (thường là control-plane):
   ```bash
   sudo mkdir -p /mnt/postgres-data && sudo chmod 777 /mnt/postgres-data
   ```
2. Deploy Postgres bằng manifest dùng hostPath:
   ```bash
   kubectl apply -f deploy/infrastructure/postgresql-local-pv.yaml
   ```
3. **Lưu ý**: `deploy-fortuna-robust.sh` mặc định dùng `postgresql-with-age.yaml`. Nếu dùng Option B, cần:
   - Hoặc sửa script để dùng `postgresql-local-pv.yaml` khi không có StorageClass,  
   - Hoặc apply `postgresql-local-pv.yaml` tay trước khi chạy script (và đảm bảo script không ghi đè lại bằng postgresql-with-age).

---

## 5. Gợi ý cải thiện (script tự động)

Để có “script tự động” cho StorageClass, có thể:

1. **Thêm vào pre-deployment-checks**: Kiểm tra `kubectl get storageclass`; nếu không có `local-path` (và không có default) thì báo lỗi hoặc cảnh báo, kèm gợi ý cài local-path hoặc dùng `postgresql-local-pv.yaml`.
2. **Script mới `scripts/deploy/ensure-storage-class.sh`**:
   - Nếu đã có StorageClass (local-path hoặc default) → thoát 0.
   - Nếu chưa có → apply manifest local-path-provisioner (URL trên), đợi pods Ready, rồi kiểm tra lại.
3. **Pipeline / deploy-fortuna-robust**: Gọi `ensure-storage-class.sh` trước bước deploy infrastructure (PostgreSQL), hoặc cho phép biến môi trường (ví dụ `USE_LOCAL_PV=1`) để deploy PostgreSQL bằng `postgresql-local-pv.yaml` thay vì `postgresql-with-age.yaml`.

Tài liệu này có thể cập nhật khi các script trên được thêm vào repo.
