# Finding actions and runtime evidence

Runtime signal lists and suppression statistics enforce the signed-in user's cluster scope. The optional `clusterId` parameter narrows that scope; it never grants access. Explicit out-of-scope or unknown pod selections return 403 for restricted users. Retained evidence ownership is checked against soft-deleted pods as well.

`GET /runtime/signals` supports `search` (signal type, category, pod UID) and `sort` (`newest`, `oldest`, `confidence_desc`, `confidence_asc`, `signal_asc`). Search and sorting run before pagination, and totals describe the filtered selection. Dates use UTC; explicit `startDate`/`endDate` override `sinceMinutes`.

Acknowledging persists `acknowledged` (shown as In review). This remains an unresolved finding and contributes to scoring and unresolved summaries. Resolved and dismissed findings must be reopened before acknowledgement. Resolving records `resolved_at`, preserves the original recommendation, and stores resolution notes in the existing audit trail. PATCH checks the permission for the requested action.

Bulk actions validate the scope of every existing selected finding before writing any item. IDs are deduplicated. An unauthorized selection returns 403 without changing earlier items. Missing records and storage failures remain per-item results; successful items are audited and scheduled for rescoring. This is a partial-success API, not an all-or-nothing database transaction.

The dashboard preserves dismissed and unknown statuses instead of displaying them as In review. Runtime evidence follows the selected cluster; filter, sort, time scope, and page-size changes reset pagination.


## Follow-up audit: scope guards and action feedback

- Pod scope middleware checks soft-deleted inventory. Unknown pods return 403 to cluster-restricted users and 404 to unrestricted users; lookup failures return 500 without executing the downstream handler.
- The combined permission/scope guard validates both checks before invoking the handler. Risk-score reads and recalculation routes now use the pod ownership guard.
- Bulk actions require the individual acknowledge/resolve/dismiss permission as well as the route's bulk permission. Empty selections return 400.
- HTTP 200 on a bulk action does not mean every record succeeded. The dashboard reports the server counts and per-item errors, retains failed IDs, and does not invoke single-finding completion on a failed response.
- Dashboard unresolved counts include acknowledged findings.
- Both runtime signal persistence paths use a UTC day boundary; regression coverage includes a UTC+7 host with UTC records.

This audit covers these paths and their regression tests; it does not establish complete multi-cluster isolation. Aggregate analytics/list endpoints and broadcasts still require a separate scope audit, including counts, cache keys and subscriptions. PostgreSQL integration and live lab workflows also remain to be verified.

## Aggregate and notification isolation

Summary (including global and by-cluster), histogram, dashboard statistics and threat velocity now apply the current user's cluster scope before aggregation. An explicit unauthorized cluster returns 403 before cache lookup. Database failures return errors instead of successful empty summary/statistics responses.

Cache keys use structured encoding and include authorization scope. Empty values, literal underscores, colon-containing values, zero versus one minute, and an absent versus explicit zero score bin remain distinct. Histogram counts use one preferred score per resource; score 100 is included in the last bin and `sinceMinutes=0` means all time.

The global risk WebSocket emits only `{"type":"insights_updated"}`. Producer IDs and change types are not broadcast because producers do not supply authoritative cluster ownership. Clients refetch through scoped HTTP endpoints. Pod subscriptions use the same route UID as authorization, and the hub holds its read lock until sends finish so disconnect cannot close a channel during a broadcast.

Regression coverage warms caches as an unrestricted admin, then requests the same resources as users in two separate clusters, checks explicit cross-cluster denial, filtered list cache identity, SQL errors, score-history duplication and concurrent pod disconnect/broadcast.

Remaining work: scope review of the other risk analytics/inventory/graph endpoints; WebSocket session expiry and permission revocation after connection; cluster-qualified agent identity (the current agent inventory identifies nodes by name); PostgreSQL and live multi-cluster integration. Generic global invalidation still reveals that some update occurred, without entity identifiers.

## WebSocket authorization lifetime

Risk and pod WebSocket connections now capture the validated JWT expiry and server session identity. They reload the user, session and required permission before each notification and every 15 seconds while idle. Pod connections also reload cluster scope and ownership. Revoked/expired sessions, disabled/deleted users, password-change restrictions, loss of permission/scope, or failed authorization queries close the socket with code 1008 and a generic reason. JWT expiry has its own timer.

Idle revocation is detected on the next 15-second check, with a 3-second database timeout; this is not an instantaneous logout push. Each outgoing notification performs its own check. Writes have a five-second deadline capped by JWT expiry; input frames are limited to 4 KiB. Ping/pong detects an unresponsive peer after three intervals. No token value is added to notifications or close reasons.

