# Post-merge audit implementation plan

Status verified after PR #50 merged on 2026-09-22 (merge commit `5024e15`).
Changes continue as focused PRs and are reviewed/merged manually. A–I are work packages. PR numbers for
unopened work are estimates: D is split into D1 and D2 inventory/runtime work, so later PR numbers may shift.

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
Every security finding fixed in packages C–F must become a named regression or static CI
invariant. The package F PostgreSQL/two-cluster integration suite becomes a permanent
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
- C3c foundation: merged in PR #45. Introduces canonical
  `{cluster_id, resource_uid}`, adds ClusterID storage fields to workload-derived
  state, performs only unambiguous legacy ownership backfill, runs a fail-closed
  startup schema invariant, and adds static/named CI ratchets. Legacy UID-only
  readers/writers and hard constraints intentionally remain for #46/#47.
- #46 merged: Pod reads/writes and risk/cache paths use cluster-qualified
  keys. The remaining UID-only InsightStatusUpdater lookups are addressed by D1. Retired HTTP routes and dead handlers are removed; runtime senders use v2.
- #47 merged: mutable observations use
  cluster/Pod/container/digest; immutable `sbom_image_contents` deduplicates the
  actual package snapshot and extraction provenance independently of ownership.
  Existing component rows remain a workload-local read projection.
  Startup backfills only resolved active observations, checks partial uniqueness,
  rejects conflicting duplicates/associations without deleting evidence, and
  installs PostgreSQL ownership/content guards. Ingest locks concurrent first
  inserts; matching validates event ownership; CVE/malware carry cluster identity.
  Combined finding writes are retired. The mandatory PostgreSQL gate exercises a
  populated pre-content schema, reruns, concurrent ingest, and rejected cross-owner
  writes. Live two-cluster deployment validation remains work package F.
- #48 merged (2026-09-22, merge commit `6fda0c6`): Agent writes/uniqueness use
  `{cluster_id, agent_id}`; unowned legacy records remain quarantined. Unscoped
  gRPC writes/streams are denied and its Ping reports identity_required. Operator
  tooling issues separate client certificates with digest-only registry bindings;
  versioned node-local mounts support overlap rotation/revocation. Agent TLS reloads
  credentials on new handshakes, and streams recheck revocation after blocked reads.
  Populated PostgreSQL/concurrency and credential lifecycle regressions passed.
  All 11 merge-commit checks passed; the separate PR AI scanner failed before
  analysis because its selected model was unsupported. That scan remains unverified.
  Live multi-cluster rollout remains package F; source tests do not close that gate.
- D1 / #49 merged (merge commit `f2128b7`): fail-closed auto-resolution
  eligibility, exact cluster/UID lookups, detector dependency checks and concurrent
  update guards. All 11 merge-commit checks passed. Missing resources, runtime
  silence and incomplete evidence preserve findings.
- D2 inventory / #50 merged (merge commit `5024e15`): authenticated collection
  ID, bounded collection interval, exact per-kind List start bounds, namespace scope
  and per-kind counts; complete-empty and failed collections are distinct. Agent
  liveness plus inventory projection/receipt commit atomically, so failed inventory
  persistence cannot advance Agent health. Core rejects altered replays/older
  attempts and propagates persistence errors. Role/ClusterRole eligibility requires
  the kind List start bound to post-date the finding plus a digest matching the
  stored snapshot; batch-end time and row UpdatedAt are never freshness surrogates.
  Sequential multi-kind List calls are explicitly not an atomic Kubernetes snapshot,
  so cross-resource resolution remains blocked.
  PostgreSQL replay/concurrency, real rollback/recovery, Agent rollback, namespace
  scope/pruning, Agent failure and HTTP identity regressions are permanent gates.
  Receipt completeness does not prove deletion or runtime sensor coverage. The
  Agent DaemonSet is a normal multi-writer topology; latest-per-cluster arbitration
  is fail-closed and now has PostgreSQL regression coverage for newer failure,
  older-writer rejection and later recovery. Package F must exercise the real
  DaemonSet plus different WATCH_NAMESPACE scopes on one cluster and either validate
  the supported topology or promote the receipt key to include scope before
  multi-scope aggregation is claimed.
