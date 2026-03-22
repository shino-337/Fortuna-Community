# Verification — Tier 3 (Ruby/NuGet) + RISK-1+ (KEV/EPSS) + A5+ (skip prefixes)

Cập nhật: 2026-03-20 — đối chiếu implementation + test coverage + Risk Detail UI.

---

## Tier 3 — Ruby + NuGet parsers

| Parser | Logic | PURL | Registration | Tests |
|--------|--------|------|--------------|-------|
| **ruby** | `FindPathsBySuffix("Gemfile.lock")` → section `specs:` → regex 4-space `name (version)` | `pkg:gem/{name}@{version}` | `NewExtractor` + `languageParsers` | `TestParseGemfileLockSpecs`, `TestRubyGemsParser_Parse`, multi-gem + no-specs edge |
| **nuget** | `FindPathsBySuffix("packages.lock.json")` → JSON `dependencies` → mọi TFM → `resolved` | `pkg:nuget/{name}@{version}` | same | `TestParseNuGetPackagesLock`, multi-TF + empty |

---

## RISK-1+ — KEV + EPSS parallel + evidence merge

| Component | Verified | Details |
|-----------|----------|---------|
| `core/pkg/kev` | Yes | CISA feed, `DefaultCatalog()` singleton, `Contains(CVE)`, background refresh, `FORTUNA_KEV_ENABLED`, 3 metrics |
| `core/pkg/kev` tests | **Added** | `TestCatalog_Refresh`, `TestCatalog_Refresh_HTTPError` (httptest) |
| `core/pkg/insightevidence` | Yes | `Merge(base, patch)` — `merge_test.go` |
| `core/pkg/epss` — `LookupMany` | Yes | `(c *Client).LookupMany(ctx, ids, concurrency)`; `LookupManyDefault` delegates; **`FORTUNA_EPSS_CONCURRENCY`** khi concurrency≤0 |
| EPSS tests | **Added** | `TestClient_LookupMany_ConcurrencyLimit` (peak ≤ limit), `TestClient_LookupMany_DedupesCVEs` |
| Worker integration | Yes | Prefetch EPSS → merge `epss` + `cisa_kev` vào `Evidence` |
| Scorer — KEV | Yes | `cisa_kev: true` → exploit score max 6.0 |
| Scorer tests | **Added** | `scorer_exploit_evidence_test.go` — KEV, EPSS+KEV, `parseCISAKEVFromInsightEvidence` |

---

## A5+ — Skip path prefixes

| Item | Verified |
|------|----------|
| `SBOM_FS_SKIP_PATH_PREFIXES` | Comma-separated, `off` disables, leading `/`, `filepath.Clean` — `NewFilesystem` |
| `ExtractTar` | `shouldSkipPathForA5(path)` trước khi đọc nội dung file |
| Default | Trống → không skip (backward compatible) |
| Tests | **Added** | `TestFilesystem_ExtractTar_SkipPathPrefixes` |

---

## Doc

| Item | Status |
|------|--------|
| `agent/README.md` Overview (dòng ~20) | **12 parsers** liệt kê đầy đủ |
| `agent/README.md` § SBOM Multi-Parser | Đã có đủ 12 (trước đó đã cập nhật) |

---

## Dashboard — Risk Detail (EPSS / KEV)

| Item | Status |
|------|--------|
| API | `GET /risk/insights/:id` trả `evidence` (JSON string) — đã có |
| UI | Badge **CISA KEV** + **EPSS %** + percentile; khối raw JSON giữ nguyên — `dashboard/pages/RiskDetail.tsx`, `dashboard/lib/threatIntel.ts` |

---

## Backlog còn lại (không đổi)

| Item | Status |
|------|--------|
| A5+ Phase 2 (`SBOM_FS_MODE=indexed` + lazy read) | Plan trong `A5_STREAMING_VFS_PLAN.md`, chưa code |
| ~~UI Risk Center (EPSS/KEV evidence)~~ | **Done** (badges) |
| E2E NATS / NVD edge cases | Smoke doc `E2E_NATS_SBOM_PIPELINE.md`; CI E2E optional |
