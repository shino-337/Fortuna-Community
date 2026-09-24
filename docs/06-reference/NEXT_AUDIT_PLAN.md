# Post-merge audit implementation plan

Status verified after PR #52 merged on 2026-09-23 (merge commit `e8efc99`).
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
- D2 runtime / #52 merged on 2026-09-23 (merge commit `e8efc99`).
  Runtime producer lifecycle/coverage for file/Falco/eBPF is persisted with explicit
  complete/failed semantics, immutable receipt history, independent 30s clean
  coverage cadence, partial-record restart safety and atomic runtime-event replay
  idempotency. Physical source-record replay is scoped by authenticated Agent
  identity and exact/concurrent replays cannot duplicate REP/risk/correlation
  effects. Current producers remain non-authoritative for absence reasoning;
  runtime/Pod/cross-resource auto-resolution therefore remains blocked until an
  independent source-health proof and package F live-topology validation exist.
  Retention/partition/archive/storage metrics remain production-operations
  follow-up, not #52 correctness blockers. D is intentionally not yet complete.
- E1 / PR #53 active draft: backend availability contract for stats, node,
  capability and Agent observability APIs. Missing schema/query failures must not collapse into
  legitimate zero/empty results; Agent version/cluster identity must come from the
  persisted Agent record; data availability must remain distinct from Agent
  heartbeat/liveness.
- E2 follows E1: dashboard/detail/list retry and unavailable states consume the
  backend contract consistently without replacing errors with zero KPIs.
- D3 follows E2 and precedes final F acceptance: define a runtime source-health
  protocol that is independent of file existence/reader heartbeat, bind health to
  the exact authenticated producer/session, allow authority only for producers
  that can prove upstream sensor health, and add evaluator regressions showing
  that stale/lost/disabled source health blocks absence reasoning. D3 must not
  infer authority from configuration flags or clean-empty event windows.
- F (PR number may shift): permanent PostgreSQL/two-cluster integration gate and populated
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
security regression gates passed. #51 was retired rather than reused. #52 is merged. Active work starts E1 from merge commit `e8efc99`; E1 must remain
focused on API availability semantics and must not reopen runtime evidence or
inventory ownership contracts.


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


### D2 runtime / #52 merged gate record

#52 passed its exact-head Core, Agent, API, permanent security regressions, Secret
scan and PostgreSQL runtime-coverage/idempotency gates before merge. The following
invariants remain permanent regression requirements.

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
- Falco cursor advances only through newline-terminated records; startup tails only
  past the last complete historical record, preserving a trailing partial prefix
  across Agent restart with a dedicated regression;
- runtime/Falco/eBPF timing inputs clamp non-positive values before ticker creation;
  generic runtime performs an immediate startup read rather than waiting one poll;
- Pod UID resolution uses one bounded context budget per Falco poll so sequential
  Kubernetes lookups cannot consume an unbounded multiple of the 5s poll interval;
- full populated-legacy schema upgrade, fail-closed unowned-row handling, semantic
  validation of evidence-shaped legacy rows, and exact backfill semantics for
  pre-history latest evidence;
- failed coverage gaps begin at the last accepted coverage end when one exists,
  preserving silent uncertainty before the failed window;
- immutable `runtime_coverage_receipts` history separate from the mutable
  latest-state `runtime_coverages` projection, with replay/rollback history gates;
- coverage receipt cadence is configured independently from event poll cadence:
  event collection may remain at 5s while immutable clean coverage defaults to 30s;
  failed coverage windows bypass cadence and are persisted immediately;
- runtime event replay is idempotent before downstream REP/risk/correlation
  effects. File/Falco events carry a physical `source_record_id` derived from
  file identity + byte offset + record bytes/ordinal, independent of the older
  second-granularity `event_id`. Core scopes the uniqueness claim by
  `{cluster_id, authenticated agent_id, source_record_id}` and performs that
  claim in the same SQL transaction as runtime_events, facts, signals, incidents,
  risk score and capability effects. Scoped ingest obtains `agent_id` only from
  the authenticated principal; explicit legacy shared-token mode uses a pod-local
  compatibility replay namespace and is not treated as trusted Agent ownership.
  Exact replay is a no-op; changed replay is a
  conflict; concurrent duplicate submissions create one committed effect; distinct
  same-second physical records remain distinct. Generic reader restart replay,
  exact replay, same-second identity and PostgreSQL concurrency are permanent gates.

### D3 runtime authority/source-health gate

D2/#52 intentionally closed correctness without enabling absence-based runtime
auto-resolution. D can only be marked complete after D3 proves an independent
upstream health signal. The D3 acceptance contract is:

- source health is produced independently from runtime event emptiness and file
  reader activity;
- health identity is scoped by authenticated `{cluster_id, agent_id, producer_id,
  session_id}` and cannot be self-rebound by payload aliases;
- authority expires on producer disable/stop, session restart, health lease expiry,
  source restart or an explicit source-health failure;
