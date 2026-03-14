# Kế hoạch xử lý Architecture Findings (03142026)

**Nguồn:** [Architecture_Finding_03142026.md](./Architecture_Finding_03142026.md)  
**Mục tiêu:** Phân tích hiện trạng, đánh giá và lập kế hoạch xử lý từng finding với thứ tự ưu tiên và task cụ thể.

---

## Tổng quan ưu tiên

| # | Finding | Risk | Ưu tiên | Effort ước lượng |
|---|---------|------|--------|-------------------|
| 3 | Full sync xóa dữ liệu khi payload thiếu | Critical | P0 | Medium |
| 2 | SBOM queue key + parse image ref | High | P1 | Medium |
| 1 | Core không cluster-aware (WS, dedup) | High | P1 | High |
| 8 | SBOM không cover distroless/system pods | Medium (gap) | P2 | High |
| 6 | Multi-cluster thiếu isolation / rate limit | High | P2 | Medium |
| 4 | mTLS rotation thủ công | Medium | P2 | Medium |
| 5 | Observability thiếu tracing / audit context | Medium | P3 | Medium |
| 7 | Deploy phụ thuộc thứ tự thủ công | Low | P3 | Low |

---

## Finding #3: Full sync xóa toàn bộ resource khi payload thiếu (Critical)

### Hiện trạng đã xác minh

- **File:** `core/internal/service/agent_service.go`
- **Logic:** Khi full sync mà key tương ứng **vắng mặt hoặc mảng rỗng** (`!ok || len(arr) == 0`), Core **xóa toàn bộ** resource của cluster đó:
  - **ServiceAccounts:** `processSyncedServiceAccounts` (L320–333): `data["serviceAccounts"]` empty + `isFullSync` → delete all SAs của cluster.
  - **Pods:** `processSyncedPods` (L908–912): `data["pods"]` empty → `Delete(&models.Pod{})` theo cluster.
  - **Roles:** `processSyncedRoles` (L492–496): empty → delete all roles.
  - **ClusterRoles:** `processSyncedClusterRoles` (L605–609): empty → delete all cluster roles.
  - **RoleBindings:** `processSyncedRoleBindings` (L697–701): empty → delete all role bindings.
  - **ClusterRoleBindings:** `processSyncedClusterRoleBindings` (L804–808): empty → delete all CRBs.
- **Nguyên nhân:** Thiết kế “full sync = source of truth”: coi empty = “cluster không còn resource đó”, không phân biệt “agent không gửi được” vs “thật sự empty”.

### Đánh giá

- **Impact:** Lỗi thu thập tạm thời (RBAC, throttle, network) có thể dẫn đến xóa sạch SAs/Pods/Roles/… → mất dữ liệu, audit trail, dashboard sai.
- **Risk:** Critical – đúng như finding.

### Kế hoạch xử lý

| Bước | Nội dung | Chi tiết |
|------|----------|----------|
| 3.1 | Không xóa khi payload thiếu | Với mỗi `processSynced*`: nếu `!ok \|\| len(arr)==0` **không** gọi Delete all. Chỉ log warning (và metric nếu có). Return nil. |
| 3.2 | Delete chỉ khi có danh sách đầy đủ | Chỉ thực hiện “delete những record không còn trong sync” khi payload **có** mảng tương ứng và đã parse thành công (syncedUIDs/syncedKeys). Giữ logic hiện tại: “trong DB nhưng không trong syncedUIDs → delete”. |
| 3.3 | Optional: soft-delete + reconciliation | (Phase 2) Có thể thêm job nền: sau N lần full sync liên tiếp mà một resource vẫn “absent” trong payload mới soft-delete hoặc đánh dấu stale; không làm trong P0. |
| 3.4 | Alerting | Log rõ khi payload thiếu key (ví dụ `[AgentService] WARN full sync missing key 'serviceAccounts' for cluster X, skipping delete`). Có thể thêm metric `fortuna_sync_missing_payload_total{resource="serviceAccounts",cluster_id="..."}`. |
| 3.5 | Test | Unit test: full sync với `data["pods"] = nil`, `data["pods"] = []` → không xóa pod nào. Tương tự cho SAs, Roles, RoleBindings, ClusterRoles, ClusterRoleBindings. |

