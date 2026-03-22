# A5+ — Streaming / Selective VFS: Kế hoạch thực hiện chi tiết

**Cập nhật:** 2026-03-10  
**Trạng thái:** Phase 1 done (cap) · **Phase 2a+2b+2c done** (indexed + spool + **metrics / log**) · Phase 3 future

---

## Vấn đề hiện tại

`Filesystem.files` là `map[string][]byte` — toàn bộ nội dung file từ mọi layer được giữ trong RAM.

**Peak RAM** ≈ tổng kích thước uncompressed của các file regular (sau whiteout) trong image.
Với cap hiện tại (`SBOM_FS_MAX_TOTAL_BYTES=1GiB`), agent node cần ít nhất 1GiB RAM headroom.
Images lớn (multi-stage Java/Node, ML frameworks) dễ vượt cap → file bị skip → parser thiếu data.

---

## Phân tích truy cập Filesystem theo Parser

### Discovery-only (không cần content)

| Parser | API | Pattern |
|--------|-----|---------|
| distroless | `PathsUnder` | `/bin/`, `/usr/bin/`, `/usr/lib/`, `/usr/local/bin/`, `/usr/local/sbin/` |
| pip | `FindPathsBySuffix` | `"METADATA"` |
| gomod | `FindPathsBySuffix` | `"go.sum"`, `"go.mod"` |
| npm | `FindPathsBySuffix`, `FindPathsContaining`, `Glob` | `"package-lock.json"`, `"node_modules"`, `"package.json"` |
| maven | `FindPathsBySuffix` | `"pom.xml"` |
| cargo | `FindPathsBySuffix` | `"Cargo.lock"` |
| ruby | `FindPathsBySuffix` | `"Gemfile.lock"` |
| nuget | `FindPathsBySuffix` | `"packages.lock.json"` |
| rpm | `FileExists` | 8 candidate paths |

→ **Path index đủ cho discovery.** Không cần content.

### Content readers — phân loại theo kích thước

| Nhóm | Parser | Files | Size | Đặc điểm |
|-------|--------|-------|------|-----------|
| **Tiny** (<10KB) | apk, pip (METADATA), gomod, maven, cargo, rpm (os-release) | 1–30 files | Bytes–50KB | Materialize ngay OK |
| **Medium** (<1MB) | dpkg (`/var/lib/dpkg/status`), npm (package.json ×N), pip (requirements.txt) | 1–100 files | KB–1MB | Materialize OK nếu selective |
| **Large** (1–50MB) | npm (package-lock.json), rpm (rpmdb.sqlite) | 1 file | 1–50MB | Cần lazy hoặc spool |
| **Huge** (1–200MB) | gobinary (binary ×2000) | Lên đến 2000 files | 1–200MB mỗi file | **Chiếm >90% RAM.** Cần lazy read. |

### Kết luận thiết kế

1. **Path index** (tên file + size + layerRef) đủ cho tất cả discovery APIs.
2. **Selective materialize** cho tiny/medium files matching known suffixes → phủ 9/10 parsers.
3. **Lazy read** (spool layer ra temp file, seek khi cần) cho gobinary và large files.
4. **Whiteout** phải xử lý tại index-build time (giống hiện tại).

---

## Kiến trúc Phase 2

### Hai mode: `materialize` (hiện tại) và `indexed` (mới)

```
SBOM_FS_MODE=materialize     # Hành vi hiện tại (default, backward compat)
SBOM_FS_MODE=indexed          # Phase 2: index + selective + lazy
```

### Indexed mode — data structures

```go
// IndexEntry stores metadata about a file in the image without holding content.
type IndexEntry struct {
    LayerIdx   int    // which layer (0 = base)
    Offset     int64  // byte offset within uncompressed tar stream
    Size       int64  // file size from tar header
    Whiteout   bool   // true if removed by higher layer
}

// IndexedFilesystem: path index + selective content cache + layer spool handles.
type IndexedFilesystem struct {
    index           map[string]*IndexEntry  // path → metadata (ALL regular files)
    content         map[string][]byte       // path → content (selectively materialized)
    layerSpools     []*os.File              // temp files holding uncompressed layer streams
    maxContentBytes int64                   // SBOM_FS_MAX_TOTAL_BYTES (for content cache)
    maxFileBytes    int64                   // SBOM_FS_MAX_FILE_BYTES
    contentBytes    int64                   // current content cache usage
    logger          *log.Logger
}
```

