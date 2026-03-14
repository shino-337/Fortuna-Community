# Kết quả kiểm tra SBOM – pod_uid e945678b-e332-4bd9-9a7a-8683e560b3b3

**Thời gian kiểm tra:** 2026-03-12 (chạy script `scripts/verify/verify-sbom-pod.sh`).

---

## Kết luận: Nguyên nhân đã xác định

**SBOM không load được vì bản ghi SBOM cho pod này đã bị soft-delete (deleted_at IS NOT NULL).**

API `GET /api/v1/inventory/pods/:uid/sbom` chỉ lấy bản ghi có `deleted_at IS NULL`, nên trả 404 mặc dù row vẫn tồn tại trong DB.

---

## Chi tiết kiểm tra

### 1. Bảng `sboms`

| Cột           | Giá trị |
|---------------|---------|
| id            | 23      |
| pod_uid       | e945678b-e332-4bd9-9a7a-8683e560b3b3 |
| pod_name      | fortuna-dashboard-679f56db64-8dd2b |
| namespace     | fortuna |
| container_name| dashboard |
| package_count | 66      |
| created_at    | 2026-03-12 07:16:34.170926+00 |
| **deleted_at**| **2026-03-12 07:16:37.33738+00** |

→ SBOM đã được Agent gửi và Core lưu lúc 07:16:34, nhưng bị **soft-delete** chỉ ~3 giây sau (07:16:37).

### 2. Bảng `pods`

Pod tồn tại trong Core với cùng UID, name `fortuna-dashboard-679f56db64-8dd2b`, namespace `fortuna`, node `k8s-master`, `deleted_at` = NULL.

### 3. Pod trên cluster

- `kubectl get pod -n fortuna fortuna-dashboard-679f56db64-8dd2b -o jsonpath='{.metadata.uid}'` → **e945678b-e332-4bd9-9a7a-8683e560b3b3** (trùng với DB).
- Pod vẫn đang chạy trên node **k8s-master**; có Agent trên node này (fortuna-agent-9s9tf).

→ Pod chưa bị xóa rồi tạo lại (UID giữ nguyên). SBOM bị xóa trong khi pod vẫn tồn tại.

### 4. Cơ chế soft-delete SBOM trong Core

Trong `core/pkg/worker/correlator_worker.go`, khi xử lý event **pod deleted** (normalized message), worker gọi:

```go
db.Where("pod_uid = ?", uid).Delete(&models.SBOM{})
```

GORM dùng soft-delete (cột `deleted_at`), nên row SBOM không bị xóa vật lý mà chỉ set `deleted_at`.

**Nguyên nhân khả dĩ:** CorrelatorWorker đã nhận một message normalized coi pod này là **deleted** (ví dụ: event delete nhầm, trùng UID, hoặc thời điểm ngắn pod bị coi là deleted rồi lại có lại). Sau đó pod vẫn tồn tại (cùng UID) nhưng SBOM đã bị đánh dấu xóa.

---

## Cách xử lý

### Cách 1: Khôi phục SBOM (khuyến nghị nếu muốn xem lại SBOM cũ)

Chạy trên DB Core:

```sql
UPDATE sboms
SET deleted_at = NULL
WHERE pod_uid = 'e945678b-e332-4bd9-9a7a-8683e560b3b3' AND deleted_at IS NOT NULL;
```

Sau đó gọi lại `GET /api/v1/inventory/pods/e945678b-e332-4bd9-9a7a-8683e560b3b3/sbom` → sẽ trả 200 và dữ liệu SBOM.

### Cách 2: Để Agent gửi lại SBOM

- Restart pod dashboard để pod bị xóa rồi tạo lại (UID mới) → Agent sẽ queue và gửi SBOM cho pod mới.
- Hoặc thêm cơ chế “request rescan” (nếu có) để Agent gửi lại SBOM cho cùng pod_uid mà không cần xóa pod.

### Cách 3: Tránh xóa nhầm về sau

- Rà soát nguồn normalized event “pod deleted”: đảm bảo chỉ gửi khi pod thực sự bị xóa trên cluster.
- Có thể bổ sung: trước khi soft-delete SBOM, kiểm tra pod còn tồn tại trong bảng `pods` (hoặc còn trên cluster) hay không; nếu vẫn còn thì không xóa SBOM (hoặc chỉ xóa khi có thêm bằng chứng pod đã terminated).

---

## Script đã dùng

- `scripts/verify/verify-sbom-pod.sh e945678b-e332-4bd9-9a7a-8683e560b3b3`
- Cần cập nhật label Agent trong script: dùng `app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent` thay cho `app=fortuna-agent`.
