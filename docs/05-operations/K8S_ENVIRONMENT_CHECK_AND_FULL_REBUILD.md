# Kiểm tra môi trường K8s & chuẩn bị full rebuild / deploy (integration & E2E)

**Mục đích:** Tóm tắt **trạng thái cluster**, **điều kiện tiên quyết**, và **bước chuẩn bị** trước khi chạy pipeline clean rebuild + deploy, phục vụ **integration test** và **E2E** (theo `CLEAN_REBUILD_REDEPLOY_AND_VERIFY.md`, `SBOM_PIPELINE_REMAINING_PLAN.md`).

> **Lưu ý:** Báo cáo snapshot dưới đây lấy từ một lần chạy `kubectl` trên môi trường dev; khi triển khai thật hãy chạy lại các lệnh trong mục 1.

---

## 1. Kiểm tra nhanh cluster (chạy trên máy có kubeconfig)

```bash
kubectl version --client
kubectl config current-context
kubectl cluster-info
kubectl get nodes -o wide
kubectl get ns
```

**Kỳ vọng tối thiểu:** API server phản hồi; tất cả node `Ready`.

### 1.1 Namespace `fortuna` (hoặc `NAMESPACE` bạn dùng)

```bash
export NAMESPACE=fortuna   # đổi nếu khác
kubectl get all -n "$NAMESPACE"
kubectl get pvc -n "$NAMESPACE"
kubectl get secrets -n "$NAMESPACE" 2>/dev/null | head -20
```

**Ý nghĩa:**

| Quan sát | Hành động gợi ý |
|----------|------------------|
| **Không có Deployment/StatefulSet/Pod** nhưng **có PVC** (postgres, nats) | Cluster từng deploy; workload đã gỡ. Full deploy cần apply manifest lại; **`--db-reset`** trong pipeline có thể tác động DB — đọc script pipeline trước khi chạy. |
| **Pod Pending / ImagePullBackOff** | Kiểm tra image registry, `imagePullSecrets`, node disk. |
| **Metrics không có (`kubectl top`)** | Cài **metrics-server** nếu cần HPA/monitoring (không chặn deploy Fortuna). |

---

## 2. Snapshot môi trường (ví dụ — cập nhật khi bạn audit)

| Hạng mục | Giá trị ví dụ (điền lại khi chạy) |
|----------|-----------------------------------|
| Context | `kubernetes-super-admin@kubernetes` |
| Control plane | `https://<API>:6443` |
| Nodes | 2× Ready, Kubernetes **v1.29.15** |
| Namespace app | `fortuna` tồn tại |
| Workloads trong `fortuna` | *(trống — không có Deployment/Pod tại thời điểm kiểm tra)* |
| PVC trong `fortuna` | NATS (3× 10Gi), Postgres 20Gi — **Bound** (dữ liệu cũ có thể còn trên disk) |
| Disk node build (nếu build local) | Đảm bảo đủ dung lượng image + layer (~hàng chục GB khuyến nghị) |

---

## 3. Chuẩn bị full rebuild & deploy

### 3.1 Tài liệu chính

| Tài liệu | Nội dung |
|----------|----------|
| **`CLEAN_REBUILD_REDEPLOY_AND_VERIFY.md`** | Pipeline: `scripts/pipeline/full-clean-database-rebuild-deploy.sh`, async log, `--db-reset`. |
| **`PRODUCTION_DEPLOYMENT.md`** | Triển khai production (nếu dùng). |
| **`NVD_API_KEY.md`** | Secret/env cho Core (CVE matcher) — không bắt buộc cho “lên pod”, cần cho test NVD thật. |

### 3.2 Biến môi trường thường gặp trước pipeline

- `NAMESPACE=fortuna` (hoặc namespace triển khai thực tế).
- Registry / `nerdctl` / quyền push image (nếu script build local và push lên node).
- **Secrets:** DB password, JWT nếu bật auth, **`NVD_API_KEY`** (optional), NATS nếu external.

### 3.3 Chạy pipeline (tóm tắt — chi tiết trong doc clean rebuild)

```bash
cd /path/to/KSAM
export NAMESPACE=fortuna

# Khuyến nghị: chạy nền + log file (tránh timeout)
RUN_ASYNC=1 bash scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset
tail -f /tmp/clean-rebuild-deploy.log
```

- **`--db-reset`:** reset DB (DROP) — chỉ dùng khi chấp nhận mất dữ liệu hoặc môi trường dev.
- Nếu **không** muốn xóa DB: bỏ `--db-reset` hoặc dùng tùy chọn tương ứng trong script (đọc đầu file script).

### 3.4 Verify sau deploy + E2E script có sẵn

```bash
export NAMESPACE=fortuna
export REPORT_DIR=docs/test-results
bash scripts/verify/verify-after-deploy-and-e2e.sh
```

Kết quả: pods, API health/integrity, E2E pod-delete cleanup — báo cáo trong `docs/test-results/verify-after-deploy-*.md`.

---

## 4. Chuẩn bị riêng cho **integration / E2E** (SBOM & pipeline)

| Mục | Trạng thái trong repo | Sau khi deploy cluster |
|-----|------------------------|-------------------------|
| **Unit/integration Go** | `SBOM_FLOWS_VERIFICATION_REPORT.md` — chạy trên CI/dev không cần cluster | Tùy chọn: chạy lại `go test` trên máy build. |
| **E2E NATS + Core trong CI** | **Chưa có** (xem `SBOM_PIPELINE_REMAINING_PLAN.md` mục CI E2E) | Thay bằng **verify-after-deploy** + workload thật trên cluster. |
| **Agent → Core → SBOM** | Cần **Fortuna Agent + Core + NATS + Postgres** Running | Sau deploy: tạo pod test image (`docs/03-components/sbom/README.md` có ví dụ nginx), xem log Agent/Core, query SBOM API. |
| **NVD** | Key qua Secret/env | `NVD_API_KEY` trên Core để không bị 429 khi test CVE fallback. |

**Checklist tối thiểu sau khi pods Ready:**

1. `kubectl get pods -n fortuna` — Core, Agent, Dashboard, Postgres, NATS = Running.  
2. Core logs: migrations OK, không crash loop.  
3. `curl`/exec health endpoints (trong `CLEAN_REBUILD_REDEPLOY_AND_VERIFY.md` §3).  
4. (Tùy chọn) Deploy pod sample → chờ SBOM → kiểm tra API `/api/v1/sboms` hoặc DB.

---

## 5. Rủi ro / lưu ý

1. **PVC cũ:** Nếu giữ PVC Postgres/NATS nhưng đổi image/schema mạnh, có thể cần migration hoặc `--db-reset` (mất data).  
2. **`continue-on-error` trên GitHub Actions:** test CI có thể đỏ mà không fail PR — không thay verify trên cluster.  
3. **metrics-server:** `kubectl top` có thể lỗi nếu chưa cài — không ảnh hưởng trực tiếp Fortuna trừ khi bạn dựa vào HPA.

---

## 6. Liên kết

- `SBOM_PIPELINE_REMAINING_PLAN.md` — backlog CI E2E tự động.  
- `SBOM_FLOWS_VERIFICATION_REPORT.md` — phạm vi `go test`.  
- `CLEAN_REBUILD_REDEPLOY_AND_VERIFY.md` — quy trình rebuild đầy đủ.