The explicit development principal continues to support auth-disabled local development. Production JWT connections require a validated expiry. Legacy databases without a session table retain the existing HTTP middleware compatibility behavior; session revocation requires the session schema.

These changes cover both `/ws/risks` and `/ws/pod/:uid`. Other analytics/inventory/graph scope checks and cluster-qualified agent identity remain separate audit items.

### Risk analytics cluster authorization

The five `/api/v1/risk/analytics/*` endpoints (trends, comparison, correlation,
supply-chain, runtime-cve) resolve the authenticated user's cluster allow-list
before querying data. Both `cluster` and `clusterId` are accepted and trimmed;
conflicting nonempty values return 400, and an explicitly forbidden cluster
returns 403. Omitting the filter means all authorized clusters. Admin and
unrestricted users retain global access. Missing user context returns 401.

Scope applies before aggregation and SQL limits, including optional entry lists.
Supply-chain and runtime-CVE use SBOM pod UIDs to resolve cluster ownership;
namespace and node names are not authorization keys. Restricted users can access
historical SBOMs only while pod ownership remains in the database (soft-deleted
pods retain ownership). Orphan SBOMs are excluded when any cluster filter applies.
Supply-chain's default active-pod filter and `includeHistorical` option remain.

Namespace correlation groups by both cluster and namespace and joins SBOMs via
pod ownership. It averages scores before the CVE join so differing SBOM/CVE
multiplicity cannot weight risk scores. Trends and comparison still measure stored
score observations, rather than a latest-score inventory snapshot. Their calendar
buckets use UTC. Database failures return 500, without raw SQL errors in responses.

Audit continuation order:

1. Remaining risk APIs: priorities, top/grouped scores, legacy trends, global score
   synchronization and evaluation actions; confirm scope and score-history semantics.
2. Inventory list/detail/export APIs and cluster-qualified agent identity.
3. Graph traversal, caches, and node/edge authorization.
4. Remaining runtime summaries, network APIs, and correlation count semantics.

This patch does not certify those remaining APIs. Correlation summaries are based
on the limited result set; runtime event/CVE pair counts are not distinct event or
CVE counts. PostgreSQL behavior still requires integration validation in the lab.

### Risk score lists, statistics and synchronization

`GET /risk/scores`, `/risk/priorities`, `/risk/top`, `/risk/grouped`,
`/risk/trends` and `POST /risk/scores/sync` now use the same authenticated
cluster scope resolution as analytics, including both cluster filter aliases.

Lists, priority statistics and groups select the latest non-deleted V3 row per
(resource type, resource UID, cluster), breaking equal timestamps by descending
row ID. Score, namespace and priority filters apply after that selection, so an
old observation cannot reappear simply because the latest score fails a filter.
Priority labels still reflect stored P0–P3 metadata; final risk levels remain
derived from total score. Legacy trends retain historical score observations
and use UTC calendar dates.

Optional grouped entries are scoped as strictly as their counts; query failures
return 500 instead of silently omitting entries. Score list page size is bounded
at 500, invalid nonpositive pagination values are normalized, and large page
numbers cannot overflow slice offsets. Equal-sort-value rows have stable ID
ordering.

Sync resolves a distinct UID set once, from active/acknowledged insights. Cluster
filtered requests require retained pod ownership; unrestricted requests preserve
all-resource selection. The background worker processes exactly that captured
set with a 15-minute timeout; it does not invoke global backfill. The response
count describes queued resources, not successful recalculations. Authorization
is checked at acceptance, not continuously during the job.

Remaining risk audit: global finding evaluation/historical evaluation, exceptions,
and attack-step aggregates. Inventory, graph and other runtime surfaces follow.
Live PostgreSQL validation and end-to-end lab validation remain pending.

### Evaluation, exceptions and attack-step summaries

The legacy `/risk/insights/evaluate` and `/risk/insights/evaluate/historical`
operations run across the database. They still require `risk.evaluate`, and now
also require unrestricted cluster scope. Restricted principals receive 403 before
worker construction. Explicit `cluster` or `clusterId` filters are rejected (400
for otherwise authorized global users); these workers do not implement selective
cluster evaluation. Scoped score synchronization remains available via
`/risk/scores/sync`.

Historical evaluation reports resource-query, evaluation and persistence failures
instead of always returning success. A failed evaluation does not proceed to
status reconciliation. Reconciliation errors also return 500; both stages can
have partially applied changes before failure and are not one atomic transaction.

