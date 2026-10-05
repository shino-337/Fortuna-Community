# Security invariants and regression-prevention contract

Fortuna treats security fixes as invariants, not one-time patches. Known security
failure classes are made machine-detectable so a later change cannot silently
reintroduce them.

This contract reduces regression risk; it is not a claim that software can never
contain a new defect. New findings must be converted into a reproducible test or
static invariant before their fixing PR is considered complete.

## Invariant 1 — cluster-qualified resource identity

A Kubernetes resource UID is not a Fortuna-wide identity. Any persisted or
authorized workload reference is identified by the pair:

`{cluster_id, resource_uid}`

For Pods this is `{cluster_id, pod_uid}`. The same Pod UID in two clusters must be
able to coexist without read, write, cache, reconciliation, runtime, finding or
graph state crossing the cluster boundary.

Rules:

- Authorization must never infer ownership from `pod_uid LIMIT 1`.
- Storage keys, upserts, caches and reconciliation keys must retain cluster ID.
- A request-supplied cluster cannot override the authenticated/scoped cluster.
- Missing or ambiguous ownership fails closed. Migration/backfill must not guess.
- Existing non-empty ownership that conflicts with authoritative Pod identity must
  be rejected rather than silently rewritten.
- Generic resource references without a trustworthy resource type must not be
  guessed to be Pod references merely because their UID happens to match a Pod.
- Image digest identifies reusable image content, not workload ownership.

`core/pkg/resourceidentity` is the canonical in-process identity primitive.
`scripts/verify/check-cluster-resource-models.py` is the first static ratchet: a
persisted model carrying `PodUID` must also carry `ClusterID`, and persisted Pod
models plus explicitly classified resource-typed Pod models must remain represented
in the cluster-resource migration manifest.

## Invariant 2 — incomplete evidence is not clean evidence

Database, collector, runtime, SBOM, CVE or authorization-state failure must not be
converted into an empty, zero, healthy or clean security result. Required evidence
that is unavailable, stale, malformed or incomplete must remain explicit and must
not auto-resolve an active finding.

The same rule covers source health, observation time, completeness, freshness
and UI/API availability.

### API availability contract

Availability is a data property, not a zero value. For observability, inventory and
security-summary endpoints:

- a missing required schema or failed backing query must return an explicit
  unavailable/error response and must never be represented as a successful empty
  list, zero counter, healthy status or synthetic version;
- `404`/empty remains valid only when the backing query succeeded and the requested
  resource or collection is genuinely absent;
- transient backing-store/query unavailability uses a machine-readable code and
  `retryable=true` so UI clients can preserve the last known state and expose a
  retry path instead of resetting KPIs to zero;
- a missing required schema/migration prerequisite uses the same explicit
  unavailable contract with `retryable=false`; repeated client retry must not hide
  the need for deployment/operator repair;
- Agent liveness is derived from persisted `last_seen_at`; data-source availability
  is separate. A missing heartbeat is disconnected, not healthy;
- Agent cluster identity and version come from the persisted Agent record. UI/API
  layers must not substitute a global latest cluster or a hardcoded version;
- detail/list/summary views over the same backing dataset must agree on unavailable
  semantics;
- contract-critical dashboard clients must preserve the distinction between
  unavailable and empty: retryable `503` keeps last-known-good data and exposes a
  retry path, non-retryable schema/migration `503` exposes operator guidance, and
  only a successful `200` empty payload may render the normal empty/zero state;
- client API adapters must not catch these endpoint failures and normalize them to
  `[]`, `0`, or `null`. Malformed successful payloads on required fields are
  protocol failures, not empty evidence.

These rules are permanent named regressions under
`scripts/verify/check-security-regressions.py`.

## Invariant 3 — authentication is preserved through storage

Successful scoped HTTP or gRPC authentication is only the first boundary. The
trusted `{cluster_id, agent_id}` principal must remain authoritative while resolving
Pods and while persisting every derived object. A handler may not authenticate a
cluster-qualified Pod and later write to a UID-only storage key.

## Invariant 4 — batches are authorized before effects

For batch ingest, every target must be authenticated, ownership-resolved and
validated before the first deduplication, database, runtime-processing or other
handler effect. A mixed valid/foreign batch must not partially persist.

## Invariant 5 — ambiguous historical state is quarantined

