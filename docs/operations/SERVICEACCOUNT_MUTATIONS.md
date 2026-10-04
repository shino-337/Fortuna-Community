# ServiceAccount revocation and durable deletion

Kubernetes has no disabled ServiceAccount field. Legacy disable endpoints still
return 501. Explicit revocation is available through a persisted preview:

1. `POST /api/v1/inventory/serviceaccounts/{uid}/mutations/preview?clusterId={cluster}`
   with `{"action":"revoke"}` or `{"action":"delete"}`. Inspect `plan.steps`,
   `plan.limitations`, `operation.digest` and its ten-minute `expiresAt`.
2. `POST /api/v1/inventory/serviceaccount-mutations/{operation.id}/execute?clusterId={cluster}`
   with `{"digest":"<the inspected operation.digest>"}`. A 202 queues durable intent.
3. `GET /api/v1/inventory/serviceaccount-mutations/{operation.id}?clusterId={cluster}`
   shows completed steps, attempts and `queued`, `running`, `retry`, `blocked`
   or `succeeded`. Cluster scope, creator identity and permissions are checked
   again before execution. Revocation requires inventory.modify and
   inventory.delete; deletion requires inventory.delete.

A revoke plan removes only direct ServiceAccount subjects from the listed
RoleBindings/ClusterRoleBindings, preserving all other subjects and roles. It
also deletes only listed legacy ServiceAccount token Secrets whose name/UID
annotations match the exact ServiceAccount. The preview stores no token data.
Bound TokenRequest tokens, existing Pods, group/inherited permissions, external
credentials and grants created after the preview remain outside this operation.
This does not disable the account or claim complete credential revocation.

Plans bind the ServiceAccount UID, each binding/Secret UID and observed resource
version. Changes in identity, role references or subjects block further writes
and require a fresh preview. Shared bindings are updated with optimistic version
checks; replacement objects cannot be touched. Empty/NotFound results are only
accepted for an already approved exact step. A replaced ServiceAccount blocks
revocation even if its name and subjects match the old account.

A worker claims one step under a 45-second database lease and a 15-second request
budget. Progress and step audit commit together after Kubernetes succeeds. A
crash or persistence failure leaves durable intent: replay recognizes the exact
post-update subjects or NotFound deletion without repeating broader effects.
Transient failures retry after 30 seconds, up to 20 attempts (about 10 minutes),
then become blocked; identity/permission conflicts become blocked immediately. The existing explicit single/bulk DELETE endpoints also persist a
cluster/UID-keyed intent before Kubernetes deletion. Inventory soft deletion and
completion audit commit atomically only after successful UID-guarded deletion.
Workers resume after Core restart; watch the operation status rather than treating
a failed HTTP request as proof that no Kubernetes effect occurred.

The Dashboard ServiceAccount identity detail now exposes the same preview,
execute and status workflow for users with the required permissions. It shows
each target UID, binding subject change and plan limitation before execution,
and stores the operation ID in the page URL so progress can be reopened.
The UI is part of the post-#55 follow-up and requires deployment validation.
Legacy Dashboard disable actions remain unavailable; no automatic
inactive-account revocation is enabled.

## Details

- A preview fails instead of silently omitting effects when the cluster has more
  than 500 RoleBindings, ClusterRoleBindings or Secrets in the namespace to scan.
- Plan digests are computed over canonical typed JSON, so PostgreSQL JSONB key
  ordering or whitespace cannot invalidate a reviewed plan.
- Bulk DELETE requires `inventory.bulk` plus `inventory.delete`, validates the
  whole set first and reports per-item failures with HTTP 207.
- Coverage: unit regressions for scope, actor, digest, binding drift, replacement
  UIDs and persistence failure, plus the live two-cluster
  [integration gate](../maintainers/INTEGRATION_ACCEPTANCE_20260929.md).
