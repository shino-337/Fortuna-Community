# Post-merge audit implementation plan

Baseline reviewed after PR #44 merged. Changes continue as focused PRs and are
reviewed/merged manually. IDs below are work packages; PR #45–#51 references are
the current implementation sequence, not a claim that later work is already done.

| Order | Package | Deliverables | Acceptance gate | Dependencies |
| --- | --- | --- | --- | --- |
| A | Runtime input errors | Propagate projector, enrichment and required evidence errors into all evaluators; preserve findings on failure | Query/decode failure yields an evaluation error, no partial snapshot or successful clean result; worker regression preserves active finding | Baseline |
| B | Shared RBAC semantics | Common subject, role reference and namespace/cluster scope resolution for permissions and risk | SA/User/Group and RB/CRB matrix agrees across API and rules | A |
| C | Agent/resource identity | Bind credentials to agent/cluster for HTTP and gRPC, then preserve cluster-qualified resource identity through storage | Cluster A credential/resource cannot ingest, read, write or reconcile B; ambiguous ownership fails closed | Baseline |
| D | Evidence freshness | Collection status, observation time and completeness; auto-resolution eligibility; UI explanation | Missing/stale telemetry cannot mean clean; static and runtime evidence distinguished | A, B, C |
| E | API and UI availability | Explicit stats/node/capability errors, retry states, actual agent version and separate health signals | DB failure is not zero counts; consistent unavailable state across detail/list | A, C |
| F | Integration gates | Populated PostgreSQL migration, AGE, worker reconciliation, Kubernetes deletion retry, two-cluster isolation | Reproducible integration results including duplicate UID/name/digest, concurrent ingestion/manual changes and replacement UID | Extend incrementally with A–E |
| G | Scoped AGE | Scope all traversal entry points, nodes and edges before reopening restricted-user access | Duplicate names across two clusters never expose foreign nodes/edges | C, F |
| H | Mutations | Explicit revocation workflow and durable deletion/retry/audit semantics | User knows the exact Kubernetes effects; retry cannot delete replacement objects | C, F |
| I | Performance and first investigation | Benchmark trend queries/cache, optimize measured bottlenecks, versioned docs and validated walkthrough | Recorded load/resource results and successful first-finding investigation | A–F; feature claims reflect G/H status |

## Permanent regression-prevention contract

`SECURITY_INVARIANTS.md` defines the rules that survive the individual fixing PRs.
Every security finding fixed in #45–#51 must become a named regression or static CI
invariant. The #51 PostgreSQL/two-cluster integration suite becomes a permanent
gate for later changes touching cluster identity, ingest, authorization, storage,
runtime evidence, findings or migrations.

The target is to make recurrence of known defect classes fail CI. This does not
claim that arbitrary future software defects are impossible; new findings must be
converted into a reproducible invariant before their fixing PR is complete.

## Review checklist for every PR

- Document the problem, behavior change and remaining limitations.
- Add regressions for the failure scenario and retain successful-path coverage.
- Run relevant tests, then the repository CI gates before declaring ready.
- Add security-critical tests to the named regression contract when removal or
  silent fallback would recreate a previous finding.
- Preserve `{cluster_id, resource_uid}` in any new cluster-owned storage/query path.
- Treat missing/ambiguous ownership and unavailable required evidence as fail-closed.
- Include migration/compatibility notes where applicable.
- Leave merge to the repository owner.

## Deployment/demo gate

Complete A–F before claiming the live investigation flow is validated. Keep
unsupported disable and scoped legacy AGE behavior explicit until H/G land.
The VMware lab should verify two-cluster isolation, populated migrations, scoped
HTTP/gRPC agent identity, duplicate Pod UID/node name/image digest, evidence
loss/recovery, deletion retry and actual UI/API/worker flow.

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
  telemetry across transient Core rejection; retry invariants are named regressions.
- C2f: merged in PR #41. Per-node token-file provisioning, digest-only Core
  registry, overlap rotation/revocation and deployment overlays are available.
- C2e3: merged in PR #42. Runtime v1/v2 use scoped HTTP identity when configured;
  complete runtime batches are ownership-validated before handler effects and
  registered-route regressions cover cross-cluster/mixed-batch/revocation cases.
- C3a: merged in PR #43. Scoped gRPC identity is opt-in through
  `FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY`, requires verified mTLS, installs a
  trusted principal for unary/stream calls and reauthenticates before each stream
  receive so revocation/expiry applies to established streams.
- C3b: merged in PR #44. Scoped gRPC method/resource authorization treats the
  trusted Principal as authoritative, canonicalizes cluster metadata, verifies
  Agent claims and Pod ownership before handlers, reauthorizes streamed SBOMs and
  fails closed for unknown future scoped RPCs.
- C3c foundation: current draft PR #45. Introduces canonical
  `{cluster_id, resource_uid}`, adds ClusterID storage fields to workload-derived
  state, performs only unambiguous legacy ownership backfill, runs a fail-closed
  startup schema invariant, and adds static/named CI ratchets. Legacy UID-only
  readers/writers and hard constraints intentionally remain for #46/#47.
- #46 planned: migrate authorization/query/read/write/cache/reconciliation paths
  to cluster-qualified keys; remove ambiguous UID-only ownership resolution.
- #47 planned: separate reusable image/SBOM content from workload ownership, repair
  legacy digest upserts/conflict keys and cluster-qualify CVE/malware associations.
- #48 planned: remove or explicitly quarantine legacy gRPC finding paths as
  compatibility requires; provision per-Agent client certificates and define
  rotation/revocation/rollback. Resolve global AgentID collision semantics.
- #49 planned: D evidence freshness and auto-resolution eligibility.
- #50 planned: E API/UI availability and scoped observability, including Agent status.
- #51 planned: F permanent PostgreSQL/two-cluster integration gate and populated
  migration evidence. Completing #51 is the point at which A–F behavior can be
  claimed as validated end to end.
- G–I remain pending after the A–F gate: scoped AGE, explicit mutation/revocation
  workflows, then performance/load validation and the first-investigation demo.

## Repository governance prerequisite

Security-sensitive paths are covered by CODEOWNERS, but repository rules must
require CODEOWNER review and required CI checks on `main`. Direct/force pushes or
merges that bypass those checks defeat the regression-prevention contract and must
remain disabled by owner-side branch/ruleset configuration.
