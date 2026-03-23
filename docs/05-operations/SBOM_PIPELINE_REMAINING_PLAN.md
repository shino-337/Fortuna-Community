# SBOM Pipeline — Trạng thái & Kế hoạch phần còn lại

Cập nhật: 2026-03-22 — Xác nhận lại kết quả + chi tiết **CI / E2E**  
Mục đích: đóng vòng **implementation + verification**; phân tier cho backlog dài hạn.

---

## Xác nhận kết quả đã thực hiện (đối chiếu tài liệu & test)

| Hạng mục trong plan | Trạng thái | Bằng chứng / ghi chú |
|---------------------|------------|----------------------|
| Agent parsers + VFS A5 (2a–2c) | **Done** | Code `agent/pkg/sbom/extractor/`; plan `A5_STREAMING_VFS_PLAN.md` |
| Core gRPC SBOM + PURL + NATS retry/DLQ | **Done** | Test `core/internal/grpc/*sbom*` |
| Worker CVE + EPSS + KEV + evidence | **Done** | `core/pkg/worker/`, `core/pkg/epss`, `core/pkg/kev` |
| Tier 3 Maven/Cargo/Ruby/NuGet (best-effort) | **Done** | Parser tests trong `agent/pkg/sbom/extractor/*_test.go` |
| UI Risk EPSS/KEV | **Done** | `dashboard/lib/threatIntel.ts` (theo plan) |
| **`go test` toàn pipeline unit/integration** | **Đã chạy & PASS** (workspace) | `SBOM_FLOWS_VERIFICATION_REPORT.md` |
| **NVD_API_KEY** (user gán env/Secret) | **Hỗ trợ** | `core/pkg/cve/database/manager.go`; doc `NVD_API_KEY.md` |
| **CI GitHub Actions** | **Chỉ build + `go test`** | Xem mục **CI E2E** bên dưới — **không** có job E2E NATS tự động |

---

## CI E2E — Chi tiết trạng thái (2026-03-22)

### Định nghĩa trong plan

- **“CI E2E — NATS + Core (optional testcontainers)”** (mục 9 ở cuối file): nghĩa là **một job CI** chạy Core (và/hoặc Agent) với **NATS JetStream + PostgreSQL** thật hoặc **testcontainers**, rồi **publish/consume** luồng gần giống production (vd. `fortuna.sbom.created` → worker), có assertion.

### Thực tế trong repo hiện tại

| Thành phần | Có trong CI? | Chi tiết |
|------------|----------------|----------|
| **`go test ./...` (Core + Agent)** | **Có** | `.github/workflows/build.yml` — job `test-go`: `cd core && go test ./...`, `cd agent && go test ./...`. |
| **NATS / JetStream trong CI** | **Không** | Không service NATS, không compose, không testcontainers trong workflow. |
| **PostgreSQL trong CI** | **Không** | Test Core dùng mock/sqlite tùy test; không spin DB cho full pipeline. |
| **E2E “Agent → gRPC → Core → NATS → worker”** | **Không** | Chưa có job; mục 9 vẫn là **backlog**. |
| **`continue-on-error: true`** trên test | **Có** | Job test **không fail** PR nếu test đỏ (cần lưu ý khi đọc badge CI). |

### Thay thế đã có (không phải CI E2E full stack)

- **Unit / integration trong Go:** publish retry, PURL, replay determinism, matcher confidence — xem `SBOM_FLOWS_VERIFICATION_REPORT.md`.
- **Smoke / cluster thủ công:** các doc triển khai (`PRODUCTION_DEPLOYMENT`, `CLEAN_REBUILD_*`, checklist SBOM trong `docs/03-components/sbom/README.md`) — **không** chạy tự động trên GitHub.
- **NATS smoke (dev/staging):** [E2E_NATS_SBOM_PIPELINE.md](E2E_NATS_SBOM_PIPELINE.md). **E2E SBOM trên cluster:** [E2E_SBOM_SCRIPTS.md](E2E_SBOM_SCRIPTS.md) (`run-e2e.sh --suite=sbom-full`).

### Đề xuất khi triển khai mục 9 (tóm tắt)

1. Thêm job (vd. `e2e-sbom`) hoặc workflow riêng: `services: nats` + `postgres` (GitHub Actions service containers) **hoặc** testcontainers-go.
2. Khởi tạo stream/subject JetStream tối thiểu, chạy một test integration: gửi event SBOM → assert DB/metric (hoặc subscribe DLQ).
3. Bỏ `continue-on-error` cho job test nếu muốn gate merge.

---


## Đã hoàn thành (gần đây)

