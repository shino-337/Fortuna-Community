# Rà soát: Nguồn normalized event "pod deleted" và nguyên nhân SBOM bị xóa khi pod vẫn chạy

## 1. Luồng thực tế liên quan tới "pod deleted" và xóa SBOM

### 1.1 Ai publish event "pod deleted" lên NATS?

Trong codebase hiện tại:

- **Agent:** Gửi inventory qua **HTTP sync** (POST /api/v1/agent/sync) với **danh sách pod đầy đủ** (cluster-wide, `Pods(namespace).List`). **Không** gửi từng event Added/Modified/Deleted lên NATS. `StreamInventory` gRPC là **no-op** (Core không expose, Agent dùng HTTP syncer).
- **Core:** Khi xử lý sync (processSyncedPods), chỉ **soft-delete pod trong DB** khi pod `uid NOT IN syncedUIDs` (full sync) hoặc khi cleanupStalePods / PodCleanupJob. **Core không publish** message lên subject `fortuna.normalized.*`.
- **NormalizerWorker** đã bị gỡ (“normalization done in handlers”), không còn worker nào đọc `fortuna.raw.*` rồi ghi `fortuna.normalized.*`.

**Kết luận:** Trong kiến trúc hiện tại **không có nguồn nào** publish normalized event với `event_type: "Deleted"` lên NATS. CorrelatorWorker **chỉ** xóa SBOM khi nhận message có `event_type == "Deleted"`; vì không ai publish nên **SBOM không bị xóa bởi CorrelatorWorker** trong flow hiện tại.

### 1.2 Vậy SBOM bị xóa ở đâu?

Có **hai** chỗ trong Core có thể soft-delete SBOM theo `pod_uid`:

| Nơi | Điều kiện | File |
|-----|-----------|------|
| **CorrelatorWorker** | Nhận normalized message `kind=Pod`, `event_type=Deleted` → soft-delete pod + SBOM, … | `core/pkg/worker/correlator_worker.go` |
| **SBOMReconciler** | SBOM có `pod_uid` mà **không nằm trong** tập pod có `deleted_at IS NULL` → coi SBOM là “orphaned” và soft-delete | `core/pkg/reconciler/sbom_reconciler.go` → `cleanupOrphanedSBOMs` |

Vì không có nguồn publish "Deleted" lên NATS, **nguyên nhân thực tế** khiến SBOM mất (ví dụ pod dashboard UID `e945678b-e332-4bd9-9a7a-8683e560b3b3`) là **SBOMReconciler**.

---

## 2. SBOMReconciler – logic xóa SBOM “orphaned”

### 2.1 Logic hiện tại

- Reconcile chạy **ngay khi Start()** và sau đó mỗi **1 giờ** (reconcileInterval).
- `cleanupOrphanedSBOMs`:
  1. Lấy tất cả SBOM còn active (`deleted_at IS NULL`).
  2. Thu thập `pod_uid` từ các SBOM đó.
  3. Trong DB, lấy danh sách pod **còn active**: `pods WHERE uid IN (...pod_uids...) AND deleted_at IS NULL`.
  4. SBOM nào có `pod_uid` **không** nằm trong danh sách pod active → coi là **orphaned** → soft-delete SBOM.

Tức là: “pod không còn trong bảng pods (active)” → xóa SBOM. **Không phân biệt** hai trường hợp:

- Pod đã bị soft-delete (đúng là nên xóa SBOM).
- Pod **chưa từng xuất hiện** trong bảng `pods` (sync pod tới sau SBOM) → SBOM bị xóa nhầm.

### 2.2 Race thực tế (pod dashboard)

Thứ tự thời gian hợp lý:

1. **07:16:34** – Agent gửi SBOM cho pod (UID `e945678b-...`) qua gRPC SendSBOMFinding → Core lưu SBOM (pod_uid = UID đó).
2. **07:16:34–07:16:37** – Bản ghi **pod** tương ứng **chưa có** trong bảng `pods` (sync HTTP chưa chạy hoặc chưa chứa pod này).
3. **07:16:37** – SBOMReconciler chạy (lần đầu khi Start() hoặc tick 1h). `cleanupOrphanedSBOMs` thấy: SBOM có `pod_uid = e945678b-...`, nhưng trong `pods WHERE deleted_at IS NULL` **không** có UID này → coi SBOM orphaned → **soft-delete SBOM**.
4. **07:21:04** – Sync pod tới, Core tạo bản ghi pod (cùng UID) trong `pods`. Pod hiện “đang chạy” trong DB nhưng SBOM đã bị xóa ở bước 3.

Kết luận: **SBOM bị xóa vì “pod chưa có trong DB” (race sync pod vs SBOM), không phải vì pod bị xóa.**

---

## 3. Các đường soft-delete pod (không trực tiếp xóa SBOM)

Core soft-delete pod ở:

