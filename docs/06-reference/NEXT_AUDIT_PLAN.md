# Post-merge audit implementation plan

Baseline reviewed after PRs #40 and #41 merged. Changes continue as focused PRs
and are reviewed/merged manually. IDs below are work packages, not GitHub PR
numbers.

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
- Add security-critical tests to the named regression contract when removal or
  silent fallback would recreate a previous finding.
- Include migration/compatibility notes where applicable.
- Leave merge to the repository owner.

## Deployment/demo gate

Complete A–F before claiming the live investigation flow is validated. Keep
unsupported disable and scoped legacy AGE behavior explicit until H/G land.
The VMware lab should verify two-cluster isolation, populated migrations, agent
identity, evidence loss/recovery, deletion retry and actual UI/API/worker flow.

## Implementation progress

- A: merged in PR #36. Runtime evidence read failures propagate into evaluation and
  reconciliation retains findings when required evidence is unavailable.
- B: merged in PR #37. Permissions API, Pod RBAC reports and risk evaluation use the
  shared resolver with common subject/role/namespace semantics.
- C1: merged in PR #38. Credential registry/principal and security regression
  foundation are present.
- C2 HTTP Agent ingest: merged in PR #39. `/api/v1/agent/*` uses scoped identity
  when the registry is configured; sync and Pod evidence enforce ownership before
  effects.
- C2e2: merged in PR #40. Generic runtime JSONL, Falco and eBPF senders preserve
  telemetry across transient Core rejection; the retry invariants are in the named
  regression contract.
- C2f: merged in PR #41. Per-node token-file provisioning, digest-only Core
  registry, overlap rotation/revocation and deployment overlays are available.
- C2e3: current change. Runtime v1/v2 use the same scoped HTTP identity when the
  registry/token file are configured; complete runtime batches are ownership-
  validated before handler effects. Registered-route tests prove cross-cluster,
  mixed-batch, revocation and invalid-registry rejection while explicit legacy mode
  remains backward compatible.
- C2 HTTP implementation boundary is complete after C2e3 merges and its CI gates
  pass. Two-cluster deployment evidence remains package F, not a code-level claim.
- C3: next. Add gRPC unary/stream identity enforcement, per-message
  reauthentication and SBOM/workload storage isolation.
- D–I: pending. D evidence freshness remains the next functional package after the
  required C transport/storage and F integration boundaries are sufficiently
  established.