**File cần sửa:** `core/internal/service/agent_service.go` (tất cả processSynced* nêu trên).  
**Test:** `core/internal/service/agent_service_pod_detail_test.go` và bổ sung test cho SA/Role sync.

---

## Finding #2: Agent SBOM queue key + image ref parsing (High)

### Hiện trạng đã xác minh

- **Queue key:** `agent/internal/sbom/queue.go` L76–77, L134: key = `pod.Namespace + "/" + pod.Name`. Pod mới (cùng namespace/name, UID khác) bị coi là “đã active” → skip đến khi key được release (sau khi worker xong).
- **parseImageRef:** `agent/internal/sbom/processor.go` L151–169: cắt theo **last colon**; không xử lý `registry:5000/image` (port nhầm thành tag), không tách digest `@sha256:...`. Proto vẫn gửi `ImageDigest` từ extractor (rawSBOM.ImageDigest), nhưng ImageName/ImageTag có thể sai với registry có port hoặc ref dạng digest.
- **Extractor:** `agent/pkg/sbom/extractor/extractor.go` đã dùng thư viện reference và digest đúng; vấn đề nằm ở processor khi convert sang proto (parseImageRef chỉ dùng cho name/tag).

### Đánh giá

- **Impact:** High – pod recycle nhanh (cùng name) có thể bỏ qua SBOM; image identity sai làm CVE/dedupe không tin cậy.
- **Risk:** High – đúng như finding.

### Kế hoạch xử lý

| Bước | Nội dung | Chi tiết |
|------|----------|----------|
| 2.1 | Key queue theo UID | Đổi key từ `namespace/name` sang `string(pod.UID)` (hoặc `clusterID/uid` nếu có). Cập nhật `active`, `retryCount`, log. Đảm bảo release key sau khi worker xong vẫn dùng cùng key (UID). |
| 2.2 | Parser image chuẩn | Thay `parseImageRef` bằng parser chuẩn: dùng `github.com/google/go-containerregistry/pkg/name` (agent đã dùng trong extractor) hoặc `docker/distribution/reference` để parse ref. Tách đúng: registry (kể cả host:port), repo, tag, digest. |
| 2.3 | Proto / Core | Đảm bảo Core lưu/so sánh image bằng digest khi có; tag/name dùng cho hiển thị. Kiểm tra model SBOM/Finding có digest; nếu thiếu thì bổ sung và dùng làm identity khi dedupe. |
| 2.4 | Test | Unit test: Enqueue hai pod cùng namespace/name, UID khác → cả hai đều được queue (hoặc ít nhất không bị skip vĩnh viễn). Test parseImageRef: `reg:5000/ns/img:tag`, `img@sha256:xxx` → name/tag/digest đúng. |

**File cần sửa:** `agent/internal/sbom/queue.go`, `agent/internal/sbom/processor.go`; kiểm tra `core` model/handler SBOM nếu cần dùng digest làm identity.

---

## Finding #1: Core không cluster-aware cho stateful workers và WebSocket (High)

### Hiện trạng đã xác minh

- **Docs:** COMPONENTS.md, DEPLOYMENT_AND_ARCHITECTURE_FAQ: Core replicas dùng chung PostgreSQL/NATS, không có Redis/leader election; dedup và WebSocket hub in-memory per process.
- **Code:** `core/internal/api/risks_ws_hub.go`: `defaultRisksHub` in-memory, map connections; không pub/sub shared. Pod detail WS tương tự. Dedup (ví dụ pod detail ingest) theo instance.
- **Hệ quả:** Replica fail hoặc scale → sticky session mất, client WS mất kết nối; có thể duplicate event (dedup không shared).

### Đánh giá

