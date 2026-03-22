# Backlog Execution Plan (2026-03-19)

## Metadata

- Created: 2026-03-19 (UTC)
- Scope: `core/`, `agent/`, `docs/`
- Baseline branch: `main`
- Purpose: execution-first backlog for correctness, reliability, and operational tracking

---

## Prioritization model

- **Tier 0 (P0)**: if not done, system correctness/trust can break
- **Tier 1 (P1)**: system runs, but operational risk is high
- **Tier 2 (P2)**: quality/intelligence improvements
- **Tier 3 (P3)**: docs/hygiene optimization

---

## Tier 0 (P0) - Must do first

| ID | Item | Why critical | Main files |
|---|---|---|---|
| A1 | Remove hardcoded Cluster ID | Mis-attribution risk in multi-cluster ingestion | `core/internal/grpc/handler_combined.go` |
| A3 | Replay guard E2E determinism proof | Repo-level and worker-level guards exist, but need full pipeline correctness proof | `core/pkg/worker/*`, `core/internal/repository/*`, event tests |
| B2 | Production-grade rate limiting | Availability and abuse control for ingest paths | `core/internal/middleware/security.go` |
| C0 | SBOM correctness (ground truth) | SBOM sai/lỗi làm mọi layer phía trên trở thành “fake intelligence” | `core/pkg/sbom/*`, `core/internal/grpc/*`, `agent/pkg/sbom/*` |
| C0.1 | Emit SBOM on pull failure | No SBOM means unknown state; block downstream risk decisions deterministically | SBOM pull worker + SBOM pipeline |
| C0.2 | Distroless signal preservation | Avoid dropping “unknown binaries” signals (no silent SBOM holes) | SBOM extractor + SBOM normalization |
| C0.3 | Unknown version controlled matching | Unknown versions must not be skipped; match should be bounded + observable | matcher inputs + matcher fallback |
| C0.4 | Ecosystem fallback without PURL | Preserve ecosystem context even when PURL coupling fails | ecosystem mapping + matcher |
| C0.5 | NVD fallback trigger based on match failure | Only fall back when match actually fails, not on missing metadata | NVD integration + matcher |
| C0.6 | Work queue key = pod_uid | Ensure replay/idempotency correctness under concurrency | queue + ingest handlers |
| C0.7 | SBOM status model | Downstream must know whether SBOM is `complete|partial|failed` | SBOM models + storage + API |
| C1 | RPM parser minimum viable completion | Coverage gap = data integrity gap for RPM ecosystems | `agent/pkg/sbom/extractor/rpm.go`, `agent/pkg/sbom/extractor/parsers/rpm.go` |

### Tier 0 acceptance criteria

- A1: no hardcoded `"default"` cluster in ingestion path.
- A3: **deterministic system outcome** — same SBOM digest must produce the same DB state and the same CVE/match outputs, regardless of event order/replay.
- B2: limiter policy documented; tests for burst + sustained; and **operational behavior** is correct under load (no starvation of valid agents; no bypass via concurrent connections).
- C0: SBOM pipeline is **ground-truth correct** — pull failures still emit a SBOM with `sbom_status=failed`; distroless signals are preserved; unknown versions do not skip matches; ecosystem fallback works without PURL; NVD fallback triggers only on actual match failure; ingestion queue uses `pod_uid` as key; `sbom_status` is propagated end-to-end.
- C1: for RPM fixtures, component count is stable, version normalization is correct, PURL is deterministic, and downstream CVE match results are stable.

---

## Tier 1 (P1) - Stability and operability

| ID | Item | Why important | Main files |
|---|---|---|---|
| G1 | SBOM firewall (ingestion contract validation) | Prevent malformed/unsafe SBOM from entering risk decisions; blocks D1/D2 | `core/internal/grpc/*`, `core/pkg/sbom/*` |
| B1 | DLQ alerting integration | Prevent silent failures in worker system | `core/pkg/worker/dlq.go` |
| D1 | Policy events -> insight pipeline (basic) | Event-only flow without insights is incomplete | `core/pkg/policy/events.go` |
| G2 | Backpressure strategy | Define behavior when queue saturation occurs (survival mechanism) | worker queue + ingest handlers |

### Tier 1 acceptance criteria

- B1: alerts emitted on DLQ threshold + runbook entry.
- D1: policy event creates at least baseline insight/alert output.
- G1: ingestion rejects/quarantines deterministically based on **required fields** (`image`, `digest`, `components`) + **semantic validation** (version format, ecosystem mapping). Malformed SBOM must be marked with `sbom_status` and have an observable operator signal (metric/log/event).
- G2: explicit policy (`drop|retry|defer|block`) documented and tested.

---

## Tier 2 (P2) - Intelligence and diagnostics

| ID | Item | Outcome | Main files |
|---|---|---|---|
| D2 | Real correlation calculation | Remove placeholder analytics output | `core/internal/api/risk/analytics_handlers.go` |
| C2 | Extractor coverage parity report | Visibility across ecosystem extraction quality | extractor tests + docs |
| A2 | Build metadata wiring | Accurate version/build traceability | `core/internal/grpc/handler_sbom.go` |
| G3 | Agent trust model hardening | Clear trust boundary for agent-originated data | core firewall + policy docs |
| G4 | Versioned event compatibility policy | Upgrade-safe event evolution | `core/pkg/sbom/events.go`, worker schema checks |

