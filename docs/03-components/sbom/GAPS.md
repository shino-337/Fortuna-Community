# SBOM Component — Known Gaps

This document tracks all known gaps in the SBOM pipeline, ordered by priority.

---

## Summary

| ID | Gap | Severity | Status |
|----|-----|----------|--------|
| G-01 | NVD fallback ignores version constraints | High | Open |
| G-02 | No local NVD mirror — runtime API dependency | High | Open |
| G-03 | OSV `generic` ecosystem mostly empty | High | Open |
| G-04 | Component resolver not yet wired into pipeline | Medium | Design done |
| G-05 | Confidence not stored as a DB column | Medium | Partial (derived) |
| G-06 | Match-type field missing from `cve_matches` | Medium | Open |
| G-07 | Distroless cataloger not implemented (full bin/lib walk) | Medium | Open |
| G-08 | On-disk SBOM cache not implemented | Medium | Open |
| G-09 | Worker parallelism limited (single subscriber) | Low | Open |
| G-10 | Go pseudo-version normalization missing | Low | Open |
| G-11 | EPSS / KEV integration not started | Low | Open |
| G-12 | E2E test for distroless + control-plane badge | Low | Open |
| G-13 | Global NVD rate limiter across workers | Low | Open |

---

## Gap Details

### G-01 · NVD Fallback Ignores Version Constraints

**Priority:** High  
**Component:** `core/pkg/cve/database/nvd/client.go`

NVD keyword search (`keywordSearch=<name>`) does not filter by version range. The CVE record is persisted with `Constraint = ""`, which means the matcher treats every result as "potentially affected" regardless of the installed version. This produces false positives (e.g., a CVE for CoreDNS < 1.9 is matched to CoreDNS 1.11.1) and false negatives (dependency-level CVEs like `golang.org/x/net` are missed because keyword search is by binary name).

**Mitigation:** NVD matches are tagged `matched_by=nvd-fallback`, `confidence=low`. Dashboard can filter by confidence. Long-term fix: parse NVD `versionStartIncluding` / `versionEndExcluding` from CPE configurations.

---

### G-02 · No Local NVD Mirror

**Priority:** High  
**Component:** CVE Manager / NVD Client

The NVD client queries the NIST API in real time during matching. This introduces latency, rate-limit risk (5 req/30 s without key, 50 req/30 s with key), and a runtime dependency on an external service.

**Plan:** Mirror NVD locally — sync every ~4 hours into a `nvd_vulnerabilities` table with (package, ecosystem, version range, CVSS) plus a `cpe_to_package` mapping table.

---

### G-03 · OSV `generic` Ecosystem Mostly Empty

**Priority:** High  
**Component:** OSV Loader, Matcher

Control-plane components emit `pkg:generic/<name>@<version>`. OSV has very few entries under the `generic` ecosystem, so the Postgres query returns 0 CVEs for these components. CVE detection relies entirely on NVD fallback, which has the issues described in G-01.

**Plan:** Map control-plane components to their actual Go modules via `k8s_component_map.yaml` (e.g., `kube-apiserver` → `k8s.io/kubernetes`). When Go binary analysis (gobinary parser) is used, modules are queried under `ecosystem=Go` which has extensive OSV coverage.

---

### G-04 · Component Resolver Not Wired

**Priority:** Medium  
**Component:** Core / Worker

The conflict resolution spec (source-priority model, canonical identity grouping, shadow logic) is fully designed but not yet integrated into the processing pipeline. Without it, the same logical package may appear from multiple sources (gomod + gobinary + heuristic), leading to duplicate CVE matches and inflated risk scores.

**Workaround:** Deduplication at the `cve_matches` level (`ON CONFLICT DO NOTHING` on `sbom_id, package_name, cve_id`) prevents duplicate CVE records, but component-level duplication still exists.

---

### G-05 · Confidence Not Stored as DB Column

**Priority:** Medium  
**Component:** `cve_matches`

There is no `confidence` column in `cve_matches`. Confidence is derived at API response time from `matched_by`: `nvd-fallback` → `low`, `fortuna-core-cve-matcher` → `high`. This is fragile and prevents efficient DB-level filtering.

**Current state:** SBOM-level confidence (`sbom.confidence`) is stored. Component-level and match-level confidence are derived.

---

### G-06 · Match-Type Field Missing

**Priority:** Medium  
**Component:** `cve_matches`

No `match_type` field distinguishes between `osv-package`, `binary-module`, and `nvd-keyword` matches. Adding this would improve observability and allow dashboard filtering.

---

### G-07 · Distroless Cataloger Not Implemented

**Priority:** Medium  
**Component:** `agent/pkg/sbom/extractor/`

The current distroless parser only checks the signature DB allowlist. A full cataloger that walks bin/lib directories to discover unknown binaries and classify them is not yet built. This limits coverage to the pre-defined signature list.

**Implemented:** Signature-based detection with PURL + source/confidence. The parser correctly skips junk files (.pl, .so, share/locale).

**Remaining:** Walk-based discovery + binary classification for binaries not in the signature DB.

---

### G-08 · On-Disk SBOM Cache Not Implemented

**Priority:** Medium  
**Component:** Agent

The spec calls for an on-disk SBOM cache at `/var/lib/fortuna/sbom-cache` keyed by `image_digest + signature_version`. Currently SBOMs are regenerated on every Agent restart for all running pods (though the server-side upsert handles dedup).

---

### G-09 · Worker Parallelism Limited

**Priority:** Low  
**Component:** CVE Matcher Worker

Only one JetStream subscriber processes `fortuna.sbom.created` events. Scaling to multiple replicas or multiple subscriptions is the recommended approach (CVE matching is I/O-bound, not CPU-bound).

---

### G-10 · Go Pseudo-Version Normalization

**Priority:** Low  
**Component:** Matcher

Go pseudo-versions (e.g., `v0.0.0-20230101-abcdef`) are not normalized for comparison. The OSV database may use different pseudo-version formats. A normalize-for-comparison (not mutate-stored-data) strategy is needed.

---

### G-11 · EPSS / KEV Integration

**Priority:** Low  
**Component:** Risk Engine

EPSS (Exploit Prediction Scoring System) from FIRST.org and KEV (Known Exploited Vulnerabilities) from CISA are not integrated. These would improve risk prioritization.

**Design note:** Technical confidence (match quality) must remain separate from risk prioritization (EPSS/KEV) — do not mix into a single field.

---

### G-12 · E2E Test for Distroless + Control-Plane

**Priority:** Low  
**Component:** Testing

No end-to-end test deploys a distroless image (e.g., `gcr.io/distroless/static:nonroot`) or control-plane component and asserts that the Agent produces a non-empty SBOM with PURL, the Dashboard shows the heuristic badge, and at least one CVE is matched.

---

### G-13 · Global NVD Rate Limiter

**Priority:** Low  
**Component:** NVD Client

The current retry logic handles 429 per-worker (exponential backoff: 30 s, 60 s, 120 s, max 3 retries). However, multiple workers or Core replicas may simultaneously hit NVD, exhausting the shared rate budget. A global token-bucket limiter is needed.
