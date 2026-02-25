# Tiến độ công việc – Cập nhật

*Cập nhật: 2026-02-05.*

---

## 1. Tổng quan

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|--------|
| **API /api/v1/insights/summary?clusterId** | ✅ Hoàn thành | Bỏ vulnFilter, chuẩn hóa clusterId, thêm log chẩn đoán; doc `DEBUG_INSIGHTS_SUMMARY_API.md` |
| **Kiểm tra DB thực tế** | ✅ Hoàn thành | clusterId=sha256-4016171f29742e37: 19 pods, 31 insights, JOIN=0 (insight orphan – resource_uid không có trong pods) |
| **Logic insight pod đã xóa** | ✅ Hoàn thành | Phân tích trong `INSIGHT_ORPHAN_POD_LOGIC.md`; thêm case "Pod" trong `InsightStatusUpdater` (auto-resolve khi pod không còn) |
| **Chạy test cases** | ✅ Hoàn thành | check-full-deployment, test-priority1-apis, test-runtime-signals-e2e, verify-dashboard-api (exec) – PASS |
| **Risk Center UI/UX (theo spec)** | ✅ Hoàn thành | Threat Velocity, Exposure 2×2, bảng cột mới, drawer chi tiết, runtime chỉ ở Reference; xem §2.3 |
| **Rebuild Core** | ⏳ Chưa thực hiện | Cần build để áp dụng: insights_handlers, insight_status_updater, cluster_id |
| **Rebuild Dashboard** | ⏳ Chưa thực hiện | Cần build để áp dụng thay đổi Risk Center (Insights.tsx, api, types) |
| **Redeploy (rollout restart)** | ⏳ Chưa thực hiện | Sau khi rebuild, rollout restart core/dashboard (hoặc dùng pipeline) |

---

## 2. Đã thực hiện chi tiết

### 2.1 Code & docs

- **`core/internal/api/insights_handlers.go`**
  - Khi có `clusterId`: bỏ filter `insight_type = 'vulnerability'` → đếm mọi insight của cluster.
  - Chuẩn hóa: `clusterID = NormalizeClusterID(db, clusterID)`.
  - Log: `pods_in_cluster`, `insights_global`, `join result total`; khi total=0 và có pods/insights thì log `insights_with_resource_uid_in_cluster_pods`.
- **`core/pkg/worker/insight_status_updater.go`**
  - Thêm case `"Pod"` trong `checkIfRiskStillExists`: nếu không có pod nào `uid = insight.ResourceUID` và `deleted_at IS NULL` → auto-resolve insight.
- **`docs/DEBUG_INSIGHTS_SUMMARY_API.md`**: Hướng dẫn debug API trả 0, kèm SQL kiểm tra và kết quả kiểm tra clusterId=sha256-4016171f29742e37.
- **`docs/INSIGHT_ORPHAN_POD_LOGIC.md`**: Phân tích luồng xử lý insight khi pod đã xóa / không còn podId (CorrelatorWorker, InsightStatusUpdater, InsightsCleanupJob, PodCleanupJob).

### 2.2 Test cases đã chạy (kết quả)

| Test | Kết quả |
|------|--------|
| check-full-deployment | PASS (8 pods, services, RBAC, health) |
| test-priority1-apis | PASS (JWT, promotion-rules, runtime-signals) |
| test-runtime-signals-e2e | PASS (POST runtime-events, DB counts, GET signals) |
| verify-dashboard-api (exec) | PASS (totalClusters=1, totalRisks=30) |
| GET /insights/summary (no clusterId) | total=31 |
| GET /insights/summary?clusterId=sha256-... | total=0 (đúng – insight orphan) |

*Lưu ý:* Script pipeline dùng selector `app=fortuna-core`; Core pod thực tế dùng `app.kubernetes.io/component=core`. Test verify-dashboard-api chạy trực tiếp với selector đúng.

### 2.3 Risk Center UI/UX (đã triển khai theo `docs/UIUX_Redesign_Specification.md`)

| Thành phần | File / vị trí | Mô tả |
|------------|----------------|--------|
| **Layer 1 – Threat Velocity** | `dashboard/pages/Insights.tsx` | Chart 7 ngày full-width (GET /dashboard/metrics/threat-velocity?days=7). Click ngày → lọc bảng theo ngày (sinceMinutes từ 0h). Nút "Clear" xóa lọc. |
| **Layer 2 – Exposure Overview** | Cùng trang, tab Risks | Thẻ severity 2×2 (Critical, High, Medium, Low). Resolved (24h) hiển thị phụ, tooltip giải thích nguồn (backend summary). |
| **Layer 3 – Bảng risk** | Cùng trang | Cột: Severity, Title, Risk Type (Vulnerability/Behavior), Detection (Static/Runtime), Affected Asset, Score, Namespace (khi không scope cluster), Status, First Detected, Last Seen. Click dòng mở drawer (không navigate). `colSpan` empty state: 9 khi có clusterId, 10 khi không. |
| **Risk Detail – Drawer** | Thay modal cũ | Drawer bên phải (max-w-lg). Các block: Risk Summary, MITRE Mapping (placeholder), Capability Context (placeholder), Runtime Evidence (placeholder), PCE Impact (placeholder), Affected Asset, Actions (Close, Full page, Attack Path). |
| **Runtime signals** | Tab Reference | Đã bỏ khỏi nội dung chính tab Risks; chỉ còn trong tab Reference. |
| **Metrics** | API + UI | Số liệu severity lấy từ GET /insights/summary; fallback đếm theo trang chỉ khi API summary lỗi. |

