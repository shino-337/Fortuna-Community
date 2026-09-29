# Audit remediation status — September 2026

Source/GAP checkpoint: 2026-09-29 UTC. PRs #29–#50, #52 and #53 are merged;
#51 was retired. PR #54 remains open on `fix/ui-availability-e2`; review and
merge it manually after its exact-head gates. PR #47 added immutable image-content
snapshots with separate workload observations,
container-qualified ingest, ownership validation before matching/linking, retired
combined finding writes, and cluster propagation to CVE/malware matches. Its startup
migration backfills resolved observations, rejects conflicting legacy evidence,
and enforces PostgreSQL uniqueness and ownership guards. Review/CI and the live
multi-cluster gate are tracked separately in
[NEXT_AUDIT_PLAN.md](NEXT_AUDIT_PLAN.md).
A passing test suite confirms the tested source behavior, not live deployment
coverage or a guarantee that the repository has no further defects.

## Current GAP status

| Package / GAP | Source and recorded evidence | Remaining closure gate |
| --- | --- | --- |
| A / runtime input failures | Merged #36; errors preserve findings | Real evaluation/reconciliation flow in F |
| B / RBAC semantics | Merged #37; shared scoped resolution | Two-cluster API/rule agreement in F |
| C / Agent and resource identity | HTTP/gRPC credentials, composite Agent identity and cluster-qualified storage merged through #48 | Real two-cluster duplicate UID/node/digest and credential lifecycle in F |
| D1 / inventory resolution | Merged #49; evidence eligibility and concurrent-update guards | Live topology in F |
| D2 / inventory and runtime receipts | Merged #50/#52; immutable receipts, session lifecycle and replay guards | Multi-writer/namespace-scope DaemonSet acceptance in F |
| D3 / independent runtime source health | Open; every current producer remains non-authoritative | Independent sensor-health protocol, authenticated producer/session binding, expiry/restart regressions, then F |
| E / availability | E1 merged #53; E2/residual E1 fixes on #54; full working-tree CI passed 2026-09-29 | Clean exact committed-head CI, owner review and manual merge |
| F / integration | Seven PostgreSQL 16 workflow selections passed 2026-09-29 against a temporary database | Populated legacy-data rehearsal, real two-cluster/DaemonSet tests, worker/manual-action concurrency and Kubernetes deletion retry |
| G / scoped AGE | Open; legacy unscoped routes removed, relational views available | Scoped traversal entry points/nodes/edges and two-cluster leak regressions |
| H / mutations and revocation | Open; UID-guarded deletion exists, unsupported disable returns 501 | Explicit revocation workflow and durable retry/audit/live acceptance |
| I / performance and investigation | Open | Measured load/cache/storage results and a verified first-investigation walkthrough |

The historical single-node rollouts and database resets in
[the plan](NEXT_AUDIT_PLAN.md) do not close F's populated migration or two-cluster
gates. No live rollout of the 2026-09-28/29 ingest fixes is recorded. The previous
#54 hosted CI run has seven failed jobs with zero executed steps; its local
success status does not establish validation of a later head. Exact tested SHA,
source fingerprint and log hashes belong in external CI evidence.

## New findings and deployment follow-ups

These IDs identify this register's entries; they are not upstream advisory IDs.
Operational and correctness findings are not assigned vulnerability severity
without an established attack path.

