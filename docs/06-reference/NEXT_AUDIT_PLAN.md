# Post-merge audit implementation plan

Status verified after PR #53 merged on 2026-09-24 (merge commit `a6e49ff`).
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
- E1 / PR #53 merged on 2026-09-24 (merge commit `a6e49ff`): backend
  availability plus primary Dashboard/Clusters/Resources/Monitoring compatibility
  work landed. A follow-up audit of #54 found residual E1 paths: resource/capability
  dashboard adapters still converted failures to empty data, the resource API
  itself ignored inventory query errors, a selected dashboard
  cluster bypassed the active-inventory filter, and cluster/worker observability
  did not consistently distinguish schema/query failure from empty activity;
  catalog checks could also misclassify a disconnected database as a missing
  migration. The follow-up checks connectivity before marking a schema absent.
  The #54 branch carries focused fixes and named regressions for those paths;
  they are not merged or exact-head validated until the #54 gate passes. Required
  query/schema failures must remain distinct from
  zero/empty data, primary clients preserve last-known-good state, retryable versus
  operator-action `503` is explicit, and genuine successful empty responses keep
  their normal empty semantics.
- E2 / PR #54 remains active from merge commit `a6e49ff`; closure fixes were
  reviewed on 2026-09-27. The exact merge-candidate SHA is recorded with the local
  CI evidence rather than embedded here so a documentation-only commit cannot make
  the recorded head stale. Cluster, Node, Capability and Pod detail/list availability behavior
  is implemented; the closure review additionally found and fixed ServiceAccount
  canonical identity loss and AttackPaths stale/malformed-response handling.
  ServiceAccount detail and permissions now preserve `{cluster_id, uid}` from
  navigation through Core resolution, duplicate UID reads fail closed when cluster
  ownership is ambiguous, and dashboard callers that already know the cluster
  retain it. AttackPaths now rejects malformed successful graph/bundle payloads
  and ignores stale page/pod responses after cluster/entity changes. E2 is not
  merge-ready until the residual E1 follow-ups and owner-recorded exact-head local CI evidence pass the gate
  below.
- A single-node `fortuna` deployment attempt on 2026-09-27 built the Core,
  Agent and Dashboard images from the dirty #54 worktree, but did not complete
  rollout. On the existing PostgreSQL 15 database, Core's cluster-resource
  ownership backfill failed on the `risk_scores` unique key: read-only
  projection found 10 Pod risk-score keys with one unowned legacy row and one
  already cluster-owned row targeting the same `{resource_type, resource_uid,
  cluster_id}`. This is a data reconciliation decision, not permission to delete
  either row. Core was returned to its prior GHCR image; Agent and Dashboard
  were not updated. Migration 145 (idempotent policy baseline seed) was recorded
  before the fail-closed startup check. The build cache also caused temporary
  node DiskPressure; it was pruned without deleting images or PVC data. A
  populated migration rehearsal against the existing data shape and an
  explicitly approved risk-score reconciliation policy are required before
  retrying this live rollout. This attempt does not close the package F gate.
- On 2026-09-27 the operator instead authorized a complete reset of the
  dedicated `fortuna` database. A restricted-access `pg_dump -Fc` was checked
  before resetting its `public` schema. Fresh migrations completed, and local
  Core, Agent, and Dashboard images were deployed on the single-node cluster.
  HTTP Agent ingest uses a per-node scoped token; gRPC Agent ingest uses a
  per-node certificate and fingerprint registry signed by a new dedicated
  Agent-client CA. The previous Fortuna server/webhook CA and certificates were
  not rotated. Agent registration, inventory sync, SBOM ingest, Core health,
  and the full Kubernetes deployment check were exercised. Fresh-db runtime
  traffic exposed a missing JSON payload on synthetic network events and an
  overlong incident key; both were patched with regressions and rebuilt into
  the local Core image. After an Agent restart cleared its in-memory batch of
  stale Pod events (the Falco source log was retained), fresh runtime v2
  batches, network observations, and incident creation were observed without
  those database errors. This clean reset deliberately discards the legacy
  collision data, so it does **not** prove a populated migration or close the
  two-cluster package F gate. OSV/CVE catalogs remain empty until a new source
  load; the Aikido malware feed repopulates independently.