Chi tiết spec: `docs/UIUX_Redesign_Specification.md` §11. Ánh xạ UI ↔ API: `docs/RISK_CENTER_UI_AND_APIS.md`.

---

## 3. Hiện trạng môi trường (ước lượng)

- **Git:** branch `main`, commit gần nhất `3fcca458d pre to update capability`. Nhiều file M (core, agent, dashboard, deploy, docs); script đã chuyển vào `scripts/build`, `scripts/clean`, `scripts/pipeline`, `scripts/e2e`, `scripts/verify`, `scripts/utils`, v.v. File mới: `core/internal/api/cluster_id.go`, migrations 060–062, docs (RISK_CENTER_UI_AND_APIS, UIUX_Redesign_Specification, …).
- **Kubernetes (khi kiểm tra):** `kubectl` có thể trả Forbidden (user không có quyền list namespace/pods). Cần quyền cluster đúng để chạy deploy và test.
- **Image tag trong deploy:** thường `fortuna-core:v1.0.0-33-g3fcca458d-dirty`, `fortuna-dashboard:v1.0.0-33-g3fcca458d-dirty` (xem trong `deploy/*.yaml`).
- **Pods (khi cluster accessible):** Core, Dashboard, Agent, Postgres, NATS – rollout restart sau khi rebuild image.

---

## 4. Công việc đang thực hiện / Đang chờ

| Việc | Trạng thái | Ghi chú |
|------|------------|--------|
| Áp dụng Risk Center UI/UX lên môi trường chạy | ⏳ Chờ | Cần rebuild dashboard + rollout (hoặc full pipeline) |
| Rebuild Core | ⏳ Chờ | Áp dụng thay đổi insights, cluster_id, insight_status_updater |
| Rebuild Dashboard | ⏳ Chờ | Áp dụng Risk Center (Insights.tsx, api, types) |
| Chạy E2E / verify sau deploy | ⏳ Chờ | Sau khi có image mới và cluster có quyền kubectl |

## 5. Các bước tiếp theo (để áp dụng điều chỉnh)

1. **Rebuild Core và Dashboard** (có quyền nerdctl/containerd):
   ```bash
   cd /home/k8s/KSAM
   ./scripts/build/build-and-load-containerd.sh
   ```
   Hoặc chỉ dashboard: `./scripts/clean/clean-rebuild-dashboard.sh` (gọi `scripts/build/build-dashboard-containerd.sh`).
2. **Push image lên node** (nếu multi-node): `./scripts/utils/push-images-to-workers.sh`
3. **Deploy / Rollout restart** (có quyền kubectl, namespace fortuna):
   ```bash
   kubectl rollout restart deployment/fortuna-core -n fortuna
   kubectl rollout restart deployment/fortuna-dashboard -n fortuna
   ```
   Hoặc full pipeline: `./scripts/pipeline/full-clean-database-rebuild-deploy.sh` (clean + rebuild + deploy).
4. **Chạy test / E2E:** `./scripts/pipeline/clean-rebuild-redeploy-and-test.sh --skip-clean --skip-rebuild --skip-deploy` (chỉ test + monitor) hoặc từng script trong `scripts/verify`, `scripts/e2e`.

---

## 6. Script liên quan

| Script | Mục đích |
|--------|----------|
| `scripts/build/build-and-load-containerd.sh` | Build core, agent, dashboard → containerd k8s.io |
| `scripts/build/build-dashboard-containerd.sh` | Chỉ build image dashboard (nerdctl → k8s.io) |
| `scripts/clean/clean-rebuild-dashboard.sh` | Xóa image dashboard cũ + build lại + apply deploy + rollout |
| `scripts/pipeline/full-clean-database-rebuild-deploy.sh` | Clean (optional `--db`) → Rebuild → Deploy; có `--skip-rebuild`, `--skip-deploy` |
| `scripts/pipeline/clean-rebuild-redeploy-and-test.sh` | Clean/Rebuild/Deploy + test cases + monitor; có `--skip-rebuild --skip-deploy` để chỉ chạy test |
| `scripts/utils/push-images-to-workers.sh` | Đẩy image fortuna lên tất cả nodes (imagePullPolicy: Never) |
