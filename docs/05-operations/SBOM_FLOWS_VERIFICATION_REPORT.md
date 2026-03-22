# Báo cáo xác minh luồng SBOM — Test & logic

**Ngày chạy:** 2026-03-22  
**Repo:** KSAM  
**Mục đích:** Tổng hợp **logic các luồng SBOM đã triển khai**, **bộ test đã chạy**, và **kết quả** (PASS / SKIP / hạn chế).

---

## 1. Tóm tắt điều hành

| Hạng mục | Kết quả |
|----------|---------|
| **Agent** `go test ./pkg/sbom/... ./internal/sbom/...` | **PASS** |
| **Agent extractor** `go test -race ./pkg/sbom/extractor/...` | **PASS** |
| **Core** `grpc`, `repository`, `pkg/sbom`, `pkg/worker` (short) | **PASS** |
| **Tích hợp NVD đầy đủ** (`TestFullFlow_DistrolessSBOM_NVD_CVE_AndRisk`) | **SKIP** (mặc định; cần `RUN_NVD_INTEGRATION=1` hoặc `NVD_API_KEY`) |
| **E2E NATS + Core tự động** | Không chạy trong phiên này (theo `SBOM_PIPELINE_REMAINING_PLAN.md` — backlog) |

**Kết luận ngắn:** Toàn bộ test đơn vị/tích hợp **đã chạy** trong phạm vi repo **đều PASS**; một số test **tùy chọn** (NVD live) bị skip theo thiết kế.

---

## 2. Sơ đồ logic luồng (đã implement)

```
[Workload / Image]
        │
        ▼
┌───────────────────┐     cache (SBOM_CACHE_DIR)
│ Agent: ExtractSBOM │◄────────────────────────────
│  • getImage (ctr/ │
│    registry)       │
│  • buildFilesystem│  ← A5: SBOM_FS_MODE (materialize|indexed)
│  • parsers (dpkg/ │     spool + lazy ReadFile + metrics (2c)
│    apk/rpm/npm/… │
│    gobinary/…)     │
│  • synthetic / OS  │
└─────────┬─────────┘
          │ gRPC: SBOMFinding / SBOMCreated event
          ▼
┌───────────────────┐
│ Core: ingest      │  PURL sanitize, monotonic SBOM, correlation ID
│ handler_sbom      │
│ NATS publish retry│  DLQ path khi primary fail
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Worker: CVE       │  Matcher, EPSS/KEV → evidence, risk score
│ matcher + insight │
└───────────────────┘
```

**Logic đã được test chủ yếu theo lớp:**

1. **Extractor / parsers** — file `agent/pkg/sbom/extractor/*_test.go` (không cần cluster).
2. **Core gRPC** — sanitize PURL, guard, publish retry, correlation.
3. **Worker** — confidence từ SBOM partial, coverage stats, replay determinism (một phần pipeline).

---

## 3. Lệnh đã thực thi & kết quả

### 3.1 Agent — SBOM

| Lệnh | Kết quả |
|------|---------|
| `cd agent && go test ./pkg/sbom/... ./internal/sbom/... -count=1` | **ok** (extractor + internal/sbom) |
| `cd agent && go test ./pkg/sbom/extractor/... -count=1 -race` | **ok** |

**Gói:**

- `github.com/fortuna/agent/pkg/sbom/extractor` — có test.
- `github.com/fortuna/agent/pkg/sbom/signatures` — không có file test (`[no test files]`).
- `github.com/fortuna/agent/internal/sbom` — **ok**.

### 3.2 Core

| Lệnh | Kết quả |
|------|---------|
| `cd core && go test ./internal/grpc/... -count=1` | **ok** |
| `cd core && go test ./internal/repository/... -count=1 -short` | **ok** |
| `cd core && go test ./pkg/sbom/... -count=1` | **ok** |
| `cd core && go test ./pkg/worker/... -count=1 -short` | **ok** |

---

## 4. Ma trận phủ test theo chức năng

### 4.1 Agent — Virtual filesystem (A5)

| Chức năng | Test gợi ý | Ghi chú |
|-----------|------------|---------|
| Cap `maxFileBytes` / `maxTotalBytes` | `TestFilesystem_ExtractTar_*` | PASS |
| Skip prefix `SBOM_FS_SKIP_PATH_PREFIXES` | `TestFilesystem_ExtractTar_SkipPathPrefixes` | PASS |
| Indexed: discovery + selective + whiteout | `TestFilesystem_IndexedMode_*` | PASS |
| Lazy spool + ReadFile | `TestFilesystem_IndexedMode_DiscoveryWithoutMaterialize`, metrics | PASS |
| Metrics 2c | `TestFilesystem_MetricsSnapshot_LazyReadCounts`, `TestFormatBytesIEC` | PASS |

