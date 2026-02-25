# E2E & Kiểm tra hiển thị API trên Dashboard

*Thực hiện: 2026-02-12.*

---

## 1. Tổng kết test cases

| Bước | Test | Kết quả |
|------|------|--------|
| 1 | **verify-dashboard-apis** | ✅ PASS – Tất cả API dùng bởi Dashboard trả dữ liệu |
| 2a | **test-priority1-apis** | ✅ PASS – JWT, promotion-rules, runtime-signals |
| 2b | **test-runtime-signals-e2e** | ✅ PASS – POST runtime-events, DB counts, GET signals |
| 3 | **API hiển thị (dashboard/stats, insights/summary, clusterId)** | ✅ PASS – Có dữ liệu với và không có clusterId |
| 4 | **check-full-deployment** | ✅ PASS – 0 errors, 0 warnings |
| 5 | **test-pod-critical-risk-cluster-id** (2 test case) | ✅ PASS – Pod critical risk trên clusterId + API với clusterId active |

### 5.1 Test case 1: Pod có risk critical trên clusterId hiện tại

- Lấy clusterId active từ `GET /clusters`.
- Nếu chưa có critical insight cho pod trong cluster: insert 1 critical insight (vulnerability) cho một pod trong cluster.
- Gọi `GET insights/summary?clusterId=` và `GET dashboard/stats?clusterId=` → xác nhận **critical >= 1**.

### 5.2 Test case 2: Kiểm tra API với clusterId đang active

- Gọi `dashboard/stats?clusterId=`, `insights/summary?clusterId=`, `risks?clusterId=` với clusterId active.
- Xác nhận cấu trúc response và tính nhất quán (critical count, risks list có critical).

---

## 2. Kết quả API dùng cho hiển thị Dashboard

### 2.1 Dashboard stats (số liệu trang chủ)

| API | Kết quả |
|-----|--------|
| `GET /api/v1/dashboard/stats` (không filter) | totalClusters=1, runningPods=19, totalRisks=32, activeAgents=2, affectedPodCount=2 |
| `GET /api/v1/dashboard/stats?clusterId=sha256-4016171f29742e37` | totalRisks=2, clusterName="kubernetes", affectedPodCount=2 |

→ **Dashboard** khi chọn cluster "kubernetes" sẽ hiển thị totalRisks=2, clusterName; khi không chọn cluster sẽ hiển thị tổng toàn cục (32 risks).

### 2.2 Insights summary (Security Risks cards – Critical/High/Medium/Low)

| API | Kết quả |
|-----|--------|
| `GET /api/v1/insights/summary` (không clusterId) | total=3, critical=1, medium=2, byType: rbac=1, vulnerability=2 |
| `GET /api/v1/insights/summary?clusterId=sha256-4016171f29742e37` | total=2, medium=2, byType: vulnerability=2 |

→ **Dashboard** dùng API này cho thẻ Security Risks; khi có clusterId sẽ hiển thị breakdown theo cluster (2 risks, medium).

### 2.3 Các API khác (verify-dashboard-apis)

| API | Kết quả |
|-----|--------|
| `GET /api/v1/clusters` | 1 cluster (id=sha256-4016171f29742e37, name=kubernetes) |
| `GET /api/v1/risks` | 2 insights (Critical Risks / Risk Center) |
| `GET /api/v1/notifications` | 0 notifications |
| `GET /api/v1/dashboard/metrics/threat-velocity?days=7` | 7 trend points |
| `GET /api/v1/pod-capabilities/summary/capability` | 7 summary entries |
| `GET /api/v1/pod-capabilities/trends?days=7` | 7 points (PCE Trend chart) |

→ **Dashboard** có đủ dữ liệu cho: Cluster Health, Critical Risks, Recent Activity, Threat Velocity chart, Pod Capabilities list, PCE Trend chart.

---

## 3. Kết luận

- **E2E:** Các test case đã chạy đều PASS.
- **Hiển thị trên Dashboard:** Các API mà Dashboard gọi đều trả dữ liệu đúng; khi chọn cluster (clusterId) thì dashboard/stats và insights/summary đều scope theo cluster (totalRisks=2, clusterName=kubernetes).
- **Cách kiểm tra trên trình duyệt:** Port-forward dashboard `kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80`, mở http://localhost:8081, đăng nhập admin/admin123, kiểm tra trang Dashboard và Risk Center với/không chọn cluster.