- D2 runtime / #52 is now the active draft. #51 was retired after #50 merged
  because its stacked branch carried stale inventory history. #52 was rebuilt from
  `5024e15` and contains runtime-only changes. Current scope: authenticated
  producer coverage windows for file/Falco/eBPF, explicit drop/invalid/error
  accounting, immutable coverage retry, continuity tracking and complete-empty
  runtime intervals. A transient delivery failure breaks continuity even if a later
  retry succeeds. Runtime/Pod/cross-resource auto-resolution remains blocked until
  producer semantics, persistence and live topology gates are complete.
  Persisted/API/UI explanations and availability remain coordinated with E. D is
  not complete.
- E (originally #50; PR number may shift): API/UI availability and scoped observability, including Agent status.
- F (originally #51; PR number may shift): permanent PostgreSQL/two-cluster integration gate and populated
  migration evidence. Completing package F is the point at which A–F behavior can be
  claimed as validated end to end.
- G–I remain pending after the A–F gate: scoped AGE, explicit mutation/revocation
  workflows, then performance/load validation and the first-investigation demo.

## Repository governance prerequisite

Security-sensitive paths are covered by CODEOWNERS, but repository rules must
require CODEOWNER review and required CI checks on `main`. Direct/force pushes or
merges that bypass those checks defeat the regression-prevention contract and must
remain disabled by owner-side branch/ruleset configuration.


## Merge-readiness discipline

For #50 and subsequent security packages, runtime code is reviewed as one complete
state machine rather than a sequence of isolated findings. Any runtime-code commit
resets readiness and requires re-review of identity, scope, failure/replay,
concurrency, rollback, alternate writers, migrations and deployment topology.
Merge only the exact head for which Core, Agent, API, PostgreSQL and permanent
security regression gates passed. #51 was retired rather than reused. Active runtime work is #52 on a fresh branch
from merge commit `5024e15`; it must remain runtime-only and must not reintroduce
an older inventory contract.


### Final #50 merge blockers closed

Before merge, #50 must retain permanent regression coverage for these four
boundaries:

- namespaced upserts use `cluster_id + namespace + uid` and reject persisted
  cross-namespace UID collisions;
- `complete-empty` means collection-complete only; deletion remains
  non-authoritative and retained rows cannot auto-resolve;
- scoped authenticated HTTP sync without a collection envelope is rejected and
  cannot enter the unverified compatibility path;
- duplicate UID across namespaces is rejected both inside one payload and against
  already persisted inventory.

Any future change weakening one of these tests resets merge/release readiness.


### D2 runtime / #52 merge gates

#52 remains draft until the final runtime-evidence state machine is reviewed and
the exact head passes Core, Agent, API, permanent security regressions, Secret scan
and the PostgreSQL runtime-coverage gate.

Required invariants include:

- scoped producer identity and fixed producer/source binding;
- positive, non-overlapping observation windows;
- immutable at-least-once coverage retry;
- explicit drop/invalid/error accounting and complete-empty semantics;
- file/Falco partial-write and rotation handling;
- eBPF fail-closed while the built-in sensor remains no-op;
- bounded interval coverage rather than freshness-only absence reasoning;
- concurrent first-report arbitration and SQL rollback/recovery on PostgreSQL;
- generic runtime-file cursor never advances past a partial record prefix and a
  restart regression proves the record cannot disappear merely because lineBuf was lost;
- full populated-legacy schema upgrade, fail-closed unowned-row handling, and
  exact backfill semantics for pre-history latest evidence;
- immutable `runtime_coverage_receipts` history separate from the mutable
  latest-state `runtime_coverages` projection, with replay/rollback history gates.

Runtime/Pod/cross-resource auto-resolution remains disabled in #52. Enabling a
consumer is a separate change and must verify both the required bounded interval
and that the required producer/capability is currently enabled. Package F still
owns live restart, DaemonSet and two-cluster acceptance.


### Runtime lifecycle blocker review

#52 now persists producer lifecycle separately from coverage under
`{cluster_id, agent_id, producer_id}`. Agent execution `session_id`, enablement,
operational state, lease heartbeat, last coverage end and evidence-gap markers are
persisted. Restart creates a new session and breaks old coverage eligibility;
disable/stop and lease expiry fail closed.

The review also found that file/Falco reader liveness cannot prove the upstream
writer/sensor is alive. To avoid a false complete-empty claim, manifest v1 now
rejects every `Authoritative=true` declaration. Current file, Falco and built-in
eBPF coverage is operational telemetry only; no current producer may satisfy
absence-based auto-resolution. A later protocol must add an independent source
health proof before authority can be enabled.
