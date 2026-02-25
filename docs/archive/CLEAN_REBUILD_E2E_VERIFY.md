# Kiểm tra loại bỏ MOCK data – Clean, Rebuild, E2E

*Cập nhật: 2026-02-02*

## 1. Kiểm tra API dùng MOCK/stub (trước rebuild)

### Đã rà soát

- **Dashboard** (`dashboard/lib/api.ts`): Tất cả API đã chuyển sang real (clusters, risks, resources, rules, certificates, agents, notifications, audit-logs, reports, sync-status, users). Không còn constant MOCK.
- **Core API**:
  - `QueryPrometheusMetrics`: **không** được đăng ký trong `routes.go` → không expose mock.
  - `GetCertificateRotationHistory`: Trả về `_dataSource: "stub"`, `history: []`, `total: 0`.
  - `GetPolicyEvaluationCost`: Trả về `_dataSource: "partial"`, `message: "avgPerRule/peak/CPU/memory require Prometheus"`, `totalEvaluations` (từ DB).
  - Các handler khác (agents, dashboard/stats, error-logs, notifications, …): dữ liệu từ DB.

### Thay đổi đã thực hiện (loại bỏ stub/mock trong response)

| File | Thay đổi |
|------|----------|
| `core/internal/api/cert_handler.go` | `GetCertificateRotationHistory`: Xóa `_dataSource: "stub"`. Chỉ trả về `{ "history": [], "total": 0 }`. |
| `core/internal/api/metrics_handlers.go` | `GetPolicyEvaluationCost`: Xóa `_dataSource`, `message`. Chỉ trả về `{ "totalEvaluations": <số từ DB> }`. |
| `core/internal/api/dashboard_data_integrity.go` | Endpoint rotation/history: `Source` từ `"stub"` → `"real"`, Note: empty list until rotation_history table. |
| `core/internal/api/routes.go` | Comment rotation/history: bỏ "stub", ghi rõ "real API, no mock". |

Sau khi deploy Core mới, hai endpoint trên **không còn** trả về field stub/mock trong JSON.

---

## 2. Clean

- **Core**: `go clean -cache` trong `core/`; xóa binary cũ (nếu có).
- **Dashboard**: Xóa `dashboard/dist` (build output).
- **Agent**: Không clean (build vẫn lỗi do code khác: converter, models).

---

## 3. Rebuild

- **Core**: `go build -o fortuna-core -buildvcs=false ./cmd` → **thành công** (binary `core/fortuna-core`).
- **Dashboard**: Build chỉ qua containerd (không cần npm/Node.js trên host). Chạy `./scripts/build/build-and-load-containerd.sh` hoặc `./scripts/build/build-dashboard-containerd.sh`; npm chỉ chạy trong Dockerfile (trong container).
- **Agent**: Không build (lỗi `internal/converter`, `pkg/models`).

---

## 4. E2E và kiểm tra API

- **Cluster**: Có `kubectl`, namespace `fortuna` với Core, Dashboard, Agent, Postgres, NATS.
- **scripts/e2e/test-priority1-apis.sh**: Chạy xong, tất cả test **PASS**:
  - GET /api/v1/promotion-rules (count 9)
  - GET /api/v1/promotion-rules/capability/ESC_HOSTPATH_NODE (count 3)
  - GET /api/v1/promotion-rules/signal/PROC_ROOT_PIVOT (count 4)
  - GET /api/v1/runtime-signals (count/total 0)
- **API đã gọi thủ công** (Core đang chạy image cũ):
  - `GET /api/v1/dashboard/stats`: 200, totalClusters 1, runningPods 21, totalRisks 5, activeAgents 0.
  - `GET /api/v1/agents/status`: 200, agents [], total 0 (Core cũ chưa có fix Ping cập nhật `last_seen_at`).
  - `GET /api/v1/metrics/policy-evaluation-cost`: Vẫn có `_dataSource`, `message` (image cũ; sau deploy image mới sẽ chỉ còn `totalEvaluations`).

Báo cáo chi tiết E2E: `docs/test-results/E2E-FULL-<timestamp>.md`.

---

## 5. Luồng end-to-end cần làm sau khi deploy image mới

1. **Build image Core** từ code mới (đã bỏ stub trong rotation/history và policy-evaluation-cost).
2. **Deploy Core** (và nếu có, Agent với Ping gửi `agent_id`/`node_name` để Core cập nhật `last_seen_at`).
3. **Kiểm tra**:
   - `GET /api/v1/certificates/rotation/history` → chỉ `{ "history": [], "total": 0 }`, không có `_dataSource`.
   - `GET /api/v1/metrics/policy-evaluation-cost` → chỉ `{ "totalEvaluations": <số> }`, không có `_dataSource`/`message`.
   - `GET /api/v1/agents/status` → sau vài phút có agent (nếu Agent mới đã deploy và Ping cập nhật `last_seen_at`).
4. **Dashboard**: Trang Certificates, Monitoring dùng API trên; sau deploy Core mới sẽ nhận response không còn stub/mock.

---

## 6. Tóm tắt

- Đã **loại bỏ** response stub/mock ở `GetCertificateRotationHistory` và `GetPolicyEvaluationCost`.
- Đã **clean** (Core cache, dashboard dist) và **rebuild Core** thành công.
- **E2E** (priority-1 APIs) chạy trên cluster hiện tại: **pass**.
- Sau khi **build image Core (và Agent) mới và deploy**, chạy lại test-priority1-apis.sh và kiểm tra 3 endpoint trên để xác nhận không còn MOCK data.
