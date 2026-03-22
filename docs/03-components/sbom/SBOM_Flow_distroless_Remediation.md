Vấn đề lớn nhất của flow hiện tại

Điểm yếu chính:

Fortuna đang match CVE theo package name + version, nhưng distroless/control-plane thực tế cần binary/module matching.

Ví dụ component:

pkg:generic/coredns@v1.11.1

OSV query:

ecosystem = generic
name = coredns

Vấn đề:

OSV database chủ yếu có ecosystem:

Go

npm

PyPI

Maven

crates

generic ecosystem gần như trống.

→ query trả về 0 CVE là chuyện bình thường.

3. NVD fallback đang có vấn đề logic

Flow hiện tại:

keywordSearch=<name>

ví dụ:

keywordSearch=coredns

Nhưng NVD search này:

không filter version

không map binary/module

Kết quả:

CVE-2022-XXXX
CVE-2019-XXXX
CVE unrelated

Matcher hiện tại:

Constraint = empty

Điều này tạo ra 2 rủi ro:

false positive

CVE không liên quan version.

false negative

CVE chỉ apply cho dependency.

Ví dụ:

CoreDNS
binary có dependency:

golang.org/x/net

NVD keyword:

coredns

→ không ra CVE của dependency.

4. Whitelist logic hơi nguy hiểm

Code:

isNVDFallbackWhitelisted(component)

whitelist:

coredns
etcd
kube-*
openssl
runc

Vấn đề:

component name từ SBOM có thể là:

coredns/coredns
registry.k8s.io/coredns

→ không match whitelist.

Bạn đã thấy symptom:

chỉ thấy version nhưng không có CVE

Khả năng cao do name normalization chưa đủ tốt.

5. Soft delete gây race condition

Trong flow:

load components
WHERE deleted_at IS NULL

Nếu component bị soft delete sau khi publish event:

worker load:

0 component

→ matcher bỏ qua.

Đây là classic event/data race.

Nhiều hệ thống tránh bằng:

SBOM snapshot version

hoặc

immutable component table
6. Constraint model từ NVD đang thiếu

NVD fallback tạo CVE:

Constraint = ""

Trong matcher:

version match check

bị bypass.

Điều này làm risk scoring sau này sai.

7. Dashboard logic có thể gây hiểu nhầm

UI hiển thị:

component
version
CVE list

Nếu CVE empty:

User hiểu:

component safe

Trong khi thực tế:

SBOM incomplete

Nên cần thêm field:

analysis_status

ví dụ:

binary-only
heuristic
full-sbom
8. Performance issue tiềm ẩn

NVD fallback:

per component query

Nếu 1 SBOM:

50 components

→ 50 API call.

NVD limit:

5 req / 30s (no API key)
50 req / 30s (API key)

Worker scale lên sẽ dính rate limit rất nhanh.

9. Kiến trúc nên bổ sung cho Fortuna

Để scan control-plane đúng chuẩn, matcher nên có thêm Binary Matching Layer.

Ví dụ:

kube-apiserver

là Go binary.

Trong ELF có:

Go buildinfo
module list

có thể extract:

k8s.io/apimachinery
k8s.io/client-go
golang.org/x/net

Sau đó query OSV theo ecosystem:

ecosystem = Go
package = golang.org/x/net

Lúc đó CVE match chính xác hơn rất nhiều.

10. Kiến trúc matcher lý tưởng cho Fortuna

Nên có pipeline:

SBOM
   │
   ├─ Package matching
   │     OSV
   │
   ├─ Binary module matching
   │     Go modules
   │
   ├─ Image metadata matching
   │     distro packages
   │
   └─ Heuristic fallback
         NVD

NVD nên là last resort.

11. Các cải tiến nhỏ nhưng rất đáng làm
normalize component name

trước khi match:

registry.k8s.io/coredns
coredns/coredns
coredns

→ normalize thành:

coredns
cache NVD result
cache_key = name + version
ttl = 24h

giảm API call.

add matcher confidence
match_type:
- osv-package
- binary-module
- nvd-keyword
12. Đánh giá tổng thể
Thành phần	Đánh giá
Event architecture	rất tốt
SBOM ingestion	ổn
OSV matching	đúng hướng
NVD fallback	tạm dùng
distroless support	còn yếu
control-plane support	cần cải thiện
Kết luận thực tế

Flow hiện tại hoạt động, nhưng:

phù hợp với normal container SBOM

chưa đủ tốt cho distroless + Kubernetes control-plane

Điểm cần nâng cấp nhất:

Binary dependency reconstruction

Nếu thêm module này, Fortuna sẽ:

scan được
kube-apiserver
etcd
CoreDNS

