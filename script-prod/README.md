# Script Production – Fortuna (KSAM)

Script vận hành chuẩn production: **build**, **deploy**, **clean**, **verify**. Dùng cấu hình qua file `config.env` (copy từ `config.env.example`).

---

## Yêu cầu

- **kubectl** trỏ tới cluster đích.
- **nerdctl** (hoặc **docker**) để build image; **containerd** namespace `k8s.io` nếu chạy K8s bằng containerd.
- **Bash** 4+ (set -euo pipefail).

---

## Cấu hình

```bash
cp config.env.example config.env
# Chỉnh: VERSION, REGISTRY, NAMESPACE, ...
```

| Biến | Mô tả | Ví dụ |
|------|--------|--------|
| `VERSION` | Tag image (bắt buộc cho production) | `v1.0.0` |
| `REGISTRY` | Registry (để trống = chỉ build local) | `registry.company.com/fortuna` |
| `NAMESPACE` | Namespace Kubernetes | `fortuna` |
| `CONTAINERD_NAMESPACE` | Namespace containerd (nerdctl) | `k8s.io` |
| `SKIP_BUILD_CORE` | Bỏ qua build Core | `0` hoặc `1` |
| `SKIP_BUILD_AGENT` | Bỏ qua build Agent | `0` hoặc `1` |
| `SKIP_BUILD_DASHBOARD` | Bỏ qua build Dashboard | `0` hoặc `1` |
| `PUSH_IMAGES` | Push lên registry sau build | `0` hoặc `1` |
| `LOG_DIR` | Thư mục log script | `./logs` (tạo tự động) |

---

## Script

### build.sh

Build image Core, Agent, Dashboard với tag **VERSION**. Ghi log vào `$LOG_DIR/build.log`.

```bash
./script-prod/build.sh
# Hoặc với config đã load:
source config.env 2>/dev/null || true
VERSION=v1.0.0 ./script-prod/build.sh
```

- Build bằng **nerdctl** (ưu tiên) hoặc **docker**.
- Image: `fortuna-core:$VERSION`, `fortuna-agent:$VERSION`, `fortuna-dashboard:$VERSION`.
- Nếu `PUSH_IMAGES=1` và `REGISTRY` set: tag và push lên registry.

### deploy.sh

Deploy lên cluster: namespace, hạ tầng (PostgreSQL, NATS), mTLS (nếu cần), RBAC, Core, Agent, Dashboard. Dùng image tag **VERSION**.

```bash
./script-prod/deploy.sh
```

- Đọc `config.env` nếu có.
- Gọi script sẵn có: ensure-flannel, ensure-storage-class, deploy-fortuna-robust (hoặc apply manifest với image tag từ env).
- Log: `$LOG_DIR/deploy.log`.

### clean.sh

Dọn tài nguyên: xóa deployment/daemonset/service trong namespace, tùy chọn xóa image local và/hoặc dữ liệu DB. **Có xác nhận** trước khi xóa.

```bash
./script-prod/clean.sh
# Tùy chọn:
#   --images     Xóa image fortuna-* khỏi containerd/docker
#   --db         Clear DB (DELETE data, giữ schema)
#   --db-reset   Full reset DB (DROP tables)
#   -y / --yes   Bỏ qua xác nhận (dùng trong CI cẩn thận)
```

- Mặc định chỉ xóa workload trong namespace (Core, Agent, Dashboard); không xóa infra (PostgreSQL, NATS) trừ khi chỉ định.
- Log: `$LOG_DIR/clean.log`.

### verify.sh

Kiểm tra health và trạng thái sau deploy: namespace, pod Running, Core /health, Dashboard HTTP, Agent DaemonSet.

```bash
./script-prod/verify.sh
```

- Exit 0 nếu tất cả check pass; ngược lại exit 1 và in lỗi.
- Có thể gọi `scripts/verify/check-full-deployment.sh` bên trong.

---

## Thứ tự vận hành điển hình

1. **Lần đầu / sau khi đổi code:**
   ```bash
   ./script-prod/build.sh
   ./script-prod/deploy.sh
   ./script-prod/verify.sh
   ```
2. **Chỉ nâng cấp image:** Cập nhật VERSION trong config → build → deploy → verify.
3. **Dọn môi trường:** `./script-prod/clean.sh` (rồi chọn có xóa images/DB hay không).

---

## Log

- Mặc định: `./script-prod/logs/` (build.log, deploy.log, clean.log, verify.log).
- Set `LOG_DIR` trong config để đổi thư mục.

---

## Liên kết

- **Tài liệu production:** [docs-prod/README.md](../docs-prod/README.md)
- **Deploy manifests:** [deploy/README.md](../deploy/README.md)
- **Pipeline dev (clean+rebuild+deploy):** [scripts/pipeline/full-clean-database-rebuild-deploy.sh](../scripts/pipeline/full-clean-database-rebuild-deploy.sh)