- Follow-up runtime check on 2026-09-27 found two live ingestion/evaluation gaps
  despite all eight Pods being Ready. Scoped Pod-event batches repeatedly returned
  `pod_ownership_mismatch` 403 and no new `k8s_events` rows arrived after 07:18
  UTC; historical RBAC evaluation reported 153 duplicate-key errors per sync.
  The Core rule engine read but did not persist `cluster_id` on generated
  insights, and the generic insight path did not restore a soft-deleted row
  covered by the unconditional unique index. Core/Agent fixes now carry
  cluster identity, restore the exact soft-deleted key, and quarantine only
  individually ownership-rejected Pod events so valid events can proceed;
  resync of the same Kubernetes Event UID cannot immediately requeue it.
  Follow-up Core regressions also use the actual cluster-qualified PCE conflict
  keys and normalize stored RBAC rules/role references before CEL evaluation,
  without converting malformed evidence into a successful clean result.
  Local full Core/Agent tests, vet, the named regression gate and a PostgreSQL
  16 restore regression passed. Core `gapfix-20260927-0941` and Agent
  `gapfix-20260927-0929` were deployed to the single-node cluster. The first
  post-rollout historical evaluation reported `Insights=166, Errors=0` (down
  from 153, then 13); PCE populated 24 Pod risk profiles and 98 capabilities;
  `k8s_events` advanced from 1003 to 1070. The full deployment check passed
  with zero errors and warnings. This is single-cluster runtime evidence, not
  the package F two-cluster acceptance gate. At that snapshot no active insight
  had an empty `cluster_id`; 157 unowned soft-deleted rows remain as historical
  evidence. Do not delete or bulk-assign those rows by node/name inference.
  A transient Falco 403 during Core replacement cleared after the new Pod
  appeared in the accepted inventory. At the next accepted inventory sync
  (09:51 UTC), historical evaluation again reported `Insights=166, Errors=0`,
  Falco batches continued `send_ok`, runtime events reached 1019, and no
  active insight had an empty cluster ID. OSV/CVE catalogs remain empty.
- A later runtime check found that both seeded Pod policy templates failed CEL
  compilation: they referenced undeclared `object`, while the policy evaluator
  receives the inner Pod spec as `resource` and treats a true result as
  compliant. The corrected 1.0.1 templates cover app, init, and ephemeral
  containers and host namespace flags; a forward migration repoints legacy
  instances and retires only exact stock broken 1.0.0 templates. Source and
  PostgreSQL regressions confirm safe Pod acceptance and unsafe Pod violations,
  including a populated legacy-template repair. A fresh live rollout on
  2026-09-27 confirmed two active 1.0.1 templates and two enabled Pod-scoped
  instances; the Core evaluator logged successful CEL compilation of both.
  The live webhook responds on HTTPS :8443, but no Kubernetes
  ValidatingWebhookConfiguration is installed; a cluster admission denial was
  not claimed. This finding is not closed by a database reset alone.
- The same fresh single-node rollout used local image tag
  `auditfix-20260927-ee927de95` for Core, Agent and Dashboard. A restricted
  `pg_dump -Fc` was checked before the dedicated `fortuna` schema reset;
  its restricted host backup is `/var/backups/fortuna/fortuna-pre-reset-20260927-1115.dump`
  (SHA-256 `f9edbc448f9721d2750de6522085534556d4b85fb570025ba697601a0902e89a`).
  All 146 migrations applied. Core/Agent/Dashboard rollouts and the full
  deployment check passed (0 errors, 0 warnings). The first post-deploy
  process-snapshot check ran before the Agent's two-minute retry, returned
  nonzero, and was followed by 20 Pod snapshots; the pipeline now waits up
  to 150 seconds before failing that check. Existing scoped HTTP/mTLS
  registries and client CA were reapplied after the Core Deployment replacement;
  subsequent robust deploys detect the provisioned secret set and reapply
  these overlays before rollout. A no-clean/no-rebuild/no-reset replay of the
  deploy and verification phases finished with all 10 checks passing, including
  scoped Agent connectivity, runtime events and process snapshots. The
  server/webhook CA was not rotated.
  At the verification snapshot, 23 Pods, 17 runtime events, 61 Kubernetes
  events and 23 Pod risk profiles were persisted; Falco runtime batches were
  accepted. CVE/OSV catalog loading was intentionally skipped because the
  local source is large and node free space was limited; vulnerability matching
  remains unavailable until a separately verified catalog load. This reset
  still does not satisfy package F's populated migration or two-cluster gate.
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
security regression gates passed. #51 was retired rather than reused. #52 and #53
are merged. Active work is E2 from merge commit `a6e49ff`. GitHub-hosted Actions
for #54 are currently failing before runner execution because of the repository
account/runner entitlement state; zero-step jobs are not code evidence. For #54
only, the repository owner may satisfy the exact-head functional gate with a
recorded local run of the same commands/jobs. Automatic Secret Scan remains
manual-only and is not part of this functional gate. E2 should remain focused on
availability/identity correctness of the affected detail/list consumers and must
not enable runtime absence authority or claim package-F live multi-cluster
acceptance.


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

### E1 API availability merge gates — merged in #53

E1 is closed on merge commit `a6e49ff`. Its permanent contract remains:

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

### E2 detail/list availability merge gates