| ID | Finding and evidence | Fix / regression | Status and remaining gate |
| --- | --- | --- | --- |
| INGEST-01 | A Falco batch containing an old rollout Pod UID returned ownership 403 indefinitely, blocking valid siblings. Restart restored fresh traffic but did not preserve a durable retry state. | Optional principal/source-bound disk cursor/outbox; isolate only `runtime_ownership_mismatch`, preserve canonical payloads/source IDs, bounded quarantine/backoff. `TestFalcoDurableMixedBatchRestartAndRecovery`, `TestFalcoDurableRotationReplaysPendingBeforeReadingReplacement`, `TestFalcoDurableBackoffSurvivesRestartAndMissingSource`, `TestFalcoDurablePersistenceFailureDoesNotSendOrAdvanceCursor`. | Source fixed; pending live image plus state env/volume/mount rollout, restart/rotation and retained-evidence recovery. Without the state path, legacy whole-slice retry still applies. |
| INGEST-02 | Pod-event retry/quarantine generated repeated rate limiting: the 2026-09-28 bounded log sample contained 75 HTTP 429 failures and 705 quarantine entries. Recursive splitting and individual quarantine retry could consume excessive requests. | Shared 12-request flush budget; stop siblings on transient/non-ownership failure; honor `Retry-After`; keep deferred quarantine separate from fresh events. `TestEventsCollectorRateLimitStopsFlushAndPreservesQuarantine`, `TestEventsCollectorQuarantineRetryHasSharedRequestBudget`, `TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden`. | Source fixed; pending live rate-limit/backoff/recovery verification. The in-memory Pod-event queues remain bounded and are not restart-durable. |
| INGEST-03 | Under a bounded flush, an ownership-rejected quarantine prefix was retried first every time, starving a recovered event at the tail. The new reproduction failed after 33 retry turns before the fix. | Deferred quarantine records get the next turn before already-rejected records. `TestEventsCollectorQuarantineBudgetDoesNotStarveRecoveredEvents` verifies eventual delivery, request limits and retained evidence; informer-update regressions preserve the newest version. | Source fixed; full working-tree CI passed 2026-09-29. Live rollout still open with INGEST-02. |
| INSIGHT-01 | Historical RBAC evaluation generated insights without `cluster_id` and failed to restore an exact soft-deleted unique key; the 2026-09-27 log reported 153 duplicate-key errors per sync. | Persist cluster-qualified identity and restore only the exact key. `TestRuleInsightCarriesClusterIdentity`, `TestGenericInsightRestoresSoftDeletedRowWithinCluster`, `TestGenericInsightRestorePostgres`. | Source fix and single-node success recorded 2026-09-27 (`Insights=166, Errors=0`). Real two-cluster/concurrency acceptance remains F. Unowned historical rows must not be guessed or deleted. |
| POLICY-01 | Stock Pod policies referenced undeclared `object` and had incompatible evaluator semantics, preventing CEL compilation. | Corrected 1.0.1 templates use `resource`; forward repair preserves customized templates. `TestBaselinePodPoliciesCompileAndDetectUnsafeSpec`, `TestMigration151PreservesModifiedLegacyTemplate`, `TestBaselinePodPolicySeedAndRepairPostgres`. | Source fix and live compilation recorded 2026-09-27. No installed ValidatingWebhookConfiguration was observed, so admission denial remains unverified. |
| MIGRATION-01 | A populated PostgreSQL 15 rollout hit 10 risk-score key collisions between unowned legacy and cluster-owned rows. The later approved fresh-database reset removed that data shape. | Startup continues to fail closed; backup retained. A fresh database cannot prove collision reconciliation or populated migration safety. | Open F gate: rehearse against the populated shape and define an explicit evidence-preserving reconciliation policy. |
| CI-01 | A dirty/changed tree or inherited test-selection/database settings could make local results unsuitable for an exact-head claim. This is a validation limitation, not a reported production exploit. | Native runner reads workflow jobs, isolates PostgreSQL, clears inherited test selectors/live DB URL, hashes source/logs and permits publishable results only for clean unchanged all-job success. `test-local-ci-native.py` covers failure, source changes and publishability. | Tooling implemented; each new candidate commit requires a fresh clean all-job result. Hosted entitlement failure remains external to functional validation. |