| Khu vực | Nội dung |
|--------|----------|
| Agent | Pip/Go multi-root + fallback; RPM rpmdb fallback; VFS mem cap; npm A6; `go-binary` → `PACKAGE_TYPE_GO_MOD` |
| Agent / stdlib | **GO-1:** `go_version` trên `SBOMFinding`; toolchain; cache JSON |
| Core gRPC | PURL/sanitize; monotonic SBOM; drift; NATS publish retry + DLQ |
| Core worker | CVE matcher; metrics; **EPSS** + **KEV** optional → `Insight.evidence` + risk scorer |
| Core DLQ | `SBOMDLQWorker`; **`fortuna_sbom_created_dlq_stream_messages`** (poll JetStream); `fortuna_sbom_store_upsert_commits_total` |
| **CACHE-1 / MET-1 / OBS-1 / INS-1** | Như verification 2026-03-10 |
| **GAP (OBS)** | DLQ stream depth gauge + drift denominator counter + PromQL trong `SBOM_ALERTING_SLO.md` |
| **RISK-1 (MVP)** | `core/pkg/epss` (FIRST API + cache); matcher enrichment; `fortuna_epss_*` metrics; scorer dùng `epss` trong evidence |
| **Tier 3 (partial)** | **`maven`**, **`cargo`**, **`ruby`**, **`nuget`** (`packages.lock.json` + `project.assets.json` best-effort) |
| **Dashboard** | **Risk Detail**: badge CISA KEV + EPSS (từ `evidence`) — `dashboard/lib/threatIntel.ts` |
| **RISK-1+** | **CISA KEV** (`core/pkg/kev`, `FORTUNA_KEV_ENABLED`); **EPSS** song song (`LookupManyDefault`, `FORTUNA_EPSS_CONCURRENCY`); evidence merge (`insightevidence`) |
| **A5+** | Phase 1 cap + skip-prefix; **2a–2c** `SBOM_FS_MODE=indexed`, spool, lazy read, `[SBOM FS]` metrics (`SBOM_FS_METRICS=off` để tắt log) |

---

## Verification — CACHE-1 / MET-1 / OBS-1 / INS-1 (2026-03-10)

(Xem bản cũ trong git history nếu cần chi tiết từng dòng.) **Bổ sung:** Gap DLQ depth + drift ratio đã đóng bằng metric/PromQL (mục *Đã đóng* trong `SBOM_ALERTING_SLO.md`).

---

## Tier 1 — Còn lại (1–2 sprint)

*Không có mục Tier 1 đang mở.*

---

## Tier 2 — Trung hạn

| ID | Trạng thái | Ghi chú |
|----|------------|---------|
| **A5+** | **Phase 2 done (2a–2c)** | Indexed + spool + metrics log — xem `A5_STREAMING_VFS_PLAN.md` |
| **RISK-1** | **MVP + KEV done** | EPSS + KEV + evidence + scorer; mở rộng: policy/UI, true batch EPSS nếu API hỗ trợ |

---

## Tier 3 — Phase 4 parsers

| Trạng thái | Nội dung |
|------------|----------|
| **Đã có** | `maven`, `cargo`, `ruby`, `nuget` (lock + **project.assets.json** `targets`) + npm/pip/gomod/… |
| **Chưa làm** | Ruby **gemspec**/transitive sâu; NuGet **central package management** / lock v2 nâng cao; Maven parent/BOM |

---

## Không gói trong sprint hiện tại

- **E2E NATS tự động trong CI** — xem mục **CI E2E** ở trên (chưa làm; smoke cluster vẫn thủ công).
- **NVD/K8s mapping** edge cases — production-driven.

**Verification matrix & kết quả test:** `SBOM_FLOWS_VERIFICATION_REPORT.md` (báo cáo chạy `go test` + phạm vi luồng).  
**Workflow CI:** `.github/workflows/build.yml` (test + docker build).

---

## Thứ tự đề xuất (next commits)

1. ~~CACHE-1 / MET-1 / OBS-1 / INS-1~~ **Done**
2. ~~GAP DLQ + drift denominator~~ **Done**
3. ~~RISK-1 EPSS MVP~~ **Done**
4. ~~Tier 3 Maven/Cargo~~ **Done** (mở rộng Gem/NuGet sau)
5. ~~**Tier 3** Ruby/NuGet~~ **Done** (best-effort)
6. ~~**RISK-1+** KEV + EPSS parallel~~ **Done**
7. ~~**A5+ Phase 2 (2a–2c)**~~ **Done**
8. ~~**UI** EPSS/KEV trên Risk Detail~~ **Done** (badge + raw JSON vẫn có)
9. **CI E2E** — NATS + Core (optional testcontainers)