- **Impact:** High – đúng như finding (session break, duplicate, không failover mượt).
- **Risk:** High. Effort lớn (Redis hoặc NATS pub/sub + có thể leader election).

### Kế hoạch xử lý

| Bước | Nội dung | Chi tiết |
|------|----------|----------|
| 1.1 | Document + sticky session | Ghi rõ trong DEPLOYMENT_AND_ARCHITECTURE_FAQ/COMPONENTS: “Core replicas: WebSocket và dedup in-memory; để tránh disconnect nên dùng sticky session (cookie hoặc LB). Sticky là optional cho correctness nhưng recommended cho UX.” |
| 1.2 | Correlation ID (chuẩn bị cho 1.3) | Propagate request/correlation ID từ Agent → Core → NATS → worker; log và metric theo ID. Làm nền cho tracing và sau này shared state. |
| 1.3 | WebSocket qua pub/sub | Chọn broker: NATS (đã có) hoặc Redis. Khi insight/risk update: publish event; mỗi Core replica subscribe và push tới WS clients local. Client chỉ cần kết nối bất kỳ replica nào. |
| 1.4 | Dedup shared | Dedup (ví dụ pod detail message_id): dùng Redis set/key TTL hoặc NATS JetStream consumer ack/dedup. Cần cấu hình Redis (hoặc quy ước subject + message ID trong NATS). |
| 1.5 | Leader election (optional) | Chỉ cần nếu có job “chạy trên một replica” (ví dụ cleanup định kỳ). Có thể dùng Postgres advisory lock hoặc NATS KV/leader election. Triển khai sau khi 1.3–1.4 ổn định. |

**Thứ tự đề xuất:** 1.1 (doc + sticky) → 1.2 (correlation ID) → 1.3 (WS pub/sub) → 1.4 (dedup). 1.5 khi có nhu cầu job single-active.

---

## Finding #8: SBOM không cover distroless/system pods (Enhancement)

### Ngữ cảnh workflow SBOM hiện tại

- **Agent:** `agent/internal/watcher/pod_watcher_local.go` watch pods, queue Running pods; `agent/internal/sbom/processor.go` xử lý từng container.
- **Extractor:** `agent/pkg/sbom/extractor/extractor.go` – materialize image (containerd local-first), rebuild filesystem, `detectOS()` (từ `/etc/os-release`, debian_version, alpine-release), `selectParsersForOS()` chọn dpkg/apk/rpm/npm/pip/gomod; output `RawSBOM` (ImageName, ImageDigest, OS, Packages).
- **Core:** Nhận SBOM, persist pod/image metadata, fan-out CVE/insight workers và dashboard. Docs: `docs/02-architecture/`, `docs/03-components/podDetail/`, deploy/README (agent–core mTLS).

### Ràng buộc pipeline CVE (NVD) – giữ nguyên pipeline, chỉ đảm bảo định danh

- **Nguồn CVE của Fortuna vẫn là NVD:** CVE loader cào NVD (vd. `core/pkg/scanner/cve_updater.go`, `core/pkg/cve/database/nvd/client.go`); `core/pkg/cve/database/manager.go` đã mô hình “Trivy DB ưu tiên, NVD API fallback”. Các bảng `cves`, `package_vulnerabilities`, `insights` dùng **package_name / version / ecosystem** tương thích NVD. **Pipeline CVE không đổi** – không thay CVE loader hay manager.
- **Vấn đề distroless là nguồn metadata:** Distroless không có dpkg/apk nên agent hiện trả packages rỗng. Để NVD còn **match** được, agent cần:
  - **Sinh Package có Name, Version, Ecosystem, PURL** từ heuristics/inference trong `agent/pkg/sbom/extractor` (OCI labels, layer history, binary tên cố định, signature DB).
  - **Gắn Source/Confidence** (vd. `distroless-heuristic`) để Core biết đây là heuristic và chọn đúng luồng (Trivy DB hay NVD API) khi matching.
  - **Mỗi package heuristic phải map thành PURL chuẩn** (vd. `pkg:generic/kube-apiserver@1.29.0`); Core dùng **package_name + version** để join với `package_vulnerabilities` (nuôi từ NVD). Miễn là agent gửi name/version hợp lệ thì CVE matching hiện có vẫn dùng được.
