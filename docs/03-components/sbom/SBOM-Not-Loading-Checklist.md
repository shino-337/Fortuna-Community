# SBOM không load được – Checklist kiểm tra

Khi API trả lỗi:

```json
{
  "error": "sbom not found for pod",
  "hint": "Ensure Agent has sent SBOM for this pod (pod watcher → SBOM queue → Core gRPC SendSBOMFinding).",
  "podUid": "e945678b-e332-4bd9-9a7a-8683e560b3b3"
}
```

Nghĩa là **Core không có bản ghi SBOM nào** trong bảng `sboms` với `pod_uid = <podUid>` và `deleted_at IS NULL`. Luồng tạo SBOM hoàn toàn từ **Agent → gRPC SendSBOMFinding → Core**. Dưới đây là các nguyên nhân có thể và cách kiểm tra.

---

## 1. Luồng tạo SBOM (tóm tắt)

| Bước | Thành phần | Điều kiện / Hành động |
|------|------------|------------------------|
| 1 | Agent: Pod Watcher | Chỉ watch pod có `spec.nodeName = NODE_NAME` (node mà Agent chạy). Pod phải **Running** và có ít nhất 1 container. |
| 2 | Agent: Enqueue | Pod được đưa vào SBOM queue (channel buffer **30**). Nếu queue đầy → pod bị **bỏ qua** (log "Queue full, dropping pod" hoặc "Queue full during initial sync, skipping pod"). |
| 3 | Agent: Worker | Worker lấy pod từ queue, gọi `ProcessPod` → với **từng container** gọi `ExtractSBOM(image)` rồi `SendSBOMFinding(ctx, sbomFinding)`. `sbomFinding.PodUid = string(pod.UID)`. |
| 4 | Agent: ExtractSBOM | Cần pull được image (local hoặc registry nếu `SBOM_PREFER_REGISTRY=1`). Parse filesystem trong image, chạy parser (dpkg, rpm, npm, pip, …). Nếu lỗi (image không có, layer lỗi, parser panic) → container đó fail, log "SBOM extraction failed". |
| 5 | Agent → Core: gRPC | `SendSBOMFinding` gửi qua gRPC (mTLS). Nếu Core không reachable hoặc TLS/network lỗi → "SendSBOMFinding RPC failed". |
| 6 | Core: handler_sbom | `SendSBOMFinding` upsert `sboms` theo `pod_uid`, insert `sbom_components`. Nếu DB lỗi → trả lỗi cho Agent. |
| 7 | Core: CorrelatorWorker | Khi nhận event **pod deleted** (normalized), Core **soft-delete** sboms theo `pod_uid`. Pod sau khi xóa sẽ không còn SBOM (đúng hành vi). |

**Kết luận:** Để có SBOM cho một pod UID, ít nhất **một Agent trên node chứa pod đó** phải đã chạy qua bước 1→6 thành công cho pod đó. Nếu pod nằm trên node không có Agent, hoặc Agent chưa kịp xử lý, hoặc bị drop/error ở bất kỳ bước nào → Core sẽ không có bản ghi.

---

## 2. Checklist kiểm tra (theo thứ tự)

### Bước A: Xác định pod và node

- Lấy thông tin pod từ cluster (hoặc từ bảng `pods` trong Core nếu đã sync):
  - `kubectl get pod -n <namespace> <name> -o jsonpath='{.metadata.uid}'` → phải trùng `podUid` trong lỗi.
  - `kubectl get pod -n <namespace> <name> -o jsonpath='{.spec.nodeName}'` → **node mà pod đang chạy**.
- Nếu pod đã bị xóa khỏi cluster: UID trong lỗi là UID cũ. Pod mới (recreate) sẽ có **UID mới** → SBOM cũ (nếu có) đã bị Correlator soft-delete, pod mới chưa có SBOM cho đến khi Agent gửi.

### Bước B: Kiểm tra DB Core

Trên DB Core:

```sql
SELECT id, pod_uid, pod_name, namespace, created_at, deleted_at
FROM sboms
WHERE pod_uid = 'e945678b-e332-4bd9-9a7a-8683e560b3b3';
```

