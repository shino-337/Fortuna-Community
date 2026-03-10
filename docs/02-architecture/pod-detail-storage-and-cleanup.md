# Pod Detail: Lưu trữ và xóa dữ liệu (Process, Runtime Metrics)

## 1. Process (pod_processes) – **có lưu history + retention 30 ngày**

- **Cách lưu:** Mỗi lần agent gửi `POST /api/v1/pod-processes`, Core **chỉ insert** thêm snapshot mới (cùng `observed_at` cho cả batch), **không xóa** snapshot cũ.
- **Kết quả:** Nhiều snapshot theo thời gian cho mỗi pod; API GET trả về **snapshot mới nhất** (theo `MAX(observed_at)`).
- **Giới hạn kích thước** (tránh làm đầy DB):
  - `command`: tối đa **1024** ký tự (cắt bớt khi insert).
  - `binary_path`: tối đa **512** ký tự.
  - `user_name`: tối đa **128** ký tự.
  - `container_name`: tối đa **256** ký tự.
- **Retention:** Job nền (mặc định mỗi 6 giờ) xóa mọi bản ghi có `observed_at` cũ hơn **30 ngày**. Cấu hình qua env `POD_PROCESS_RETENTION_DAYS` (mặc định 30).
- **API GET:** Trả về tối đa 1000 process thuộc **snapshot mới nhất** (cùng `observed_at` lớn nhất cho pod đó).

```text
Agent gửi lần 1 → insert N process (observed_at = T1)
Agent gửi lần 2 → insert M process (observed_at = T2)  → DB có N + M bản ghi
GET /processes → chỉ trả về M process (snapshot tại T2)
Sau 30 ngày → job xóa bản ghi có observed_at < now - 30d
```

---

## 2. Runtime metrics (pod_runtime_metrics) – **có tích lũy theo thời gian**

- **Cách lưu:** Mỗi lần agent gửi `POST /api/v1/pod-runtime-metrics`, Core **chỉ insert** thêm bản ghi, **không xóa** bản ghi cũ.
- **Kết quả:** Nhiều dòng theo thời gian cho cùng một pod.
- **API GET:** Trả về tối đa 100 bản ghi mới nhất (`ORDER BY last_observed_at DESC LIMIT 100`).

---

## 3. Khi pod bị xóa hoặc deploy mới

Khi Core soft-delete pod (sync không còn thấy pod đó), Core **xóa luôn** mọi bản ghi trong `pod_processes`, `pod_runtime_metrics`, `pod_network_connections` có `pod_uid` tương ứng. Pod mới (UID mới) sẽ có dữ liệu mới khi agent gửi.

---

## 4. Tóm tắt

| Bảng                     | Lưu history? | Retention / giới hạn | Khi pod bị xóa |
|--------------------------|-------------|----------------------|----------------|
| pod_processes            | Có (nhiều snapshot) | 30 ngày (POD_PROCESS_RETENTION_DAYS), field truncate | Xóa theo pod_uid |
| pod_runtime_metrics      | Có          | Không TTL (GET limit 100) | Xóa theo pod_uid |
| pod_network_connections  | Không (replace) | — | Xóa theo pod_uid |

---

## 5. Cấu hình Core (env)

- `POD_PROCESS_RETENTION_DAYS`: số ngày giữ process history (mặc định 30). Job chạy mỗi 6 giờ.
