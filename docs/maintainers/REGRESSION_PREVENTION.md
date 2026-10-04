# Preventing repeat findings

A remediation is complete only when its failure scenario has a regression test,
the production path uses the corrected shared implementation, and CI exercises
that path. Passing tests do not establish that unknown defects cannot exist.

## Required test contract

`scripts/verify/check-security-regressions.py` contains the named regression
contract. CI runs it in the Core job in addition to all package tests. The script
requires explicit run and pass events for every listed test. Missing/renamed tests,
package failures and skipped subtests fail the gate. Parser tests cover false
success and skipped-subtest cases. New findings must add a focused reproduction
to this contract where appropriate; changing a required case needs review.

The contract currently covers runtime input failure, reconciliation retention,
shared RBAC semantics, inventory/mutation scope, scoped HTTP/gRPC identity,
cluster-qualified storage, receipt lifecycle/replay, API availability, graph/cache
boundaries and bounded durable ingest delivery. Other existing tests
remain covered by the normal full suite; this contract is not complete evidence
of all future HTTP/gRPC isolation behavior.

## Review and merge controls

- Require functional CI (Core, Agent, API, PostgreSQL, Dashboard, scripts and
  hygiene), PR review and current-base validation. PR #54 has the owner-recorded
  exact-head local exception described in [the plan](NEXT_AUDIT_PLAN.md#historical-54-local-exact-head-verification)
  while hosted jobs fail before execution. Secret scanning is manual-only under
  the current repository plan/license and remains a separate check.
- Restrict bypass/direct pushes so failing checks cannot be merged routinely.
- Review changes to CI, the required test list and permission/scope helpers as
  changes to security controls.
- Preserve the finding-to-test mapping and document any remaining mitigation.
- Add PostgreSQL/Kubernetes integration coverage in package F; mocked UI tests and
  SQLite unit tests do not prove deployment behavior.

The current GitHub connection cannot read branch protection (403 Resource not
accessible by integration). Required-check enforcement and bypass settings must
be verified by the repository owner; their absence has not been established.

## Finding-to-test map

| Fix area | Required regression examples |
| --- | --- |
| #28–30 inventory scope and mutations | TestServiceAccountInventoryScope, TestServiceAccountMutationsProtectIdentity, TestServiceAccountBulkFailuresPreserveInventory |
| #29/#37 RBAC semantics | TestServiceAccountRBACResolution, TestClusterAdminBindingForPod, TestPodRiskReportUsesResolvedRBACScope |
| #31 workload/capability scope | TestInventoryWorkloadCapabilityScope |
| #32 agent association | TestAgentClusterIdentityIsolation |
| #33 graph/cache boundaries | TestLegacyGraphFailsBeforeGlobalQuery, TestNetworkServiceCacheSeparatesClustersAndCredentials |
| #34 evaluator/reconciliation failures | TestEvaluationReportsRuleFailure, TestConfiguredCatalogRejectsPartialAndEmptyLoad, TestReconciliationAuditRollbackAndCatalogFailure, TestReconciliationPreservesDisabledDetector |
| #36 runtime input errors | TestRuntimeInputFailureReachesEvaluators, TestRuntimeInputRejectsCorruptSnapshot, TestRuntimeInputRejectsMalformedBindings, TestReconciliationPreservesFindingOnRuntimeInputFailure |
| Earlier aggregate/action scope | TestAggregateCacheIsolation, TestRuntimeScopeAndFindingActions, TestBulkRequiresActionPermissionAndNonemptySelection |
| C1 credential foundation | TestCredentialIdentityIsolation, TestCredentialRotationRevocationAndExpiry, TestCredentialRegistryFailsClosed, TestCredentialRequiresVerifiedTLS |
| INGEST-01 durable Falco ownership isolation/replay | TestFalcoDurableMixedBatchRestartAndRecovery, TestFalcoDurableRotationReplaysPendingBeforeReadingReplacement, TestFalcoDurableBackoffSurvivesRestartAndMissingSource, TestFalcoDurablePersistenceFailureDoesNotSendOrAdvanceCursor, TestFalcoDurableStateFailsClosedOnCorruptionBindingAndConcurrentWriter, TestFalcoDurableIsolationBudgetAndCapacityKeepEvidence, TestFalcoDurableLargeBacklogDrainsWithinCapacity |
| INGEST-02 bounded Pod/Falco retry and backoff | TestDeliveryBudgetBoundsIsolationAndRetainsUnsent, TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden, TestPostErrorHonorsRetryAfterAndCancelledDelivery, TestEventsCollectorRateLimitStopsFlushAndPreservesQuarantine, TestEventsCollectorQuarantineRetryHasSharedRequestBudget |
| INGEST-03 quarantine fairness/informer updates | TestEventsCollectorQuarantineBudgetDoesNotStarveRecoveredEvents, TestEventsCollectorResyncDuringDeliveryPreservesQueueClassAndNewestVersion |
| INSIGHT-01 generated/restored insight ownership | TestRuleInsightCarriesClusterIdentity, TestGenericInsightRestoresSoftDeletedRowWithinCluster; PostgreSQL job: TestGenericInsightRestorePostgres |
| INSIGHT-02 generic manual acknowledgement | TestGenericInsightRetainsAcknowledgedState/key-0 and /key-1; PostgreSQL: TestGenericInsightRestorePostgres/key-0 and /key-1 |
| POLICY-01 stock Pod CEL repair | Full Core suite: TestBaselinePodPoliciesCompileAndDetectUnsafeSpec, TestMigration151PreservesModifiedLegacyTemplate; PostgreSQL job: TestBaselinePodPolicySeedAndRepairPostgres |
| D3 signed health | TestSourceHealthAuthorityReplayFailureAndRestart, TestSourceHealthRejectsUntrustedEvidence (see named contract for exact subtests); PostgreSQL: TestSourceHealthPostgresConcurrencyAndRollback |
| G scoped AGE | TestAGECanonicalScopeAndIdentifiers; PostgreSQL: TestAGEScopedTraversalPostgres |
| H reviewed durable mutations | TestMutationIdentityReplayAndDurability, TestMutationBlocksReplacementAndChangedBindings, TestDurableDeletionRetainsUIDPrecondition, TestMutationDigestSurvivesJSONBRepresentation |
| MIGRATION-01/02 populated ownership/schema | PostgreSQL: TestRiskScoreOwnershipQuarantinePostgres, TestMigrationMetadataUsesCurrentSchemaPostgres; isolated populated-backup rehearsal |
| GRAPH-01 unavailable snapshot | TestAttackPathBuildFailsClosedOnSnapshotError |
| PERF-01 scoped trend/cache allocation | TestRiskTrendsBoundedAggregationScopeAndCalendar, TestAttackPathCacheNestedMutationAndInvalidEncoding; PostgreSQL: TestRiskTrendsAggregationPostgres |
| F real topology | Mandatory TestTwoClusterDaemonSetLive receipt; scripts test-two-cluster-integration.py rejects missing/skipped/failed tests and incomplete/overstated evidence |
| CI-01 exact-head local evidence | Scripts job: test-local-ci-native.py verifies complete workflow groups/matrix, rejected unknown controls, sanitized inherited environment, failure/source-change reports and clean stable all-job publishability |

These are named coverage anchors, not a guarantee against deleting assertions or
introducing a different failure mode. Review remains required. C2/C3 registered
HTTP/gRPC isolation regressions are now required by the named gate; permanent
PostgreSQL selections additionally cover real database contracts. The real kind gate covers HTTP/DaemonSet/Kubernetes/backend investigation; mocked Dashboard tests do not establish a live browser walkthrough or mTLS/gRPC rollout.