- **Không có row:** Agent chưa bao giờ gửi SBOM thành công cho pod UID này (hoặc chưa tới Core).
- **Có row nhưng `deleted_at` khác NULL:** SBOM đã bị xóa (thường do Correlator khi pod bị xóa trên cluster). Pod hiện tại với cùng tên nhưng UID mới sẽ không có row này.

### Bước C: Kiểm tra Agent có chạy trên node của pod không

- Agent chạy dạng DaemonSet, mỗi node một pod. Pod cần SBOM **phải** nằm trên **cùng node** với Agent.
- So sánh:
  - `spec.nodeName` của pod (bước A)
  - Node mà Agent pod đang chạy: `kubectl get pods -n <agent-namespace> -l app=fortuna-agent -o wide` (hoặc label tương ứng).
- Nếu pod nằm trên node **không có** Agent (hoặc Agent chưa start / bị OOMKilled): **sẽ không có SBOM**. Đây là nguyên nhân rất thường gặp.

### Bước D: Kiểm tra log Agent (node có pod)

Trên pod Agent của **đúng node** (node mà pod đang chạy):

- Log khi **đưa pod vào queue:**  
  `[LocalPodWatcher] → Queued pod <namespace>/<name> for async processing` hoặc `... for async processing (transitioned to Running)`.
- Nếu thấy **"Queue full, dropping pod"** hoặc **"Queue full during initial sync, skipping pod"**: pod chưa được xử lý do queue đầy (buffer 30, 2 workers). Giải pháp: tăng `SBOM_WORKERS` (2–4), hoặc đợi queue rỗng rồi restart pod để nó vào queue lại (hoặc chờ lần sync sau).
- Log khi **bắt đầu xử lý:**  
  `[SBOMQueue] [Worker N] Processing pod <namespace>/<name>`.
- Log khi **extract:**  
  `[SBOMProcessor] 🔍 Extracting SBOM: pod=... container=... image=...` rồi `✅ Extracted N packages` hoặc lỗi `SBOM extraction failed for <image>: ...`.
- Log khi **gửi Core:**  
  `✅ SBOM sent to Core: sbom_id=...` hoặc `SendSBOMFinding RPC failed`, `Core rejected SBOM: ...`.

Nếu không thấy dòng "Queued pod ..." cho namespace/name của pod:
- Pod chưa bao giờ ở trạng thái Running khi watcher chạy (thêm/update), hoặc
- Đã bị coi là "already processed" (processedPods map), hoặc
- Watcher chưa sync đến pod (informer chỉ watch `spec.nodeName = NODE_NAME`).

### Bước E: Kiểm tra log Core

Trên Core, tìm log SBOM cho đúng `pod_uid`:

- `[SBOM] Received SBOM from agent=... pod=... image=...`
- `[SBOM] Created new SBOM id=... for pod_uid=...` hoặc `Updated existing SBOM id=...`

Nếu **không có** dòng nào với pod name/uid tương ứng: Core chưa nhận được gRPC SendSBOMFinding cho pod đó (Agent chưa gửi hoặc gRPC lỗi).

### Bước F: Các nguyên nhân thường gặp (tóm tắt)

| Nguyên nhân | Triệu chứng | Hướng xử lý |
|-------------|-------------|-------------|
| **Pod không nằm trên node có Agent** | Pod ở node A, Agent chỉ chạy trên node B. | Đảm bảo Agent chạy DaemonSet trên mọi node có workload cần SBOM. |
| **Queue đầy** | Agent log "Queue full, dropping pod" / "Queue full during initial sync, skipping pod". | Tăng `SBOM_WORKERS` (2–4), hoặc tăng buffer trong `queue.go` (hiện 30). Đợi queue rỗng hoặc restart pod để re-queue. |
| **SBOM extraction failed** | Agent log "SBOM extraction failed for <image>: ...". | Kiểm tra image pull (quyền, network, image có tồn tại). Với image minimal/alpine có thể 0 packages → vẫn gửi được SBOM (0 packages). |
| **gRPC lỗi** | Agent log "SendSBOMFinding RPC failed". | Kiểm tra Core reachable, mTLS (cert, CA), network từ node Agent tới Core. |
| **Pod mới (UID mới)** | Pod bị xóa rồi tạo lại → UID mới. SBOM cũ đã bị Correlator xóa. | Bình thường. Đợi Agent xử lý pod mới (khi Running và được queue). |
| **Pod chưa Running khi watcher sync** | AddFunc/UpdateFunc chỉ queue khi `Phase == Running`. | Pod vừa tạo có thể chưa Running; khi chuyển Running, UpdateFunc sẽ queue (nếu chưa "already processed"). |

