# SBOM: Kiểm tra rác (junk) và control-plane

Tài liệu này mô tả cách kiểm tra SBOM sau khi áp dụng **distroless allowlist** (chỉ emit component khi binary nằm trong signature DB) và lý do **control-plane pods có thể chưa có SBOM**.

---

## 1. Rác SBOM đã xử lý

**Vấn đề:** SBOM của một số pod (agent, core) có rất nhiều component "rác": setpriv, zless, ln, mkdir, rm, perl, dpkg-deb, docker, egrep, … (unknown, application, version 0).

**Nguyên nhân:** Parser distroless quét `/bin`, `/usr/bin`, `/usr/lib` và trước đây emit **mọi** file dưới dạng component. Với image Debian (agent/core) khi dpkg không trả về gì, distroless chạy và thêm hàng trăm binary OS.

**Cách xử lý:** Distroless parser chỉ emit component khi **binary nằm trong signature DB** (`agent/pkg/sbom/signatures/distroless.json`). Danh sách hiện tại: kube-apiserver, kube-controller-manager, kube-scheduler, kube-proxy, coredns, etcd, pause, containerd-shim. Các binary khác (setpriv, ln, mkdir, …) **không** được thêm vào SBOM.

**Test:** `cd agent && go test ./pkg/sbom/extractor/ -v -run TestDistrolessParserJunkNotEmitted`

---

## 2. Control-plane pods chưa có SBOM

**Hiện tượng:** Pod control-plane (kube-apiserver, kube-controller-manager, kube-scheduler, etcd, coredns) không có SBOM trên dashboard.

**Nguyên nhân thường gặp:** Agent chỉ xử lý pod **trên node mà agent đang chạy**. Nếu control-plane chạy trên node A (vd. master) mà agent **không** chạy trên node A (vd. agent chỉ chạy trên worker), thì pod trên node A sẽ không được extract SBOM.

**Cách kiểm tra:**

- Pod nào có SBOM: xem bảng `sboms` có `pod_uid` tương ứng.
- Pod control-plane đang chạy trên node nào: `kubectl get pods -n kube-system -o wide`.
- Agent chạy trên node nào: `kubectl get pods -n fortuna -l app=fortuna-agent -o wide`.

**Khắc phục:** Đảm bảo agent chạy trên **mọi node** có workload cần SBOM (ví dụ DaemonSet). Khi agent chạy trên cùng node với control-plane pod, nó sẽ extract SBOM (distroless allowlist chỉ emit kube-apiserver / coredns / etc. nếu có trong image).

---

## 3. Query DB để kiểm tra

Chạy trong Postgres (vd. `kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna`).

### 3.1 SBOM theo pod / namespace

```sql
-- SBOM theo namespace (số lượng)
SELECT namespace, COUNT(*) AS sbom_count
FROM sboms
WHERE deleted_at IS NULL
GROUP BY namespace
ORDER BY sbom_count DESC;

-- Pod nào có SBOM (pod_uid, namespace, image)
SELECT pod_uid, namespace, pod_name, container_name, image_name, image_tag, sbom_source, confidence, package_count, generated_at
FROM sboms
WHERE deleted_at IS NULL
ORDER BY generated_at DESC
LIMIT 50;
```

### 3.2 Component “rác” (unknown, tên binary OS)

Sau khi áp dụng allowlist, các component dạng `unknown` + tên binary (setpriv, ln, mkdir, …) **không còn được thêm mới**. Để kiểm tra dữ liệu cũ còn lại:

```sql
-- Component có source distroless-heuristic và tên giống binary OS (rác)
SELECT c.id, c.sbom_id, c.component_name, c.component_version, c.source, s.namespace, s.pod_name, s.image_name
FROM sbom_components c
JOIN sboms s ON s.id = c.sbom_id AND s.deleted_at IS NULL
WHERE c.source = 'distroless-heuristic'
  AND c.component_version IN ('unknown', '0', '')
  AND c.component_name IN ('setpriv','zless','resizepart','addpart','dircolors','ln','script','expand','mkdir','mesg','toe','gpgv','rm','perl','dpkg-deb','groups','install','rmdir','docker','egrep','umount','prlimit')
ORDER BY c.id DESC
LIMIT 100;
```

Nếu truy vấn này trả 0 row sau khi rebuild agent và gửi SBOM mới, nghĩa là không còn component rác từ distroless.

### 3.3 Control-plane: pod kube-system có SBOM không?

```sql
SELECT pod_uid, pod_name, namespace, container_name, image_name, image_tag, sbom_source, package_count
FROM sboms
WHERE deleted_at IS NULL AND namespace = 'kube-system'
ORDER BY generated_at DESC;
```

Nếu không có row → agent chưa gửi SBOM cho pod kube-system (thường do agent không chạy trên node có pod đó).

### 3.4 Số component theo SBOM (phát hiện SBOM quá nhiều component)

```sql
SELECT s.id, s.namespace, s.pod_name, s.image_name, s.package_count,
       (SELECT COUNT(*) FROM sbom_components c WHERE c.sbom_id = s.id) AS actual_components
FROM sboms s
WHERE s.deleted_at IS NULL
ORDER BY actual_components DESC
LIMIT 20;
```

SBOM từ image agent/core sau khi sửa thường có số component từ dpkg (hàng chục đến vài trăm package thật), không còn hàng trăm binary “unknown” từ distroless.

---

## 4. Log cần xem khi xử lý SBOM

### 4.1 Agent (extract SBOM)

- `Parser distroless: skipped (OS package manager already returned packages)` → Image có dpkg/apk, distroless không chạy (đúng, tránh rác).
- `Parser distroless: 0 packages` → Distroless chạy nhưng không có binary nào trong signature (chỉ emit khi có kube-apiserver, coredns, …).
- `[control-plane version] binary=... version_after=...` → Version được điền từ digestMap/tag/label cho control-plane.
- `🆕 Pod added: kube-system/...` / `Queued pod ... for async processing` → Pod được đưa vào queue SBOM (kể cả kube-system nếu trên cùng node).

Cách xem: `kubectl logs -n fortuna -l app=fortuna-agent -f --tail=200`

### 4.2 Core (nhận SBOM, CVE matching)

- Handler SBOM: lưu `sboms` + `sbom_components`, sau đó gửi event cho CVE matcher worker.
- CVE matcher: log NVD fallback, `MatchedBy=nvd-fallback`, `EnsureCVEExists`.

Cách xem: `kubectl logs -n fortuna -l app=fortuna-core -f --tail=200`

---

## 5. Testcase đã thêm

| Test | Mục đích |
|------|----------|
| `TestDistrolessParserJunkNotEmitted` | Có setpriv, ln, mkdir, … trong FS nhưng chỉ kube-apiserver (trong signature) được emit. |
| `TestDistrolessParser` | Chỉ binary trong signature (vd. coredns) được emit; busybox, nginx, libc.so không. |
| `TestDistrolessParserSkipsJunk` | Path skip: .pl, .so không phải lib*, share/locale. |
| `TestDistrolessParserUsesSignatures` | Coredns lấy PURL/confidence từ distroless.json. |

Chạy: `cd agent && go test ./pkg/sbom/extractor/ -v -run 'TestDistrolessParser|TestDistrolessParserJunk|TestDistrolessParserSkipsJunk|TestDistrolessParserUsesSignatures'`
