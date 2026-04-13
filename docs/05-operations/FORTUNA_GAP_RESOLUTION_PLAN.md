# Fortuna GAP Resolution & Optimization Plan

**Date:** 2026-04-10
**Status:** Draft v1.0
**Scope:** Toàn bộ GAP hiện có + tối ưu dung lượng/hiệu năng cho Fortuna
**Branch:** `plan/fortuna-gap-resolution-and-optimization`

---

## Mục lục

1. [Tổng quan hiện trạng GAP](#1-tổng-quan-hiện-trạng-gap)
2. [Phân tích tối ưu dung lượng & hiệu năng](#2-phân-tích-tối-ưu-dung-lượng--hiệu-năng)
3. [Chiến lược xử lý tối ưu](#3-chiến-lược-xử-lý-tối-ưu)
4. [Chi tiết từng GAP và kế hoạch xử lý](#4-chi-tiết-từng-gap-và-kế-hoạch-xử-lý)
5. [Sprint Execution Plan](#5-sprint-execution-plan)
6. [Tracking & Review](#6-tracking--review)

---

## 1. Tổng quan hiện trạng GAP

### 1.1 Inventory — Tổng hợp GAP từ tất cả nguồn

Dựa trên phân tích toàn bộ tài liệu (`GAP_IMPLEMENTATION_STATUS.md`, `GAP_STATUS_RUNTIME.md`, `BACKLOG_EXECUTION_PLAN_2026-03-19.md`, `FORTUNA_RUNTIME_IMPLEMENTATION_BACKLOG.md`, `Risk-Center-Gap-Analysis-And-Improvement.md`, `TECHNICAL_DEBT_ANALYSIS.md`), dưới đây là bảng tổng hợp **toàn bộ GAP đang mở**:

#### A. Runtime & Detection GAPs (Deferred từ GAP_IMPLEMENTATION_STATUS)

| ID | GAP | Mô tả | Trạng thái | Ưu tiên |
|----|-----|--------|------------|---------|
| G-RE-01 | Risk Scorer v3 cut-over | Replace V2 total với v3 dimensions + persist `scorerVersion` | Preview done, cut-over deferred | **P1** |
| G-REP-01-adv | Detector dynamic cooldown | Per-detector dynamic cooldown from confidence; time-based decay | Deferred | P2 |
| G-R9 | Real eBPF exec/connect | eBPF sensor thực sự thu thập pid/args/dst + map PID→podUID | Phase-1 done (noop), real impl deferred | **P1** |
| G-R6 | Multi-signal chain calibration | Multi-signal chain, correlation process/network, calibration nâng cao | Phase-1 done, advanced deferred | P2 |
| G-R5 | Byte/flow per connection | Packet/throughput counters theo flow | Baseline done, flow counters deferred | P2 |
| G-P2-COV | Formal coverage store | Runtime coverage model formal | Deferred | P2 |
| G-P2-GRAPH | Graph edges for attack path | Attack-path substrate (relational graph) | Deferred | P2 |
| G-UI-01 | Pod Detail IA redesign | UX information architecture redesign | Deferred | P3 |
| G-E2E-01 | Full detector matrix automation | E2E test cho tất cả detectors | Deferred | P2 |

#### B. Backlog Execution Plan GAPs (BACKLOG_EXECUTION_PLAN)

| ID | GAP | Trạng thái | Ưu tiên |
|----|-----|------------|---------|
| G1 | SBOM firewall (ingestion contract validation) | **In Progress → PR** | **P1** |
| G2 | Backpressure survival policy | **In Progress → PR** | **P1** |
| D2 | Real correlation calculation | Todo | P2 |
| C2 | Extractor coverage parity report | Todo | P2 |
| A2 | Build metadata wiring | Todo | P3 |
| G3 | Agent trust model hardening | Todo | P2 |
| G4 | Versioned event compatibility policy | Todo | P2 |
| E1 | SBOM docs canonicalization | Todo | P3 |
| E2 | Architecture docs current-state index | Todo | P3 |
| E3 | Risk-center docs trim | Todo | P3 |
| F1/F2 | Cleanup tasks | Todo | P3 |

#### C. Risk Center GAPs (Risk-Center-Gap-Analysis)

| ID | GAP | Trạng thái | Ưu tiên |
|----|-----|------------|---------|
| RC-5 | Retention configurable — document env | Doc missing | P3 |
| RC-6 | Observability (OTel, alerting) | Prometheus metrics partial | P2 |
| RC-7 | Audit "view" insight | Missing | P3 |
| RC-8 | Global view endpoint + UI | Missing | P2 |

#### D. Technical Debt GAPs

| ID | GAP | Trạng thái | Ưu tiên |
|----|-----|------------|---------|
| TD-1 | Proto import path mismatch | Known, not fixed | P2 |
| TD-2 | Multiple conflicting gRPC clients | Known, not fixed | P2 |
| TD-3 | Config namespace mismatch (ksam vs fortuna) | Known, not fixed | P3 |
| TD-4 | Agent doing CVE matching (architecture violation) | Partially addressed | P2 |

#### E. Risk Processing GAPs (từ phân tích risk processing logic)

| ID | GAP | Mô tả | Ưu tiên |
|----|-----|--------|---------|
| RP-1 | CVSS vector decomposition | Scorer chỉ dùng scalar CVSS, không tách AV/AC/PR | **P1** → **Fixed** |
| RP-2 | EPSS/KEV staleness | EPSS/KEV chỉ enrich lần đầu, không refresh | **P1** |
| RP-3 | ExploitAvailable/ExploitMaturity plumbing | Có field nhưng Scorer không dùng | P2 → **Fixed** |
| RP-4 | Asset context — network exposure, tiers | ResourceInfoV2 fields luôn rỗng | P2 |
| RP-5 | Persistent false-positive / exception model | **Completed** (migration 116 + batch upsert fix) | **P0** → **Fixed** |
| RP-6 | Insight triage states incomplete | Acknowledge chỉ update timestamp, resolve ghi đè recommendation | P2 |
| RP-7 | SLA tracking | Không có deadline/breach tracking | P2 |
| RP-8 | CVE-to-Insight scan provenance | Insight không ghi resolver_version/mirror_version | P2 |
| RP-9 | Cross-resource CVE roll-up | N pod cùng image tạo N insight riêng | P2 |
| RP-10 | License risk evaluation | License field stored nhưng không evaluate | P2 |
| RP-11 | Dedup on pod spec hash | `LastEvaluatedHash` tồn tại nhưng không check | P2 |
| RP-12 | CVSS base score avg vs max | `calculateCVEBaseScore()` dùng weighted average thay vì max | P2 → **Fixed** |
| RP-13 | Network policy gap detection | Không detect pod thiếu NetworkPolicy | P2 |
| RP-14 | Audit trail snapshot | Audit log chỉ ghi action, không snapshot state | P3 |

### 1.2 Thống kê tổng hợp

| Category | Total GAPs | P0 | P1 | P2 | P3 |
|----------|-----------|----|----|----|----|
| Runtime & Detection | 9 | 0 | 2 | 5 | 2 |
| Backlog Execution | 11 | 0 | 2 | 4 | 5 |
| Risk Center | 4 | 0 | 0 | 2 | 2 |
| Technical Debt | 4 | 0 | 0 | 3 | 1 |
| Risk Processing | 14 | 1 | 2 | 9 | 2 |
| **Tổng** | **42** | **1** | **6** | **23** | **12** |

---

## 2. Phân tích tối ưu dung lượng & hiệu năng

### 2.1 Phân tích hiện trạng codebase

| Component | Files | Size | Ghi chú |
|-----------|-------|------|---------|
| Core (Go) | 419 .go files | 129 MB | Bao gồm ~96 migrations |
| Agent (Go) | 94 .go files | 59 MB | Bao gồm SBOM extractors |
| Dashboard (TS) | 72 .tsx/.ts | 1.4 MB | React 19 + Vite |
| Docs (MD) | 229 .md files | 3.3 MB | Nhiều doc trùng lặp/outdated |
| cve-data | — | ~phần lớn core | NVD/OSV mirror data |

### 2.2 Chiến lược tối ưu dung lượng

#### A. Docs consolidation (Tiết kiệm ~40% docs size)

**Vấn đề:** 229 markdown files, nhiều nội dung trùng lặp:
- `docs/03-components/risk-center/` — consolidated into `README.md` and `GAPS.md`
- `docs/06-reference/` — nhiều doc cũ từ 2024-2025 chưa archive
- `docs/06-reference/migration/` — 13 files migration docs đã xong

**Giải pháp:**
1. Consolidate risk-center docs: giữ 1 operational doc + 1 plan doc + GAP status
2. Archive migration docs cũ vào `docs/06-reference/migration/archive/`
3. Remove các analysis docs đã incorporate vào implementation
4. Tạo `docs/00-index.md` — single entry point cho toàn bộ docs

#### B. Core binary optimization (Tiết kiệm ~15-25% binary size)

**Vấn đề:**
- Legacy scanner code (Trivy-based) vẫn còn (`//go:build legacy_trivy`)
- `cve-data/` embedded or loaded at runtime — lớn
- 96 migration files compile thành binary

**Giải pháp:**
1. Remove legacy_trivy scanner code (`core/pkg/scanner/`) — đã deprecated
2. Optimize migration: consolidate old migrations thành baseline
3. Lazy-load CVE data thay vì embed toàn bộ
4. Review `go.sum` — remove unused indirect dependencies

#### C. Agent memory optimization (Critical cho DaemonSet)

**Vấn đề:**
- Agent DaemonSet hiện cần **6Gi memory limit** — rất cao cho DaemonSet
- SBOM extraction cho large images tiêu tốn memory
- FalcoReader giữ state trong memory

**Giải pháp:**
1. Stream-based SBOM extraction thay vì load toàn bộ layers vào memory
2. Giới hạn concurrent SBOM processing (đã có `SBOM_WORKERS=1`)
3. Implement memory-bounded FalcoReader buffer
4. Đặt target: **agent memory ≤ 2Gi** (giảm 67%)

#### D. Database optimization

**Vấn đề:**
- `insights` table chứa JSONB evidence — bloat
- `runtime_events` append-only growth
- Nhiều N+1 queries đã fix cho SBOM, nhưng risk scoring có thể tương tự
- Missing database indexes cho common query patterns

**Giải pháp:**
1. Implement insights retention policy (đã có scheduler, cần tune)
2. Partition `runtime_events` theo time range
3. Decompose large JSONB columns vào structured fields
4. Add composite indexes cho risk scoring queries
5. Implement read replicas cho dashboard queries

#### E. Dashboard optimization

**Vấn đề:**
- Bundle size chưa optimize (no code splitting)
- No service worker / caching strategy

**Giải pháp:**
1. Implement React lazy loading cho routes
2. Add gzip/brotli compression
3. Optimize chart data fetching (pagination/aggregation server-side)

### 2.3 Ước tính impact tối ưu

| Optimization | Effort | Impact | Priority |
|-------------|--------|--------|----------|
| Docs consolidation | 2h | Giảm ~40% docs, dễ navigate | P3 |
| Remove legacy scanner | 1h | Giảm ~5% core binary, cleaner code | P2 |
| Migration consolidation | 4h | Giảm compile time, cleaner schema | P3 |
| Agent memory reduction | 8-16h | Giảm 67% agent memory (6Gi→2Gi) | **P1** |
| DB partitioning/indexing | 8h | Query perf improvement 2-10x | **P1** |
| Dashboard lazy loading | 2h | Giảm ~30% initial load time | P2 |

---

## 3. Chiến lược xử lý tối ưu

### 3.1 Nguyên tắc phân nhóm

Thay vì xử lý 42 GAP tuần tự, nhóm theo **dependency chain** và **shared context** để tối ưu effort:

```
Batch 1: Foundation & Safety (P0/P1)
├── RP-5  (Exception model)          ─── cần trước khi fix scoring
├── G1    (SBOM firewall)            ─── cần trước khi trust data
├── G2    (Backpressure)             ─── cần cho production safety
└── Agent memory optimization        ─── operational necessity

Batch 2: Scoring & Intelligence (P1/P2) — shared scorer.go context
├── G-RE-01 (Scorer v3 cut-over)    ─┐
├── RP-1    (CVSS vector)            │ Tất cả sửa trong scorer.go
├── RP-2    (EPSS refresh)           │ và CVEMatcherWorker
├── RP-12   (Score avg→max)          │
├── RP-3    (ExploitAvailable)       ─┘
└── DB optimization (indexes)        ─── support scoring queries

Batch 3: Detection & Coverage (P1/P2) — shared runtime context
├── G-R9     (Real eBPF)            ─┐
├── G-R6     (Multi-signal chain)    │ REP + detector pipeline
├── G-E2E-01 (Detector E2E)         ─┘
├── G-R5     (Byte/flow counters)    ─── network monitoring
└── G-REP-01-adv (Dynamic cooldown) ─── detector governance

Batch 4: Risk Operations (P2) — shared insight/API context
├── RP-6  (Triage states)           ─┐
├── RP-7  (SLA tracking)             │ Insight model changes
├── RP-8  (Scan provenance)          │ + API handlers
├── RP-14 (Audit trail snapshot)     │
├── RC-7  (Audit view)              ─┘
├── RC-8  (Global view)
├── RP-9  (Image-level CVE rollup)
├── RP-11 (Pod spec hash dedup)
└── RP-10 (License risk eval)

Batch 5: Platform & Coverage (P2)
├── G-P2-COV   (Coverage store)
├── G-P2-GRAPH (Attack path graph)
├── D2         (Real correlation)
├── RP-4       (Asset context)
├── RP-13      (NetworkPolicy gaps)
└── G3/G4      (Agent trust + event compat)

Batch 6: Cleanup & Docs (P3) — parallel work
├── TD-1/TD-2/TD-3  (Tech debt cleanup)
├── RC-5            (Retention docs)
├── E1/E2/E3/F1/F2  (Docs cleanup)
├── A2              (Build metadata)
├── G-UI-01         (Pod Detail redesign)
└── Docs consolidation
```

### 3.2 Tối ưu qua batching

**Ước tính tiết kiệm effort khi batch vs sequential:**

| Approach | Estimated Total Effort | Context Switch Overhead |
|----------|----------------------|------------------------|
| Sequential (42 items) | ~320h | ~80h (25%) overhead |
| Batched (6 batches) | ~260h | ~30h (12%) overhead |
| **Tiết kiệm** | **~60h (~19%)** | **~50h context switch** |

Lý do:
- Batch 2: cùng `scorer.go` + `cve_matcher_worker.go` → sửa 1 lần, test 1 lần
- Batch 4: cùng insight model + handler → 1 migration, 1 API review
- Batch 6: docs/cleanup có thể parallel với development batches

### 3.3 Cross-cutting concerns

Mỗi batch cần đảm bảo:
1. **Migration backward compat** — migrations phải additive, không break rolling deploy
2. **API backward compat** — new fields optional, old clients không gãy
3. **Test coverage** — mỗi GAP fix phải có test (unit + integration)
4. **Observability** — mỗi change phải có metric/log cho ops

---

## 4. Chi tiết từng GAP và kế hoạch xử lý

### Batch 1: Foundation & Safety

#### RP-5: Persistent False-Positive / Exception Model
**Vấn đề:** `InsightManager.createOrUpdateInsightTx()` re-activates dismissed insights.
**Files:**
- `core/pkg/riskengine/insight_manager.go` (lines 71–104)
- `core/internal/api/insights_handlers.go`

**Kế hoạch:**
1. Tạo model `ExceptionPolicy` (resource_uid, cve_id, insight_type, reason, expires_at, created_by)
2. Migration: `core/migrations/112_add_exception_policies.go`
3. Sửa `createOrUpdateInsightTx()`: check exception_policies trước khi re-activate
4. API: `POST/GET/DELETE /api/v1/risk/exceptions`
5. Dashboard: Exception management panel trong Risk Center Settings

**Effort:** ~12h | **Risk:** Low | **Dependencies:** None

#### G1: SBOM Firewall
**Vấn đề:** Malformed SBOM có thể enter risk decisions.
**Files:**
- `core/internal/grpc/handler_sbom.go`
- `core/pkg/sbom/validation.go` (new)

**Kế hoạch:**
1. Tạo validation layer: required fields (image, digest, ≥1 component), semantic checks
2. Reject/quarantine invalid SBOM với `sbom_status=rejected`
3. Metric: `fortuna_sbom_validation_total{result=accepted|rejected|quarantined}`
4. Test: fixtures cho malformed SBOM

**Effort:** ~8h | **Risk:** Low | **Dependencies:** None

#### G2: Backpressure Survival Policy
**Vấn đề:** Không có behavior rõ ràng khi queue saturation.
**Files:**
- `core/pkg/worker/pool.go`
- `core/internal/ingest/rate_limiter.go`

**Kế hoạch:**
1. Define policy: `drop|retry|defer|block` per queue type
2. Implement circuit breaker cho SBOM ingest
3. NATS consumer flow control configuration
4. Metric: `fortuna_queue_pressure{queue,policy,result}`
5. Document survival behavior

**Effort:** ~8h | **Risk:** Medium | **Dependencies:** None

#### Agent Memory Optimization
**Vấn đề:** Agent DaemonSet yêu cầu 6Gi memory.
**Files:**
- `agent/internal/sbom/processor.go`
- `agent/internal/runtime/falco_reader.go`
- `deploy/fortuna-agent-daemonset.yaml`

**Kế hoạch:**
1. Profile memory usage: SBOM extraction vs Falco vs runtime collection
2. Stream-based SBOM extraction: process layers incrementally
3. Bounded Falco buffer (ring buffer, max 10K events)
4. Memory-pool reuse for SBOM components
5. Target: ≤2Gi memory limit
6. Test: load test với 200+ pods/node, large images

**Effort:** ~16h | **Risk:** Medium-High | **Dependencies:** None

---

### Batch 2: Scoring & Intelligence

#### G-RE-01: Scorer v3 Cut-over
**Vấn đề:** V3 dimensions chỉ ở preview, V2 vẫn là production scorer.
**Files:**
- `core/pkg/risk/scorer.go`
- `core/pkg/risk/score_v3_preview.go`
- `core/pkg/riskengine/score_v3_types.go`

**Kế hoạch:**
1. Promote v3 dimensions thành primary scorer
2. Persist `scorerVersion` trong `risk_scores`
3. Feature flag: `FORTUNA_SCORER_VERSION=v2|v3` (default v3)
4. Migration: add `scorer_version` column
5. Test: golden test files cho score stability

**Effort:** ~12h | **Risk:** Medium | **Dependencies:** RP-1, RP-12

#### RP-1: CVSS Vector Decomposition
**Files:** `core/pkg/risk/scorer.go:318-496`, `core/pkg/models/cve.go`

**Kế hoạch:**
1. Parse CVSS v3 vector từ NVD/OSV data (đã có raw data)
2. Store parsed components: AV, AC, PR, UI, S → `cves.cvss_vector_parsed jsonb`
3. Scorer dùng AV thay cho `scoreAttackVector()` heuristic
4. Scorer dùng PR thay cho `scoreAuthRequirement()` heuristic

**Effort:** ~8h | **Risk:** Low | **Dependencies:** None

#### RP-2: EPSS/KEV Periodic Refresh
**Files:** `core/pkg/epss/client.go`, `core/pkg/kev/catalog.go`, `core/pkg/worker/cve_matcher_worker.go:307-370`

**Kế hoạch:**
1. New scheduler job: `InsightEnrichmentRefresher` (6h interval)
2. Batch query insights with vulnerability type (last 90 days)
3. Bulk EPSS lookup + KEV check
4. Merge vào `insight.Evidence` via `insightevidence.Merge()`
5. Re-trigger risk score calculation cho affected resources
6. Add `evidence_refreshed_at` timestamp

**Effort:** ~8h | **Risk:** Low | **Dependencies:** None

#### RP-12: Score Average → Maximum
**Files:** `core/pkg/risk/scorer.go:547-625`

**Kế hoạch:**
1. Change `calculateCVEBaseScore()` → use max CVSS score (top-1) + log-scale count bonus
2. Backward compat: configurable via `FORTUNA_SCORER_CVE_AGGREGATION=max|weighted_avg`
3. Update golden tests

**Effort:** ~3h | **Risk:** Low | **Dependencies:** None

#### RP-3: ExploitAvailable/ExploitMaturity
**Files:** `core/pkg/risk/scorer.go:428-494`, `core/pkg/models/cve.go`

**Kế hoạch:**
1. Plumb `CVE.ExploitAvailable` + `ExploitMaturity` vào `CVEMatch`
2. Store trong `insight.Evidence` structured fields
3. Rewrite `scoreExploitAvailability()` dùng structured fields thay vì string matching

**Effort:** ~4h | **Risk:** Low | **Dependencies:** RP-1

#### DB Optimization — Indexes
**Kế hoạch:**
1. Add composite index: `insights(resource_uid, insight_type, status)`
2. Add index: `cve_matches(sbom_id, severity)`
3. Add partial index: `insights(status) WHERE status = 'active'`
4. Benchmark risk scoring query performance before/after

**Effort:** ~4h | **Risk:** Low | **Dependencies:** None

---

### Batch 3: Detection & Coverage

#### G-R9: Real eBPF Implementation
**Kế hoạch:**
1. Implement exec tracepoint: capture pid, comm, args
2. Implement connect tracepoint: capture dst_ip, dst_port
3. PID→podUID resolution via cgroup path
4. Integration test trên cluster

**Effort:** ~32h | **Risk:** High | **Dependencies:** G-R6

#### G-R6: Multi-Signal Chain Calibration
**Kế hoạch:**
1. Correlation engine: process chain + network chain
2. Confidence model cho combined signals
3. Calibration dataset (labeled examples)

**Effort:** ~24h | **Risk:** High | **Dependencies:** G-R9

#### G-E2E-01: Full Detector Matrix
**Kế hoạch:**
1. Test harness cho mỗi detector type
2. Fixture generation từ real telemetry
3. CI pipeline integration

**Effort:** ~16h | **Risk:** Medium | **Dependencies:** G-R9, G-R6

---

### Batch 4: Risk Operations

#### RP-6: Insight Triage States
**Kế hoạch:**
1. Add states: `acknowledged`, `in_progress`, `accepted_risk`
2. `AcknowledgedAt`, `AcknowledgedBy` columns
3. `ResolutionNote` separate from `Recommendation`
4. `accepted_until` expiry for accepted_risk

**Effort:** ~6h

#### RP-7: SLA Tracking
**Kế hoạch:**
1. `sla_policies` table: severity → deadline_hours
2. `sla_deadline`, `sla_breached` columns on insights
3. Daily scheduler: mark breached
4. API filter: `?sla_breached=true`

**Effort:** ~8h

#### RP-8: Scan Provenance
**Kế hoạch:**
1. Add `resolver_version`, `mirror_version` to insights
2. Populate in `buildVulnInsightFromEvent()`

**Effort:** ~3h

#### RC-8: Global View
**Kế hoạch:**
1. `GET /api/v1/insights/summary/global` endpoint
2. Dashboard global summary widget

**Effort:** ~3h

#### RP-9: Image-Level CVE Roll-up
**Kế hoạch:**
1. Add `image_digest` to vulnerability insights
2. `GET /api/v1/risk/images` — aggregated by image
3. Dashboard view

**Effort:** ~8h

#### RP-10: License Risk Evaluation
**Kế hoạch:**
1. License policy YAML rules
2. Evaluator in CVEMatcherWorker
3. InsightType = `license_violation`

**Effort:** ~8h

#### RP-11: Pod Spec Hash Dedup
**Kế hoạch:**
1. Check `LastEvaluatedHash` in RiskWorker before re-evaluation
2. Skip if hash unchanged + no new runtime signals

**Effort:** ~3h

---

### Batch 5: Platform & Coverage

#### G-P2-COV: Coverage Store
**Effort:** ~16h

#### G-P2-GRAPH: Attack Path Graph
**Effort:** ~24h

#### D2: Real Correlation Calculation
**Effort:** ~8h

#### RP-4: Asset Context
**Effort:** ~8h

#### RP-13: NetworkPolicy Gap Detection
**Effort:** ~8h

---

### Batch 6: Cleanup & Docs

#### Docs Consolidation
**Kế hoạch:**
1. Consolidate 38 risk-center docs → 5 canonical docs
2. Archive outdated migration docs
3. Create `docs/00-index.md`
4. Remove duplicate analysis docs

**Effort:** ~4h

#### TD-1/TD-2/TD-3: Tech Debt Cleanup
**Kế hoạch:**
1. Fix proto import paths
2. Consolidate gRPC clients
3. Standardize config namespace

**Effort:** ~8h

---

## 5. Sprint Execution Plan

### Phase 1: Foundation (Tuần 1-2)

**Sprint 1 (Week 1):**
- [x] RP-5: Exception model (12h)
- [x] G1: SBOM firewall (8h)
- [x] DB Optimization indexes (4h)
- **Exit criteria:** Exception policies work, malformed SBOM rejected, queries faster

**Sprint 2 (Week 2):**
- [x] G2: Backpressure policy (8h)
- [ ] Agent memory profiling + optimization bước 1 (16h)
- **Exit criteria:** Queue overflow handled gracefully, agent ≤4Gi

### Phase 2: Intelligence (Tuần 3-4)

**Sprint 3 (Week 3):**
- [ ] RP-1: CVSS vector decomposition (8h)
- [ ] RP-12: Score avg→max (3h)
- [ ] RP-3: ExploitAvailable plumbing (4h)
- [ ] RP-2: EPSS/KEV refresh scheduler (8h)
- **Exit criteria:** Scorer uses structured CVSS, EPSS auto-refreshes

**Sprint 4 (Week 4):**
- [ ] G-RE-01: Scorer v3 cut-over (12h)
- [ ] Agent memory optimization bước 2 (8h)
- **Exit criteria:** V3 scorer production, agent ≤2Gi

### Phase 3: Operations (Tuần 5-6)

**Sprint 5 (Week 5):**
- [ ] RP-6: Triage states (6h)
- [ ] RP-7: SLA tracking (8h)
- [ ] RP-8: Scan provenance (3h)
- [ ] RP-11: Pod spec hash dedup (3h)
- **Exit criteria:** Full triage workflow, SLA tracking operational

**Sprint 6 (Week 6):**
- [ ] RC-8: Global view (3h)
- [ ] RP-9: Image-level CVE roll-up (8h)
- [ ] RP-10: License risk eval (8h)
- **Exit criteria:** Global risk view, image-level view, license scanning

### Phase 4: Detection (Tuần 7-10)

**Sprint 7-8 (Week 7-8):**
- [ ] G-R9: Real eBPF implementation (32h)
- **Exit criteria:** eBPF exec+connect capture working in cluster

**Sprint 9-10 (Week 9-10):**
- [ ] G-R6: Multi-signal chain (24h)
- [ ] G-E2E-01: Detector E2E (16h)
- **Exit criteria:** Multi-signal correlation working, E2E test suite

### Phase 5: Platform (Tuần 11-14)

**Sprint 11-12 (Week 11-12):**
- [ ] G-P2-COV: Coverage store (16h)
- [ ] RP-4: Asset context (8h)
- [ ] RP-13: NetworkPolicy gaps (8h)

**Sprint 13-14 (Week 13-14):**
- [ ] G-P2-GRAPH: Attack path graph (24h)
- [ ] D2: Real correlation (8h)

### Phase 6: Polish (Tuần 15-16)

**Sprint 15-16 (Week 15-16):**
- [ ] Docs consolidation (4h)
- [ ] TD-1/TD-2/TD-3: Tech debt (8h)
- [ ] G-UI-01: Pod Detail IA (16h)
- [ ] RC-5, RC-7, RP-14, A2 (8h)
- [ ] E1/E2/E3/F1/F2 cleanup (4h)

### Timeline Summary

```
Week 1-2:   ████ Foundation & Safety (P0/P1)
Week 3-4:   ████ Scoring & Intelligence (P1/P2)
Week 5-6:   ████ Risk Operations (P2)
Week 7-10:  ████████ Detection & Coverage (P1/P2)
Week 11-14: ████████ Platform (P2)
Week 15-16: ████ Polish & Cleanup (P3)
```

**Tổng effort ước tính:** ~260h (~16 tuần, 1 developer full-time)
**Parallel work possible:** Phase 6 tasks có thể chạy song song với Phase 3-5

---

## 6. Tracking & Review

### Weekly Review Checklist

- [ ] Tất cả P0 items phải done trước khi bắt đầu P2
- [ ] Mỗi sprint có acceptance criteria rõ ràng
- [ ] Mỗi GAP fix có test coverage ≥80%
- [ ] Migration backward compatible
- [ ] API backward compatible
- [ ] Performance regression test cho DB changes
- [ ] Memory profiling cho agent changes

### Success Metrics

| Metric | Current | Target (Phase 1) | Target (Final) |
|--------|---------|-------------------|----------------|
| Agent memory | 6Gi | 4Gi | ≤2Gi |
| Open GAPs | 42 | 35 | 0 |
| Risk score accuracy | Heuristic | Structured CVSS | Full context-aware |
| EPSS freshness | One-time | Auto-refresh 6h | Auto-refresh 6h |
| False-positive handling | Re-activates | Exceptions persist | Exceptions + SLA |
| Insight dedup | Per-pod | Per-pod + hash | Image-level roll-up |
| Test coverage | Partial | +30 tests | +100 tests |
| Docs files | 229 | 229 | ~150 (consolidated) |

### Risk Register

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| eBPF implementation delays | High | Medium | Can operate without eBPF (Falco fallback) |
| Scorer v3 regression | Medium | High | Feature flag + golden tests + canary |
| Agent memory target miss | Medium | Medium | Iterative optimization + profiling |
| Migration breaks rolling deploy | Low | High | Always additive migrations + rollback tested |
| Attack path graph complexity | High | Medium | Start with relational graph, defer AGE |

---

## Appendix A: File Touchpoint Map

### Core — Most Modified Files

| File | GAPs that touch it | Batch |
|------|-------------------|-------|
| `core/pkg/risk/scorer.go` | G-RE-01, RP-1, RP-2, RP-3, RP-12 | B2 |
| `core/pkg/worker/cve_matcher_worker.go` | RP-2, RP-3, RP-8, RP-10 | B2, B4 |
| `core/pkg/riskengine/insight_manager.go` | RP-5, RP-6, RP-8 | B1, B4 |
| `core/internal/api/insights_handlers.go` | RP-6, RP-7, RC-7, RC-8 | B4 |
| `core/pkg/models/implementation_guide.go` | RP-6, RP-7, RP-8 | B4 |
| `core/pkg/worker/risk_worker.go` | RP-11, G-RE-01 | B2 |
| `core/internal/grpc/handler_sbom.go` | G1 | B1 |
| `core/pkg/worker/pool.go` | G2 | B1 |

### Agent — Most Modified Files

| File | GAPs | Batch |
|------|------|-------|
| `agent/internal/sbom/processor.go` | Memory opt | B1 |
| `agent/internal/runtime/falco_reader.go` | Memory opt | B1 |
| `agent/internal/runtime/ebpf_sensor.go` | G-R9 | B3 |

### Dashboard — Most Modified Files

| File | GAPs | Batch |
|------|------|-------|
| `dashboard/pages/Insights.tsx` | RP-6, RP-7, RC-8 | B4 |
| `dashboard/pages/RiskDetail.tsx` | G-RE-01, RP-9 | B2, B4 |
| `dashboard/pages/Settings.tsx` | RP-5 (exceptions) | B1 |

---

## Appendix B: Migration Plan

### Batch 1 Migrations
```sql
-- 116_add_exception_policies.go (RP-5: implemented in this PR)
CREATE TABLE exception_policies (
    id SERIAL PRIMARY KEY,
    resource_uid VARCHAR(255),
    cve_id VARCHAR(100),
    insight_type VARCHAR(50),
    reason TEXT,
    expires_at TIMESTAMP,
    created_by VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);
CREATE INDEX idx_exception_policies_lookup
    ON exception_policies(resource_uid, cve_id, insight_type)
    WHERE deleted_at IS NULL;

-- 117_add_risk_scoring_indexes.go (DB optimization: implemented in this PR)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_insights_resource_type_status
    ON insights(resource_uid, insight_type, status) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cve_matches_sbom_severity
    ON cve_matches(sbom_id, severity) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_insights_active
    ON insights(status) WHERE status = 'active' AND deleted_at IS NULL;
```

### Batch 2 Migrations
```sql
-- 113_add_cvss_vector_parsed.go
ALTER TABLE cves ADD COLUMN cvss_vector_parsed JSONB;

-- 114_add_scorer_version.go
ALTER TABLE risk_scores ADD COLUMN scorer_version VARCHAR(20) DEFAULT 'v2';

-- 115_add_evidence_refreshed_at.go
ALTER TABLE insights ADD COLUMN evidence_refreshed_at TIMESTAMP;
```

### Batch 4 Migrations
```sql
-- 116_add_triage_fields.go
ALTER TABLE insights ADD COLUMN acknowledged_at TIMESTAMP;
ALTER TABLE insights ADD COLUMN acknowledged_by VARCHAR(255);
ALTER TABLE insights ADD COLUMN resolution_note TEXT;
ALTER TABLE insights ADD COLUMN accepted_until TIMESTAMP;

-- 117_add_sla_tracking.go
CREATE TABLE sla_policies (
    id SERIAL PRIMARY KEY,
    severity VARCHAR(20) NOT NULL UNIQUE,
    deadline_hours INT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
INSERT INTO sla_policies (severity, deadline_hours) VALUES
    ('critical', 24), ('high', 168), ('medium', 720), ('low', 2160);

ALTER TABLE insights ADD COLUMN sla_deadline TIMESTAMP;
ALTER TABLE insights ADD COLUMN sla_breached BOOLEAN DEFAULT FALSE;
CREATE INDEX idx_insights_sla ON insights(sla_breached) WHERE sla_breached = TRUE;

-- 118_add_scan_provenance.go
ALTER TABLE insights ADD COLUMN resolver_version VARCHAR(32);
ALTER TABLE insights ADD COLUMN mirror_version VARCHAR(64);
ALTER TABLE insights ADD COLUMN image_digest VARCHAR(255);
CREATE INDEX idx_insights_image_digest ON insights(image_digest);
```

---

*Tài liệu này là living document — sẽ được cập nhật sau mỗi sprint review.*
