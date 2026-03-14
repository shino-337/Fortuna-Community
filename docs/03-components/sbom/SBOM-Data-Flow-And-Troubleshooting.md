# SBOM Data Flow & Troubleshooting

## API truy vấn SBOM (đồng nhất theo pod UID)

- **GET /api/v1/sbom** – Danh sách SBOM (phân trang, lọc theo `podName`, `namespace`). Mỗi item trả về `podId` = **pod UID** (K8s uid) dùng cho chi tiết.
- **GET /api/v1/sbom/:podUid** – Chi tiết SBOM theo **pod UID** (K8s uid). Luôn dùng UID, không dùng Core pod id (số). Dashboard: Pod Detail gọi `getPodSbom(pod.uid)`, trang SBOM gọi `getPodSbom(pod.podId)` với `podId` từ list = pod UID.

Khi không có SBOM cho pod, API trả **404** với body có `error`, `podUid` và `hint` gợi ý kiểm tra pipeline.

---

## Luồng dữ liệu (end-to-end)

```
[Agent trên node]
  Pod Watcher (informer) → phát hiện pod mới/cập nhật
       ↓
  SBOM Work Queue (async, mặc định 2 workers)
       ↓
  SBOM Processor: ExtractSBOM (image) → convertToProto (pod.UID, pod.Name, ...)
       ↓
  gRPC SendSBOMFinding → Core
       ↓
[Core]
  handler_sbom.SendSBOMFinding: upsert sboms (theo pod_uid) + sbom_components
       ↓
  NATS Publish "fortuna.sbom.created" → CVE Matcher Worker → cve_matches + insights
       ↓
[API]
  GET /api/v1/sbom/:podId → tra sboms theo pod_uid (hoặc resolve pod id → uid)
```

- **Bảng DB:** `sboms` (pod_uid, image_*, package_count, ...), `sbom_components`, `cve_matches`.
- Agent gửi **pod_uid = string(pod.UID)** (K8s UID). Core lưu đúng vào `sboms.pod_uid`. Dashboard Pod Detail gọi `getPodSbom(pod.uid)` nên khớp với dữ liệu trong DB.

---

## Không truy vấn được SBOM (404 / list rỗng)

1. **Kiểm tra có bản ghi SBOM trong DB**
   - Trên DB: `SELECT id, pod_uid, pod_name, namespace, created_at FROM sboms WHERE deleted_at IS NULL LIMIT 20;`
   - Nếu rỗng → SBOM chưa bao giờ được Core lưu (xem bước 2–3).

2. **Kiểm tra Core đã nhận SBOM từ Agent**
   - Log Core: `[SBOM] Received SBOM from agent=...`, `[SBOM] Created new SBOM id=...` hoặc `Updated existing SBOM id=...`.
   - Nếu không thấy → Agent chưa gửi hoặc gRPC lỗi (mạng, TLS, Core down).

3. **Kiểm tra Agent đã gửi SBOM**
   - Log Agent: `[SBOMProcessor]`, `Extracting SBOM`, `SBOM sent to Core`, `SendSBOMFinding`.
   - Pod phải chạy trên **node có Agent**; Agent chỉ xử lý pod trên node local.
   - SBOM queue có thể backlog (2 workers, ~2–3 phút/pod) → đợi hoặc tăng `SBOM_WORKERS`.

4. **Đúng pod nhưng vẫn 404**
   - Xác nhận Dashboard/API gửi đúng **pod UID** (UUID) hoặc **pod id** (số). Nếu gửi sai định dạng (ví dụ nhầm id cluster), resolve có thể ra pod khác hoặc không tìm thấy.
   - Kiểm tra pod có bị xóa (soft-delete): `SELECT * FROM sboms WHERE pod_uid = '<uid>';` xem `deleted_at`.

5. **Sau khi xóa pod**
   - CorrelatorWorker sẽ soft-delete SBOM theo `pod_uid`. GET /sbom/:podId đúng UID sẽ 404 (đúng hành vi).

---

## Tài liệu liên quan

- [README](README.md) – Tổng quan SBOM, extractor, cache.
- [ASYNC_SBOM_QUEUE_IMPLEMENTATION.md](ASYNC_SBOM_QUEUE_IMPLEMENTATION.md) – Hàng đợi và worker Agent.
- Core: `internal/grpc/handler_sbom.go`, `internal/api/sbom_handlers.go`.