- **processSyncedPods (full sync):** `uid NOT IN syncedUIDs` → soft-delete pod; chỉ xóa PodProcess, PodRuntimeMetrics, PodNetworkConnection, **không** xóa SBOM.
- **cleanupStalePods:** pod có `updated_at` quá cũ → soft-delete pod (không đụng SBOM).
- **PodCleanupJob:** pod còn trong DB nhưng không còn trong K8s (ghost) → soft-delete pod (không đụng SBOM).

SBOM chỉ bị xóa khi:

- CorrelatorWorker nhận normalized `event_type=Deleted` (hiện không xảy ra), hoặc
- **SBOMReconciler** coi SBOM là orphaned (pod không nằm trong tập pod active).

---

## 4. Đề xuất sửa

### 4.1 SBOMReconciler (quan trọng nhất)

**Vấn đề:** Coi mọi SBOM có `pod_uid` không nằm trong “pods active” là orphaned → xóa. Điều này sai khi pod chưa được sync (SBOM tới trước pod).

**Hướng sửa:**

- Chỉ coi SBOM là **orphaned** khi **pod đã tồn tại trong DB và đã bị soft-delete** (có bản ghi trong `pods` với `deleted_at IS NOT NULL` cho đúng `uid`).
- Hoặc: nếu pod **chưa có** trong bảng `pods`, **không** xóa SBOM ngay; chỉ xóa nếu SBOM đã “quá cũ” (ví dụ `created_at` hoặc `updated_at` > 1–2 giờ) và vẫn không có pod → tránh race sync.

Cụ thể có thể:

- Trong `cleanupOrphanedSBOMs`: với mỗi `pod_uid` của SBOM, kiểm tra `pods` (kể cả soft-deleted: `Unscoped()`). Nếu có bản ghi pod với `deleted_at IS NOT NULL` → pod đã bị xóa → cho phép xóa SBOM. Nếu **không** có bản ghi pod nào cho `uid` đó → coi là “pod chưa sync”, **không** xóa SBOM (hoặc chỉ xóa nếu SBOM cũ hơn ngưỡng grace, ví dụ 2 giờ).

### 4.2 Đồng bộ xóa SBOM khi xóa pod (tùy chọn)

Để nhất quán và giảm phụ thuộc reconciler: khi Core **chủ động** soft-delete pod (processSyncedPods, cleanupStalePods, PodCleanupJob), có thể **ngay tại chỗ** soft-delete luôn SBOM theo `pod_uid` (và nếu cần, CVE/insight liên quan). Như vậy “pod deleted” trong DB được phản ánh ngay; SBOMReconciler chỉ cần xử lý edge case (ví dụ pod xóa bằng tay trong DB).

### 4.3 Normalized event "pod deleted" (tương lai)

Nếu sau này bật lại luồng event (ví dụ Agent hoặc Core publish lên `fortuna.normalized.*` với `event_type: Deleted`):

- Cần đảm bảo chỉ gửi Deleted khi pod **thực sự** đã biến mất trên cluster (hoặc đã bị xóa trong nguồn tin cậy).
- CorrelatorWorker khi xóa SBOM theo event Deleted nên vẫn nhất quán với DB (ví dụ chỉ xóa SBOM nếu pod trong DB cũng đã deleted hoặc không còn tồn tại).

---

## 5. Tóm tắt

| Câu hỏi | Trả lời |
|--------|---------|
| Nguồn normalized "pod deleted"? | Hiện **không có**. Không component nào publish `event_type: Deleted` lên `fortuna.normalized.*`. |
| SBOM bị xóa bởi ai? | **SBOMReconciler** (cleanupOrphanedSBOMs), do coi SBOM “orphaned” khi `pod_uid` không nằm trong tập pod active. |
| Vì sao pod vẫn chạy mà SBOM mất? | **Race:** SBOM được lưu trước khi bản ghi pod xuất hiện trong DB. Reconcile chạy, thấy “không có pod active” cho `pod_uid` đó → xóa SBOM. Sau đó sync pod mới tạo bản ghi pod. |
| Sửa chính | Thay đổi SBOMReconciler: chỉ xóa SBOM khi pod **đã có trong DB và đã bị soft-delete**, hoặc thêm grace period cho trường hợp pod chưa sync. |

---

## 6. Đã sửa trong code (SBOMReconciler)

- **File:** `core/pkg/reconciler/sbom_reconciler.go`
- **Thay đổi:** `cleanupOrphanedSBOMs` chỉ coi SBOM là orphaned khi:
  1. **Pod đã có trong DB và đã bị soft-delete** → xóa SBOM (đúng hành vi).
  2. **Pod không có trong DB** → chỉ xóa SBOM nếu SBOM **cũ hơn 2 giờ** (`orphanGracePeriod`), tránh race SBOM tới trước pod sync.
- Reconcile vẫn chạy ngay khi Start() và mỗi 1 giờ; SBOM mới (pod chưa sync) sẽ không bị xóa trong 2 giờ đầu.

Tài liệu này mô tả đúng với mã nguồn tại thời điểm rà soát; khi thay đổi luồng sync hoặc bật lại NormalizerWorker / event Deleted cần cập nhật lại.
