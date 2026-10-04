# ServiceAccount mutation contract

Explicit revocation uses a reviewed, immutable plan. In a selected authorized
cluster, call `POST /api/v1/inventory/serviceaccounts/:uid/mutations/preview`
with `{"action":"revoke"}` or `{"action":"delete"}`. The response returns
`operation` and `plan`, including exact targets, Kubernetes UIDs, resource versions,
subject changes and limitations. Execute with
`POST /api/v1/inventory/serviceaccount-mutations/:operationID/execute` and
`{"digest":"<reviewed digest>"}` within ten minutes. Read progress at the same
operation path with GET. Preview, execute and readback recheck JWT permissions,
cluster scope and the initiating user. Revoke requires both inventory.modify and
inventory.delete; delete requires inventory.delete.

Revocation removes only direct subjects for the selected ServiceAccount from
RoleBindings/ClusterRoleBindings, preserving other subjects and role references.
It deletes only legacy ServiceAccount-token Secrets whose name and UID annotations
match the target. It does not remove inherited group grants, revoke externally
copied credentials or bound TokenRequest tokens, delete Pods, prevent new grants,
or constitute a general Kubernetes account-disable mechanism. A preview rejects
truncated binding/Secret lists rather than silently omitting effects.

Execution rechecks the account UID, binding UID/resource version/role reference
and reviewed subjects. Drift, replacement objects and authorization denial block
the operation and require a new preview. Exact resulting subjects or NotFound
allow idempotent replay. Secret deletion uses UID and resource-version
preconditions; account deletion uses its UID. Plan digests use canonical typed
JSON, so PostgreSQL JSONB key ordering/whitespace cannot invalidate a valid plan.

Intent and its audit record commit before Kubernetes effects. A durable worker
claims one step with a 45-second lease and a 15-second Kubernetes request deadline.
Transient failures retry after 30 seconds. Progress and completion audit commit
in one database transaction; a persistence failure leaves the lease for replay.
Inventory soft deletion happens only after successful Kubernetes deletion and in
the completion transaction. Kubernetes and PostgreSQL cannot share a transaction.

Existing single/bulk DELETE endpoints also queue durable cluster/UID deletion
intent and attempt it immediately. Bulk requires inventory.bulk plus inventory.delete,
prevalidates the complete set and reports individual failures with HTTP 207.
Labels remain synchronized Inventory metadata. The post-#55 Dashboard identity
detail offers explicit preview, execute and operation-status controls, subject to
the same backend permissions and cluster scope. Its live deployment is still an
acceptance gate. Legacy disable/disable-inactive still return HTTP 501; those
actions do not imply Kubernetes account disablement.

Named regressions cover scope/actor/digest rejection, binding drift, replacement
UIDs and persistence failure after Kubernetes success. The permanent two-cluster
kind gate additionally exercises real JWT routes, Kubernetes revocation and
SubjectAccessReview, replacement UID protection and PostgreSQL audit-failure
recovery. See [integration evidence](INTEGRATION_ACCEPTANCE.md).
