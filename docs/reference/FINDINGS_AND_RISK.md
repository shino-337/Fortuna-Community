# Findings and risk

How the risk APIs apply cluster scope, how finding actions behave, how findings are evaluated, resolved and re-scored, and how ServiceAccount permissions are resolved.

## Cluster scope

Every risk endpoint resolves the signed-in user's cluster allow-list before it reads, aggregates, caches or paginates data. This covers finding actions, runtime signal lists and suppression statistics, summaries, histograms, dashboard statistics, threat velocity, the five `/api/v1/risk/analytics/*` endpoints, risk score lists, exceptions, attack-step summaries and the findings export (`/risk/insights/export`, CSV and print-ready HTML). The export applies the same filters as the Risk Center list, including `finalLevel`, and prefixes CSV cells that start with `=`, `+`, `-`, `@`, tab or carriage return with `'` so spreadsheets do not evaluate them. Outside the risk API, notifications (list, mark read, mark all read), agent status and the counts in `/metrics/system` follow the same allow-list, while `/metrics/workers` and `/health/dashboard-data-integrity` return 403 to restricted users because their counts span every cluster; a restricted user does not see notifications without a cluster.

- Both `cluster` and `clusterId` filters are accepted and trimmed. Conflicting non-empty values return 400; an explicitly forbidden cluster returns 403 before any cache lookup. A filter narrows the scope; it never grants access. Omitting it means all authorized clusters.
- Admin and unrestricted users keep global access. Missing user context returns 401.
- Pod ownership is checked against retained inventory, including soft-deleted pods. Unknown or out-of-scope pods return 403 to cluster-restricted users and 404 to unrestricted users; lookup failures return 500 without running the handler. The combined permission and scope guard checks both before the handler runs.
- Supply-chain and runtime-CVE analytics resolve ownership through SBOM pod UIDs. Namespace and node names are never authorization keys. Restricted users see historical SBOMs only while pod ownership remains in the database, and orphan SBOMs are excluded whenever a cluster filter applies.
- Database failures return errors (500, or 503 for missing schema) instead of successful empty results, and responses never include raw SQL errors.
- Cache keys use structured encoding and include the authorization scope, so empty values, underscores, colons, zero versus one minute and an absent versus explicit zero score bin stay distinct.

Regression tests warm caches as an unrestricted admin, then request the same resources as users in two separate clusters, and check cross-cluster denial, filtered cache identity, SQL errors, score-history duplication and concurrent pod disconnect and broadcast.

## Risk levels

A finding has two severities, and only one of them is its risk level.

- **Risk level** (critical, high, medium, low) is the band of the preferred risk score of the finding's resource: 70 and above is critical, 40 to 69 high, 20 to 39 medium, below 20 low. The preferred score is the latest V3 score row for that cluster and resource, or the latest row of any scorer version when there is no V3 row. The findings list and its `finalLevel` filter, the finding detail page, `riskLevelCounts` in `/risk/insights/summary`, `criticalRisks` in `/dashboard/stats`, finding notifications and the `risk_level` / `risk_score` export columns all use it. A finding whose resource has no score yet has no risk level.
- **Rule severity** is the `severity` stored on the finding by its rule or CVE. It is shown as a hint, kept in the export's `severity` column, counted in the summary's `critical`/`high`/`medium`/`low` fields, and used by the `severity` filter and the "Rule severity" sort.

`riskLevelCounts` is always returned (zeros when nothing is scored), so clients never count rule severities as risk levels.

## Finding actions

Acknowledging persists `acknowledged` (shown as In review). It stays an unresolved finding: it contributes to scoring and to unresolved counts in the API and Dashboard. Resolved and dismissed findings must be reopened before acknowledgement: `PATCH /risk/insights/:id` with `{"status":"active"}` reopens a resolved, dismissed or acknowledged finding, requires `findings.reopen` and clears `resolved_at`. The Dashboard shows Reopen on a closed finding's detail page to users with that permission. Resolving records `resolved_at`, preserves the original recommendation and stores resolution notes in the audit trail. PATCH checks the permission for the requested action.

Bulk actions:

- require the individual acknowledge, resolve or dismiss permission as well as the route's bulk permission; empty selections return 400;
- deduplicate IDs and validate the scope of every existing selected finding before writing any item; an unauthorized selection returns 403 without changing anything;
- are partial-success, not one database transaction. Missing records and storage failures are per-item results; successful items are audited and scheduled for rescoring. HTTP 200 does not mean every item succeeded, so the Dashboard reports the server counts and per-item errors and keeps failed IDs selected.

The Dashboard preserves dismissed and unknown statuses instead of displaying them as In review.

## Runtime signal lists

`GET /runtime/signals` supports `search` (signal type, category, pod UID) and `sort` (`newest`, `oldest`, `confidence_desc`, `confidence_asc`, `signal_asc`). Search and sorting run before pagination, and totals describe the filtered selection. Dates use UTC; explicit `startDate`/`endDate` override `sinceMinutes`. Both runtime signal persistence paths use a UTC day boundary. In the Dashboard, runtime evidence follows the selected cluster, and changing the filter, sort, time scope or page size resets pagination.

## Risk scores and analytics

`GET /risk/scores`, `/risk/priorities`, `/risk/top`, `/risk/grouped` and `/risk/trends` select the latest non-deleted V3 row per (resource type, resource UID, cluster), breaking equal timestamps by descending row ID. Score, namespace and priority filters apply after that selection, so an old observation cannot reappear because the latest score fails a filter. Priority labels reflect stored P0–P3 metadata; the final risk level is derived from the total score.

- Grouped entries are scoped as strictly as their counts. Query failures return 500 instead of omitting entries.
- Score list page size is capped at 500, non-positive pagination values are normalized, large page numbers cannot overflow offsets, and rows with equal sort values are ordered by ID.
- Histogram counts use one preferred score per resource; score 100 falls in the last bin and `sinceMinutes=0` means all time.
- Trends and comparison measure stored score observations, not a latest-score snapshot, and bucket by UTC calendar day.
- Namespace correlation groups by cluster and namespace, joins SBOMs through pod ownership and averages scores before the CVE join, so SBOM or CVE multiplicity cannot weight scores. Correlation summaries describe the limited result set, and runtime event/CVE pair counts are not distinct event or CVE counts.
- Supply-chain analytics keep their default active-pod filter and the `includeHistorical` option.

`POST /risk/scores/sync` resolves a distinct set of resource UIDs once, from active and acknowledged insights. Cluster-filtered requests require retained pod ownership. A background worker processes exactly that set with a 15-minute timeout and does not run a global backfill. The response count is the number of queued resources, not successful recalculations, and authorization is checked when the request is accepted, not during the job.

Attack-step summaries include only active pods in the authorized clusters; scope is applied before grouping and averaging.

## Exceptions

Exception lists and mutations enforce resource ownership through retained pod records, including soft-deleted pods, for both history and create/delete. Orphan policies are hidden from scoped lists and cannot be changed by scoped users. Scoped users cannot manage exceptions on non-pod resources. Invalid exception IDs return 400; query errors return a generic 500.

## Notification center

Notifications are derived from current data each time the list is read:

- **Findings:** an open finding raises an alert when its risk level is high or critical. The alert's severity is that risk level, the text gives the score (and the rule severity when it differs), and it opens the finding (`/risks/:id`). Stored finding alerts are re-checked on every read: they take the finding's current level, and they are removed when the finding is resolved, dismissed, deleted or rescored below high, or when its pod is gone.
- **Attack paths:** a path with risk 7.0/10 or more raises a high alert, 9.0 or more a critical one, matching the Attack Paths page.
- **CVEs and malware:** the alert carries the CVE severity (critical or high) or the package verdict, and opens the pod's SBOM tab, where the same CVE or package is listed.

Stored alerts also take a refreshed title, text and link when their source changes; read state is kept.

`GET /notifications` returns the caller's notifications newest first (`limit` up to 100, `offset`, `unreadOnly=true`) with `total` and `unreadCount`. Read state is per user and stored in `notification_reads`: marking one notification or all of them read changes only the caller's bell. Migration 152 keeps notifications that were already marked read under the old shared `read_at` column read for every existing user.

## List limits

Every list endpoint caps the rows it returns. A missing, zero, negative or non-numeric `limit` uses the endpoint's default, and a larger value is lowered to its maximum; the values are in `core/internal/api/list_limits.go`. When rows are cut the response sets `truncated` (or `signalsTruncated`, `insightsTruncated`); existing `total` fields keep their meaning. Graph responses keep the highest-risk nodes, drop links to removed nodes and report `totalNodes`/`totalLinks`. The risk rules YAML export sets `X-Fortuna-Export-Truncated` when cut.

## Live notifications

The global risk WebSocket (`/ws/risks`) emits only `{"type":"insights_updated"}`. Producer IDs and change types are not broadcast because producers do not supply authoritative cluster ownership; clients refetch through scoped HTTP endpoints. The notification still reveals that some update occurred. Pod subscriptions (`/ws/pod/:uid`) authorize the same UID the route names, and the hub holds its read lock until sends finish, so a disconnect cannot close a channel mid-broadcast.

Both sockets capture the validated JWT expiry and server session. They reload the user, session and required permission before each notification and every 15 seconds while idle; pod sockets also reload cluster scope and ownership. A revoked or expired session, a disabled or deleted user, a pending password change, lost permission or scope, or a failed authorization query closes the socket with code 1008 and a generic reason. JWT expiry has its own timer.

- Idle revocation is detected at the next 15-second check (3-second database timeout); it is not an instant logout push.
- Writes have a five-second deadline capped by JWT expiry, input frames are limited to 4 KiB, and ping/pong drops an unresponsive peer after three intervals. Tokens never appear in notifications or close reasons.
- With authentication disabled for local development, the explicit development principal is used. Session revocation requires the session table; databases without it keep the HTTP middleware's compatibility behavior.

## Evaluation

RBAC YAML rules declare the Kubernetes kinds they apply to through resource-kind tags. Role rules run on Role and ClusterRole, binding rules on their binding kinds, and ServiceAccount and Pod rules on their own kinds. Database overrides inherit the shipped kind tags unless configured otherwise, so a CEL expression for one object shape never runs against another.

Evaluation errors are never read as "no findings":

- The engine and YAML evaluators return rule errors. Historical evaluation and auto-resolution use the configured YAML/CEL engine with no silent fallback to a simpler evaluator.
- An empty, partially invalid or unreadable configured catalog stops these workers. Malformed database rule conditions are reported, not skipped. Database rule storage is optional when its table is not deployed.
- Pod evaluation propagates failures from security-state projection and reads, runtime and capability queries, binding queries and malformed persisted evidence. A failed refresh cannot fall back to the previous snapshot or to zero/false inputs.

Deploy the runtime, capability, binding and security-state schema before enabling live Pod evaluation; a partial migration produces explicit errors. Database-free rule previews still work. Pod security state is cached for up to five minutes (`FORTUNA_SECURITY_STATE_CACHE_TTL`). Successful reads do not prove that a sensor is healthy or that every event has arrived; see [runtime evidence](RUNTIME_EVIDENCE.md).

`POST /risk/insights/evaluate` and `/risk/insights/evaluate/historical` run across the whole database. They require `risk.evaluate` and unrestricted cluster scope; restricted principals get 403, and explicit `cluster`/`clusterId` filters are rejected with 400 because these workers cannot evaluate one cluster. Use `/risk/scores/sync` for scoped recalculation. Historical evaluation reports resource-query, evaluation and persistence failures, and a failed evaluation does not continue to reconciliation. Both stages can leave partially applied changes when they fail.

## Auto-resolution

Reconciliation locates ServiceAccounts, Roles, ClusterRoles, Pods and bindings by the finding's resource UID. Same-name resources in another cluster, or replacements with a new UID, are distinct identities. A missing UID or a database failure preserves the finding and reports incomplete processing.

For an existing resource, auto-resolution requires the same enabled detector to still apply to that resource kind (matched by rule ID, or by title for older findings). A removed or disabled detector is not evidence of remediation. Invalid synchronized container JSON preserves the finding and reports a failure. Findings for deleted UIDs resolve as before. Supply-chain findings are reconciled by their own pipeline. Role and ClusterRole resolution additionally requires a fresh inventory receipt; see [Inventory](INVENTORY.md#inventory-observation-receipts).

Each resolution and its audit record commit in one transaction; an audit failure rolls back the status change. The status predicate keeps manual resolved or dismissed changes made during evaluation, and resolved findings record `resolved_at`. The batch as a whole is not atomic: per-finding transactions that completed stay committed if a later finding fails, and the caller receives an incomplete-reconciliation error. Runtime absence never resolves a finding.

## ServiceAccount permissions and inventory

ServiceAccount list totals and pages are restricted to authorized clusters, with the same `cluster`/`clusterId` rules as above. `pageSize=-1` is capped at 1000 authorized records, pages use stable ID ordering, non-positive page numbers normalize to 1 and oversized offsets return empty pages. Detail, permission, update and delete handlers check the account's stored cluster before returning related data or mutating anything.

Updates accept **labels only**, as a JSON object of string values or its JSON-encoded string. They change Fortuna's inventory metadata, not the Kubernetes object, and the next Agent sync can overwrite them. Unknown fields reject the whole update. Deletion uses the target cluster's stored kubeconfig: missing configuration returns 503 and client or API failures return 502 with the inventory record kept. Kubernetes and database deletion are not one transaction. Reviewed binding revocation is described in [ServiceAccount mutations](../operations/SERVICEACCOUNT_MUTATIONS.md).

The permissions API, Pod RBAC reports and the risk engine's cluster-admin projection share one resolver, `core/pkg/rbacinventory`:

- It matches ServiceAccount subjects, the canonical `system:serviceaccount:` user name and the standard authenticated and ServiceAccount groups. Duplicate matching subjects do not duplicate a binding. RoleBinding namespace defaults apply only to ServiceAccount subjects.
- Role and ClusterRole references are resolved separately within one cluster, and the roleRef API group, kind and referenced object are validated. RoleBinding grants keep the binding namespace; ClusterRoleBinding grants have cluster scope. Non-resource URL rules are excluded from namespaced grants.
- A RoleBinding that references `cluster-admin` stays a namespace grant. `isClusterAdmin` and `service_account_bound_to_cluster_admin` require a ClusterRoleBinding to the exact `cluster-admin` ClusterRole.
- Storage or JSON failures, malformed role references and missing roles return HTTP 500, and the Dashboard shows the error instead of an empty list.

`effectiveRules` is an array with `scope`, `namespace`, `bindingKind` and `bindingName` on each row; a RoleBinding to a ClusterRole uses `clusterRole` instead of `role`. These are synchronized RBAC grants, not live authorization decisions: resource discovery, other authorizers, admission policy, token validity and inventory freshness are not evaluated, and custom roles equivalent to `cluster-admin` are not flagged as cluster-admin. Aggregated ClusterRoles use the rules Kubernetes has already aggregated. See the [Kubernetes RBAC documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).