### Build flow (thay thế `buildFilesystem`)

```
                 Layers (base → top)
                      │
         ┌────────────▼────────────────┐
         │   Pass 1: Index-only scan    │   ← duyệt tar header, KHÔNG đọc content
         │   → build index[path]        │   ← xử lý whiteout tại đây
         │   → spool layer ra temp file │   ← giữ handle để seek sau
         └────────────┬────────────────┘
                      │
         ┌────────────▼────────────────┐
         │   Pass 2: Selective extract  │   ← chỉ ReadFull cho files match suffixes
         │   → materialize tiny/medium  │
         │   → skip large (lazy later)  │
         └────────────┬────────────────┘
                      │
                      ▼
              IndexedFilesystem ready
              → Discovery: dùng index
              → ReadFile: content cache hoặc lazy seek từ spool
```

### Selective extract — suffix allowlist

Chỉ materialize content cho files matching:

```go
var selectiveSuffixes = []string{
    // OS package managers
    "/lib/apk/db/installed",
    "/var/lib/dpkg/status",
    "rpm-packages.list",
    "/etc/os-release",
    "/etc/debian_version",
    "/etc/alpine-release",

    // Language parsers
    "requirements.txt",
    "METADATA",           // *.dist-info/METADATA
    "go.sum",
    "go.mod",
    "package-lock.json",
    "package.json",
    "pom.xml",
    "Cargo.lock",
}
```

**Tổng kích thước selective**: cho image điển hình ≈ 5–50MB (thay vì 500MB–1GB+ khi materialize tất cả).

### Lazy ReadFile — cho gobinary và large files

Khi `ReadFile(path)` gọi cho file KHÔNG có trong `content`:

```go
func (fs *IndexedFilesystem) ReadFile(path string) ([]byte, error) {
    // 1. Check content cache
    if data, ok := fs.content[path]; ok {
        return data, nil
    }

    // 2. Check index
    entry, ok := fs.index[path]
    if !ok || entry.Whiteout {
        return nil, fmt.Errorf("file not found: %s", path)
    }

    // 3. Size guard
    if fs.maxFileBytes > 0 && entry.Size > fs.maxFileBytes {
        return nil, fmt.Errorf("file too large: %s (%d bytes)", path, entry.Size)
    }

    // 4. Seek + read from layer spool
    spool := fs.layerSpools[entry.LayerIdx]
    // ... seek to entry.Offset, read entry.Size bytes ...

    return data, nil
}
```

**Vấn đề**: tar stream là sequential — không thể seek random. Giải pháp:

| Option | Cách thức | Tradeoff |
|--------|-----------|----------|
| **A. Spool whole layer** | Ghi uncompressed tar ra temp file. Lưu offset từ Pass 1. Seek + read khi cần. | Disk I/O cho mỗi layer, nhưng RAM rất thấp. |
| **B. Re-stream on demand** | Khi cần file, decompress + scan lại layer tar đến đúng offset. | Không cần disk, nhưng chậm (O(n) per read). |
| **C. Sparse spool** | Chỉ spool layer khi có ≥1 file cần lazy read (e.g. layer chứa /usr/bin). | Tối ưu: skip layers chỉ chứa tiny files (đã selective extract). |

**Đề xuất: Option C (sparse spool)** — chỉ spool layers chứa files >selective threshold.

---

## Kế hoạch thực hiện

### Phase 2a — Index + Selective Extract ✅ **implemented**

**Scope:** Giảm RAM 60-80% cho images điển hình mà KHÔNG thay đổi parser interface.

| Step | File | Việc |
|------|------|------|
| 1–4 | `filesystem.go` | **`Filesystem.pathSizes`** (indexed mode) + overlay trong `ExtractTar`; whiteout xóa cả `files` và `pathSizes`; **selective materialize** một pass (`shouldMaterializeIndexed`) |
| 5 | `extractor.go` | `NewFilesystem()` đọc `SBOM_FS_MODE`; log `indexed paths / materialized` |
| 6 | `filesystem_test.go` | Tests: discovery không materialize, selective `package.json`, whiteout trên index |

