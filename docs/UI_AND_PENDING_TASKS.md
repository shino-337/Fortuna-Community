# Kiểm tra giao diện và công việc tồn đọng

*Cập nhật: 2026-02-02. Danh sách task tổng hợp: **docs/PENDING_TASKS_SUMMARY.md**.*

---

## 1. Tổng quan giao diện (Dashboard UI)

### 1.1 Cấu trúc điều hướng (Layout)

- **Sidebar (trái):** Logo Fortuna, menu chính, user profile, Sign Out.
- **Header:** Tiêu đề trang, ô tìm kiếm (Global search), Refresh interval, chuông Notifications, badge Production.
- **Nội dung:** `max-w-7xl`, padding đồng nhất, scroll độc lập.

### 1.2 Các trang (routes)

| Route | Trang | Ghi chú |
|-------|--------|--------|
| `/` | Dashboard | Tổng quan: clusters, risks, pods, agents; biểu đồ threat velocity & PCE; top risks, notifications. |
| `/risks` | Risk Center (Insights) | Danh sách insights (vulnerability), filter, pagination. |
| `/capabilities` | Capabilities | Metadata capability, state chart. |
| `/sbom` | SBOM Analysis | Danh sách pod + chi tiết SBOM, scroll riêng 1/3–2/3. |
| `/attack-paths` | Attack Paths | Attack steps, timeline. |
| `/clusters` | Clusters | Danh sách cluster, PageLayout + Pagination. |
| `/resources` | Resources | Tài nguyên K8s, PageLayout + Pagination. |
| `/rules` | Rules | Quản lý rules. |
| `/certificates` | Certificates | Cert + rotation history (tab). |
| `/reports` | Reports | Báo cáo. |
| `/monitoring` | Monitoring | Metrics, workers, queue, error logs. |
| `/audit` | Audit Logs | Audit logs, PageLayout + Pagination. |
| `/notifications` | Notifications | Lịch sử thông báo, tabs: History / Channels / Rules. |
| `/settings` | Settings | Tabs: General, Users, API Keys, etc. |
| `/login` | Login | Đăng nhập (admin / admin123). |

Redirect: `/insights` → `/risks`, `/metrics` → `/monitoring`.

### 1.3 Component dùng chung

- **PageLayout:** Tiêu đề, mô tả, actions (nút).
- **Pagination:** Phân trang client-side cho bảng/danh sách.
- **StatCard:** Thẻ số liệu (title, value, icon, optional subtitle).
- **Card, Button:** UI cơ bản.
- **RefreshIntervalSelector:** Chọn khoảng refresh (Dashboard, Risk Center, …).

### 1.4 Luồng dữ liệu

- **API base:** `VITE_CORE_API_URL` hoặc proxy `/api/v1` (Nginx trong pod dashboard).
- **Auth:** JWT từ `/auth/login`, header `Authorization: Bearer <token>`; 401 → logout và redirect về `/login`.
- **Dashboard:** `getStats()`, `getClusters()`, `getRisks()`, `getNotifications()`, `getThreatVelocity()`, `getPceSummaryByCapability()`, `getPceTrend()`.

---

## 2. Công việc tồn đọng (Backend / API)

### 2.1 API chưa có hoặc đang stub

| API | Trạng thái Core | Trạng thái Dashboard |
|-----|------------------|----------------------|
| `GET /api/v1/users` | Chưa có route | `getUsers()` gọi API, catch lỗi → trả về `[]` |
| `GET /api/v1/notifications` | Có route, stub `{ notifications: [], total: 0 }` | Layout (unread count), Notifications trang, Dashboard widget |
| `GET /api/v1/error-logs` | Có route, trả về unsupported | Monitoring gọi `getErrorLogs()` → `[]` |
| `GET /api/v1/certificates/rotation/history` | Chưa có | Certificates tab gọi `getRotationHistory()` → `[]` |

### 2.2 Core – TODO (từ docs/TODO_LIST.md)

- **GetWorkerMetrics:** Thay hardcode 3/3 bằng dữ liệu thật (NATS/Prometheus) hoặc "unknown".
- **GetQueueMetrics:** Hiện 0,0,0; thay bằng queue depth thật hoặc ghi rõ.
- **API Latency:** GetSystemMetrics có avgLatency hardcode "234ms" → metrics thật hoặc bỏ.
- **GetNotifications:** Khi có bảng `notifications` → trả về từ DB.
- **GET /users:** Implement khi có bảng users.
- **GET /certificates/rotation/history:** Implement khi có bảng rotation history.
- **Error logs API:** Endpoint lấy error logs (bảng hoặc log aggregation).

### 2.3 Database migrations – TODO

- Bảng **notifications** (nếu product cần).
- Bảng **rotation_history** (nếu cần lịch sử xoay cert).
- Bảng/view **error_logs** (nếu API cần).

### 2.4 Dashboard – TODO trong code

- `api.ts`: getUsers, getRotationHistory – comment TODO khi endpoint chưa có.
- `CapabilityStateChart.tsx`: state history – "TODO: Implement API endpoint to get state history".

### 2.5 Đã xong (đồng bộ Dashboard ↔ Core)

- Dashboard stats: clusters, risks, pods, agents, resolved24h từ Core.
- Risk Center: `/risks` mặc định `insight_type=vulnerability` đồng bộ với "Security Risks" trên Dashboard.
- Agents: Core dùng bảng `agents`; Dashboard hiển thị từ API.
- Resolved (24h): từ `api.getStats().resolved24h`.
- Workers / Queue / API Latency: hiển thị "—" hoặc giá trị từ API khi có.

---

## 3. Kiểm tra nhanh giao diện (manual)

1. **Port-forward (nếu test local):**
   ```bash
   kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
   kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80 &
   ```
2. Mở **http://localhost:8081** (hoặc URL dashboard).
3. **Login:** admin / admin123.
4. **Cần xác nhận:**
   - Dashboard: 4 thẻ số (Clusters, Security Risks, Pods, Agents), biểu đồ, danh sách risk/notifications.
   - Risk Center: có bản ghi, filter, phân trang.
   - Clusters, Resources, SBOM, Audit: có bảng/danh sách, scroll, phân trang.
   - Notifications: tab History/Channels/Rules; History có thể trống (API stub).
   - Settings → Users: có thể trống (chưa có GET /users).
   - Certificates → Rotation history: có thể trống (chưa có API).
   - Monitoring → Error logs: có thể trống (API unsupported).
5. **401:** Đăng xuất hoặc hết phiên → redirect về `/login`, không hiển thị dữ liệu.

---

## 4. Tóm tắt

- **Giao diện:** Cấu trúc rõ (Layout, PageLayout, Pagination), đủ trang chính và đồng bộ với API hiện có.
- **Tồn đọng:** Chủ yếu ở Core (users, notifications thật, error-logs, rotation history, metrics thật) và DB (migrations cho notifications, rotation_history, error_logs). Dashboard đã xử lý fallback (mảng rỗng / "—") cho các API chưa sẵn sàng.