- **Version "unknown":** Nếu heuristic không xác định được version chính xác, vẫn gửi package với version `"unknown"`; Core có thể fallback NVD API (manager đã có logic) để query theo CPE/name khi cần.
- **Tài liệu tham chiếu:** `docs/03-components/sbom/CUSTOM_SBOM_ZERO_DEPENDENCY_PART2.md` (hybrid Trivy DB + NVD API, CycloneDX-like); CVE loader/implementation docs (CVE_MASTER_IMPLEMENTATION_GUIDE hoặc tương đương) mô tả NVD daily load và schema `package_vulnerabilities`.

### Phân tích: System pods (CoreDNS, agent, core) “không lấy được SBOM”

- **Pod nào được queue:** Watcher lọc theo `spec.nodeName` (chỉ pod trên cùng node với Agent) và `Phase == Running`, không loại namespace. Pod Core, Agent, CoreDNS được đưa vào SBOM queue nếu chạy trên cùng node.
- **Hai lý do “không có SBOM”:** (1) **Extract lỗi:** image không có trên node / registry fail → `ExtractSBOM` error → không gửi SBOM (có thể thử `SBOM_PREFER_REGISTRY=1`). (2) **SBOM rỗng:** image distroless/minimal → 0 packages; agent vẫn gửi SBOM (0 packages); đã xử lý bằng synthetic component (xem “Đã thực hiện” bên dưới).

### Vấn đề

- **Distroless / system images** (kube-apiserver, CoreDNS, etc.) thường không có package DB (dpkg/apk/rpm); không có `/etc/os-release` hoặc có nhưng không map được parser → `detectOS` trả "unknown", `selectParsersForOS` chạy “all parsers” nhưng vẫn không có package list → SBOM rỗng hoặc rất ít. Mục tiêu: **cover system/distroless pods mà không phụ thuộc tool ngoài** (Trivy/Syft), tăng cường extractor nội bộ để sinh **Package có định danh đủ cho NVD matching** (Name, Version, Ecosystem, PURL).

### Hiện trạng đã xác minh

- **detectOS:** Chỉ đọc filesystem (`/etc/os-release`, debian_version, alpine-release); không đọc OCI config/labels/layer history; không nhận diện distroless (binary-only, busybox-style).
- **RawSBOM:** Chỉ có `ImageName`, `ImageDigest`, `OS`, `Packages`, `ExtractedAt`; không có `Source` (parsers vs heuristic vs label) hay `Confidence` (low/medium/high).
- **Parser:** Chỉ có package-manager parsers; không có plugin “distroless” quét binary/lib hoặc đọc metadata có sẵn trong image.
- **Cache:** Digest đã dùng trong extractor (ImageDigest); không có cache on-disk theo digest (`/var/lib/fortuna/sbom-cache`) để bỏ qua re-scan khi digest trùng.
- **Core/Dashboard:** Không lưu/hiển thị nguồn SBOM (heuristic vs parser) hay cảnh báo “inferred SBOM”.

### Đánh giá

- **Impact:** System/distroless pods gần như “mù” trên Risk Center và SBOM view; CVE/insight thiếu cho workload quan trọng (control-plane, CoreDNS).
- **Risk:** Medium (gap chức năng, không phải lỗi data hiện tại). Effort High (extractor + parser mới + proto/model + UI + cache + docs + e2e).

### Kế hoạch xử lý (SBOM standardization – distroless/system)