**Triển khai thực tế:** không tách `IndexedFilesystem` — cùng struct `Filesystem`, `pathSizes == nil` ⇔ materialize mode.

**Output Phase 2a:** Parser discovery dùng index; `ReadFile` chỉ cho path đã materialize. **Gobinary** trong indexed mode thường **không** đọc được ELF (binary không nằm allowlist) → ít/không package `go-binary` cho đến Phase 2b.

**RAM profile:** Tiny/medium files ≈ 5–50MB thay vì 500MB–1GB+.

### Phase 2b — Lazy Spool + ReadFile ✅ **implemented**

**Scope:** Trong `SBOM_FS_MODE=indexed`, mọi file không giữ trong `files` map vẫn đọc được qua **spool**: mỗi layer `ExtractTar` tee toàn bộ stream nén không gzip ra **temp file** (`SBOM_FS_SPOOL_DIR` hoặc `os.TempDir`), lưu `(layerIdx, offset, size)` trong `lazyRefs`; `ReadFile` dùng `ReadAt` trên spool.

| Step | File | Việc |
|------|------|------|
| 1–3 | `filesystem.go` | `countingReader` + `io.TeeReader` → `layerSpools[layerIdx]`; `lazyRefs[path]`; `Close()` xóa temp |
| 4 | `extractor.go` | `ExtractTar(ctx, i, r)`; `defer fs.Close()` sau build FS (khi SBOM xong) |
| 5 | `filesystem_test.go` | Indexed: ReadFile từ lazy; whiteout layer 0/1 |

**Tradeoff disk:** ~1× kích thước mỗi layer uncompressed trên disk trong lúc extract (giải phóng khi `Close`). **RAM:** vẫn chỉ selective + index.

**Gobinary:** `ReadFile` trên binary paths hoạt động nhờ lazy spool (không cần materialize toàn bộ vào RAM).

### Phase 2c — Metrics + Tuning ✅ **implemented**

| Step | File | Việc |
|------|------|------|
| 1 | `filesystem.go` | `atomic` counters `lazyReadOps`, `lazyReadBytes` (tăng khi `ReadFile` đọc từ spool); `FSMetrics` + `MetricsSnapshot()`; `formatBytesIEC` |
| 2 | `extractor.go` | Sau parsers + go toolchain: `[SBOM FS] mode=indexed paths=N materialized=M (…) lazy_reads=R (…)` hoặc `mode=materialize files=… stored=…`. Tắt log: `SBOM_FS_METRICS=off` |
| 3 | Doc | Bảng env + `SBOM_PIPELINE_REMAINING_PLAN.md` |

---

## Interface compatibility

Indexed mode phải implement cùng interface với `Filesystem` hiện tại:

```go
// FilesystemReader is the interface parsers use (implicit in current code).
type FilesystemReader interface {
    ReadFile(path string) ([]byte, error)
    FindPathsBySuffix(suffix string) []string
    FindPathsContaining(sub, end string) []string
    Glob(pattern string) []string
    PathsUnder(prefix string) []string
    FileExists(path string) bool
}
```

**Hiện tại** parsers nhận `*Filesystem` (concrete type). Để chuyển sang indexed mode:

| Approach | Effort | Breaking change |
|----------|--------|-----------------|
| A. Extract interface, parsers nhận interface | Medium | Sửa tất cả parsers (signature change) |
| B. `IndexedFilesystem` embed `Filesystem` hoặc cùng package, parsers vẫn nhận `*Filesystem` nhưng field `files` được replace bằng index | Low | Không sửa parsers |
| **C. `Filesystem` struct giữ nguyên, thêm field `index` + `layerSpools`. `ReadFile` check index trước, `files` map sau.** | **Lowest** | **Không sửa gì cả** |

**Đề xuất: Approach C** — thêm fields vào `Filesystem` hiện tại, không cần interface mới.

```go
type Filesystem struct {
    files         map[string][]byte       // materialize mode (giữ nguyên)
    index         map[string]*IndexEntry  // indexed mode (nil khi materialize)
    layerSpools   []*os.File              // indexed mode: temp layer files
    // ... existing fields ...
}
```

`ReadFile` mới:

```
1. Check files map (materialize mode hoặc selective cache)
2. If index != nil && entry exists → lazy read from spool
3. Else → file not found
```

