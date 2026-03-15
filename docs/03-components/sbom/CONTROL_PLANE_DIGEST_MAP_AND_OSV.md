# Digest map cho control-plane pods và tra CVE từ local OSV

**Mục đích:** Có cách lấy **toàn bộ digest → version** cho các image control-plane (kube-apiserver, kube-controller-manager, kube-scheduler, kube-proxy, etcd, coredns, pause) để Agent điền đúng version vào SBOM; Core tra CVE từ **local OSV** (PostgreSQL đã load OSV JSON).

---

## 1. Luồng tổng thể

```
Registry (registry.k8s.io, etc.)
    → Script/Crane lấy digest theo tag
    → digestMap trong distroless.json (hoặc file riêng merge vào)
    → Agent extractor: image digest → version
    → SBOM component: pkg:generic/kube-apiserver@1.29.0
    → Core CVE matcher: ecosystem=generic, package_name=kube-apiserver, version=1.29.0
    → Query package_vulnerabilities (OSV đã load) → CVE
```

---

## 2. Cách lấy digest map theo version (control-plane)

### 2.1 Dùng Crane (khuyến nghị)

[crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane) (go-containerregistry) lấy digest của image theo tag:

```bash
# Cài: go install github.com/google/go-containerregistry/cmd/crane@latest
crane digest registry.k8s.io/kube-apiserver:v1.29.0
# → sha256:63d191b8532ebd1a9ff7c5eeeacca9f855084adae4666b4f720bc0e3b6bcba4d
```

**Script mẫu:** `scripts/docs/build-control-plane-digest-map.sh` (xem bên dưới) – lặp danh sách tag (từ release hoặc cố định), gọi `crane digest` từng image, xuất JSON `digestMap` theo đúng schema `distroless.json`.

### 2.2 Từ Kubernetes release manifest / SBOM

Kubernetes publish SBOM và image list theo version:

```bash
# Version stable
KVER=$(curl -sL https://dl.k8s.io/release/stable.txt)
# Image list (SPDX) – có thể parse lấy digest
curl -sL "https://sbom.k8s.io/${KVER}/release" | grep -E "registry.k8s.io|digest"
```

Có thể parse SPDX hoặc dùng `kubeadm config images list --kubernetes-version=$KVER` rồi với mỗi image chạy `crane digest`.

### 2.3 Image ↔ binary name (control-plane)

| Binary (trong distroless.json) | Image mặc định (registry.k8s.io) | Ghi chú |
|--------------------------------|----------------------------------|--------|
| kube-apiserver | kube-apiserver | Tag: v1.x.y |
| kube-controller-manager | kube-controller-manager | v1.x.y |
| kube-scheduler | kube-scheduler | v1.x.y |
| kube-proxy | kube-proxy | v1.x.y |
| etcd | etcd | Tag 3.5.x (theo k8s version) |
| coredns | coredns/coredns | Tag riêng (vd. v1.11.1) |
| pause | pause | Tag 3.9, 3.10, ... |

Mỗi **image** (ref bằng tag hoặc digest) có **một digest** (manifest digest). Script nên map: **image ref (tag)** → **digest** → **version** (string dùng cho PURL, vd. `1.29.0` hoặc `v1.29.0`). Core matcher đã hỗ trợ so sánh version dạng semver cho ecosystem `generic` (kể cả `v1.29.0` sau khi clean).

---

## 3. Local OSV và CVE matching

### 3.1 Core đã dùng Postgres (OSV)

- **FORTUNA_CVE_SOURCE=postgres** (mặc định): Core query bảng `package_vulnerabilities` + `cves` (load từ OSV JSON).
- Matcher gọi `GetVulnerabilitiesForPackage(ctx, ecosystem, package_name, version)`; với PURL `pkg:generic/kube-apiserver@1.29.0` thì **ecosystem = "generic"**, **package_name = "kube-apiserver"**, **version = "1.29.0"**.

### 3.2 Ecosystem "generic" trong OSV

- OSV.dev có thể có package với **ecosystem** kiểu `"Go"` hoặc tên khác cho Kubernetes components; trong DB Fortuna lưu theo **ecosystem** mà loader chuẩn hóa (vd. từ PURL ecosystem).
- Để **local OSV** trả CVE cho kube-apiserver, coredns, etc. cần:
  1. **OSV data** có bản ghi affected package với **ecosystem** và **package name** tương ứng (vd. `generic` + `kube-apiserver`, hoặc ecosystem mà loader map từ OSV).
  2. **Loader** (OSV → Postgres) ghi đúng `ecosystem` / `package_name` mà matcher dùng (xem `core/pkg/cve/loader`, normalize ecosystem).
  3. **Version comparator** đã hỗ trợ **generic** với semver (vd. `v1.29.0`, `1.11.1`) để so version với constraint trong OSV.

### 3.3 Nếu OSV không có ecosystem "generic"

