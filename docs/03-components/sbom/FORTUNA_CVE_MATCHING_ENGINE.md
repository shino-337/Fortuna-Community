# FORTUNA_CVE_MATCHING_ENGINE.md

## 1. Objective

Thiết kế **Fortuna CVE Matching Engine** để:

* Map **SBOM components → CVE** với độ chính xác cao.
* Hỗ trợ các trường hợp:

  * container image chuẩn (apk/deb/rpm)
  * distroless image
  * static binary (Go / Rust / C)
  * Kubernetes control plane components
* Giảm false positive từ keyword matching.
* Giảm dependency vào external APIs.

Engine phải hoạt động **event-driven**, scalable, và phù hợp với kiến trúc hiện tại của Fortuna.

---

# 2. High-Level Architecture

```
Agent
   │
   ▼
SBOM Ingestion
   │
   ▼
SBOM Stored (Postgres)
   │
   ▼
Event: fortuna.sbom.created
   │
   ▼
CVE Matcher Worker
   │
   ├── Package Matcher (OSV)
   ├── Binary Dependency Matcher
   ├── Image Metadata Matcher
   ├── Kubernetes Component Matcher
   └── Heuristic / NVD Fallback
   │
   ▼
Persist CVE Matches
   │
   ▼
Risk Scoring Engine
   │
   ▼
Dashboard / API
```

Messaging layer sử dụng
NATS
với JetStream để đảm bảo durability.

---

# 3. Matching Pipeline

CVE matching được thực hiện theo **priority pipeline** (thiết kế mục tiêu).

## Pipeline order (design)

```
1. Package Manager Matching   ← Đã có (Postgres / package_vulnerabilities)
2. Binary Dependency Matching ← Go: đã có (GoBinaryParser, buildinfo, OSV mirror)
3. Image Metadata Matching    ← Metadata dùng ở Agent khi tạo SBOM; Core không bước riêng
4. Kubernetes Component Matching ← Component → module mapping (k8s_component_map), không bảng riêng (xem §7)
5. Heuristic / NVD Fallback   ← Đã có (whitelist + NVD API)
```

**Hiện tại:** Worker chạy (1) bulk Package/OSV — với ecosystem Go: **Go module matcher** (prefix + alias resolver) + **Go stdlib matcher** — sau đó (5) NVD fallback cho SBOM heuristic với component trong whitelist. Chi tiết đối chiếu: Phụ lục A.

---

# 4. Package Manager Matching

Áp dụng cho image có package manager.

Supported ecosystems:

| Ecosystem | Package Manager                |
| --------- | ------------------------------ |
| Linux     | apk / deb / rpm                |
| Language  | npm / pip / maven / cargo / go |

CVE database:

* Open Source Vulnerabilities
* Fortuna internal CVE DB

Matching logic:

```
SBOM component
    ↓
parse PURL
    ↓
ecosystem + package name
    ↓
query package_vulnerabilities
```

Example:

```
pkg:apk/alpine/openssl@3.0.8
```

Query:

```
ecosystem = alpine
package_name = openssl
```

---

# 5. Binary Dependency Matching

Áp dụng cho:

* distroless images
* static binaries
* Go compiled binaries

Ví dụ:

* CoreDNS
* etcd
* kube-apiserver

## Binary analysis

Agent hoặc worker sẽ parse binary:

```
ELF
PE
Mach-O
```

Đặc biệt với Go:

```
.buildinfo section
```

Extract:

```
module path
module version
```

Example output:

```
module: k8s.io/apimachinery v0.28.3
module: golang.org/x/net v0.17.0
```

Matching:

```
ecosystem = Go
package = module path
```

Query OSV.

## 5.1 Trạng thái hiện tại (P2-1 – Agent Go binary)

Sau P2-1, Agent đã có Go binary analyzer production-baseline:

