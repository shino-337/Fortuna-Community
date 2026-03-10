# Phân tích SBOM Extractor và lý do không extract được package (website-vuln-lodash)

## 1. Tổng quan luồng SBOM

```
Pod Running (k8s-master)
    → LocalPodWatcher / SBOM Queue
    → SBOM Processor.ProcessPod() → processContainer(container.Image)
    → Extractor.ExtractSBOM(ctx, imageRef)
        → getImage() → getImageFromContainerd() [export image ra temp tar]
        → img.Digest() [có thể lỗi: file đã bị xóa]
        → img.Layers()
        → buildFilesystem(ctx, layers)  ← layer.Uncompressed() đọc từ temp file
        → detectOS(fs)
        → selectParsersForOS() → chạy dpkg, apk, rpm, npm, pip, gomod
        → deduplicate → RawSBOM
    → convertToProto → SendSBOMFinding → Core
```

- **Kích hoạt:** Pod `website-vuln-lodash` chuyển sang Running trên node có agent → được queue → worker gọi `ProcessPod` → `ExtractSBOM(website-vuln-lodash:latest)`.
- **Image:** Lấy từ containerd (local) qua `getImageFromContainerd`: export image ra **một file tar tạm**, rồi `tarball.ImageFromPath(tmp.Name(), nil)` để có `v1.Image`.
- **Filesystem:** `buildFilesystem` gọi từng `layer.Uncompressed()` rồi `ExtractTar` vào một `Filesystem` (map path → nội dung file).
- **Parser:** Với OS = "unknown", chạy tất cả parser; npm parser tìm `package-lock.json` và `node_modules/*/package.json` theo 3 strategy (common roots + fallback discovery).

---

## 2. Cấu hình pod / image website-vuln-lodash

- **Dockerfile:** `FROM node:18-alpine`, `WORKDIR /app`, `COPY package.json package-lock.json* ./`, `RUN npm install --omit=dev`, ...
- **Trong image:** Có `/app/package.json`, `/app/package-lock.json` (do `npm install` tạo), `/app/node_modules/` (lodash, serve, ...). Các layer OCI lưu đường dẫn dạng `app/package-lock.json`, `app/node_modules/...`.
- **Chuẩn hóa path trong extractor:** `ExtractTar` dùng `filepath.Clean(header.Name)` và thêm `/` nếu thiếu → path trong `fs.files` là `/app/package-lock.json`, `/app/node_modules/lodash/package.json`, ...

Vì vậy **về cấu hình và đường dẫn**, image có đủ dữ liệu và npm parser (Strategy 1/2/3) đủ path để tìm thấy package.

---

## 3. Nguyên nhân gốc: temp file bị xóa trước khi đọc layer

### 3.1 Đoạn code hiện tại (`getImageFromContainerd`)

```go
tmp, err := os.CreateTemp("", "fortuna-image-*.tar")
// ...
defer func() {
    _ = tmp.Close()
    _ = os.Remove(tmp.Name())  // ← file bị xóa ngay khi hàm return
}()

// Export image vào tmp
archive.Export(..., tmp, ...)
tmp.Seek(0, 0)

img, err := tarball.ImageFromPath(tmp.Name(), nil)  // mở đọc từ path
return img, nil   // ← defer chạy: Close + Remove → file không còn
```

- `tarball.ImageFromPath` tạo một `v1.Image` **lazy**: khi gọi `img.Layers()` rồi `layer.Uncompressed()`, nó mới đọc dữ liệu từ file tar (theo path hoặc handle).
- Khi `getImageFromContainerd` return, **defer chạy ngay**: `tmp.Close()` và `os.Remove(tmp.Name())` → file tạm bị đóng và xóa.
- Sau đó, trong `ExtractSBOM`:
  - `img.Layers()` và `layer.Uncompressed()` được gọi trong `buildFilesystem`.
  - Lúc này file tar đã không còn → đọc layer **thất bại** (ví dụ "no such file or directory").

### 3.2 Hậu quả trong code

Trong `buildFilesystem`:

```go
for _, layer := range layers {
    uncompressed, err := layer.Uncompressed()  // ← lỗi vì file đã xóa
    if err != nil {
        continue   // bỏ qua layer
    }
    fs.ExtractTar(ctx, uncompressed)
}
```