Legacy data may lack cluster ownership. Backfill is permitted only where one
resource UID maps to exactly one observed cluster. If the same UID maps to more
than one cluster, ownership remains unresolved until an explicit repair process
can establish it. Namespace/name/digest similarity is not ownership proof.

## Invariant 6 — security regressions are named CI requirements

A security fix is incomplete until its regression is anchored in
`scripts/verify/check-security-regressions.py` or an equivalent mandatory CI
contract. Removal, rename, skip or failure of a required regression must fail CI.
Static architectural invariants belong under `scripts/verify/` and are run by CI.

CI runs the script in the Core job in addition to all package tests. It requires
explicit run and pass events for every listed test; missing or renamed tests,
package failures and skipped subtests fail the gate, and parser tests cover false
success and skipped-subtest cases. The contract currently covers runtime input
failure, reconciliation retention, shared RBAC semantics, inventory/mutation scope,
scoped HTTP/gRPC identity, cluster-qualified storage, receipt lifecycle/replay, API
availability, graph/cache boundaries and bounded durable ingest delivery. Other
tests remain covered by the normal full suite; this contract is not complete
evidence of all future HTTP/gRPC isolation behavior. The
[finding-to-test map](#appendix-finding-to-test-map) lists the anchors per fix.

## Invariant 7 — migration history is append-only

The migration runner records migration versions by slice position.
Existing entries in the migration list therefore must not be reordered or inserted
in front of already-shipped entries. New versioned migrations are append-only.
Security-critical schema invariants that cannot safely rely on the runner are
checked at startup and fail it explicitly.

## Invariant 8 — real multi-cluster behavior is a release gate

Unit and SQLite tests are necessary but do not establish end-to-end isolation.
The `cluster-identity-postgres` CI job runs the PostgreSQL contracts and
`scripts/verify/run-two-cluster-integration.py`, which builds the real Agent,
creates two disposable kind clusters (three nodes, four DaemonSets, six scoped
Agents across two namespace scopes) and requires `TestTwoClusterDaemonSetLive`
to run and pass with a complete evidence receipt. Together they cover at least:

- identical Pod UID in different clusters;
- identical node/Agent names in different clusters;
- identical image digest used by workloads in different clusters;
- concurrent ingestion and retry/replay;
- certificate rotation/revocation on established streams;
- missing/stale evidence and recovery;
- populated-database schema migration and upgrade behavior;
- authorization for restricted users and aggregate endpoints;
- deletion/replacement UID race behavior.

Changes touching cluster identity, ingest, storage, authorization, runtime
evidence, findings or migration code must keep passing it.
`scripts/verify/rehearse-populated-migration.py` rehearses an upgrade against a
restored copy of a populated database.

## Invariant 9 — Agent privileges are an allowlist

A compromised Agent pod must not be able to run code in other pods, read
Secrets, write to the Kubernetes API or reach node root from the container that
parses untrusted data. The Agent's RBAC, pod spec and code are therefore pinned
to the allowlist in [Agent privileges](SECURITY.md#agent-privileges):
read-only verbs on listed resources; no exec, attach, port-forward or proxy
subresource; no host namespaces, added capabilities, privilege escalation or
writable root filesystem; and the containerd socket only in the credential-less
`image-export` container. `scripts/verify/test-agent-privileges.py` enforces
this in CI and applies each known escalation to prove it is rejected. Adding a
privilege requires changing that test and the security reference together.

## Repository governance prerequisite

CODEOWNERS covers security-sensitive paths, but CODEOWNERS alone does not enforce
review. The repository owner must protect `main` (ruleset or branch protection) and
require the relevant CI checks plus CODEOWNER review. Direct or force pushes that
bypass those gates defeat the regression-prevention model.

- Require functional CI (Core, Agent, API, PostgreSQL, Dashboard, scripts, Helm
  and hygiene), PR review and current-base validation. The secret-scan workflow
  runs on manual dispatch and is a separate check.
- Review changes to CI, the required test list and permission/scope helpers as
  changes to security controls.
- Mocked UI tests and SQLite unit tests do not prove deployment behavior; storage
  or scope changes need PostgreSQL/Kubernetes integration coverage.

## Completion rule for a finding

For every security finding:

1. reproduce the failure;
2. encode the expected invariant;
3. implement the fix;
4. add a named regression/static check;
5. exercise negative and failure paths, not only success;
6. include PostgreSQL/two-cluster evidence where storage or scope is involved;
7. document any remaining migration/compatibility limitation;
8. keep the gate after merge.

A finding is not considered permanently closed merely because its current code
path was patched.

## Evidence eligibility for automatic resolution

Database absence is not an authoritative deletion observation. Reconciliation
must retain findings for missing/ambiguous owners, stale/invalid snapshots,
malformed evidence, unavailable detectors and unknown collection coverage.
Runtime silence and expiry of a lookback window never prove remediation.

Automatic resolution is permitted only for self-contained Role/ClusterRole CEL
checks reading fields from that exact static snapshot. It additionally requires an authenticated collection receipt: the resource-kind List start bound
must be at most ten minutes old, not future-dated, not older than the finding, and
its digest must match that exact cluster/kind/UID/name/namespace/rules projection. A missing, failed, unverified or
stale receipt blocks resolution. The rules array must be structurally valid. CEL dependencies are checked
from the parsed expression; aliases/bracket access cannot introduce unverified
runtime or cross-resource inputs. Unknown dependencies are denied.

A snapshot update timestamp is NOT a collector observation or completeness
receipt. Agent heartbeats, Cluster.LastSync, runtime event timestamps and projector
UpdatedAt must not be substituted for such receipts. Runtime, Pod and
cross-resource auto-resolution stays disabled until a verified
producer-to-evaluator coverage contract exists. This is intentionally conservative:
findings may remain active after remediation and need explicit human review.

The resource snapshot is locked and version-checked before committing resolution;
the finding update compares its original UpdatedAt and active status. A concurrent
sync/detection/manual update must preserve the newer state. The status change and
cluster-qualified resolution audit commit together or both roll back.

Diagnostics are emitted by InsightStatusUpdater with a resolution-evidence reason.
Inventory observation status/timestamps and complete-empty lists are persisted
and returned to the scoped Agent. Inventory receipts do not establish sensor
coverage.

## Inventory receipt integrity

Only scoped Agent identity can issue a verified receipt. A request authenticated
as a scoped Agent must include collection evidence; missing collection metadata is
a hard reject and must never downgrade into an unverified write. Agent health metadata
from the same HTTP sync commits with the inventory transaction; failed inventory
persistence must not advance Agent liveness. Scoped HTTP mode is exclusive when
the credential registry is configured and does not fall back to the legacy shared
token. Any explicit unverified compatibility write still invalidates prior verified
status because it may alter the projection without trustworthy evidence. Core startup
must establish the receipt schema before accepting traffic; old rows are never
backfilled as complete. Collection namespace, all eight list counts and all eight
per-kind Kubernetes List start bounds are checked before inventory effects. The eight List calls are
sequential: the receipt is a bounded interval, not an atomic Kubernetes snapshot.
Cross-resource consistency must not be inferred from it. Missing/null lists, pagination left by Kubernetes and
collection errors cannot be represented as successful empty observations.

Inventory writes and receipts commit in one transaction serialized per cluster.
Asynchronous capability/cleanup work starts after commit. Persistence errors roll
back the projection and record a failed attempt when storage is available; request
cancellation does not prevent this bounded failure recording. Failures to persist
the failure are returned, never hidden. Receipt and resource locks have consistent
ordering during resolution. Identical accepted replays are idempotent; changed
replays and older observations are rejected. Failed attempts are terminal: retry
by collecting a new attempt ID, as Syncer does on its next cycle.

A complete receipt proves the collection completed and its included rows were
accepted. It does not claim authoritative projection reconciliation or deletion.
The HTTP contract must expose non-authoritative deletion semantics, and retained
rows absent from a complete-empty collection must not become eligible for
auto-resolution. Existing
empty/partial deletion safeguards remain. Namespace-limited collections must reject out-of-scope namespaced rows and must
not prune namespaced inventory elsewhere. Namespaced object upsert identity is
`cluster_id + namespace + uid`; a same UID already persisted in another namespace
is an integrity conflict and must fail closed rather than move/adopt that row. ClusterRole/ClusterRoleBinding lists still
cover cluster-scoped objects. The PostgreSQL CI gate exercises concurrent replay,
SQL-trigger failure, rollback and recovery, including timestamp precision.


### Multi-Agent arbitration

The production Agent is a DaemonSet: more than one scoped Agent in the same
cluster can submit full inventory. The current receipt is intentionally the latest
accepted attempt per cluster. Ordering is by collection start time and the receipt
row is serialized with a database lock. Therefore:

- a newer failed Agent attempt may make resolution evidence unavailable but must
  not mutate the previously accepted inventory projection;
- an older Agent attempt must not roll the receipt or projection backwards;
- a newer complete Agent attempt must recover the receipt deterministically;
- different namespace scopes may supersede one another and reduce eligibility,
  but a scope mismatch must preserve findings rather than resolve them.

This is a fail-closed availability trade-off, not multi-scope aggregation. The
two-cluster CI gate exercises it with two namespace scopes; a topology that needs
independent per-scope completeness would require a scope-qualified receipt key.

## Security-change review protocol

For security-sensitive changes, review the complete state machine before writing
the fix. Every PR touching identity, ingest, persistence, reconciliation,
runtime evidence or migrations must explicitly map these dimensions:

1. trust boundary and authoritative identity;
2. cluster / agent / namespace / resource ownership keys;
3. success, empty, partial, failed, replay and out-of-order states;
4. concurrent writers and transaction lock ordering;
5. rollback behavior and side effects that occur before/after commit;
6. legacy/admin/compatibility writers that can mutate the same projection;
7. evidence freshness, loss and negative evidence semantics;
8. migration from populated data and restart/rerun behavior;
9. supported deployment topology, including DaemonSet multi-writer behavior;
10. permanent named regression tests for every accepted invariant and every
    fail-closed boundary.

A code change resets merge readiness. After the final runtime-code commit, rerun
the full invariant review against the resulting diff, then require Core, Agent,
API, PostgreSQL and security-regression gates to pass on that exact head. Test/doc
commits may follow, but any further runtime-code change requires the review cycle
again.

## Runtime producer coverage integrity

Runtime event silence is never clean evidence. Verified coverage requires a scoped
Agent producer window. Complete windows have positive duration and zero
drop/invalid/error counts with `delivered == emitted`. Exact coverage retries are
immutable; changed replay, overlap, out-of-order windows and producer/source
rebinding fail closed.

Continuity extends only over adjacent complete windows. A gap or failed window
resets `continuous_since`. Historical queued windows may be persisted after an
outage, but freshness is evaluated independently. Any absence-based consumer must
require an explicit bounded interval whose required end is covered; receipt
freshness alone is insufficient.

File and Falco producers must treat truncate/replacement, partial records and
delivery failures as coverage-breaking conditions. For the generic runtime file,
the cursor may advance only past newline-terminated records; a partial trailing
prefix must remain unread in the durable source file and be re-read after poll or
Agent restart. In-memory line buffering must never be the sole copy of unconsumed
source bytes. Unresolved Falco Pod identity is a drop, not clean silence.

The built-in eBPF sensor currently uses no-op tracepoints and is non-authoritative.
It must not emit complete coverage, including in simulation mode. Removing this
fail-closed rule requires a real event collector plus permanent observation/drop/
shutdown regressions.

Runtime coverage remains producer-specific. Producer operational state is
persisted independently from evidence authority. A new Agent execution session,
disable, stop or expired lifecycle lease invalidates old-session coverage and
creates/extends an evidence gap.

Protocol v1 has no independent upstream source-health proof. Therefore no runtime
producer may self-assert `Authoritative=true`; Core rejects such manifests.
File/Falco reader activity or complete-empty windows may prove the Agent-side
collector loop is active, but must not establish `continuous_since` or satisfy
absence-based `CoversInterval`. Built-in eBPF remains non-authoritative as well.

Before any future auto-resolution consumes runtime coverage, a later protocol must
prove both current producer enablement and independent upstream source health. Old
receipts, config flags, file existence and reader heartbeats are not sufficient.

Runtime storage has two roles that must not be conflated:
`runtime_coverages` is the mutable latest projection used for arbitration, while
`runtime_coverage_receipts` is immutable accepted history. Every accepted
non-replay window must append one history row in the same transaction as latest
projection/lifecycle mutation. Replay must not duplicate history and SQL rollback
must roll back both.

Runtime schema migration must repair every required column/index from populated
legacy tables. Rows whose cluster/Agent/producer ownership cannot be established
must fail startup rather than be silently adopted. Partial legacy rows may be
preserved as latest-state data but must not be fabricated into immutable evidence;
a valid pre-history latest receipt may be backfilled exactly once.

PostgreSQL coverage arbitration is a permanent CI gate: concurrent first reports
must converge without unique-key 500s, storage failure must roll back latest state
and immutable history, and recovery/startup migration must preserve evidence.

### Bounded ingest and durable Falco delivery

Only an explicit machine-readable ownership rejection may trigger batch
isolation. A transient, authentication or other forbidden response stops all
remaining sibling requests in the same flush. Fresh delivery, isolation and
quarantine share a 12-request budget; 429/503 backoff honors `Retry-After`.
Deferred Pod-event quarantine stays out of the fresh-event queue, takes its next
turn before already-rejected records, and retains informer updates received
while HTTP delivery is in progress.

When `FALCO_DELIVERY_STATE_PATH` is configured, complete canonical payloads,
physical source-record identities and cursor must be persisted together before
delivery. The state is bound to the exact cluster, Agent, Core URL and source
path, exclusively locked, atomically replaced and synced. Missing persistence,
corruption, binding mismatch, another writer or exhausted capacity blocks
ingestion without discarding retained evidence. Restart/rotation preserves
pending/quarantined payload identity and retry deadlines. Accepted events may
replay after an acknowledgement/checkpoint failure; Core's authenticated
source-record deduplication prevents repeated effects against an unchanged DB.

Pending/quarantined evidence is failed coverage, never clean or authoritative.
These guarantees do not extend to Falco delivery without a state path, and do not
make the in-memory Pod-event queue restart-durable; its capacity and drop
behavior stays explicit. The ingest regressions in the named gate protect these
boundaries.

### Independently verified source health

Runtime authority requires an independent Ed25519 key bound to the exact
cluster/Agent/producer and both Agent and sensor executions. Reader lifecycle
cannot assert it. Signed windows must be fresh, sequential and loss-free;
replay cannot renew authority. Failure, expiry or restart breaks continuity.
Accepted signed receipts and latest state commit atomically. The eBPF stub is
ineligible and absence-based automatic resolution remains disabled.

### Scoped graph and explicit mutation effects

AGE constructors require a nonempty cluster; its physical graph is derived from
that cluster. Controlled traversals filter nodes and edges and validate every
returned intermediate object. Unscoped/raw Cypher entry points remain retired.
AGE connection search paths are transaction-local. Relational graph input errors
must return unavailable rather than a partial successful graph.

Mutation intent and audit must precede external effects. A reviewed canonical
plan, UID/resource-version preconditions, lease ownership and transactional
completion protect replacement objects and retries after persistence failure.
A changed preview blocks execution; deleting Inventory is not revocation.

### Populated migrations and bounded analytics

Metadata lookup is restricted to CURRENT_SCHEMA(). Colliding unowned legacy
risk snapshots remain unowned, with their complete original row recorded in
risk_score_ownership_quarantines. Migration must retain snapshot uniqueness used
by production upserts; it may not select a winner or delete collision evidence.

Trend queries aggregate authorized observations into UTC calendar buckets in the
database. Database failure remains an error. Cache entries must remain independent
of caller mutation, including nested properties; invalid encoding cannot cache a
successful empty graph. Live CI must prove the named integration test ran and
passed and read back its complete receipt; an empty or skipped selection fails.

## Appendix: finding-to-test map

| Area | Required regression examples |
| --- | --- |
| Inventory scope and mutations | TestServiceAccountInventoryScope, TestServiceAccountMutationsProtectIdentity, TestServiceAccountDeleteFailuresPreserveInventory, TestServiceAccountUpdateUsesSelectedClusterRow |
| RBAC semantics | TestServiceAccountRBACResolution, TestClusterAdminBindingForPod, TestPodRiskReportUsesResolvedRBACScope |
| Workload and capability scope | TestInventoryWorkloadCapabilityScope |
| Agent cluster association | TestAgentClusterIdentityIsolation |
| Graph and cache boundaries | TestLegacyGraphFailsBeforeGlobalQuery, TestNetworkServiceCacheSeparatesClustersAndCredentials |
| Evaluator and reconciliation failures | TestEvaluationReportsRuleFailure, TestConfiguredCatalogRejectsPartialAndEmptyLoad, TestReconciliationAuditRollbackAndCatalogFailure, TestReconciliationPreservesDisabledDetector |
| Runtime input errors | TestRuntimeInputFailureReachesEvaluators, TestRuntimeInputRejectsCorruptSnapshot, TestRuntimeInputRejectsMalformedBindings, TestReconciliationPreservesFindingOnRuntimeInputFailure |
| Aggregate and finding-action scope | TestAggregateCacheIsolation, TestRuntimeScopeAndFindingActions, TestBulkRequiresActionPermissionAndNonemptySelection |
| Agent credentials | TestCredentialIdentityIsolation, TestCredentialRotationRevocationAndExpiry, TestCredentialRegistryFailsClosed, TestCredentialRequiresVerifiedTLS |
| Durable Falco ownership isolation and replay | TestFalcoDurableMixedBatchRestartAndRecovery, TestFalcoDurableRotationReplaysPendingBeforeReadingReplacement, TestFalcoDurableBackoffSurvivesRestartAndMissingSource, TestFalcoDurablePersistenceFailureDoesNotSendOrAdvanceCursor, TestFalcoDurableStateFailsClosedOnCorruptionBindingAndConcurrentWriter, TestFalcoDurableIsolationBudgetAndCapacityKeepEvidence, TestFalcoDurableLargeBacklogDrainsWithinCapacity |
| Bounded Pod/Falco retry and backoff | TestDeliveryBudgetBoundsIsolationAndRetainsUnsent, TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden, TestPostErrorHonorsRetryAfterAndCancelledDelivery, TestEventsCollectorRateLimitStopsFlushAndPreservesQuarantine, TestEventsCollectorQuarantineRetryHasSharedRequestBudget |
| Quarantine fairness and informer updates | TestEventsCollectorQuarantineBudgetDoesNotStarveRecoveredEvents, TestEventsCollectorResyncDuringDeliveryPreservesQueueClassAndNewestVersion |
| Generated and restored insight ownership | TestRuleInsightCarriesClusterIdentity, TestGenericInsightRestoresSoftDeletedRowWithinCluster; PostgreSQL job: TestGenericInsightRestorePostgres |
| Manual acknowledgement of generic insights | TestGenericInsightRetainsAcknowledgedState/key-0 and /key-1; PostgreSQL: TestGenericInsightRestorePostgres/key-0 and /key-1 |
| Stock Pod CEL policies | Full Core suite: TestBaselinePodPoliciesCompileAndDetectUnsafeSpec, TestMigration151PreservesModifiedLegacyTemplate; PostgreSQL job: TestBaselinePodPolicySeedAndRepairPostgres |
| Signed source health | TestSourceHealthAuthorityReplayFailureAndRestart, TestSourceHealthRejectsUntrustedEvidence (see named contract for exact subtests); PostgreSQL: TestSourceHealthPostgresConcurrencyAndRollback |
| Cluster-scoped AGE graph | TestAGECanonicalScopeAndIdentifiers; PostgreSQL: TestAGEScopedTraversalPostgres |
| Reviewed durable mutations | TestMutationIdentityReplayAndDurability, TestMutationBlocksReplacementAndChangedBindings, TestDurableDeletionRetainsUIDPrecondition, TestMutationDigestSurvivesJSONBRepresentation |
| Populated-database ownership and schema | PostgreSQL: TestRiskScoreOwnershipQuarantinePostgres, TestMigrationMetadataUsesCurrentSchemaPostgres; isolated populated-backup rehearsal |
| Unavailable graph snapshot | TestAttackPathBuildFailsClosedOnSnapshotError |
| Scoped trends and cache allocation | TestRiskTrendsBoundedAggregationScopeAndCalendar, TestAttackPathCacheNestedMutationAndInvalidEncoding; PostgreSQL: TestRiskTrendsAggregationPostgres |
| Real two-cluster topology | TestTwoClusterDaemonSetLive receipt; `scripts/verify/test-two-cluster-integration.py` rejects missing/skipped/failed tests and incomplete/overstated evidence |
| Local CI evidence | Scripts job: test-local-ci-native.py verifies complete workflow groups/matrix, rejected unknown controls, sanitized inherited environment, failure/source-change reports and clean stable all-job publishability |

These are named coverage anchors, not a guarantee against deleting assertions or
introducing a different failure mode. Review remains required. Scoped HTTP and
gRPC isolation regressions are part of the named gate, and the PostgreSQL job
covers real database contracts. The kind-based two-cluster test covers HTTP
ingest, DaemonSets, Kubernetes and backend investigation; it does not run a
browser against the Dashboard or roll out gRPC mTLS.