All three ingest fixes retain Core ownership authorization and mark pending or
quarantined Falco evidence as failed coverage. They do not grant runtime authority
or enable absence-based auto-resolution. Permanent test mappings are in
[REGRESSION_PREVENTION.md](REGRESSION_PREVENTION.md); rollout configuration and
state retention are in
[DEPLOYMENT_CONTAINERD.md](../05-operations/DEPLOYMENT_CONTAINERD.md#preserve-falco-delivery-state).

## Earlier merged findings

| Finding | Change | Review |
| --- | --- | --- |
| Wrong RBAC role kind, namespace loss, duplicate/incorrect subject matches | Kind-aware resolution, scoped grants, standard SA subjects/groups, batch reads, explicit failures | [#29](https://github.com/shino-337/Fortuna-Community/pull/29) |
| Bulk operations falsely claimed Kubernetes disable/delete | Real UID-guarded deletion with prevalidation and per-item results; unsupported disable returns 501 without changes | [#30](https://github.com/shino-337/Fortuna-Community/pull/30) |
| Deletion could affect a replacement object and leave inconsistent audit | UID precondition, bounded requests, transactional inventory/audit and retry reconciliation | [#30](https://github.com/shino-337/Fortuna-Community/pull/30) |
| Deployment/ReplicaSet and capability list/aggregate scope gaps | Scope before reads/counts; aliases, pagination validation, UTC trend boundaries | [#31](https://github.com/shino-337/Fortuna-Community/pull/31) |
| Agent/node-name correlation could mix clusters | Explicit agent cluster identity, additive migration, atomic assignment and no node-name Ping fallback | [#32](https://github.com/shino-337/Fortuna-Community/pull/32) |
| Legacy graph traversal could expose other clusters | Scoped relational main graph; legacy unscoped operations restricted to unrestricted callers | [#33](https://github.com/shino-337/Fortuna-Community/pull/33) |
| Graph cache mutation/collision and wrong-cluster network enrichment | Copy path results; separate global keys; target-cluster service lookup and credential-aware cache | [#33](https://github.com/shino-337/Fortuna-Community/pull/33) |
| Risk evaluation failures could look like remediation | Resource-kind rule targeting, error propagation, configured evaluator, preserve disabled/missing detectors, transactional resolution audit | [#34](https://github.com/shino-337/Fortuna-Community/pull/34) |
| Identity UI confused errors with empty/missing data | Preserve API failures, display grant scope, retry and discard obsolete requests; browser regression fixtures | #29 and accompanying UI handoff PR |

## Historical merge and deployment order

PRs #29–34 branch from the main commit containing #28. Their source changes can be
reviewed separately. The UI handoff PR contains #29 as an ancestor: merge #29 first,
then review its remaining diff. Suggested order is #29, #30, #31, #32, #33, #34,
then UI handoff. The combined #29–34 source tree was checked for merge conflicts
locally; no GitHub pull request was merged automatically.

Before deployment, apply migration 150 and retain its additive column on rollback.
Allow successful HTTP inventory sync to assign old agents to clusters. Do not
infer missing agent ownership from a node name. Configure the YAML rule directory
and verify catalog initialization before running historical evaluation.

Behavior changes worth reviewing:

- Bulk delete now reaches Kubernetes and requires target kubeconfig plus both
  inventory.bulk and inventory.delete. HTTP 207 identifies partial failures.
- Disable and disable-inactive intentionally return 501. Inventory disappearance
  is not credential revocation. A real revocation workflow remains unimplemented.
- Legacy AGE HTTP routes and handlers were removed in #46. Scoped
  relational graph/attack-path views remain available; scoped AGE support remains
  unimplemented.
- Existing agents without cluster assignment are quarantined until authenticated
  ownership can be established. Scoped mTLS authorization and per-Agent
  certificate lifecycle/composite identity are merged through #48; the real
  multi-cluster credential lifecycle remains an F acceptance gate.
- Risk reconciliation stops on invalid catalogs/evaluation errors. Missing or
  disabled detectors preserve findings; operators must inspect reported failures.

## Lab validation still required

1. Migrate a populated PostgreSQL database; verify legacy agents and rollback.
2. Use two clusters with repeated node, namespace and Service IP values; verify
   scoped accounts across Inventory, Agent, graph and network views.
3. Test Kubernetes deletion success, unavailable credentials, RBAC denial, UID
   replacement, and retry after database/audit persistence failure.
4. Validate live Service discovery and optional-name behavior during API failures.
5. Exercise risk reconciliation during concurrent ingestion/manual finding actions;
   verify audit records and incomplete-batch reporting.
6. Test actual login/navigation and the end-to-end demo flow in the lab. Browser
   fixtures mock API responses and do not replace this test.

The remaining backlog includes independent runtime source health, live
multi-cluster credential/topology validation, scoped AGE, revocation and load tests.
Source freshness/lifecycle and scoped credential implementations are merged but
do not establish completion of these live gates. The documents below record
specific behavior and remaining boundaries, rather than treating a mitigation as
completion of the underlying feature.

## Contracts

- [RBAC grants](FINDING_RUNTIME_CONTRACT.md)
- [ServiceAccount mutations](SERVICEACCOUNT_MUTATIONS.md)
- [Workload and capability scope](INVENTORY_SCOPE.md)
- [Agent identity and migration](AGENT_CLUSTER_IDENTITY.md)
- [Graph/runtime boundaries](GRAPH_RUNTIME_BOUNDARIES.md)
- [Risk reconciliation](RISK_RECONCILIATION.md)
