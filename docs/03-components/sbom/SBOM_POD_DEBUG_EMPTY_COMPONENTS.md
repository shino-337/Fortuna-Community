# Debug: Pod có SBOM (packageCount > 0) nhưng components rỗng

Khi API trả về `packageCount: 1`, `sbomSource`, `confidence` nhưng `components: []` rỗng, thường do bảng `sbom_components` không có row nào cho `sbom_id` tương ứng.

---

## 1. Query DB trực tiếp

Pod UID: `0f2e43e9-9f91-48ee-b98e-40a0def36902`  
Image: `registry.k8s.io/coredns/coredns:v1.11.1`  
Namespace: `kube-system`  
Pod name: `coredns-76f75df574-r8mh8`

### 1.1 SBOM theo pod_uid

```sql
SELECT id, pod_uid, pod_name, namespace, container_name, image_name, image_tag, image_digest,
       package_count, sbom_source, confidence, generated_at, created_at, updated_at
FROM sboms
WHERE pod_uid = '0f2e43e9-9f91-48ee-b98e-40a0def36902'
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 5;
```

Kỳ vọng: ít nhất 1 row, `package_count = 1`, `sbom_source = 'distroless-heuristic'`.

### 1.2 Components của SBOM đó

Thay `{sbom_id}` bằng `id` lấy từ câu trên (vd. 42):

```sql
SELECT id, sbom_id, component_name, component_version, component_type, purl, source, created_at
FROM sbom_components
WHERE sbom_id = {sbom_id}
  AND deleted_at IS NULL;
```

- Nếu **0 rows** → đây là nguyên nhân API trả về `components: []`. Có thể:
  - Agent gửi `Packages` rỗng nhưng Core vẫn ghi `package_count = 1` (không nhất quán),
  - Hoặc component insert thất bại / rollback không đúng,
  - Hoặc components từng tồn tại nhưng bị xóa (cascade, script, migration).
- Nếu **có rows** mà API vẫn trả rỗng → lỗi API (filter, join, hoặc đọc nhầm sbom_id).

**Kiểm tra soft-delete:** có thể component bị soft-delete (deleted_at NOT NULL):

```sql
SELECT id, sbom_id, component_name, component_version, purl, deleted_at
FROM sbom_components
WHERE sbom_id = {sbom_id};
```

Nếu có row với `deleted_at` khác NULL → API dùng `deleted_at IS NULL` nên không trả về; cần tìm lý do component bị đánh dấu xóa.

### 1.3 CVE matches cho SBOM đó

```sql
SELECT id, sbom_id, package_name, cve_id, severity, matched_by
FROM cve_matches
WHERE sbom_id = {sbom_id}
  AND deleted_at IS NULL;
```

Giúp kiểm tra CVE matcher đã chạy cho SBOM này và package name có khớp với component không.

---

## 2. Log Core khi nhận SBOM

Tìm lần Core xử lý SBOM cho pod này (pod_uid hoặc pod name):

```bash
kubectl logs -n fortuna -l app=fortuna-core --tail=5000 | grep -E "0f2e43e9-9f91-48ee-b98e-40a0def36902|coredns-76f75df574-r8mh8|SBOM.*coredns"
```

Cần thấy ít nhất một trong các dòng:

- `[SBOM] correlation_id=... received SBOM from agent=..., pod=..., image=...`
- `[SBOM] Created new SBOM id=... for pod_uid=0f2e43e9-9f91-48ee-b98e-40a0def36902` hoặc `Updated existing SBOM id=... for pod_uid=...`
- `[SBOM] correlation_id=... successfully stored SBOM id=... with N components` → **N phải = 1** (số package agent gửi).

Nếu có:

- `[SBOM] Failed to delete old components` hoặc `[SBOM] Failed to insert components` → lỗi DB khi lưu components; xem message chi tiết.
- `successfully stored SBOM id=X with 0 components` → Agent gửi `Packages` rỗng; cần xem log Agent.

---

## 3. Log Agent khi extract / gửi SBOM

Agent chạy trên **cùng node** với pod `coredns-76f75df574-r8mh8`. Xem log agent (có thể nhiều replica, chọn pod trên đúng node):

```bash
# Pod coredns chạy trên node nào
kubectl get pod -n kube-system coredns-76f75df574-r8mh8 -o wide

# Log agent trên node đó (thay <agent-pod> nếu cần)
kubectl logs -n fortuna -l app=fortuna-agent --tail=5000 | grep -E "coredns|kube-system|0f2e43e9|SBOM|Extracting|SendSBOM|packages"
```

Cần xác nhận:

- Có dòng extract cho image `registry.k8s.io/coredns/coredns:v1.11.1` (hoặc digest tương ứng).
- Parser distroless: với image coredns, binary `coredns` nằm trong signature → phải có **1 package** (coredns). Nếu thấy "0 packages" từ distroless thì có thể image không có `/usr/bin/coredns` hoặc logic allowlist bỏ qua.
- Sau khi extract, agent gửi gRPC SendSBOMFinding với **ít nhất 1 package**; nếu log báo "sent 0 packages" thì Core sẽ lưu `package_count=0` và không có component (trường hợp này khác với hiện tượng packageCount=1 nhưng components rỗng).

---

## 4. Nguyên nhân thường gặp khi packageCount=1 nhưng components rỗng

| Nguyên nhân | Cách kiểm tra |
|-------------|----------------|
| Component insert thất bại (constraint, lỗi DB) | Core log: `Failed to insert components`; Postgres log / constraint violation. |
| Agent gửi 1 package nhưng Core ghi nhầm package_count | So sánh Core log "stored SBOM with N components" vs `package_count` trong DB; kiểm tra code gán `package_count` từ `len(req.Packages)`. |
| Components bị xóa sau khi tạo (cascade, script) | Query 1.2 trả 0 rows; kiểm tra trigger, job, migration có xóa theo sbom_id. |
| Unique (sbom_id, purl): conflict khi insert | OnConflict DoNothing có thể bỏ qua insert; sau khi Delete components, purl trùng có thể do logic dedup agent. Kiểm tra có nhiều package cùng purl trong một request không. |

---

## 5. Fix nhanh (sau khi xác định)

- Nếu **components chưa bao giờ được insert** (Core log "with 0 components" hoặc lỗi insert): sửa Core (insert/transaction) hoặc đảm bảo agent gửi đúng 1 package cho coredns; rollout restart Core/Agent rồi để pod coredns được extract và gửi SBOM lại.
- Nếu **agent gửi 0 packages** cho image này: kiểm tra distroless allowlist (coredns có trong signature), và đường dẫn binary trong image (vd. `/usr/bin/coredns`); sửa agent nếu cần rồi rebuild/restart agent, xóa SBOM cũ (hoặc để pod restart để tạo SBOM mới).

Sau khi sửa, chạy lại query 1.1 và 1.2 để xác nhận `sbom_components` có đủ row và API trả về `components` không rỗng.