- GoBinaryParser (`agent/pkg/sbom/extractor/gobinary.go`) scan filesystem dưới các thư mục binary phổ biến (`/bin`, `/usr/bin`, `/usr/local/bin`, `/usr/sbin`, `/sbin`, `/app`, `/`) với guardrail (`maxFilesScanned`, `maxBinarySize`) và dùng `debug/buildinfo.Read` để lấy **Go module graph** từ chính binary (không phụ thuộc go.mod).  
- Mỗi binary Go emit:
  - **Main module**: `Package{Type="go-binary", Source="gobinary-main", Name=bi.Main.Path, Version=bi.Main.Version, Confidence="high"}` (ví dụ `k8s.io/kubernetes/cmd/kube-apiserver@v1.29.2`).  
  - **Dependencies**: `Package{Type="go", Source="gobinary", Name=dep.Path, Version=dep.Version}`, với pseudo-version (`v0.0.0-…`, `v1.2.3-0.20…`) được gắn `Confidence="medium"`.  
- Extractor (`agent/pkg/sbom/extractor/extractor.go`) đã đăng ký parser `gobinary` và sau khi gom tất cả package từ các parser sẽ **deduplicate theo (Type, Name, Version)** trước khi build SBOM, nên control-plane có nhiều binary (kube-apiserver, controller-manager, scheduler…) nhưng dependency graph chỉ xuất hiện một lần.  
- Test integration (`TestGoBinaryParser_ParsesRealGoBinary`) build một binary Go thật từ `testdata/gobinary` (import `github.com/sirupsen/logrus`) và verify rằng SBOM có cả main binary (`go-binary`) và ít nhất một dependency `Type="go", Source="gobinary"`, đảm bảo luồng Go binary → buildinfo → SBOM thực sự hoạt động trên binary thật chứ không chỉ qua mock.

---

# 6. Image Metadata Matching

**Ghi chú:** Image metadata (os-release, labels) được dùng ở **Agent** khi tạo SBOM; Core không có bước matcher riêng cho metadata.

Một số image có metadata:

```
/etc/os-release
/usr/lib/os-release
```

hoặc label:

```
org.opencontainers.image.version
org.opencontainers.image.source
```

Fortuna sẽ:

```
detect base OS
map package ecosystem
```

Example:

```
ID=alpine
VERSION_ID=3.18
```

Mapping:

```
ecosystem = alpine
```

Sau đó query CVE.

---

# 7. Kubernetes Component Matching (component → module mapping)

Áp dụng cho control plane.

Components phổ biến: kube-apiserver, kube-controller-manager, kube-scheduler, etcd, CoreDNS.

**Hiện trạng:** Không dùng bảng `kubernetes_component_vulnerabilities`. Control-plane dùng **component → module mapping** (file `k8s_component_map.yaml`): ví dụ `kube-apiserver` → `k8s.io/kubernetes`, version lấy từ SBOM; matcher query OSV Go mirror với module prefix đó. Fallback: whitelist + NVD (§8) khi không có mapping.

---

# 8. Heuristic / NVD Fallback

Fallback cuối cùng khi các matcher khác không trả kết quả.

Query:

```
keywordSearch=<component name>
```

API:

National Vulnerability Database

Restrictions:

```
component must be in whitelist
confidence < high
```

Example whitelist:

```
coredns
etcd
kube-apiserver
runc
openssl
```

Result flagged:

```
match_source = nvd-fallback
confidence = low
```

---

# 9. Matching Confidence Model

Fortuna gắn confidence cho mỗi match. Có thể suy từ MatchedBy/source; dashboard/API map source → confidence (SBOM detail API trả field `confidence`: high | low).

| Match Type           | Confidence |
| -------------------- | ---------- |
| package manager      | high       |
| binary dependency    | high       |
| image metadata       | medium     |
| kubernetes component | medium     |
| nvd fallback         | low        |

Dashboard hiển thị confidence để tránh hiểu nhầm.

---

# 10. Match Deduplication

Tránh duplicate CVE:

```
unique key:
(sbom_id, component_name, cve_id)
```