- a clean coverage interval is eligible for absence reasoning only while the exact
  producer has authoritative source health covering the required interval;
- old-session receipts, file existence, config enablement and Agent heartbeat alone
  never satisfy authority;
- PostgreSQL replay/concurrency and Agent restart regressions are permanent gates;
- package F validates the protocol on the real DaemonSet/two-cluster topology
  before runtime auto-resolution is enabled in production.

### E1 API availability merge gates

E1 starts after #52 and is the current implementation package. Merge only when:

- required DB/schema/query failures return an explicit unavailable/error response
  rather than a successful zero/empty projection; transient query/storage failures
  are retryable while missing migration/schema prerequisites are non-retryable;
- cluster node list/detail/overview/inventory endpoints propagate query failures;
- capability detail/list/summary use the same unavailable semantics when capability
  persistence is absent or unreadable;
- Agent status uses each Agent's persisted `cluster_id`, `version`,
  `last_seen_at` and node identity; missing heartbeat is not reported healthy;
- cluster security summaries join findings/capabilities with Pods on both
  `cluster_id` and Pod UID so duplicate UIDs across clusters cannot contaminate
  aggregate counts;
- Pod list risk-count enrichment and risk-based ordering remain
  cluster-qualified when the same Pod UID exists in more than one cluster;
- Agent data availability is represented separately from heartbeat-derived
  healthy/slow/disconnected state;
- system metrics and dashboard stats count Pods by `{cluster_id, uid}`, keep
  Pod Insight aggregates resource-type/cluster qualified, and do not report
  healthy/zero values when their backing queries fail;
- cluster totals and cluster-list membership come from the authorized fresh
  `clusters` inventory itself, not from whether Pod rows currently exist; an
  active empty cluster remains visible/countable while stale and legacy synthetic
  cluster rows remain excluded by the cluster-inventory contract;
- dashboard data-integrity cross-checks, catalog health and runtime health do not
  convert query/schema failures into zero counts, no-events, degraded or healthy
  states; required backing-query failure returns retryable 503;
- the primary Dashboard, Clusters, Resources and Monitoring consumers preserve the
  backend availability contract: contract-critical API methods throw instead of
  normalizing failures to empty/zero/null, last-known-good data survives retryable
  refresh failures, non-retryable schema failures show migration/operator guidance,
  and successful `200` empty responses retain the normal empty state;
- dashboard Playwright regressions cover retryable 503 preservation + Retry,
  non-retryable schema guidance, and genuine 200 empty semantics;
- named regressions are added to the permanent security contract;
- exact-head Core/API/dashboard validation and Secret scan pass before merge.

Current #53 execution order after the merged #52 baseline:
1. close backend availability semantics for Agent, cluster/node, capability, system
   metrics, pipeline health and dashboard data-integrity surfaces;
2. re-scan aggregate/detail handlers for ignored DB errors and add named regressions
   for every remaining zero-on-error path in E1 scope;
3. freeze the backend response contract and run exact-head CI/security gates;
4. merge #53 manually, then start E2 dashboard retry/unavailable-state consumption;
5. after E2, implement D3 source-health/authority; only then execute final package F
   live two-cluster/DaemonSet acceptance.

### #52 production operations follow-up (not a correctness merge blocker)

Runtime auto-resolution remains disabled, and lifecycle/session gaps fail closed,
so the following scale/operations work does not block the runtime-evidence logic
merge. It must, however, be completed before claiming large-scale production
readiness:

- define an explicit retention policy; use 7 days as the initial production target
  unless archive/export requirements justify a different value;
- partition `runtime_coverage_receipts` by receipt time and implement automatic
  cleanup/archive so retention does not depend on unbounded row-by-row deletion;
- expose storage observability for receipt ingest rate, allocated bytes and oldest
  retained receipt age (`receipts_per_minute`, `receipt_bytes`,
  `oldest_receipt_age`);
- capacity planning uses 2 KiB/receipt plus 50% headroom (3 KiB effective) until
  measured PostgreSQL table/index/TOAST overhead provides an environment-specific
  value;
- composite string identities on the latest projection and immutable history remain
  acceptable for this PR but require index/storage sizing at larger scale;
- fewer than 10 Agents may use 5s coverage cadence with short retention; around
  100 Agents should use at least 30s or bound retention to 7 days; 500+ Agents
  require partition + archive and must not append receipts every 5s by default;
- Falco startup scans backwards in 64 KiB blocks to preserve a trailing partial
  record. Normal JSONL finds a newline near EOF; a malformed newline-free file can
  force an O(file-size) startup scan and should be monitored as an operational
  edge case.

The Agent coverage pending/backlog queue is intentionally memory-only in #52.
Restart can lose unacknowledged coverage history, but a new lifecycle session
invalidates old evidence and `gap_since` reaches back to the prior accepted
coverage boundary. This is an accepted fail-closed trade-off, not a regression.

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