### 4.2 Agent — Parsers (Tier 0–3)

| Vùng | File test | Ghi chú |
|------|-----------|---------|
| npm / lock v2 | `npm_test.go` | PASS |
| pip / METADATA | `pip_test.go` | PASS |
| gomod / go.sum | `gomod_test.go` | PASS |
| gobinary / buildinfo | `gobinary_test.go` (binary thật trong testdata) | PASS |
| rpm list + rpmdb fallback an toàn | `rpm_test.go` | PASS |
| maven | `maven_test.go` | PASS |
| cargo | `cargo_test.go` | PASS |
| ruby Gemfile.lock | `ruby_test.go` | PASS |
| nuget lock + project.assets | `nuget_test.go` | PASS |
| distroless / synthetic / OS | `extractor_test.go` | PASS |
| Disk cache eviction | `cache_eviction_test.go` | PASS |
| Go toolchain / GO-1 | `go_toolchain_test.go` | PASS |

### 4.3 Core — gRPC & SBOM

| Chức năng | File | Ghi chú |
|-----------|------|---------|
| NATS publish + retry + DLQ | `sbom_publish_retry_test.go` | PASS |
| PURL canonicalize / invalid / Go | `handler_sbom_purl_sanitize_test.go`, `sbom_e2e_go_purl_test.go` | PASS |
| SBOM guard (không bypass) | `handler_sbom_guard_test.go` | PASS |
| Correlation ID | `handler_sbom_correlation_test.go` | PASS |

### 4.4 Core — Worker & SBOM liên quan

| Chức năng | File | Ghi chú |
|-----------|------|---------|
| Confidence / partial SBOM | `cve_matcher_worker_confidence_test.go` | PASS |
| Coverage stats (match ratio) | `sbom_coverage_stats_test.go` | PASS |
| Replay deterministic | `replay_determinism_e2e_test.go` (khi chạy worker tests) | PASS (trong suite) |
| Full flow NVD + distroless | `cve_matcher_worker_integration_test.go` | **SKIP** mặc định |

### 4.5 Core — Event contract

| Chức năng | File | Ghi chú |
|-----------|------|---------|
| SBOMCreated contract | `core/pkg/sbom/events_contract_test.go` (nếu có trong module) | Đã có trong `pkg/sbom` — `TestSBOMCreatedEvent_ContractRoundTrip` **PASS** |

### 4.6 Agent — Queue / processor

| Gói | Kết quả |
|-----|---------|
| `agent/internal/sbom` | **ok** (`queue_test.go`, `processor_test.go`) |

---

## 5. Logic “đã kiểm” vs “chưa kiểm trong CI”

| Luồng | Đã có unit/integration test trong repo | Ghi chú |
|-------|----------------------------------------|---------|
| Extract image → FS → parsers | Có | Không bắt buộc registry thật trong hầu hết test |
| Indexed + spool + metrics | Có | Tar giả lập |
| Core nhận finding → sanitize PURL | Có | |
| Publish NATS retry / DLQ | Có | Mock / stub trong test |
| CVE match + insight + risk | Có (mock DB/cache) | |
| **Gọi NVD API thật end-to-end** | **SKIP** mặc định | Bật bằng env khi cần |
| **E2E NATS + Core trong CI** | **Chưa** (theo plan) | Smoke thủ công / testcontainers sau này |

---

## 6. Rủi ro / ghi chú tài liệu

1. **`SBOM_PIPELINE_REMAINING_PLAN.md`** tham chiếu `SBOM_TIER3_RISK_A5_VERIFICATION.md` — **file không tồn tại** trong repo tại thời điểm báo cáo; nên bổ sung file hoặc sửa link.
2. **Race:** `extractor` đã chạy `-race` — **PASS** (không phát hiện race trong test hiện tại).
3. **Production:** Cấu hình env (`SBOM_FS_*`, `FORTUNA_*`, NVD, EPSS, KEV) ảnh hưởng hành vi; test chỉ cover subset mặc định.

---

## 7. Khuyến nghị (tùy chọn)

1. Thêm job CI: `go test ./agent/pkg/sbom/... -race` và `go test ./core/... -short` (hoặc matrix module).
2. Tạo hoặc khôi phục **`SBOM_TIER3_RISK_A5_VERIFICATION.md`** để khớp plan.
3. Chạy định kỳ `RUN_NVD_INTEGRATION=1` hoặc set **`NVD_API_KEY`** trên môi trường có NVD để xác nhận `TestFullFlow_DistrolessSBOM_NVD_CVE_AndRisk`. Hướng dẫn gán key: **`NVD_API_KEY.md`**.

---

*Báo cáo được sinh từ kết quả `go test` trên workspace; không thay thế kiểm thử staging/production.*
