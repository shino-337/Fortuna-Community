# Số lượng Pod: API so với thực tế và khắc phục

## 1. Nguồn dữ liệu

- **Thực tế (cluster):** `kubectl get pods -A --no-headers | wc -l` — tổng số pod đang có trong cluster (mọi namespace).
- **API / DB:** Bảng `pods` do **Agent sync** điền (mỗi agent gửi danh sách pod của cluster qua sync). Số pod API “ghi nhận” = số bản ghi pod trong DB (đã loại soft-deleted).

**Định nghĩa thống nhất:** Một pod = một **UID** duy nhất. Mọi chỗ đếm pod phải dùng **COUNT(DISTINCT uid) FROM pods WHERE deleted_at IS NULL** để tránh trùng và khớp với thực tế (một pod một UID).

## 2. So sánh (kiểm tra 2026-02-24)

| Nguồn | Cách đếm | Kết quả |
|-------|----------|--------|
| **Cluster (thực tế)** | `kubectl get pods -A \| wc -l` | **1859** |
| **DB** | `SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL` | **1859** |
| **DB** | `SELECT COUNT(DISTINCT uid) FROM pods WHERE deleted_at IS NULL` | **1859** |

→ Hiện tại không có duplicate UID; số lượng API và thực tế khớp.

## 3. Điểm lỗi đã xử lý (triệt để)

### 3.1 Dashboard data integrity (`/health/dashboard-data-integrity`)

- **Trước:** `db.Table("pods").Where("deleted_at IS NULL").Count(&PodsCount)` → **COUNT(*)**. Nếu sau này có duplicate row (cùng uid), số sẽ **lớn hơn** thực tế.
- **Sau:** `db.Raw("SELECT COUNT(DISTINCT uid) FROM pods WHERE deleted_at IS NULL").Scan(&PodsCount)` → luôn đếm theo **distinct UID**, khớp dashboard và cluster.

### 3.2 Insights summary (diagnostic log)

- **Trước:** Diagnostic dùng `COUNT(*)` cho pod trong cluster.
- **Sau:** Đổi thành `COUNT(DISTINCT uid)` cho thống nhất với mọi API đếm pod.

### 3.3 Các API đã đúng từ trước

- **Dashboard stats** (GetDashboardStats): `SELECT COUNT(DISTINCT uid) FROM pods WHERE deleted_at IS NULL` (và theo cluster_id khi có).
- **GetClustersStats / GetClusterOverview:** `COUNT(DISTINCT uid)` theo cluster.
- **GetSystemMetrics:** `db.Model(&models.Pod{}).Distinct("uid").Count(&podCount)` → COUNT(DISTINCT uid).
- **GetResources:** Trả về danh sách pod từ `Model(&Pod{}).Find()` (đã có soft-delete); bảng `pods` có UNIQUE(uid) nên mỗi uid một dòng → `total` = số pod đúng.

## 4. Ràng buộc DB

- Bảng `pods` có **UNIQUE INDEX trên `uid`** (`pods_uid_key`) → trong DB không thể có hai dòng cùng uid (kể cả khác cluster). Số pod = số dòng (với deleted_at IS NULL) = số UID phân biệt.
- Sync agent có bước cleanup duplicate (soft-delete bản cũ) để chỉ giữ một bản ghi mới nhất theo (cluster_id, uid).

## 5. Kết luận

- **Nguyên tắc:** Mọi chỗ hiển thị hoặc kiểm tra “số pod” đều dùng **COUNT(DISTINCT uid)** và **deleted_at IS NULL**.
- **Đã sửa:** Dashboard data integrity + insights diagnostic dùng đếm theo distinct UID.
- **Kết quả:** Số lượng pod API ghi nhận thống nhất với nhau và so với thực tế cluster; nếu sau này có lệch (ví dụ agent không sync hết), so sánh `GET /health/dashboard-data-integrity` (podsCount) với `kubectl get pods -A | wc -l` sẽ thấy ngay.