- Có thể trong OSV Kubernetes/CoreDNS nằm ecosystem khác (vd. `Go`). Khi đó cần một trong hai:
  - **Option A:** Loader map package OSV (vd. `kubernetes`) → lưu vào Postgres với `ecosystem = "generic"` (hoặc tên mà matcher dùng) và `package_name` chuẩn (kube-apiserver, coredns, ...).
  - **Option B:** Matcher với PURL `pkg:generic/...` map sang ecosystem khác khi query (vd. `generic` → `Go` cho một số package); cần cấu hình hoặc bảng map.

Hiện matcher dùng **ecosystem từ PURL** (generic) và **package_name** từ PURL; nên đảm bảo OSV loader khi import CVE cho Kubernetes/CoreDNS/etcd ghi đúng cặp (ecosystem, package_name) đó.

---

## 4. Định dạng digestMap (distroless.json)

Mỗi binary trong `agent/pkg/sbom/signatures/distroless.json` có thể có:

```json
"kube-apiserver": {
  "purl": "pkg:generic/kube-apiserver@unknown",
  "confidence": "medium",
  "versionFromTag": true,
  "labelKeys": ["org.opencontainers.image.version", "io.k8s.display-version"],
  "digestMap": {
    "sha256:63d191b8532ebd1a9ff7c5eeeacca9f855084adae4666b4f720bc0e3b6bcba4d": "1.29.0",
    "sha256:...": "1.30.0"
  }
}
```

- **digestMap:** key = image manifest digest (đủ `sha256:...`), value = version string dùng cho PURL và so sánh CVE (nên thống nhất format, vd. không `v` prefix hoặc có `v` tùy OSV).
- Script build digest map nên xuất đúng format này để merge thủ công hoặc tự động vào `distroless.json`, sau đó tăng `version` trong JSON (vd. `"version": "3"`) để invalidate SBOM cache (Finding #8.5).

---

## 5. Script mẫu: build digest map

**Script:** `scripts/docs/build-control-plane-digest-map.sh`

- **Prereq:** [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane) – `go install github.com/google/go-containerregistry/cmd/crane@latest`
- **Cách chạy:**  
  `./scripts/docs/build-control-plane-digest-map.sh [stable|v1.29.0|v1.30.0 ...]`  
  Không truyền tham số thì dùng stable + vài version mặc định.
- **Output:** JSON object: mỗi key = tên binary (kube-apiserver, coredns, ...), value = object `{ "sha256:...": "version", ... }`. Merge từng value vào field `digestMap` của binary tương ứng trong `agent/pkg/sbom/signatures/distroless.json`, sau đó tăng `version` trong distroless.json để invalidate SBOM cache.
- **Lưu ý:** etcd / coredns / pause có thể dùng tag riêng (vd. coredns v1.11.1, pause 3.9). Có thể mở rộng script với danh sách tag per-image hoặc chạy nhiều lần với bộ tag khác nhau rồi merge.

---

## 6. Debug: control-plane version không hiển thị

Khi component control-plane (kube-apiserver, coredns, …) vẫn hiển thị version = "unknown":

1. **Xem log agent** khi extract SBOM cho pod control-plane (vd. kube-apiserver trên node):
   - `[control-plane version] OCI label …` — có OCI label version/ref.name không.
   - `[control-plane version] imageTag=… imageDigest=…` — ref có tag hay chỉ digest.
   - `[control-plane version] binary=… version_after=… source=…` — nguồn version: `digestMap` / `tag` / `label:...` / `none`; `digestMap_empty=true` nghĩa là chưa có digestMap cho binary đó; `tag_is_sha=true` nghĩa là image ref dạng digest nên không dùng được versionFromTag.

2. **Cách version được chọn (thứ tự):**
   - **digestMap:** nếu trong `distroless.json` binary có `digestMap` và digest image trùng key → dùng version tương ứng.
   - **tag:** nếu image ref có tag (không phải sha256:...) và binary có `versionFromTag: true` → dùng tag làm version.
   - **label:** nếu image config có label trong `labelKeys` (vd. `io.k8s.display-version`, `org.opencontainers.image.version`) → dùng giá trị label.
   - **Fallback:** nếu vẫn unknown, dùng label `org.opencontainers.image.ref.name` (dạng `repo:tag`) — lấy phần sau `:` làm version.

3. **Nếu vẫn unknown:** populate **digestMap** cho từng binary (chạy `scripts/docs/build-control-plane-digest-map.sh` với crane, merge output vào `distroless.json`), tăng `version` trong JSON để invalidate SBOM cache, rồi rebuild agent.

---

## 7. Tóm tắt

| Bước | Công cụ / Nơi thực hiện |
|------|--------------------------|
| Lấy digest theo version | crane digest, hoặc parse SBOM/release k8s |
| Chuẩn hóa digestMap | Script → JSON → merge vào distroless.json, tăng version |
| Agent dùng digest | extractor applySignatureHints() đọc digestMap → version/PURL |
| Core tra CVE | Postgres (OSV) với ecosystem=generic, package_name + version; version comparator hỗ trợ generic (semver) |

Nhờ đó có thể **lấy toàn bộ digest map theo version** cho control-plane và **tra CVE từ local OSV** miễn là dữ liệu OSV trong Postgres có (ecosystem, package_name) tương ứng và version so sánh được (semver).
