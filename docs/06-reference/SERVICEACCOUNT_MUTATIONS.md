# ServiceAccount mutations

Updating labels changes synchronized Inventory metadata only; the agent may
replace it on the next sync. It does not patch the Kubernetes ServiceAccount.

Single and bulk deletion use the target cluster's stored kubeconfig, a 30-second
request deadline, and the observed Kubernetes UID as a delete precondition. A
replacement object with the same name is not deleted. Kubernetes NotFound is
accepted on retry, allowing reconciliation after a previous Kubernetes success
and database failure. The inventory soft-delete and audit write share a database
transaction; Kubernetes and PostgreSQL cannot share that transaction.

Bulk deletion requires both inventory.bulk and inventory.delete. The entire set
of 1–100 database IDs is checked before deletion; duplicates are collapsed and an
unavailable or unauthorized member rejects the batch. After prevalidation,
individual Kubernetes failures are possible: HTTP 207 includes results with each
ID, HTTP-style status, and message. `count` counts complete successes. Failed
Kubernetes deletes retain inventory. If database persistence fails after a
Kubernetes success, the response explicitly says to retry reconciliation.

The legacy bulk/disable and disable-inactive endpoints return HTTP 501 and do not
change data. They previously hid inventory rows without revoking Kubernetes
credentials. A real disable workflow needs defined token/RBAC revocation and
verified usage evidence; metadata updated_at is not evidence of inactivity.
These endpoints require inventory.bulk and inventory.modify.

Validation includes cluster-scope rejection before effects, permission checks,
failed-delete preservation, duplicate IDs and Kubernetes UID-precondition tests.
Live kubeconfig/RBAC, PostgreSQL rollback and retry behavior still need lab tests.