| Bước | Nội dung | Chi tiết |
|------|----------|----------|
| **8.1** | **Image ingestion & detection (extractor)** | Giữ containerd-first, registry fallback, tempfile materialization. Mở rộng `detectOS()`: (1) đọc OCI config labels (vd. `org.opencontainers.image.ref.name`) và layer history; (2) nhận diện distroless (binary-only layers, thư mục kiểu busybox). Luôn tính và cache chuỗi digest `img.Digest()` để sau này skip re-scan khi digest trùng. |
| **8.2** | **Heuristic/distroless cataloger + PURL** | Thêm parser “distroless” implement interface Parser: chạy khi không parser nào trả về gói (hoặc OS = "unknown"). (1) Quét filesystem ảo: `/bin`, `/usr/bin`, `/usr/local/sbin`, `/lib`, `/usr/lib` → tạo **Package với Name, Version, Ecosystem** (name từ binary/ref name, version từ layer history hoặc `"unknown"`). (2) **Mỗi package phải có PURL chuẩn** (vd. `pkg:generic/kube-apiserver@1.29.0`) để Core join với `package_vulnerabilities` (NVD). (3) Nếu có %DISTROLLESS_METADATA% (CycloneDX/Conan trong layer) thì parse trực tiếp và emit component giống CycloneDX để Core dùng NVD matching. |
| **8.3** | **Language/runtime inference** | Khi có `/usr/local/go` hoặc `python3` nhưng không có package manager data: emit package gomod/pip/npm suy từ `go.sum`/`requirements.txt`/`package-lock.json` trong layer. Dùng signature DB nhúng (`agent/pkg/sbom/signatures/`): glob path + known binary version file → **Name, Version, Ecosystem, PURL** có cấu trúc tương thích NVD. |
| **8.4** | **Source/Confidence + Core luồng matching** | Mở rộng `RawSBOM` và proto `SBOMFinding`: thêm `Source` (parsers | distroless-heuristic | label-metadata) và `Confidence` (low | medium | high). Agent đánh dấu entry heuristic; **Core dùng Source để chọn luồng**: Trivy DB trước, NVD API fallback (đã có trong `core/pkg/cve/database/manager.go`). Khi version = `"unknown"`, Core vẫn ghi package và có thể gọi NVD API (CPE/name query) nếu cần. Dashboard: badge “Distroless SBOM (heuristic)” và ghi chú. |
| **8.5** | **Caching & dedup (on-disk)** | Cache SBOM theo digest tại thư mục local (vd. `/var/lib/fortuna/sbom-cache`). Entry gồm timestamp + version signature DB. Khi digest trùng: bỏ qua rebuild filesystem, dùng lại package list từ cache; chỉ re-scan khi signature DB version đổi. |
| **8.6** | **Core persistence & Dashboard** | **Core:** Ghi package từ SBOM (kể cả heuristic) vào sbom_components / flow hiện có; join **package_name, version, ecosystem** với `package_vulnerabilities` (NVD) trong `pkg/cve/database`; nếu version `"unknown"` thì dùng fallback NVD API đã có trong manager. Migration + model lưu source, heuristic flag, layer digests, signature_db_version. **Dashboard:** Pod Detail hiển thị “Distroless SBOM (heuristic)” và inference method; Risk Center flag/alert khi heuristic SBOM thay đổi. |
| **8.7** | **Docs & CUSTOM_SBOM alignment** | Cập nhật `docs/03-components/podDetail/POD_DETAIL_SPEC.md`, `docs/AGENT_CORE_CONNECTIVITY.md`; tham chiếu `docs/03-components/sbom/CUSTOM_SBOM_ZERO_DEPENDENCY_PART2.md` (hybrid Trivy DB + NVD, CycloneDX-like). Ghi rõ: distroless heuristic tạo metadata đủ định danh (Name/Version/Ecosystem/PURL) để Core match NVD; logic deterministic, versioned (signature DB version per SBOM entry). |
| **8.8** | **Integration / E2E** | Thêm pod test (vd. `registry.k8s.io/coredns` hoặc distroless mẫu) trong e2e; verify heuristic trả ít nhất Name/Version/PURL, dữ liệu đi qua Core và CVE matching (package_vulnerabilities hoặc NVD API) tới insight/dashboard. |