SQL:

```
ON CONFLICT DO NOTHING
```

---

# 11. Performance Optimizations

## Batch matching

Group components:

```
SELECT vulnerabilities
WHERE ecosystem = ?
AND package_name IN (...)
```

## CVE cache (OSV / NVD)

Cache key:

- **OSV mirror (Go):** `ecosystem:package:mirror_version` (mirror_state.version); khi mirror sync xong gọi `UpdateDatabase` → tăng version → cache miss tự động.
- **Khác / single query:** `ecosystem:package:version` hoặc `ecosystem:package:*` (bulk).

TTL:

```
30 phút (entry hết hạn thì Get() xóa và trả miss).
```

## Worker parallelism

```
max_workers = cpu_count * 2
```

---

# 12. Error Handling

Worker retry logic:

```
JetStream retry
max_retry = 5
```

Failure scenarios:

| Error          | Handling            |
| -------------- | ------------------- |
| DB unavailable | retry               |
| NVD rate limit | exponential backoff |
| SBOM corrupted | mark failed         |

---

# 13. Observability

Metrics:

```
fortuna_cve_matches_total
fortuna_cve_match_latency
fortuna_nvd_queries_total
```

Logs:

```
matcher stage
component
match source
```

Tracing:

```
SBOM → matcher → DB write
```

---

# 14. Security Considerations

* sanitize external API responses
* rate limit NVD calls
* validate SBOM inputs
* isolate matcher worker privileges

---

# 15. Future Enhancements

### EPSS integration

Risk scoring từ

FIRST.org

### KEV detection

CVE nằm trong

Cybersecurity and Infrastructure Security Agency

Known Exploited Vulnerabilities.

### Supply chain correlation

Map CVE với:

```
container runtime
node OS
cluster version
```

để tạo attack path graph.

---

# 16. Expected Outcomes

Sau khi triển khai engine này, Fortuna có thể:

* match CVE chính xác cho distroless images
* detect vulnerabilities trong Kubernetes control plane
* giảm false positives từ NVD keyword search
* scale tới cluster lớn.

---

# Phụ lục A: Phân tích đáp ứng đề xuất và kế hoạch xử lý

## A.1 Bảng đối chiếu đề xuất vs hiện trạng

Mỗi đề xuất trong doc được đối chiếu với code/luồng thực tế; nếu đã có hoặc làm tốt hơn thì ghi rõ bằng chứng (file, hành vi).