---

## 3. Cấu hình Agent liên quan

- **SBOM_WORKERS** (default 2): Số worker SBOM. Tăng lên 3–4 nếu node nhiều pod và queue hay đầy.
- **SBOM_PREFER_REGISTRY**: `1` hoặc `true` để lấy image từ registry thay vì local (khi image không có trên node).
- Queue buffer: trong code `agent/internal/sbom/queue.go` là **30** (không config bằng env). Nếu cần có thể tăng và build lại.

---

## 4. API Core trả lỗi

- Handler: `core/internal/api/sbom_handlers.go` → `GetSBOMDetail`.
- Query: `db.Where("pod_uid = ? AND deleted_at IS NULL", podUID).Order("created_at DESC").Limit(1)`.
- Nếu không có row → trả 404 với `error`, `podUid`, `hint` như trên.

Dashboard gọi `GET /api/v1/inventory/pods/:uid/sbom` (route trong `core/internal/api/routes_inventory.go`). Tham số `:uid` **phải là pod UID** (K8s metadata.uid), không phải Core internal id.

---

## 5. Kết luận cho pod UID `e945678b-e332-4bd9-9a7a-8683e560b3b3`

Cần lần lượt:

1. **DB:** `SELECT ... FROM sboms WHERE pod_uid = 'e945678b-e332-4bd9-9a7a-8683e560b3b3'` → xác nhận không có bản ghi (hoặc đã deleted).
2. **Pod và node:** Tìm pod có `metadata.uid = e945678b-e332-4bd9-9a7a-8683e560b3b3` (hoặc xác nhận đây là UID pod đang xem trên Dashboard), lấy `spec.nodeName`.
3. **Agent:** Trên node đó, xem log Agent có "Queued pod ...", "Processing pod ...", "SBOM sent to Core" cho pod đó không; có "Queue full" hay "SBOM extraction failed" hay "SendSBOMFinding RPC failed" không.
4. **Core:** Tìm log `[SBOM] Received ...` / `Created new SBOM ...` với pod_uid tương ứng.

Sau khi xác định được khúc đứt (pod không trên node có Agent / queue full / extraction fail / gRPC fail), áp dụng hướng xử lý tương ứng ở bảng Bước F.

---

## 6. Monitor logs (grep) – Agent và Core

Để nhanh tìm lỗi liên quan SBOM / not found / fail / error:

**Agent (trên pod Agent của node cần kiểm tra):**

```bash
# Lỗi SBOM và gửi Core
kubectl logs -n <agent-namespace> -l app=fortuna-agent --tail=2000 | grep -iE 'SBOM|not found|fail|error|Queue full|Extract|SendSBOM'
```

Chuỗi cần chú ý: `Queue full`, `SBOM extraction failed`, `SendSBOMFinding RPC failed`, `Core rejected SBOM`, `executable file not found`, `content digest ... not found`.

**Core:**

```bash
# SBOM nhận / lưu / lỗi
kubectl logs -n <core-namespace> deployment/fortuna-core --tail=2000 | grep -iE '\[SBOM\]|sbom|not found|fail|error'
```

Chuỗi cần chú ý: `[SBOM] Received SBOM`, `[SBOM] Created new SBOM`, `[SBOM] Failed`, `database error`, `failed to insert`, `failed to update`.

**Kết hợp (tìm mọi “fail”/“error” trong log gần đây):**

```bash
# Agent
kubectl logs -n <agent-namespace> -l app=fortuna-agent --tail=500 | grep -iE 'fail|error'

# Core
kubectl logs -n <core-namespace> deployment/fortuna-core --tail=500 | grep -iE 'fail|error'
```

Nếu pod không có SBOM: so sánh thời gian log Agent (Queued → Extracting → SBOM sent) với log Core (Received SBOM → Created/Updated) cho đúng pod_uid/pod name để xác định đứt ở Agent hay Core.
