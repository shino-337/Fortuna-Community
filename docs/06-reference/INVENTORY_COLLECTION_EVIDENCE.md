# Inventory observation receipts

The HTTP Syncer now sends a version-1 `collection` envelope alongside `data`:
collection ID, collection start/end timestamps, per-kind List start bounds,
namespace scope and counts for Pods, ServiceAccounts, Roles, RoleBindings,
ClusterRoles, ClusterRoleBindings, Deployments and ReplicaSets. All eight lists must be explicit arrays on success.
An empty array is a successful empty **collection observation**; an absent/null
array is not. `status=complete` never means the persisted projection was
authoritatively reconciled to empty. V1 explicitly reports
`projectionSemantics=non_authoritative_deletion` and
`deletionAuthoritative=false`.
Unfinished Kubernetes pagination or any list error sends a failed observation,
without applying a partial inventory. If Core cannot be reached, the previous
receipt ages out after ten minutes; failure detection is not instantaneous.

Core validates the full envelope/batch before applying inventory. Scoped HTTP
credentials bind the receipt to the authenticated cluster and Agent. A scoped
authenticated sync without a `collection` envelope is rejected; it cannot fall
through to the unverified compatibility writer. Agent
liveness/node/version updates are staged in the same transaction: a failed
inventory persistence cannot advance Agent health while rolling back the evidence
that justified it.

HTTP ingest selects one authentication mode. When the scoped credential registry
is configured, the legacy shared token is not accepted and there is no mixed
scoped/legacy fallback on that route. Explicit compatibility/internal unverified
writes still invalidate prior verified status because they can change the
projection without producing trustworthy collection evidence; retaining the old
verified receipt would be unsafe.

For verified observations, Core serializes writes per cluster and commits the
inventory projection and receipt together. Persistence errors roll back inventory
and attempt to store `failed` with `failureStage: persistence` after rollback.
Collector-reported failures use `failureStage: collection`. A bounded independent
context records failures even when the original request is cancelled. Database
outages can prevent failure recording; this error is returned, and no successful
receipt is fabricated. Dependent asynchronous work starts after commit.

The eight Kubernetes List calls are sequential. The receipt therefore describes a
bounded collection interval, not one atomic Kubernetes/etcd snapshot. For each kind,
the Agent records a conservative lower bound immediately before starting the List
request. Role/ClusterRole resolution requires that List start bound to be at or
after the finding; the later batch-end timestamp cannot substitute for it. Cross-resource absence or consistency
is not inferred from this interval and remains ineligible for auto-resolution.

Identical accepted replay is idempotent. Changed replay or an older observation
is rejected. Failed attempts are terminal; collect a new ID on retry (the next
Syncer cycle does this automatically). Timestamps are normalized to microseconds
for PostgreSQL replay comparisons. Keep Agent/Core clocks synchronized; observations
older than ten minutes or more than one minute in the future are rejected. A
future-dated receipt within that tolerance still cannot authorize resolution until
its observation time is in the past.

Namespace-limited collections require exact namespace equality for every
namespaced payload row and prune only within that namespace. Namespaced upserts
lookup by `cluster_id + namespace + uid` and reject a persisted same-UID row in
another namespace instead of adopting/moving it. Duplicate UIDs across namespaces
inside one collection are also rejected. ClusterRole and
ClusterRoleBinding collections remain cluster scoped and are still collected when
the namespaced scope is restricted.

The Agent is deployed as a DaemonSet, so multiple scoped Agents in one cluster
normally submit full inventory. The receipt store keeps the latest accepted
inventory attempt per cluster and serializes writers by the receipt row. A newer
failed Agent attempt may temporarily replace a complete receipt, but does not
apply its inventory projection; an older attempt is rejected and a later complete
attempt recovers the receipt. This is fail-closed availability behavior.

If multiple Agents for one cluster are configured with different
`WATCH_NAMESPACE` values, their receipts can also supersede one another. This
fails closed for resolution because a namespace mismatch preserves the finding,
but it can reduce automatic-resolution eligibility for the displaced scope. A live
multi-scope topology test (and a scope-qualified receipt key if that topology is
required) remains part of package F; this PR does not claim concurrent
namespace-scope aggregation. A complete receipt proves
acceptance of the included projection, not authoritative Kubernetes deletion:
existing safeguards retain objects on empty/suspiciously reduced snapshots.
Derived PCE/risk processing and audit export are separate from collection coverage.

Role/ClusterRole auto-resolution requires all D1 detector constraints plus:

- a complete, fresh, authenticated receipt whose resource-kind List start bound
  is at or after finding detection;
- the correct namespace coverage;
- a digest binding the current cluster/kind/UID/name/namespace/rules snapshot to
  that observation;
- unchanged receipt/resource/finding state at the resolution transaction.

The batch end timestamp cannot substitute for the Role/ClusterRole List start bound
time. An unchanged Role can still have a fresh observation without changing
UpdatedAt.
Changes after the observation invalidate its digest. Unknown/stale/failed evidence
preserves the finding. The resolution audit records collection ID and observation
time. No legacy data is backfilled into verified evidence during migration.

Roll out **Core first, then Agent**. Core #50 with an older Agent accepts the
compatibility write as unverified and therefore blocks receipt-based resolution.
The inverse order is not equivalent: Core #49 ignores the new `collection`
envelope and retains D1's older static-snapshot freshness semantics. Do not use
Agent-first rollout as evidence that #50 protections are active.

Upgrade Core, verify the inventory receipt schema/startup invariant, then roll out
the Agent and configure the scoped HTTP token/registry described in
`deploy/scoped-agent-credentials/README.md`. Check the sync response's
`inventoryStatus`/`collection`. Old Agents continue ingesting in the configured compatibility mode, but cannot
provide verified receipts. Rolling Core back to #49 also rolls back the stronger
receipt requirement; treat that as a security-semantic rollback requiring explicit
risk acceptance rather than an ordinary transparent application rollback. The response always
reports runtime coverage as `unknown`: inventory lists and Agent heartbeats do not
prove runtime sensor coverage. Runtime/Pod/cross-resource auto-resolution remains
blocked pending D2 runtime producer windows, loss/error/recovery reporting and
complete evidence dependencies. User-facing evidence views remain package E.

CI covers HTTP ownership, Agent collection failure/empty/pagination behavior,
per-kind List start bounds, cross-namespace rejection/pruning boundaries, receipt
freshness/digests, Agent-liveness rollback, DaemonSet multi-writer arbitration and
PostgreSQL concurrent replay plus SQL-trigger rollback/recovery. Real two-cluster deployment
acceptance remains package F.