| § | Đề xuất | Hiện trạng | Chứng minh / Ghi chú |
|---|---------|------------|----------------------|
| **1** | Map SBOM → CVE chính xác; container apk/deb/rpm, distroless, static binary, control-plane; event-driven, scalable. | **Một phần** | Fortuna match tốt **container package** (OSV); distroless/control-plane thực tế vẫn là **heuristic matching**. **Nói thẳng:** Fortuna chưa có SBOM thực sự cho distroless—chỉ có **component name + version** → ảnh hưởng lớn tới accuracy CVE. Event: `fortuna.sbom.created` → cve_matcher_worker Process(). |
| **2** | Architecture: Agent → SBOM → Event → CVE Matcher (5 nhánh) → Persist → Risk → Dashboard; NATS JetStream. | **Đạt** | Luồng đúng. Worker subscribe `fortuna.sbom.created` (main.go js.Subscribe, AckWait 2min). Chỉ 2 nhánh thực tế: Package Matcher (OSV) + Heuristic/NVD. |
| **3** | Pipeline 5 bước, dừng ở bước đầu có match. | **Khác thiết kế** | **Thiết kế:** 5 matchers. **Thực tế:** OSV bulk → NVD fallback. **Ý nghĩa:** Fortuna hiện tại = **Package vulnerability matcher**, chưa phải full vulnerability engine. |
| **4** | Package Manager: PURL → ecosystem + package_name → query CVE DB (OSV, Fortuna DB). | **Đạt** (có nuance) | `matcher.go`: ParsePURL, normalizeQueryEcosystemWithOS(purl, sbom.OSName), GetVulnerabilitiesForPackages. **Nuance:** OSV ecosystem mapping đôi khi sai khi purl ecosystem ≠ OSName (vd: `pkg:generic/openssl` + OSName=alpine). Normalize không đúng → query có thể miss CVE. |
| **5** | Binary Dependency: ELF/Go buildinfo → module path+version → OSV Go. | **Chưa có** | Không có code parse ELF/buildinfo trong agent hay core. `agent/pkg/sbom/extractor/gomod.go` parse **go.mod** file, không parse binary. |
| **6** | Image Metadata: os-release, labels → base OS, ecosystem → query CVE. | **Một phần** | **Làm rõ:** metadata detection ≠ vulnerability matching. Agent chỉ giúp **detect version** (os-release, labels); matcher vẫn chỉ chạy OSV (+ NVD fallback). Không có bước “Image Metadata Matcher” riêng trong Core. |
| **7** | Kubernetes Component: bảng `kubernetes_component_vulnerabilities`, query component+version. | **Chưa có** | Bảng này không tồn tại trong migrations. Control-plane: whitelist (coredns, kube-*, etcd…) + NVD fallback (§8). |
| **8** | Heuristic/NVD: keywordSearch=name, whitelist, confidence < high, match_source=nvd-fallback. | **Đạt (code); yếu (security model)** | Về code: đúng (useNVDFallbackForHeuristic, whitelist, normalizeComponentNameForNVD, MatchedBy=nvd-fallback). **Điểm yếu:** NVD API keywordSearch=name **không có filter version range** → false positive + false negative. Roadmap cần bước **giảm phụ thuộc NVD**. |
| **9** | Confidence model: package manager=high, …, nvd=low; dashboard hiển thị. | **Một phần** | Không có cột confidence trong DB. Có thể suy từ MatchedBy (nvd-fallback → low, fortuna-core-cve-matcher → high). API trả `source`; dashboard có thể map source → confidence. |
| **10** | Dedup: unique (sbom_id, component_name, cve_id), ON CONFLICT DO NOTHING. | **Đạt** | `cve_matcher_worker.go` persistMatches: OnConflict{Columns: sbom_id, package_name, cve_id, DoNothing: true}. Migration 023/025/033: unique index cve_matches(sbom_id, package_name, cve_id). |
| **11** | Batch; cache TTL; Worker parallelism. | **Batch + cache đạt; parallelism chưa** | Batch: GetVulnerabilitiesForPackages. **Cache:** TTL 30 phút, Get() kiểm tra expiresAt; OSV cache key gồm mirror_state.version → sync xong gọi UpdateDatabase tăng version → cache miss. Worker: 1 subscriber, chưa scale. |
| **12** | JetStream retry; DB retry; NVD 429 backoff; SBOM corrupted. | **Một phần** | **Quan trọng:** JetStream redelivery **không thay thế** retry logic bên trong worker. Nếu NVD rate limit → không Ack → redelivery = **spam NVD** (cùng message retry liên tục). NVD 429: retry 1 lần sau Retry-After. |
| **13** | Metrics: fortuna_cve_matches_total, latency, fortuna_nvd_queries_total. | **CVE đạt; thiếu NVD** | fortuna_cve_matches_total, fortuna_cve_matching_duration_seconds đã instrument. **Thiếu fortuna_nvd_queries_total**—rất cần vì NVD thường là **bottleneck**. |
| **14** | Security: sanitize API response, rate limit NVD, validate SBOM, isolate worker. | **Một phần** | NVD: parse JSON có kiểm tra; không sanitize sâu. Rate limit: 429 → retry 1 lần; NVD_API_KEY tăng limit. SBOM: load components Where deleted_at IS NULL. Worker chạy trong process Core, không isolate riêng. |
| **15** | Future: EPSS, KEV, supply chain correlation. | **Chưa** | Không nằm trong scope hiện tại. |
| **16** | Outcomes: distroless CVE, control-plane detect, ít false positive, scale. | **Một phần** | CVE qua NVD fallback; false positive/negative do NVD keyword; scale nhờ batch + cache. |

