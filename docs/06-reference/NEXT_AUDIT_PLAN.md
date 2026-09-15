# Post-merge audit implementation plan

Baseline: main 63b7ae3 (PRs #29–35 merged). Each change is reviewed and merged
manually. IDs below are work packages, not GitHub PR numbers.

| Order | Package | Deliverables | Acceptance gate | Dependencies |
| --- | --- | --- | --- | --- |
| A | Runtime input errors | Propagate projector, enrichment and required evidence errors into all evaluators; preserve findings on failure | Query/decode failure yields an evaluation error, no partial snapshot or successful clean result; worker regression preserves active finding | Baseline |
| B | Shared RBAC semantics | Common subject, role reference and namespace/cluster scope resolution for permissions and risk | SA/User/Group and RB/CRB matrix agrees across API and rules | A |
| C | Agent authentication | Bind credentials to agent/cluster for HTTP, unary and streaming gRPC; migration, rotation and revocation | Cluster A credential cannot ingest or heartbeat as B; revoked credentials rejected | Baseline |
| D | Evidence freshness | Collection status, observation time and completeness; auto-resolution eligibility; UI explanation | Missing/stale telemetry cannot mean clean; static and runtime evidence distinguished | A, B |
| E | API and UI availability | Explicit stats/node/capability errors, retry states, actual agent version and separate health signals | DB failure is not zero counts; consistent unavailable state across detail/list | A |
| F | Integration gates | Populated PostgreSQL migration, AGE, worker reconciliation, Kubernetes deletion retry, two-cluster isolation | Reproducible integration results including concurrent ingestion/manual changes and replacement UID | Extend incrementally with A–E |
| G | Scoped AGE | Scope all traversal entry points, nodes and edges before reopening restricted-user access | Duplicate names across two clusters never expose foreign nodes/edges | C, F |
| H | Mutations | Explicit revocation workflow and durable deletion/retry/audit semantics | User knows the exact Kubernetes effects; retry cannot delete replacement objects | C, F |
| I | Performance and first investigation | Benchmark trend queries/cache, optimize measured bottlenecks, versioned docs and validated walkthrough | Recorded load/resource results and successful first-finding investigation | A–F; feature claims reflect G/H status |

## A: implementation boundary

Required database failures must reach the evaluator. An expired snapshot must not
be used after its refresh fails. Malformed persisted JSON must not silently become
an empty collection. Existing reconciliation error handling must retain findings.

The existing five-minute snapshot cache remains in A. Collection freshness,
source health, snapshot invalidation and live telemetry completeness belong to D.
Database-free rule preview behavior remains supported; it is not live evidence.

## Review checklist for every PR

- Document the problem, behavior change and remaining limitations.
- Add regressions for the failure scenario and retain successful-path coverage.
- Run relevant tests, then the repository CI gates before declaring ready.
- Include migration/compatibility notes where applicable.
- Leave merge to the repository owner.

## Deployment/demo gate

Complete A–F before claiming the live investigation flow is validated. Keep
unsupported disable and scoped legacy AGE behavior explicit until H/G land.
The VMware lab should verify two-cluster isolation, populated migrations, agent
identity, evidence loss/recovery, deletion retry and actual UI/API/worker flow.

## Implementation progress

- A: merged in PR #36; runtime evidence read failures propagate to reconciliation.
- B: shared resolver implemented for permissions, Pod RBAC reports and the risk
  cluster-admin projection; scope/subject/error regression matrix added.
- Next: C, per-agent/per-cluster credential enforcement across HTTP and gRPC.

- B: merged in PR #37.
- C1: credential registry/principal and explicit regression contract implemented;
  transport enforcement is not active. C2 covers HTTP and C3 covers gRPC/storage
  isolation; see [credential foundation](AGENT_CREDENTIAL_FOUNDATION.md).