mà không cần package manager.

---

## Cấu hình NVD API (kiểm tra thực tế)

**Hướng dẫn gán giá trị cho user (local, K8s Secret, Docker):** xem **[docs/05-operations/NVD_API_KEY.md](../../05-operations/NVD_API_KEY.md)**.

### Nơi cấu hình

| Nơi | Mục đích |
|-----|----------|
| **Env (Core)** | `NVD_API_KEY`: API key NVD (tùy chọn, tăng rate limit). Nếu không set: 5 req/30s; có key: 50 req/30s. |
| **Env (Core)** | `FORTUNA_NVD_DISABLED=1` hoặc `true`: tắt hoàn toàn NVD → `NewNVDClientForManager()` trả về `nil`, không gọi NVD. |
| **Deploy** | `deploy/fortuna-core-deployment.yaml`: biến `NVD_API_KEY` lấy từ Secret `fortuna-secrets`, key `nvd-api-key` (optional). |
| **Code** | `core/pkg/cve/database/manager.go`: `NewNVDClientForManager()` đọc env, tạo `nvd.Client`; `SetAPIKey(os.Getenv("NVD_API_KEY"))`. |
| **Code** | `core/pkg/cve/database/nvd/client.go`: URL cố định `https://services.nvd.nist.gov/rest/json/cves/2.0`, query `keywordSearch=<name>`, `resultsPerPage=100`. |

### Luồng xử lý NVD trong Core

1. **CVE Manager** (`core/pkg/cve/database/manager.go`): khi `source == "postgres"`, query Postgres (OSV) trước; nếu 0 CVE và `TryNVDFallback == true` và `m.nvdAPI != nil` → gọi `m.nvdAPI.Query(ctx, ecosystem, name, version)`.
2. **NVD Client** (`core/pkg/cve/database/nvd/client.go`): `Query()` gửi GET `?keywordSearch=<name>&resultsPerPage=100`, header `apiKey` nếu có; xử lý 429 (retry 1 lần theo Retry-After); parse JSON → `[]*cve.CVE` (Constraint rỗng vì NVD không cung cấp version range).
3. **Matcher** (`core/pkg/cve/matcher/matcher.go`): NVD fallback chỉ chạy khi `useNVDFallbackForHeuristic(sbom)` (sbom_source distroless-heuristic/label-metadata, confidence != high) và component nằm trong whitelist; tên component được **chuẩn hóa** trước khi gọi NVD (xem Plan điều chỉnh).
4. **Cache**: Manager dùng in-memory cache theo key `ecosystem:name:version`; không có TTL (process lifetime). Giảm gọi NVD lặp lại trong cùng worker.

### Ghi chú

- NVD API **không filter version** (keyword search theo tên); matcher coi Constraint rỗng → “potentially affected” và vẫn persist CVE (có thể false positive).
- Rate limit: không có API key dễ gặp 429 khi nhiều SBOM distroless; nên set `nvd-api-key` trong Secret và rollout restart Core.

---

## Plan điều chỉnh đã thực hiện

Đối chiếu với mục 11 (cải tiến nhỏ):

| Hạng mục | Trạng thái | Chi tiết |
|----------|------------|----------|
| **Normalize component name** | ✅ Đã làm | Trước khi kiểm tra whitelist và gọi NVD: `normalizeComponentNameForNVD(name)` trong `core/pkg/cve/matcher/matcher.go`. Ví dụ: `coredns/coredns`, `registry.k8s.io/coredns/coredns` → lấy segment cuối sau `/` → `coredns`; `registry.k8s.io/coredns` (không có `/`) → map `registryCanonicalName` → `coredns`. Whitelist và NVD keyword search dùng tên đã chuẩn hóa. |
| **Cache NVD result** | ✅ Đã có | Manager đã cache theo `ecosystem:name:version` (in-memory, không TTL). Giảm gọi NVD lặp trong cùng process. |
| **Add matcher confidence / match_type** | 🔲 Chưa | Có thể bổ sung sau: trường `match_type` (osv-package / binary-module / nvd-keyword) trong `cve_matches` hoặc API response. |

Kiểm tra thực tế: NVD được bật khi không set `FORTUNA_NVD_DISABLED`; API key đọc từ env (deploy inject từ Secret `fortuna-secrets`). Luồng SBOM → CVE matcher → Postgres 0 → NVD fallback (nếu heuristic + whitelist) → persist `cves` + `cve_matches` (MatchedBy=nvd-fallback) đã đúng; vấn đề “chỉ thấy version không thấy CVE” đã được giảm nhờ normalize tên (tránh whitelist miss khi component name là `registry.k8s.io/coredns` hoặc `coredns/coredns`).