- Mọi layer đều có thể lỗi khi `Uncompressed()` → mọi layer bị `continue` → **không có layer nào được đưa vào filesystem**.
- `fs.files` rỗng hoặc gần rỗng → không có `/etc/os-release`, không có `/app/package-lock.json`, không có `node_modules/...`.
- Kết quả:
  - `detectOS(fs)` → không đọc được `/etc/os-release` → **OS = "unknown"**.
  - Npm parser (Strategy 1/2/3) không tìm thấy file nào → **0 packages**.
  - Log: "Detected OS: unknown", "Parser dpkg/apk: not applicable", "Extracted 0 unique packages".

Log lỗi digest (`open /tmp/fortuna-image-*.tar: no such file or directory`) cũng nhất quán với việc file tạm đã bị xóa trước khi dùng lại (ví dụ khi tính digest).

---

## 4. Tại sao “có package.json” vẫn 0 package?

- Pod **có** `package.json` (và trong image có cả `package-lock.json` + `node_modules`) **ở trong image**, nhưng:
  - Filesystem mà extractor dùng **không phải** là filesystem thật của image, mà là filesystem **ảo** build từ nội dung các layer.
  - Do temp file bị xóa trước khi đọc layer, **toàn bộ nội dung layer không bao giờ được đọc** → filesystem ảo **rỗng**.
  - Npm parser chỉ làm việc trên `fs` (map path → content). Map rỗng → không có file nào → 0 package.

Vì vậy vấn đề **không phải** thiếu `package.json` hay sai path trong image, mà là **dữ liệu layer không bao giờ được nạp vào filesystem** do lifecycle của file tar tạm sai.

---

## 5. Các luồng chi tiết liên quan

| Bước | Vị trí | Mô tả |
|------|--------|--------|
| 1 | `processor.ProcessPod` | Chỉ xử lý container trên đúng node; gọi `ExtractSBOM(container.Image)`. |
| 2 | `Extractor.ExtractSBOM` | Parse reference → `getImage(ref)` → lấy digest (có thể lỗi) → `layers` → `buildFilesystem(layers)` → detectOS → chạy parsers → dedup → RawSBOM. |
| 3 | `getImageFromContainerd` | Export image vào temp tar, `tarball.ImageFromPath`, **defer Close+Remove** → return image (file đã bị xóa). |
| 4 | `buildFilesystem` | Với từng layer: `Uncompressed()` (đọc từ file đã xóa → lỗi) → `continue` → fs trống. |
| 5 | `detectOS` | Đọc `/etc/os-release` từ `fs` → không có → "unknown". |
| 6 | `selectParsersForOS("unknown")` | Trả về tất cả parser: dpkg, apk, rpm, npm, pip, gomod. |
| 7 | Npm parser | Strategy 1: ReadFile("/app/package-lock.json") → không có. Strategy 2: Glob("/app/node_modules/*/package.json") → rỗng. Strategy 3: FindPathsBySuffix/FindPathsContaining → rỗng (fs rỗng). → 0 packages. |

---

## 6. Hướng sửa (đã áp dụng trong code)

- **Vấn đề:** Image từ tarball cần file tar còn tồn tại khi gọi `layer.Uncompressed()`; hiện tại file bị xóa ngay khi `getImageFromContainerd` return.
- **Hướng sửa:** Đọc **toàn bộ nội dung layer** (từ image tarball) **trong** `getImageFromContainerd`, **trước** khi `defer` đóng/xóa file. Sau đó trả về một **image “materialized”**: `Layers()` trả về các layer mà `Uncompressed()` đọc từ bộ nhớ (bytes đã đọc), không phụ thuộc vào file tar nữa. Khi đó có thể an toàn `Close` và `Remove` file tạm.
- **Kết quả mong đợi:** `buildFilesystem` nhận được đầy đủ dữ liệu layer → fs có `/app/package-lock.json`, `/app/node_modules/...` → npm parser tìm thấy package → SBOM có package cho website-vuln-lodash.

---

## 7. Tóm tắt