**File / thư mục liên quan:**  
`agent/pkg/sbom/extractor/extractor.go`, `agent/internal/sbom/processor.go`, `agent/pkg/sbom/extractor/` (parser distroless + interface), `agent/pkg/sbom/signatures/` (JSON signature DB), proto SBOM + `core/pkg/models` + migrations, **`core/pkg/cve/database/manager.go`**, **`core/pkg/cve/database/nvd/client.go`**, `dashboard/pages/PodDetail.tsx`, `docs/03-components/podDetail/POD_DETAIL_SPEC.md`, `docs/AGENT_CORE_CONNECTIVITY.md`, **`docs/03-components/sbom/CUSTOM_SBOM_ZERO_DEPENDENCY_PART2.md`**, e2e manifests (CoreDNS/distroless job).

**Phụ thuộc:** Hoàn thành Finding #2 (queue UID + image parser) giúp identity image ổn định (digest) trước khi mở rộng cache và heuristic; có thể làm song song 8.1–8.4 với #2, rồi 8.5–8.8 sau. **Pipeline CVE (NVD loader, manager, package_vulnerabilities) giữ nguyên** – chỉ đảm bảo agent gửi đủ định danh để Core match được.

**Đã thực hiện (phase 1 – synthetic component):** Khi tất cả parser trả 0 package (distroless/system: CoreDNS, agent, core, scratch), extractor **thêm một synthetic package** (Name = segment cuối của image repo, Version = tag hoặc digest, Type = generic). Core nhận SBOM có ít nhất 1 component; dashboard và CVE pipeline (NVD) có thể dùng name/version để match. File: `agent/pkg/sbom/extractor/extractor.go` (`syntheticPackageFromImage`, gọi sau bước dedup khi `len(deduped)==0`). Test: `agent/pkg/sbom/extractor/extractor_test.go`.

**Đã thực hiện (8.1 – OCI labels):** (1) Đọc OCI image config (`img.ConfigFile()`); (2) `detectOS(fs, imageConfig)`: khi FS trả "unknown", dùng label `org.opencontainers.image.ref.name` làm Version; nếu label có chứa "distroless" thì trả OS.Name = "distroless". (3) `syntheticPackageFromImage(imageRef, imageConfig)`: version ưu tiên từ label (dạng "repo:tag" → lấy phần sau ":"). (4) `selectParsersForOS` xử lý "distroless" giống "unknown" (chạy tất cả parser). Các bước 8.2–8.8 (heuristic cataloger, PURL, cache, UI badge) triển khai tiếp theo.

---

## Finding #6: Multi-cluster thiếu isolation và rate limit (High)

### Hiện trạng đã xác minh

- **Core:** Mọi request đều có thể truyền `cluster_id` (query param, body); dữ liệu filter theo cluster. **Không có** per-cluster rate limit ở ingest (gRPC SBOM, sync, pod detail).
- **Auth:** Agent dùng chung mTLS client cert (fortuna-agent-tls); không có cluster-scoped credential trong code/deploy.
- **DB/NATS:** Một PostgreSQL, một NATS; không partition theo cluster.

### Đánh giá

- **Impact:** High – một cluster “noisy” có thể làm quá tải DB/NATS, ảnh hưởng cluster khác.
- **Risk:** High. Isolation credential (per-cluster cert) phức tạp hơn; rate limit và quota là bước đầu tiên.

### Kế hoạch xử lý

