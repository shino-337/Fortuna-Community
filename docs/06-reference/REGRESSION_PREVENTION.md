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
shared RBAC semantics, inventory/mutation scope, agent association, graph/cache
boundaries and the agent credential foundation. Other existing tests
remain covered by the normal full suite; this contract is not complete evidence
of all future HTTP/gRPC isolation behavior.

## Review and merge controls

- Require the CI checks (Core, Agent, API, Dashboard, scripts and hygiene) and
  Secret scan on main; require PR review and current-base validation.
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

These are named coverage anchors, not a guarantee against deleting assertions or
introducing a different failure mode. Review remains required. Endpoint isolation
coverage will be added when C2/C3 wire the credential library into real routes.