| Câu hỏi | Trả lời |
|--------|---------|
| Logic SBOM extractor có đúng không? | Có: từ pod → queue → processor → extractor → get image → build fs → detect OS → chạy parsers (npm với 3 strategy) → gửi Core. |
| Image website-vuln-lodash có package không? | Có: WORKDIR /app, npm install → có package-lock.json và node_modules trong image. |
| Vì sao vẫn 0 package? | File tar tạm (chứa image export từ containerd) bị **đóng và xóa ngay** khi return từ `getImageFromContainerd`. Khi `buildFilesystem` gọi `layer.Uncompressed()`, file đã không còn → mọi layer bị bỏ qua → filesystem ảo rỗng → npm parser không thấy file nào. |
| Có phải do thiếu package.json trong pod? | Không. Pod/image có đủ file; nguyên nhân là **dữ liệu layer không được nạp** do lifecycle sai của temp file. |

Sửa: materialize layer trong `getImageFromContainerd` (đọc hết layer trước khi đóng/xóa file), trả về image không phụ thuộc file tar.

---

## 8. Bổ sung: OCI overlay và whiteout (đã sửa trong code)

- **ExtractTar** trước đây chỉ lưu `tar.TypeReg`, bỏ qua thư mục, symlink và **whiteout**.
- OCI layer áp dụng theo overlay: layer sau ghi đè layer trước; file **.wh.\*** là whiteout (xóa file từ layer trước).
- **Đã sửa:** Trong `filesystem.ExtractTar`:
  - **.wh.filename** → xóa file/dir đích `dir/filename` khỏi `fs.files`, consume body rồi `continue`.
  - **.wh..wh..opq** → xóa mọi path có prefix `dir/` (opaque directory).
  - Với mọi entry không phải TypeReg (và whiteout), consume body bằng `io.CopyN(io.Discard, tr, header.Size)` để vị trí đọc tar đúng.
- **buildFilesystem:** Đóng reader sau mỗi layer; log lỗi theo chỉ số layer; sau khi áp dụng hết layer, log: `Virtual FS: N files total, M paths containing package.json` (debug).

---

## 9. Troubleshooting: "content digest ... not found" (containerd)

### 9.1 Triệu chứng

Log xuất hiện:

```
[SBOMExtractor] ⚠️  Containerd fetch failed (image/layer may be missing on this node): containerd export error: content digest sha256:...: not found
[SBOMExtractor] 🔍 Falling back to remote registry: ghcr.io/...
```

### 9.2 Nguyên nhân

- Containerd có **metadata** của image (tên/tag trong ImageService) nhưng **một hoặc nhiều blob nội dung** (layer hoặc config) **không có** trong ContentStore trên node này.
- Thường gặp khi:
  1. **Agent chạy trên node khác với node chạy pod:** image được kéo trên worker, nhưng agent chạy trên master hoặc node khác → metadata có thể được đồng bộ (listing) nhưng blob layer chỉ có trên node đã pull.
  2. **Content bị GC:** containerd đã garbage-collect blob (ví dụ image không còn container/pod nào dùng).
  3. **Pull chưa hoàn tất / lỗi:** image được tham chiếu nhưng một layer chưa được tải xuống.

Luồng hiện tại: thử containerd trước → nếu export lỗi (digest not found) → **fallback sang remote registry** và vẫn trích xuất SBOM bình thường.

### 9.3 Cách xử lý

| Cách | Mô tả |
|------|--------|
| **Không làm gì** | Fallback registry đã đủ: SBOM vẫn được extract từ registry. Chỉ cần chấp nhận log warning. |
| **Giảm log / tránh thử containerd** | Nếu agent không chạy trên node có đủ blob (ví dụ chạy central), set **`SBOM_PREFER_REGISTRY=1`** để bỏ qua containerd và luôn dùng registry → không còn lỗi "content digest not found" từ containerd. |
| **Đảm bảo image có trên node** | Nếu muốn dùng containerd (nhanh, không rate limit): đảm bảo agent chạy trên cùng node với pod và image đã được pull đầy đủ (không bị GC). |

### 9.4 Biến môi trường liên quan

- **`SBOM_PREFER_REGISTRY`** (optional): `1` hoặc `true` → bỏ qua containerd, luôn lấy image từ remote registry.
- **`CONTAINERD_SOCKET`** (optional): đường dẫn socket containerd (mặc định `/run/containerd/containerd.sock`).
- **`CONTAINERD_NAMESPACE`** (optional): namespace containerd (mặc định `k8s.io`).