| Bước | Nội dung | Chi tiết |
|------|----------|----------|
| 6.1 | Per-cluster rate limit (ingest) | Ở Core: middleware hoặc handler cho sync/gRPC nhận `cluster_id` từ payload. Rate limiter per cluster_id (in-memory map + token bucket hoặc sliding window). Cấu hình: max req/s hoặc max concurrent sync per cluster. Khi vượt: 429 hoặc queue (log + metric). |
| 6.2 | Config + doc | Env hoặc config file: `RATE_LIMIT_PER_CLUSTER_SYNC_RPS`, `RATE_LIMIT_SBOM_PER_CLUSTER_RPS` (ví dụ). Ghi trong DEPLOYMENT_AND_ARCHITECTURE_FAQ và deploy/README. |
| 6.3 | NATS / DB bảo vệ | (Optional) NATS: limit subscription per subject prefix per cluster. PostgreSQL: statement timeout hoặc pool limit per “app name” (cluster_id) nếu driver hỗ trợ. Ưu tiên thấp hơn 6.1. |
| 6.4 | Cluster-scoped credentials | (Phase 2) Cấp cert/secret riêng per cluster (hoặc per tenant); Core verify cluster_id từ cert CN/SAN. Cần thay đổi script/create_mtls và rollout agent. |

**File:** Core ingest entry (gRPC server, sync handler); config; docs.

---

## Finding #4: mTLS/secret rotation thủ công (Medium)

### Hiện trạng đã xác minh

- **Script:** `scripts/utils/create_mtls_secret.sh`: OpenSSL, 365 ngày; tạo secrets fortuna-ca-cert, fortuna-core-tls, fortuna-agent-tls, fortuna-webhook-tls. Không tích hợp cert-manager.
- **Core:** `core/internal/grpc/server.go` dùng CertManager (reload file); Agent load cert lúc connect (restart để đổi cert).
- **FAQ:** Đã ghi rotation là manual.

### Đánh giá

- **Impact:** Medium – hết hạn hoặc lộ key cần can thiệp thủ công, downtime nếu không chuẩn bị.
- **Risk:** Medium.

### Kế hoạch xử lý

| Bước | Nội dung | Chi tiết |
|------|----------|----------|
| 4.1 | Doc quy trình rotation | Trong deploy/README.md: bước rõ ràng (generate → update secret → rollout Core → rollout Agent → verify). Ghi thời gian rotate khuyến nghị (ví dụ trước 30 ngày hết hạn). |
| 4.2 | Cert-manager (optional) | Dùng Certificate + Issuer (CA hoặc ACME) cho core-tls, agent-tls, webhook-tls. Core/Agent mount secret do cert-manager cập nhật; Core đã hỗ trợ reload file. Agent cần restart hoặc thêm file watcher reload. |
| 4.3 | Automation tối thiểu | Nếu không dùng cert-manager: script “rotate” (generate mới → kubectl apply secret → kubectl rollout restart) + cron hoặc task trong pipeline; ghi trong README. |

**File:** `deploy/README.md`, `scripts/utils/` (script rotate hoặc hướng dẫn cert-manager), có thể thêm `deploy/certs/` với Certificate CR.

---

## Finding #5: Observability thiếu tracing và audit context (Medium)

### Hiện trạng đã xác minh

- **Log:** stdout, Gin log; **metric:** Prometheus `/metrics`. Không có OpenTelemetry/SkyWalking.
- **Audit:** Bảng audit log; không có trace ID nối từ Agent payload → Core → NATS → insight.

### Đánh giá

- **Impact:** Medium – debug chậm, khó nối chuỗi event qua nhiều component.
- **Risk:** Medium.

### Kế hoạch xử lý

| Bước | Nội dung | Chi tiết |
|------|----------|----------|
| 5.1 | Correlation ID | Agent gửi header/field `X-Request-ID` hoặc `trace_id` (gRPC metadata); Core nhận và truyền vào context, log, NATS message. Worker và API log cùng ID. |
| 5.2 | Audit log + trace_id | Bảng audit (và insight nếu cần) thêm cột `trace_id` (optional). Khi tạo audit record từ sync/worker, ghi trace_id. Document “trace from Agent to insight” trong ops doc. |
| 5.3 | OpenTelemetry (optional) | Gin middleware + gRPC interceptor; export span to OTLP hoặc Jaeger. Phase 2 sau khi 5.1–5.2 ổn. |

**File:** Core middleware, Agent gRPC client, NATS publish/subscribe, audit model; docs/ops.

---

## Finding #7: Deploy phụ thuộc thứ tự thủ công (Low)