---

## A.2 Những vấn đề kiến trúc chưa nêu trong bảng

Bốn điểm quan trọng chưa được đề cập trong bảng đối chiếu §:

**A.2.1 Version constraint của NVD bị bỏ qua**  
NVD fallback hiện tạo CVE với `Constraint = ""`. NVD CVE thường có `versionStartIncluding` / `versionEndExcluding`, nhưng fallback dùng **keyword search** → không check version. Ví dụ: component coredns 1.11.1, CVE áp dụng coredns &lt; 1.9 — Fortuna vẫn match (false positive). Đây là **logic flaw** trong model.

**A.2.2 Component normalization risk**  
Whitelist đã có normalize (vd: `normalizeComponentNameForNVD`). Rủi ro: component name có thể là `registry.k8s.io/coredns`, `coredns/coredns`, `coredns`. Normalize không tốt → **whitelist bypass** (không gọi NVD) hoặc **duplicate CVE match** (cùng CVE gắn nhiều lần do tên khác nhau).

**A.2.3 SBOM immutability**  
Soft-delete race (component `deleted_at` set sau khi publish event) là vấn đề **data model** lớn hơn matcher. SBOM đúng chuẩn nên **immutable**: components không bị sửa sau khi lưu; nếu không, event matching sẽ **không deterministic**.

**A.2.4 NVD dependency risk**  
Platform security không nên phụ thuộc runtime vào NVD API (rate limit, latency, availability). Best practice: **mirror NVD locally** (sync định kỳ) thay vì gọi API realtime.

---

## A.3 Kế hoạch xử lý và bổ sung

Ưu tiên: **P0** (đã đúng/ổn, chỉ chuẩn hóa doc hoặc nhỏ), **P1** (bổ sung nhanh, tác động rõ), **P2** (cải tiến lớn hoặc sau).

### P0 – Chuẩn hóa / không thay đổi code

| ID | Nội dung | Hành động |
|----|----------|-----------|
| P0-1 | §3 Pipeline | Cập nhật doc: ghi rõ “Pipeline order (design)”; “Hiện tại chỉ 2 bước: Package/OSV + NVD fallback”. |
| P0-2 | §7 Kubernetes Component | Ghi trong doc: “Chưa triển khai bảng; control-plane dùng whitelist + NVD fallback”. |
| P0-3 | §9 Confidence | Giữ thiết kế; ghi “Có thể suy từ MatchedBy/source; dashboard map source → confidence nếu cần”. |

### P1 – Bổ sung trong thời gian ngắn

| ID | Nội dung | Hành động |
|----|----------|-----------|
| P1-1 | §13 Metric NVD | ✅ Đã làm: metric `fortuna_nvd_queries_total`; tăng khi gọi NVD từ fallback Postgres (heuristic SBOM). Luồng CVE chỉ Postgres (OSV) + NVD; nhánh Trivy đã xóa. |
| P1-2 | §11 NVD cache TTL | ✅ Đã làm: CVECache lưu entry kèm `expiresAt`; Get() kiểm tra TTL, hết hạn thì xóa và trả miss. |
| P1-3 | §9 Confidence trên API | ✅ Đã làm: SBOM detail API trả `confidence` (derived): nvd-fallback → "low", fortuna-core-cve-matcher → "high". |
| P1-4 | NVD query normalization | ✅ Đã có: `normalizeComponentNameForNVD` dùng khi gọi NVD (matcher NVD fallback loop) và trong whitelist; registry alias `registry.k8s.io/coredns` → coredns. |
| P1-5 | Component snapshot | ✅ Đã làm: event `sbom.created` có `components_snapshot` (name, version, purl). Worker dùng snapshot khi có; matcher MatchSBOM(ctx, sbom, componentsOverride) nhận override, tránh race soft-delete. |

