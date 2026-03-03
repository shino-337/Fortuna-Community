# Vận hành FortunaK8s (Production)

## 1. Deploy

### 1.1 Dùng script production

Ưu tiên dùng script trong **`script-prod/`**:

```bash
# Cấu hình (copy và sửa)
cp script-prod/config.env.example script-prod/config.env
# Chỉnh VERSION, REGISTRY, NAMESPACE, ...

# Build image (tag version)
./script-prod/build.sh

# Deploy lên cluster
./script-prod/deploy.sh

# Kiểm tra
./script-prod/verify.sh
```

Chi tiết từng bước: [script-prod/README.md](../script-prod/README.md).

### 1.2 Pipeline (scripts/ – dev / đa node)

- **Clean + Rebuild + Deploy:** `./scripts/pipeline/full-clean-database-rebuild-deploy.sh` (clean images, rebuild core/agent/dashboard, deploy). Tùy chọn `--db` (xóa dữ liệu DB, giữ schema) hoặc `--db-reset` (DROP tables, Core chạy lại migrations).
- **Deploy only:** `./scripts/deploy/deploy-fortuna-robust.sh` (không build; dùng image sẵn có).
- **Push image lên worker (multi-node):** `./scripts/utils/push-images-to-workers.sh` (cần SSH tới từng node hoặc cấu hình `scripts/utils/push-images.config` / `SSH_USER`/`SSH_PASS`). Nếu không push được, Agent/Core trên node có thể chạy image cũ.
- **Clean + Rebuild + Deploy + Test:** `./scripts/pipeline/clean-rebuild-redeploy-and-test.sh`; dùng `--skip-clean --skip-rebuild --skip-deploy` để chỉ chạy test/verify.

Chi tiết: [scripts/README.md](../scripts/README.md).

### 1.3 Thủ công (tham khảo)

- Namespace → Flannel CNI (nếu cần) → StorageClass (local-path) → Infrastructure (PostgreSQL, NATS) → Certificates (mTLS) → Secrets → RBAC → Core → Agent → Dashboard.
- Đầy đủ: [deploy/README.md](../deploy/README.md).

---

## 2. Nâng cấp (Upgrade)

1. **Build image mới** với tag version mới (ví dụ `v1.1.0`).
2. **Cập nhật config** trong `script-prod/config.env` (VERSION=...).
3. **Deploy lại:** `./script-prod/deploy.sh` (Kubernetes rolling update).
4. **Core:** Migrations chạy tự động khi Core khởi động.
5. **Verify:** `./script-prod/verify.sh` và kiểm tra log Core/Agent.

**Rollback:** Deploy lại với VERSION cũ hoặc `kubectl rollout undo deployment/fortuna-core -n fortuna` (và tương tự cho dashboard/agent nếu cần).

---

## 3. Monitoring & Health

### 3.1 Health endpoint

- **Core:** `GET /health` → 200 khi DB + NATS kết nối được.
- **Dashboard:** HTTP 200 trên port 80 (serving HTML).

### 3.2 Metrics

- **Core:** `GET /metrics` (Prometheus format). Cấu hình Prometheus scrape target tới Service fortuna-core:8080.

### 3.3 Log

- **Core:** `kubectl logs -n fortuna -l app.kubernetes.io/component=core -f --tail=300`
- **Agent:** `kubectl logs -n fortuna -l app.kubernetes.io/component=agent -f --tail=300` (hoặc chỉ định pod)
- **Script monitor lỗi:** `./scripts/monitor/monitor-agent-core-errors.sh` hoặc `--follow`

### 3.4 Verify nhanh

- **Full deployment check:** `./scripts/verify/check-full-deployment.sh`
- **Agent–Core connectivity:** `./scripts/verify/verify-agent-core-connectivity.sh`
- **DB schema (gồm pod detail columns):** `./scripts/verify/verify-database-schema.sh`
- **Pod detail: schema + dữ liệu + API:** `./scripts/verify/verify-pod-detail-api-and-db.sh` (kiểm tra migration 068, mẫu dữ liệu pods, response API)
- **Production verify:** `./script-prod/verify.sh`

---

## 4. Xử lý sự cố thường gặp

| Triệu chứng | Hành động |
|-------------|-----------|
| Core/Agent **ContainerCreating** (secret not found) | Tạo mTLS secret: `./scripts/utils/create_mtls_secret.sh` (hoặc bước tương đương trong deploy). |
| Core pod **Pending** (0 nodes available) | Gắn label control-plane: `./scripts/deploy/ensure-control-plane-label.sh`. |
| Agent **CrashLoopBackOff** / **OOMKilled** | Giảm tải: set env `SBOM_WORKERS=1`, tăng memory limit nếu cần; rollout restart DaemonSet. |
| **ErrImageNeverPull** | Đẩy image tới tất cả node: `./scripts/utils/push-images-to-workers.sh` (hoặc dùng registry). |
| Agent **Sync 500** / Core log column missing | Chạy migrations: DB reset hoặc deploy Core mới (migrations tự chạy). Dùng `--db-reset` trong pipeline nếu cần. |
| Agent không kết nối Core | Kiểm tra DNS/endpoint: `./scripts/verify/verify-agent-core-connectivity.sh`. |
| **Pod detail trống** trên Dashboard | DB đã có cột (migration 068) nhưng dữ liệu rỗng: deploy Agent bản mới (có gửi podIP, startTime, owner*, qosClass), push image lên node rồi `kubectl rollout restart daemonset/fortuna-agent -n fortuna`. Kiểm tra: `./scripts/verify/verify-pod-detail-api-and-db.sh`. |

Chi tiết: [docs/AGENT_CORE_ERRORS_MONITOR.md](../docs/AGENT_CORE_ERRORS_MONITOR.md).

---

## 5. Clean (Dọn tài nguyên)

- **Script production:** `./script-prod/clean.sh` (có xác nhận; tùy chọn xóa namespace, image, DB).
- **Không dùng script:** Xóa namespace `fortuna` (sẽ xóa toàn bộ resource trong namespace); hoặc xóa từng deployment/daemonset/service. Lưu ý: xóa namespace sẽ xóa cả PVC (dữ liệu DB).

---

## 6. Backup & khôi phục

- **PostgreSQL:** Backup định kỳ database `fortuna` (pg_dump hoặc snapshot volume). Restore khi cần.
- **NATS:** JetStream state có thể backup theo hướng dẫn NATS; thường ưu tiên backup DB hơn.
- **Cấu hình:** Lưu lại manifest đã chỉnh (YAML), config.env, secret đã tạo (encoded); không commit secret plaintext.

---

**Tiếp theo:** [Cấu hình](06-CONFIGURATION.md)
