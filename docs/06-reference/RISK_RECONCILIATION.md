# Risk evaluation and reconciliation

RBAC YAML rules declare applicable Kubernetes kinds through resource-kind tags.
Role rules run on Role/ClusterRole, binding rules on their binding kinds, and
ServiceAccount/Pod rules on their own kinds. Existing database overrides inherit
the shipped kind tags unless explicitly configured. This prevents CEL expressions
for one object shape from running against an unrelated shape. Normalization sets
kind from the evaluator's resource type.

Engine and YAML evaluation return rule errors; failed evaluation must not be
interpreted as no findings. Historical evaluation and auto-resolution use the
configured YAML/CEL engine without silent fallback to the simpler evaluator.
An empty, partially invalid, or unreadable configured catalog stops these workers.
Malformed DB rule conditions are reported instead of skipped. DB rule storage is
still optional when its table has not been deployed.

For an existing resource, auto-resolution requires the same enabled detector to
still apply to that resource kind (rule ID, or title for legacy findings). A
removed/disabled detector is not evidence of remediation. Invalid synchronized
pod container JSON also preserves the finding and reports failure. Deleted exact
UIDs retain the existing resolution behavior; supply-chain findings remain owned
by their separate reconciliation pipeline.

Each resolution and its system audit record are committed in one transaction.
Audit failure rolls back the status change. The status predicate preserves manual
resolved/dismissed changes made during evaluation. The overall batch is not
atomic: completed per-finding transactions remain if a later finding fails, and
the caller receives an incomplete-reconciliation error.

Tests cover evaluator errors, partial/empty catalogs, disabled detectors, audit
rollback, exact UID matching, and shipped RBAC rules. Runtime enrichment readers
and source freshness remain separate concerns: this is not proof of end-to-end
runtime coverage. PostgreSQL transactions and concurrent live ingestion remain
lab validation gates.

## Runtime input failures

Pod evaluation now propagates failures from security-state projection and reads,
runtime/capability queries, binding queries and malformed persisted evidence.
A failed refresh cannot fall back to the previous snapshot or zero/false inputs.
The base, YAML and runtime-only evaluators return the error; reconciliation retains
the finding and reports an incomplete run instead of recording auto-resolution.

Deploy the required runtime, capability, binding and security-state schema before
enabling live Pod evaluation. Partial migrations now produce explicit errors.
Database-free rule previews remain supported. The existing five-minute snapshot
cache remains; telemetry completeness and invalidation are tracked in package D of
[NEXT_AUDIT_PLAN.md](NEXT_AUDIT_PLAN.md). Successful reads alone do not prove that
a sensor is healthy or that all events have arrived.
