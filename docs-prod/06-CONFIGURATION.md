# Cấu hình FortunaK8s (Production)

## 1. Biến môi trường chính

### 1.1 Core

| Biến | Mô tả | Mặc định |
|------|--------|----------|
| `DATABASE_URL` | Chuỗi kết nối PostgreSQL | (từ secret hoặc env) |
| `NATS_ENDPOINT` | NATS server | `nats://nats-client.fortuna.svc.cluster.local:4222` |
| `TLS_ENABLED` | Bật mTLS gRPC | `true` |
| `AUTH_ENABLED` | Bật JWT auth | `true` |
| `PCE_SCHEDULER_ENABLED` | Bật PCE scheduler | `true` |
| `PCE_SCHEDULER_INTERVAL` | Chu kỳ scheduler | `6h` |
| `DEFAULT_CLUSTER_ID` / `DEFAULT_CLUSTER_NAME` | Fallback khi chưa có cluster | (tùy chọn) |
| `FORTUNA_ENABLE_SEED_DATA` | Bật seed capability metadata, promotion rules (migration 050, 051) | (tùy môi trường) |

### 1.2 Agent

| Biến | Mô tả | Mặc định |
|------|--------|----------|
| `CORE_GRPC_ENDPOINT` | Endpoint gRPC Core | `fortuna-core.fortuna.svc.cluster.local:9090` |
| `CLUSTER_ID` / `CLUSTER_NAME` | Override cluster identity | Tự phát hiện từ K8s API |
| `SYNC_INTERVAL` | Chu kỳ sync pod/SBOM | `5m` |
| `TLS_ENABLED` | Bật mTLS client | `true` |
| `SBOM_WORKERS` | Số worker SBOM (giảm nếu OOM) | (mặc định trong code) |
| `NODE_NAME` | Tên node (Downward API) | (từ spec.nodeName) |

---

## 2. Secrets (Kubernetes)

- **fortuna-core-tls:** Certificate server Core (mTLS).
- **fortuna-agent-tls:** Certificate client Agent.
- **fortuna-ca-cert:** CA dùng cho mTLS.
- **fortuna-webhook-tls:** Certificate webhook (nếu dùng).
- **core-secrets / fortuna-secrets:** Có thể chứa `DATABASE_URL`, password admin, v.v.; không hardcode trong manifest.

Tạo mTLS: `./scripts/utils/create_mtls_secret.sh` (hoặc tương đương trong pipeline). Production: dùng PKI nội bộ, xoay cert định kỳ.

---

## 3. Image & Registry (Production)

- **Tag version:** Dùng tag có version (ví dụ `v1.0.0`), tránh `latest` cho production.
- **Registry:** Set `REGISTRY` trong script-prod (e.g. `registry.company.com/fortuna`) → build push lên registry; deploy dùng image từ registry.
- **imagePullPolicy:** Production dùng `IfNotPresent` hoặc `Always`; `Never` chỉ cho dev (image local).
- **imagePullSecrets:** Nếu registry private: tạo Secret docker-registry và khai báo trong deployment/daemonset.

---

## 4. Namespace & Cluster

- **Namespace:** Mặc định `fortuna`; đổi qua env `NAMESPACE` hoặc trong config script-prod.
- **Cluster identity:** Agent tự discovery; override bằng ConfigMap `fortuna-cluster-config` (keys `cluster_id`, `cluster_name`) hoặc env `CLUSTER_ID`/`CLUSTER_NAME` trên Agent.

---

## 5. Tùy chỉnh Resource (CPU/Memory)

- **Core:** Chỉnh `resources.requests/limits` trong deploy/fortuna-core-deployment.yaml.
- **Agent:** Trong fortuna-agent-daemonset.yaml (hiện thường limit memory 2Gi); tăng nếu node đủ tài nguyên.
- **Dashboard:** Chỉnh trong dashboard-deployment.yaml.
- **PostgreSQL/NATS:** Trong deploy/infrastructure/*.yaml.

---

## 6. Script production config

File **`script-prod/config.env`** (từ `config.env.example`):

- `VERSION`: Tag image (vd. v1.0.0).
- `REGISTRY`: Registry host (để trống nếu chỉ build local).
- `NAMESPACE`: Kubernetes namespace.
- `SKIP_BUILD_*`, `SKIP_DEPLOY_*`: Bỏ qua từng bước nếu cần.
- `LOG_DIR`: Thư mục ghi log script.

Chi tiết từng biến: [script-prod/README.md](../script-prod/README.md).

---

## 7. Database & migrations

- **Schema:** Do Core quản lý qua migrations (core/migrations/). Core chạy tất cả migrations khi khởi động.
- **Pod detail (068):** Thêm cột `pod_ip`, `start_time`, `restart_count`, `owner_kind`, `owner_name`, `replica_set_name`, `qos_class` vào bảng `pods`.
- **Spec hash (069, 070):** `spec_hash`, `last_evaluated_hash` cho PCE điều kiện và race protection.
- **Reset DB (dev):** Dùng script pipeline với `--db-reset` hoặc chạy SQL trong deploy/e2e/reset_database_full.sql (sau đó restart Core để chạy lại migrations).

---

**Về đầu:** [README](README.md)
