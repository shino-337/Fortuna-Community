# Pod Capability Engine — Known Gaps

**Last Updated**: 2026-04-13

## Open Gaps

| ID | Description | Priority | Status |
|----|-------------|----------|--------|
| PCE-1 | Attack Graph Phase 2 — use capability facts as input for automated attack path generation | P2 | Planned |
| PCE-2 | eBPF-based runtime detection — replace noop tracepoint with real syscall/namespace monitoring | P2 | Planned |
| PCE-3 | Multi-signal chain scoring — combine static capabilities with runtime signals for chain detection | P2 | In Progress |
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