### Hiện trạng đã xác minh

- **deploy/README.md:** Liệt kê thứ tự CNI → storage → infra → mTLS → RBAC → label → Core/Agent/Dashboard.
- **Script:** `scripts/deploy/deploy-fortuna-robust.sh` đóng gói các bước; Helm chart (nếu dùng) vẫn expect infra + cert đã có.

### Đánh giá

- **Impact:** Low – tăng thời gian và lỗi vận hành; không gây mất dữ liệu.
- **Risk:** Low.

### Kế hoạch xử lý

| Bước | Nội dung | Chi tiết |
|------|----------|----------|
| 7.1 | Single entrypoint | Makefile hoặc script `deploy-full` gọi tuần tự: check CNI/storage → infra (postgres, nats) → mTLS script → RBAC → label → Core → Agent → Dashboard. Exit on first failure với message rõ. |
| 7.2 | Prerequisites check | Trước Core/Agent: check secret fortuna-core-tls, fortuna-agent-tls tồn tại; check postgres/nats reachable. Fail fast với hướng dẫn. |
| 7.3 | Helm / Argo | Trong Helm README ghi rõ “prerequisites” và tham chiếu script. Nếu dùng Argo: PreSync job chạy check (hoặc chạy create_mtls_secret nếu optional). |

**File:** Makefile hoặc `scripts/deploy/deploy-full.sh`, deploy/README.md, Helm README.

---

## Thứ tự thực hiện đề xuất

1. **P0 (ngay):** Finding #3 – bỏ “delete all khi payload thiếu”, thêm log/metric và test.
2. **P1 (sprint 1):** Finding #2 – queue key UID + image parser; Finding #1.1 + 1.2 – doc + correlation ID.
3. **P2 (sprint 2):** Finding #8.1–8.4 – SBOM distroless (extractor detection, distroless cataloger, language inference, heuristic metadata); Finding #6.1–6.2 rate limit; Finding #4.1 doc rotation + (optional) 4.2 cert-manager; Finding #1.3 WebSocket pub/sub.
4. **P2 (sprint 3):** Finding #8.5–8.8 – SBOM cache on-disk, Core/Dashboard metadata & UI, docs, E2E.
5. **P3:** Finding #5.1–5.2 correlation + audit; Finding #7.1–7.2 deploy entrypoint + check; Finding #1.4 dedup shared, #6.4 credential isolation khi cần.

---

## Tài liệu tham chiếu

- [Architecture_Finding_03142026.md](./Architecture_Finding_03142026.md) – bản finding gốc
- [DEPLOYMENT_AND_ARCHITECTURE_FAQ.md](./DEPLOYMENT_AND_ARCHITECTURE_FAQ.md) – deployment & kiến trúc
- [COMPONENTS.md](./COMPONENTS.md) – components & data flow
- [POD_DETAIL_SPEC.md](../03-components/podDetail/POD_DETAIL_SPEC.md) – Pod Detail & SBOM flow
- [AGENT_CORE_CONNECTIVITY.md](../AGENT_CORE_CONNECTIVITY.md) – Agent–Core connectivity
- `core/internal/service/agent_service.go` – sync và processSynced*
- `agent/internal/sbom/queue.go`, `agent/internal/sbom/processor.go` – SBOM queue và image ref
- `agent/pkg/sbom/extractor/extractor.go` – detectOS, selectParsersForOS, RawSBOM
- `core/pkg/cve/database/manager.go` – Trivy DB + NVD API fallback, package_vulnerabilities
- `core/pkg/cve/database/nvd/client.go` – NVD API client
- [CUSTOM_SBOM_ZERO_DEPENDENCY_PART2.md](../03-components/sbom/CUSTOM_SBOM_ZERO_DEPENDENCY_PART2.md) – SBOM normalizer, CycloneDX-like, CVE matching
- CVE loader / NVD implementation (vd. cve_updater, CVE_MASTER_IMPLEMENTATION_GUIDE nếu có) – NVD daily load, schema package_vulnerabilities
