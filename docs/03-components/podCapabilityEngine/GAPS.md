# Pod Capability Engine — Known Gaps

**Last Updated**: 2026-04-13

## Open Gaps

| ID | Description | Priority | Status |
|----|-------------|----------|--------|
| PCE-1 | Attack Graph Phase 2 — use capability facts as input for automated attack path generation | P2 | **In Progress** |
| PCE-2 | eBPF-based runtime detection — replace noop tracepoint with real syscall/namespace monitoring | P2 | Planned |
| PCE-3 | Multi-signal chain scoring — combine static capabilities with runtime signals for chain detection | P2 | **In Progress** |
| PCE-4 | Capability state race condition — CSC lacks locking when multiple sources update state simultaneously | P2 | Known |
| PCE-5 | "Catalog" vs "Metadata" UI naming inconsistency — two names for same data source confuse users | P3 | Known |
| PCE-6 | Kill chain visualization — `kill_chain_stage` data seeded but not visualized in dashboard | P3 | Planned |
| PCE-7 | REP detector governance — replay expectations and detector metadata versioning | P3 | Planned |
| PCE-8 | False positive tuning — `false_positive_considerations` field seeded but not integrated into scoring | P3 | Planned |
| PCE-9 | RBAC snapshot freshness — ServiceAccount/Role data may be stale when PCE evaluates | P3 | Known |

## Completed

| Item | Description |
|------|-------------|
| Phase 1 | Core PCE evaluator with 11 capability rules, MITRE ATT&CK mapping |
| Phase 1.5 | Standardized capability IDs (`DOMAIN_VECTOR_SCOPE`), extended metadata (MITRE, kill chain, mitigations) |
| CSC | CapabilityStateController for state transitions (detected → confirmed → exploited) |
| REP Phase 1 | Runtime Escape Probe Detection specification and initial sensor framework |
| API | Full CRUD + summary + trends endpoints |
| Dashboard | CapabilityMetadataBrowser with search, filter, expandable details |
| Migrations | 047, 050, 060, 061 — capability_metadata table with extended MITRE columns |

## In-Progress (Unified Risk Pipeline — 2026-04-13)

The following items are partially implemented as part of the 5-Layer Unified Risk Pipeline:

### PCE-1: Attack Path now reads PCE capabilities

`graph/relational_path_builder.go` — `BuildPathsForPod()` now:
- Reads `pod_capabilities` for the pod and adds escape edges when `ESC_PRIV_POD`, `ESC_HOSTPATH_NODE`, or `ESC_RUNTIME_ACTIVE` are detected.
- Reads `pod_attack_steps` and adds edges for active attack steps (e.g. `NODE_CRED_DUMP` → `CAN_STEAL_CREDENTIALS`).
- Boosts `TotalRisk` and reduces `Difficulty` based on capability state (`confirmed` +1.0, `exploited` +2.5 / −0.3).
- Sets `EnrichedFromPCE=true` on each enriched path.
- Persists computed paths to the new `attack_paths` table (migration 118) via upsert.

### PCE-3: Multi-signal chain scoring via Unified Scorer V3

`risk/unified_scorer.go` — New `UnifiedScorerV3`:
- Reads from ALL pipeline sources: insights, `pod_capabilities`, `attack_paths`, runtime signals.
- Computes 7-dimensional score: `VULNERABILITY`, `CAPABILITY_EXPOSURE`, `ATTACK_PATH`, `RBAC_POLICY`, `RUNTIME_THREAT`, `EXPOSURE`, `BLAST_RADIUS`.
- Applies toxic-combo boosts (e.g. critical CVE + internet-exposed = +10).
- Persists to `risk_scores` with new V3 dimension columns (migration 119).
- Triggered after every PCE evaluation and after every insight create/update.

### Shared RBAC Analyzer

`rbac/analyzer.go` — Extracted shared RBAC logic:
- PCE evaluator's `hasAPIWriteAccess()` and Attack Path's `classifyRoleRisk()` now both call `rbac.AnalyzePod()` / `rbac.ClassifyRoleRisk()`.
- Eliminates the previous 3-way RBAC duplication.
