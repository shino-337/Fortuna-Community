# Post-merge audit implementation plan

Baseline reviewed after PR #39 merged. Changes continue as focused PRs and are
reviewed/merged manually. IDs below are work packages, not GitHub PR numbers.

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
- C2 HTTP agent ingest: merged in PR #39. `/api/v1/agent/*` uses scoped identity
  when the registry is configured; sync and Pod evidence enforce ownership before
  effects. Runtime route ownership validation exists but is not yet wired.
- C2e2: runtime sender reliability is the next dependency. Generic JSONL retry was
  merged with #39; Falco/eBPF retry and named regressions are isolated in PR #40.
- C2f: per-node credential rollout follows C2e2. The planned rollout uses a
  node-local token file for `/api/v1/agent/*`, a digest-only Core registry,
  overlap rotation/revocation, and an explicit dual-channel migration boundary so
  runtime v1/v2 stays on the shared token until C2e3.
- C2e3: pending. Cut runtime v1/v2 over to scoped authentication, wire the existing
  runtime ownership validator into registered routes, and remove the runtime shared
  token dependency after route-level cross-cluster tests pass.
- C3: pending. gRPC unary/stream identity enforcement, per-message reauthentication
  and SBOM/workload storage isolation.
- D–I: pending after C reaches the required transport/storage boundary; D evidence
  freshness remains the next functional package after C/F prerequisites are clear.