Status reconciliation locates ServiceAccounts, Roles and ClusterRoles by the
finding's resource UID, as already done for Pods and bindings. Same-name resources
in another cluster, or replacements with new UIDs, are distinct identities.
Missing UID or database failures preserve status and report incomplete processing.
Successfully resolved active/acknowledged findings record `resolved_at`; a
concurrent status change to dismissed/resolved is not overwritten. Deeper rule
engine error propagation and automatic score recalculation remain separate audit
items.

Exception lists and mutations accept both cluster filter aliases and enforce
resource ownership. Scoped history uses retained pod records (including soft
deletion), matching create/delete ownership checks. Orphan policies are hidden
from scoped lists and cannot be changed by scoped users; unrestricted users retain
global access. Non-pod ownership is still unsupported for scoped exception actions.
Invalid exception IDs return 400; query errors return generic 500 responses.

Attack-step summaries include only active pods in the authorized clusters.
Authorization and cluster filtering happen before grouping and averaging.

### ServiceAccount inventory authorization

ServiceAccount list totals and pages are restricted to authorized clusters;
`cluster` and `clusterId` are supported with conflict/foreign-filter rejection.
`pageSize=-1` remains capped at 1000 authorized records. Pages use stable ID ordering;
nonpositive page numbers normalize to 1 and oversized offsets return empty pages.
Detail, permission, update and delete handlers check the account's stored cluster
before returning related data or performing mutations (including legacy ID handlers).

Individual updates now accept **labels only**, either a JSON object of string
values or its JSON-encoded string. They update local inventory metadata, not the
Kubernetes object; subsequent agent synchronization can overwrite them. UID,
cluster, name, namespace, credentials, relationships and lifecycle fields cannot
be mass-assigned. Unknown fields reject the entire update.

Individual deletion requires the target cluster's stored kubeconfig. Missing
configuration returns 503; initialization/API failures return 502 and retain the
inventory record. The handler no longer falls back to the server's default cluster
or reports Kubernetes success when client initialization failed. Kubernetes and
DB deletion are not an atomic transaction; live integration still needs lab testing.

Remaining Inventory audit: RBAC permission calculation (including RoleBindings to
ClusterRoles and namespace-qualified grants), bulk mutation semantics, deployment/
ReplicaSet surfaces, capability summaries and cluster/agent identity. This change
secures access to the permission view but does not certify its effective-rule calculation.

### ServiceAccount RBAC grant resolution

The permissions endpoint resolves Role and ClusterRole references separately within
one cluster. RoleBinding grants retain the binding namespace; ClusterRoleBinding
grants have cluster scope. Direct ServiceAccount subjects, the ServiceAccount user
name, and its standard authenticated groups are supported. Duplicate matching
subjects do not duplicate a binding. Non-resource URL rules are excluded from
namespaced grants. The API batches inventory reads and returns HTTP 500 on a
storage failure, malformed matching role reference, or missing referenced role.
The dashboard displays these failures instead of an empty permission list.

`effectiveRules` remains an array for existing clients, with additive `scope`,
`namespace`, `bindingKind`, and `bindingName` fields. A RoleBinding referencing a
ClusterRole uses `clusterRole` instead of `role` in its binding row. These are
synchronized RBAC grants, not live authorization decisions. Resource discovery,
other authorizers, admission policy, token validity, and inventory freshness are
not evaluated. Cluster-scoped resource names in a namespaced grant must not be
interpreted as cluster-wide permission. Aggregated ClusterRoles use synchronized
rules supplied by Kubernetes.

Reference: https://kubernetes.io/docs/reference/access-authn-authz/rbac/

## Shared RBAC resolution (work package B)

Permissions API, Pod RBAC reports and the risk-engine cluster-admin projection
use `core/pkg/rbacinventory`. The resolver matches ServiceAccount subjects,
canonical ServiceAccount User identities and the standard authenticated/SA Groups.
RoleBinding namespace defaults apply only to ServiceAccount subjects; an explicit
SA namespace may differ from the binding's target namespace.

A RoleBinding referencing `cluster-admin` remains a namespace grant. It is visible
in the permissions/report response but does not set `isClusterAdmin` or
`service_account_bound_to_cluster_admin`. Those flags require a resolved
ClusterRoleBinding referencing the exact `cluster-admin` ClusterRole. RoleRef
API group, kind and referenced inventory are validated consistently. Missing roles
and storage/JSON failures return errors rather than successful empty evidence.

This is a named-binding detector, not proof of effective superuser access. Custom
roles with equivalent permissions, modifications to the built-in role, other
authorizers and credential validity require separate analysis. EffectiveRules
remain synchronized grants, not a live SubjectAccessReview. Namespaced grants
omit non-resource URLs; Kubernetes resource discovery is not performed here.

Existing security-state snapshots retain their five-minute cache lifetime before
recomputation. Source freshness/invalidation remains work package D.

Reference: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