---

## Tier 3 (P3) - Docs/hygiene (limited scope)

| ID | Item | Scope limit |
|---|---|---|
| E1 | SBOM docs canonicalization | Mark canonical docs + add archive banners (no full rewrite) |
| E2 | Architecture docs current-state index | One current-state page + links to archive |
| E3 | Risk-center docs trim | Keep one plan + one ops doc |
| F1/F2 | Cleanup tasks | Remove obsolete TODO/docs noise opportunistically |

---

## Sprint execution plan

## Sprint 1 (target: 2026-03-20 to 2026-03-27)

- A1, A3, B2
- Exit criteria:
  - Cluster attribution fixed
  - Replay determinism proven in E2E tests
  - Ingest rate limiting enabled with safe defaults

## Sprint 2 (target: 2026-03-28 to 2026-04-04)

- C0.1, C0.2, C0.3, C0.4, C0.5, C0.6, C0.7
- B1 (DLQ alerting + regression)
- Exit criteria:
  - SBOM is emitted even on pull failure and carries `sbom_status`.
  - Distroless/unknown version do not silently drop signals; fallback triggers are bounded and observable.
  - Ingestion queue uses `pod_uid` key for deterministic replay.
  - DLQ alert path operational.

## Sprint 3 (target: 2026-04-05 to 2026-04-12)

- C1 (RPM MVP) + D1 (policy->insight baseline)
- Optional: A3 regression on SBOM determinism (if needed)
- Exit criteria:
  - RPM fixtures produce deterministic components + deterministic downstream match output.
  - Policy events produce baseline `insights` on top of trustworthy SBOM.

## Sprint 4 (target: 2026-04-13 to 2026-04-17)

- G1 (SBOM firewall full hardening) + G2 (backpressure survival policy) + D2 (analytics beyond placeholders), as capacity allows
- E1, E2, E3, F1/F2 (max 20-30% total effort)

---

## Tracking board template

Use this table to update status per item in each weekly review.

| ID | Owner | Status | Start date | Due date | PR/Issue | Blocking | Risk if delayed | Notes |
|---|---|---|---|---|---|---|---|---|
| A1 |  | Done |  |  |  |  |  | Dynamic cluster ID in ingest path |
| A3 |  | Done |  |  |  |  |  | E2E replay determinism tests |
| B2 |  | Done |  |  |  |  |  | `docs/RATE_LIMITING.md` + middleware contract tests |
| C0.1 |  | Done |  |  |  | C0 | High (no SBOM = no ground truth) | Emit SBOM on pull failure |
| C0.2 |  | Done |  |  |  | C0 | High (distroless holes = missing risk coverage) | Distroless signal preservation |
| C0.3 |  | Done |  |  |  | C0 | High (unknown versions = skipped matches) | Unknown version controlled matching |
| C0.4 |  | Done |  |  |  | C0 | High (ecosystem context lost) | Ecosystem fallback without PURL |
| C0.5 |  | Done |  |  |  | C0 | High (wrong fallback = fake intelligence) | NVD fallback trigger based on match failure |
| C0.6 |  | Done |  |  |  | C0 | Medium/High (concurrency replay drift) | Work queue key = pod_uid |
| C0.7 |  | Done |  |  |  | C0 | High (downstream trust gap) | SBOM status model complete/partial/failed |
| C1 |  | Done |  |  |  |  |  | RPM inventory manifest + stable PURL |
| B1 |  | Done |  |  |  |  |  | DLQ threshold metrics/log + runbook |
| D1 |  | Done |  |  |  |  |  | Policy violation → baseline `insights` |
| G1 |  | Todo |  |  |  | D2 + policy insights | High (malformed SBOM enters risk decisions) | SBOM firewall hardening |
| G2 |  | Todo |  |  |  | pipeline safety | Medium/High (storms or data loss) | Backpressure survival policy |
| D2 |  | Todo |  |  |  | C0/D1 | Medium/High (analytics on bad data) | Analytics beyond placeholders |
| C2 |  | Todo |  |  |  | C0/C1 | Medium (coverage blind spots) | Extractor coverage parity report |
| A2 |  | Todo |  |  |  |  |  | Build metadata wiring |
| G3 |  | Todo |  |  |  |  |  | Agent trust model hardening |
| G4 |  | Todo |  |  |  |  |  | Versioned event compatibility policy |
| E1 |  | Todo |  |  |  |  |  | SBOM docs canonicalization |
| E2 |  | Todo |  |  |  |  |  | Architecture docs current-state index |
| E3 |  | Todo |  |  |  |  |  | Risk-center docs trim |
| F1/F2 |  | Todo |  |  |  |  |  | Cleanup tasks |

---

## Weekly review checklist

- [ ] Tier 0 items still prioritized above docs cleanup.
- [ ] New TODOs are categorized into Tier 0/1/2/3.
- [ ] Each in-progress item has owner + due date.
- [ ] Test/race evidence linked in PR notes.
- [ ] Any scope drift is explicitly approved.