Discovery APIs (`FindPathsBySuffix`, `PathsUnder`, etc.):

```
1. If index != nil → iterate index keys (bỏ qua whiteout entries)
2. Else → iterate files keys (hiện tại)
```

→ **Zero breaking changes.** Parsers không cần sửa. `SBOM_FS_MODE=indexed` bật tính năng mới.

---

## Ước lượng RAM cho các loại image

| Image Type | Size (uncompressed) | Materialize (hiện tại) | Indexed + Selective | Giảm |
|-----------|---------------------|------------------------|---------------------|------|
| Alpine nginx | ~50MB | ~30MB | ~5MB | 83% |
| Debian + Python app | ~300MB | ~200MB | ~15MB | 92% |
| Node.js app | ~500MB | ~350MB | ~40MB (lockfile + node_modules json) | 88% |
| Java Spring Boot | ~800MB | ~550MB | ~20MB | 96% |
| ML/CUDA image | ~5GB | **1GB (capped)** | ~30MB | 97% |
| Go distroless | ~20MB | ~15MB | ~10MB + lazy binary reads | 33% |

---

## Rủi ro và mitigations

| Rủi ro | Mitigation |
|--------|-----------|
| Spool temp files chiếm disk | Cleanup trong `Close()` + `defer`. Max spool = max 2 layers (binary layers). |
| Tar offset drift (compression) | Spool từ **uncompressed** stream. Offset chính xác. |
| Parser gọi ReadFile cho file không trong selective list và không có spool | Return error. Parser đã handle `ReadFile` error gracefully (skip). |
| Performance regression (2 passes thay vì 1) | Pass 1 chỉ đọc headers (nhanh). Pass 2 selective (ít files). Net: tương đương hoặc nhanh hơn vì ít memory allocation. |
| Concurrent parser reads từ spool | Mutex per spool file hoặc `pread` (offset-based, thread-safe). |

---

## Env vars tổng kết

| Var | Default | Mô tả |
|-----|---------|-------|
| `SBOM_FS_MODE` | `materialize` | `indexed` bật Phase 2 |
| `SBOM_FS_MAX_TOTAL_BYTES` | 1GiB | Cap cho content cache (cả 2 modes) |
| `SBOM_FS_MAX_FILE_BYTES` | 64MiB | Cap per-file (cả 2 modes) |
| `SBOM_FS_SPOOL_DIR` | OS temp dir | Thư mục cho layer spool files |
| `SBOM_FS_SKIP_PATH_PREFIXES` | *(trống)* | Danh sách prefix (`,`); không materialize file dưới prefix (giảm RAM; ví dụ `/usr/share/doc`). `off` = tắt. **Đã implement.** |
| `SBOM_FS_METRICS` | *(bật)* | `0` / `off` / `false` = không log dòng `[SBOM FS]` (Phase 2c). |

---

## Test plan

| Test | Scope | Phase |
|------|-------|-------|
| Index build + whiteout (`.wh.`, `.wh..wh..opq`) | Unit | 2a |
| Selective extract: file match suffix → có content | Unit | 2a |
| Selective extract: file không match → không trong RAM, đọc qua spool (lazy) | Unit | 2a+2b |
| Discovery APIs dùng index keys | Unit | 2a |
| `SBOM_FS_MODE=materialize` → hành vi hiện tại không đổi | Regression | 2a |
| Lazy read từ spool: seek + read chính xác | Unit | 2b |
| Spool cleanup khi `Close()` | Unit | 2b |
| Concurrent lazy reads (race detector) | Unit | 2b |
| End-to-end: image lớn (>2GB) với indexed mode | Integration | 2b |
| Metrics log output | Unit | 2c ✅ |

---

## Thứ tự commit đề xuất

1. **A5-INDEX**: `Filesystem` thêm `index` field + `BuildIndex` + discovery dùng index + `SBOM_FS_MODE`
2. **A5-SELECTIVE**: Selective extract suffix allowlist + content cache cho tiny/medium files
3. **A5-SPOOL**: Layer spool + lazy `ReadFile` cho large/binary files
4. **A5-METRICS**: Log counters + doc update
5. **A5-INTERFACE** (optional): Extract `FilesystemReader` interface nếu cần mock trong tests