E2 starts from merge commit `a6e49ff`. Merge only when:

- residual E1 paths preserve unavailable versus empty for resource/capability
  adapters, the shared cluster inventory, selected-cluster dashboard KPIs and
  required worker persistence; permanent regressions cover failure and real empty
  responses;
- Cluster, Node, Capability and Pod primary-detail reads distinguish a genuine
  not-found response from transient/schema availability failure;
- secondary/enrichment failure (cluster stats/overview, tab data, linked rules,
  SBOM/risk/runtime evidence) does not erase a successfully loaded primary entity;
- last-known-good state is keyed to the exact route/entity identity; navigation from
  entity A to entity B cannot render or retain A's primary/enrichment evidence, and
  delayed responses from A cannot overwrite B;
- malformed successful payloads (for example missing required arrays, counters or
  mismatched IDs) are protocol-unavailable failures rather than valid empty data;
- retryable refresh failure preserves last-known-good detail/list state and exposes
  Retry, while non-retryable schema/deployment failure exposes operator guidance;
- shared cluster inventory uses stale-while-revalidate semantics: failed
  revalidation cannot clear the cached cluster selector or cause ownership/scope
  UI to report a false empty platform;
- Settings user-scope cluster inventory preserves its last successful allow-list
  source and never turns a failed refresh into "No clusters available";
- Pod Detail contract-critical evidence fetches use strict adapters so transport,
  query and schema failures cannot overwrite prior metrics/process/network/events,
  SBOM, risk report, runtime signals/facts/incidents or capability state with
  empty/null values;
- successful `404`/not-found and successful `200` empty responses retain their
  genuine not-found/empty semantics;
- permanent Playwright regressions cover primary 503 versus 404, enrichment/tab
  503, capability rule failure, Node/Pod last-known-good refresh preservation and
  successful empty behavior where applicable;
- ServiceAccount detail and permissions preserve canonical `{cluster_id, uid}`
  identity. A known cluster must be propagated by Pod/Resources/findings links;
  unqualified duplicate UIDs must return an ambiguity failure rather than selecting
  an arbitrary row; permission responses return and validate UID + cluster
  ownership;
- AttackPaths cluster/entity changes cannot be overwritten by an older in-flight
  bundle/fallback/pod request, and malformed successful graph/bundle payloads are
  protocol-unavailable rather than empty graph/path state;
- the permanent route/static guard ratchets cluster-aware Pod and ServiceAccount
  navigation plus AttackPaths request-generation/strict-response markers;
- exact-head Dashboard typecheck/build/Playwright, Core permanent regressions,
  PostgreSQL gate, API/Agent tests + vet, script/shell/hygiene pass. For PR #54,
  owner-recorded local execution of these same gates is accepted while
  GitHub-hosted jobs fail before execution. Secret scanning is manual-only while
  the repository plan/license does not support it as a reliable PR/push gate;
  scanner availability must not block the functional CI contract.

#### #54 local exact-head verification

Before manual merge, record the tested commit SHA and successful local results for:

```bash
# Core
(cd core && go test ./... && go vet ./...)
python3 scripts/verify/test-security-regression-gate.py
python3 scripts/verify/check-security-regressions.py

# API + Agent
(cd api && go test ./... && go vet ./...)
(cd agent && go test ./... && go vet ./...)

# Dashboard
(cd dashboard && npm ci && npm run typecheck && npm run build)
(cd dashboard && npx playwright install chromium)
(cd dashboard && npx playwright test --config playwright.runtime.config.ts)

# Repository/script contracts
python3 scripts/verify/check-service-selectors.py
python3 scripts/e2e/test-webhook-bootstrap.py
python3 scripts/verify/test-agent-credential-tool.py
python3 scripts/verify/test-agent-certificate-tool.py
python3 scripts/verify/test-scoped-agent-credential-overlay.py
python3 scripts/verify/check-cluster-resource-models.py
python3 scripts/verify/check-cluster-qualified-pod-routes.py
python3 scripts/verify/test-cluster-qualified-pod-routes.py
python3 scripts/verify/check-retired-routes.py
python3 scripts/verify/test-retired-routes.py
find scripts -type f -name '*.sh' -print0 | xargs -0 -n1 bash -n
```

The PostgreSQL job must also be run against a local PostgreSQL 16 instance with
`FORTUNA_TEST_POSTGRES_URL` set, using the same test selections in
`.github/workflows/ci.yml`. A local pass applies only to the exact recorded head;
any subsequent runtime/security-relevant commit resets the gate.

Current execution order after #53:
1. complete E2 against the gates above and merge PR #54 manually;
2. D3: implement independent runtime source-health/authority without inferring
   authority from clean-empty windows or reader heartbeat;
3. F: execute the live PostgreSQL/two-cluster/DaemonSet acceptance gate.

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