### P2 – Cải tiến lớn / roadmap

| ID | Nội dung | Hành động |
|----|----------|-----------|
| P2-1 | §5 Binary Dependency | **Ưu tiên Go binary trước** (control-plane Kubernetes, distroless đa số là Go). **Detect Go binary trước khi parse buildinfo** (Go build ID / `.buildinfo` trong .rodata), tránh parse mọi ELF; handle **Go pseudo-version** (v0.0.0-yyyymmdd-commit) bằng normalize/commit-based lookup trước khi query OSV Go. |
| P2-2 | §7 Kubernetes Component | Có thể **không cần bảng riêng** `kubernetes_component_vulnerabilities`. Thay vào: map **component → module prefix** (vd: `kube-apiserver` → `k8s.io/kubernetes`), **version lấy từ SBOM**; matcher dùng module này thay vì NVD keyword. |
| P2-3 | §6 Image Metadata “matcher” | ✅ Đã ghi trong §6: “Image metadata được dùng ở Agent khi tạo SBOM; Core không cần bước matcher riêng”. |
| P2-4 | §12 NVD 429 + redelivery | ✅ Đã làm: NVD client sau 429 đợi Retry-After rồi retry; nếu vẫn 429 thì exponential backoff (30s, 60s, 120s) tối đa 3 lần rồi mới trả lỗi, tránh redelivery spam. Roadmap thêm: **global rate limiter** (vd. token bucket) dùng chung giữa nhiều worker để tránh cùng lúc tất cả instance đều retry vào NVD. |
| P2-5 | §11 Worker scale | Scale bằng **multiple JetStream consumers** (nhiều replica Core hoặc nhiều subscription) thường đơn giản hơn worker pool trong process. CVE matcher **IO-bound nặng, CPU nhẹ** → mỗi replica chỉ cần số goroutine vừa phải (vd. ~2×CPU) thay vì mở rộng quá nhiều goroutine trong một process. |
| P2-6 | §15 EPSS/KEV | Roadmap: EPSS score, KEV list (CISA); cột/API exploit/KEV. **Tách rõ:** technical confidence (độ tin cậy của match) ≠ risk prioritization (EPSS/KEV); không trộn chung một field. |
| P2-7 | A.2.4 NVD dependency | **Ưu tiên cao nhất:** Giảm phụ thuộc runtime NVD: **mirror NVD locally** (sync định kỳ), matcher query mirror thay vì gọi API. Mirror không nên giữ raw JSON mà **flatten** thành bảng (vd. `nvd_vulnerabilities` với package, ecosystem, version range, CVSS) + bảng `cpe_to_package` để map `cpe:2.3:a:openssl:openssl` → `openssl`. Cron sync ~4h là đủ (NVD update ~2h). |

---

## A.4 Tóm tắt

* **Đã đáp ứng tốt:** §2 architecture, §4 Package Manager (có nuance ecosystem), §8 Heuristic/NVD (code), §10 Dedup, §11 batch, §13 metrics CVE.  
* **Đáp ứng một phần:** §1 (distroless = heuristic, chưa SBOM thực), §6 (metadata detection ≠ matching), §9 (confidence suy từ source), §12 (redelivery ≠ retry, spam NVD risk), §14.  
* **Chưa đáp ứng / Điểm yếu:** §5 Binary Dependency, §7 bảng K8s (có thể thay bằng component→module), §8 security model (NVD không filter version), A.2.1–A.2.4 (version constraint, normalization risk, SBOM immutability, NVD dependency).  

Kế hoạch: P0 (chuẩn hóa doc) → P1 (metric NVD, cache TTL, confidence API, P1-4 normalization, P1-5 component snapshot) → P2 (ưu tiên: **NVD mirror** → **Binary Go** → **K8s component/module** → **worker scale** → **EPSS/KEV**).
