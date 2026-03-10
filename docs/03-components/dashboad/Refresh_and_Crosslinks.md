# Refresh và Cross-links giữa các trang

*Cập nhật: 2026-02-02*

## 1. Time window (thời gian gần nhất) – Risks & Runtime signals

**Khác với refresh interval.** Đây là bộ lọc **chỉ hiển thị dữ liệu trong khoảng thời gian gần nhất**:

- **All:** Hiển thị tất cả (không lọc theo thời gian).
- **Last 5m / 10m / 15m / 30m:** Chỉ hiển thị risks (insights) và runtime signals có thời gian **trong vòng N phút gần nhất**; dữ liệu cũ hơn N phút **không hiển thị**.

### Áp dụng

- **Risk Center:** Selector "Time window" (All, Last 5m, 10m, 15m, 30m) ở đầu trang. Risks và Runtime signals block đều dùng `sinceMinutes` gửi lên API.
- **Reference tab (Runtime Signals):** Bảng runtime signals dùng cùng time window (store) → chỉ signal có `created_at >= now - Nm`.
- **Risk Detail:** Runtime / Escape signals của pod bị ảnh hưởng cũng lọc theo time window đã chọn.

### Backend

- **GET /risks:** Query `sinceMinutes` (int). Khi set: `detected_at >= now() - sinceMinutes`.
- **GET /runtime-signals:** Query `sinceMinutes` (int). Khi set: `created_at >= now() - sinceMinutes`.
- **GET /runtime-signals/pods/:podUid:** Query `sinceMinutes` tương tự.

### Store

- **timeWindowStore:** `valueMinutes` (0 = All, 5/10/15/30). Persist localStorage key `fortuna-time-window`. Mặc định 15.

---

## 2. Cơ chế refresh (auto-refresh)

### Mặc định và lựa chọn

- **Mặc định:** Dữ liệu tự làm mới mỗi **5 phút**.
- **Lựa chọn:** Người dùng chọn khoảng refresh trong header (Refresh dropdown):
  - Off (không auto-refresh)
  - 30s, 1 min
  - **5 min**, **10 min**, **15 min**
  - 30 min
- Lựa chọn được lưu (localStorage) và áp dụng cho tất cả trang có dữ liệu.
- **Muốn xem thêm / thu hẹp dữ liệu:** Dùng **filter** trên từng trang (Severity, Status, Search, Cluster, v.v.).

### Trang áp dụng

| Trang | Dữ liệu refresh |
|-------|------------------|
| Dashboard | Stats, clusters, risks, notifications, threat velocity, PCE trend |
| Clusters | Danh sách cluster |
| Resources | Pods, ServiceAccounts, Roles, RoleBindings |
| Risk Center | Risks, PCE summary, runtime signals |
| SBOM | Danh sách SBOM |
| Monitoring (Metrics) | Metrics |

### Kỹ thuật

- **Store:** `refreshIntervalStore` (Zustand + persist) – một giá trị `intervalMs` toàn cục.
- **Selector:** `RefreshIntervalSelector` trong Layout header – dropdown "Refresh: 5 min".
- **Hook:** `usePolling(fetchFn, intervalMs)` – gọi `fetchFn` lúc mount và mỗi `intervalMs`. Khi `intervalMs === 0` (Off) thì không chạy interval.

---

## 3. Cross-links giữa các trang

### Risk Center ↔ Resources / Identities

- **Trong bảng Risk Center (cột Assets):**
  - Nếu risk có đúng **1 Pod:** hiển thị link **"1 Pod →"** → click mở **Resources → Pod Detail** (`/resources/pods/uid/:uid`).
  - Nếu risk có đúng **1 ServiceAccount:** hiển thị link **"1 ServiceAccount →"** → click mở **Identity Detail** (`/identities/uid/:uid`).
- **Trong Risk Detail (Affected Assets):**
  - Nút **"View pod"** → `/resources/pods/uid/:uid` (Pod Detail).
  - Nút **"View identity"** → `/identities/uid/:uid` (Identity Detail).

### Các link khác (đã có)

- Dashboard severity card → Risk Center (pre-filter `?severity=...`).
- Risk Center row click → Risk Detail (`/risks/:id`).
- Risk Detail "Back to Risk Center" → `/risks`.
- Resources Pod row / View → Pod Detail (`/resources/pods/:id` hoặc `/resources/pods/uid/:uid`).
- Cluster Detail → Node Detail; Node Detail → Pod Detail.
- Capabilities, Rules, Attack Paths, v.v. có link nội bộ theo từng trang.

### Route Pod Detail

- **By UID (từ Risk, link chéo):** `/resources/pods/uid/:uid` – dùng khi chỉ có pod UID (vd. từ insight `resource_uid`).
- **By ID (số):** `/resources/pods/:id` – dùng khi có pod id (vd. từ Resources list).

Cả hai route đều render `PodDetail`; component đọc `uid` hoặc `id` từ URL và gọi `getPodByUid(uid)` hoặc `getPod(id)` tương ứng.
