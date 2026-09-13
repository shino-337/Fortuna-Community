# Finding actions and runtime evidence

Runtime signal lists and suppression statistics enforce the signed-in user's cluster scope. The optional `clusterId` parameter narrows that scope; it never grants access. Explicit out-of-scope or unknown pod selections return 403 for restricted users. Retained evidence ownership is checked against soft-deleted pods as well.

`GET /runtime/signals` supports `search` (signal type, category, pod UID) and `sort` (`newest`, `oldest`, `confidence_desc`, `confidence_asc`, `signal_asc`). Search and sorting run before pagination, and totals describe the filtered selection. Dates use UTC; explicit `startDate`/`endDate` override `sinceMinutes`.

Acknowledging persists `acknowledged` (shown as In review). This remains an unresolved finding and contributes to scoring and unresolved summaries. Resolved and dismissed findings must be reopened before acknowledgement. Resolving records `resolved_at`, preserves the original recommendation, and stores resolution notes in the existing audit trail. PATCH checks the permission for the requested action.

Bulk actions validate the scope of every existing selected finding before writing any item. IDs are deduplicated. An unauthorized selection returns 403 without changing earlier items. Missing records and storage failures remain per-item results; successful items are audited and scheduled for rescoring. This is a partial-success API, not an all-or-nothing database transaction.

The dashboard preserves dismissed and unknown statuses instead of displaying them as In review. Runtime evidence follows the selected cluster; filter, sort, time scope, and page-size changes reset pagination.
