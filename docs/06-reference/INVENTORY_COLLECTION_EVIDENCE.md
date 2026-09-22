# Inventory observation receipts

The HTTP Syncer now sends a version-1 `collection` envelope alongside `data`:
collection ID, start/observation timestamps, namespace scope and counts for Pods,
ServiceAccounts, Roles, RoleBindings, ClusterRoles, ClusterRoleBindings,
Deployments and ReplicaSets. All eight lists must be explicit arrays on success.
An empty array is a successful empty observation; an absent/null array is not.
Unfinished Kubernetes pagination or any list error sends a failed observation,
without applying a partial inventory. If Core cannot be reached, the previous
receipt ages out after ten minutes; failure detection is not instantaneous.

Core validates the full envelope/batch before applying inventory. Scoped HTTP
credentials bind the receipt to the authenticated cluster and Agent. Without
scoped credentials, accepted inventory remains `unknown` for evidence purposes.
This includes older gRPC/HTTP compatibility callers: they invalidate prior verified
status and advance the ordering watermark. Mixed legacy/scoped writers can
therefore prevent automatic resolution until the rollout is completed.

For verified observations, Core serializes writes per cluster and commits the
inventory projection and receipt together. Persistence errors roll back inventory
and attempt to store `failed` with `failureStage: persistence` after rollback.
Collector-reported failures use `failureStage: collection`. A bounded independent
context records failures even when the original request is cancelled. Database
outages can prevent failure recording; this error is returned, and no successful
receipt is fabricated. Dependent asynchronous work starts after commit.

Identical accepted replay is idempotent. Changed replay or an older observation
is rejected. Failed attempts are terminal; collect a new ID on retry (the next
Syncer cycle does this automatically). Timestamps are normalized to microseconds
for PostgreSQL replay comparisons. Keep Agent/Core clocks synchronized; observations
older than ten minutes or more than one minute in the future are rejected. A
future-dated receipt within that tolerance still cannot authorize resolution until
its observation time is in the past.

Namespace-limited collections prune only within that namespace. ClusterRole and
ClusterRoleBinding collections remain cluster scoped. A complete receipt proves
acceptance of the included projection, not authoritative Kubernetes deletion:
existing safeguards retain objects on empty/suspiciously reduced snapshots.
Derived PCE/risk processing and audit export are separate from collection coverage.

Role/ClusterRole auto-resolution requires all D1 detector constraints plus:

- a complete, fresh, authenticated receipt at or after finding detection;
- the correct namespace coverage;
- a digest binding the current cluster/kind/UID/name/namespace/rules snapshot to
  that observation;
- unchanged receipt/resource/finding state at the resolution transaction.

An unchanged Role can thus have a fresh observation without changing UpdatedAt.
Changes after the observation invalidate its digest. Unknown/stale/failed evidence
preserves the finding. The resolution audit records collection ID and observation
time. No legacy data is backfilled into verified evidence during migration.

Upgrade Core and Agent, configure the scoped HTTP token/registry described in
`deploy/scoped-agent-credentials/README.md`, then check the sync response's
`inventoryStatus`/`collection`. Old Agents continue ingesting in the configured
compatibility mode, but cannot provide verified receipts. The response always
reports runtime coverage as `unknown`: inventory lists and Agent heartbeats do not
prove runtime sensor coverage. Runtime/Pod/cross-resource auto-resolution remains
blocked pending D2 runtime producer windows, loss/error/recovery reporting and
complete evidence dependencies. User-facing evidence views remain package E.

CI covers HTTP ownership, Agent collection failure/empty/pagination behavior,
validation, namespace boundaries, receipt freshness/digests and PostgreSQL
concurrent replay plus SQL-trigger rollback/recovery. Real two-cluster deployment
acceptance remains package F.
