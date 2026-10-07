#!/usr/bin/env python3
"""Run named security regressions; fail on removal, renaming, skip or failure.

Run from any directory. Requires Go on PATH. CI uses this in addition to go test
./...; an empty -run selection must never silently satisfy this gate. Critical
subtests are listed explicitly so deleting one case cannot hide behind a passing
parent test.
"""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

REQUIRED = {
    "./internal/api/risk": ["TestRiskTrendsBoundedAggregationScopeAndCalendar"],
    "./pkg/mutations": ["TestMutationIdentityReplayAndDurability", "TestMutationBlocksReplacementAndChangedBindings", "TestDurableDeletionRetainsUIDPrecondition", "TestMutationDigestSurvivesJSONBRepresentation"],
    "./pkg/graph": ["TestAGECanonicalScopeAndIdentifiers", "TestAttackPathBuildFailsClosedOnSnapshotError", "TestAttackPathCacheIsolation", "TestAttackPathCacheNestedMutationAndInvalidEncoding"],
    "./internal/service": [
        "TestInventoryCollectionCommitAndFailure", "TestInventoryCollectionEmptyMissingReplayAndScope",
        "TestInventoryCollectionEmptyMissingReplayAndScope/empty",
        "TestInventoryCollectionEmptyMissingReplayAndScope/missing",
        "TestInventoryCollectionEmptyMissingReplayAndScope/null",
        "TestInventoryCollectionEmptyMissingReplayAndScope/count",
        "TestInventoryCollectionEmptyMissingReplayAndScope/duplicate",
        "TestInventoryCollectionEmptyMissingReplayAndScope/scope",
        "TestInventoryCollectionEmptyMissingReplayAndScope/stale",
        "TestInventoryCollectionEmptyMissingReplayAndScope/failed",
        "TestInventoryCollectionEmptyMissingReplayAndScope/replay-change",
        "TestInventoryCollectionEmptyMissingReplayAndScope/out-of-order",
        "TestInventoryCollectionEmptyMissingReplayAndScope/namespace-prune",
        "TestInventoryCollectionEmptyMissingReplayAndScope/legacy-invalidate",
        "TestInventoryCollectionRejectsCrossNamespaceRows",
        "TestInventoryCollectionRejectsCrossNamespaceRows/serviceAccounts",
        "TestInventoryCollectionRejectsCrossNamespaceRows/roles",
        "TestInventoryCollectionRejectsCrossNamespaceRows/roleBindings",
        "TestInventoryCollectionRejectsCrossNamespaceRows/deployments",
        "TestInventoryCollectionRejectsCrossNamespaceRows/replicasets",
        "TestInventoryCollectionRejectsCrossNamespaceRows/pods",
        "TestInventoryCollectionRejectsDuplicateUIDAcrossNamespacesInPayload",
        "TestInventoryCollectionRejectsPersistedCrossNamespaceUIDCollision",
        "TestInventoryCollectionRejectsPersistedCrossNamespaceUIDCollision/serviceaccount",
        "TestInventoryCollectionRejectsPersistedCrossNamespaceUIDCollision/role",
        "TestInventoryCollectionRejectsPersistedCrossNamespaceUIDCollision/rolebinding",
        "TestInventoryCollectionRejectsPersistedCrossNamespaceUIDCollision/pod",
        "TestInventoryCollectionRejectsPersistedCrossNamespaceUIDCollision/deployment",
        "TestInventoryCollectionRejectsPersistedCrossNamespaceUIDCollision/replicaset",
    ],
    "./pkg/sbom": ["TestPodImageScanRejectsForeignSBOM"],
    "./pkg/capability": ["TestPCEUpsertsUseClusterQualifiedPodIdentity"],
    "./internal/repository": ["TestSBOMRepositoryRejectsMissingOwnership", "TestSBOMWorkloadIdentitySeparatesContainers", "TestSBOMContentDoesNotReuseDifferentProvenance"],
    "./pkg/reconciler": ["TestSBOMReconcilePreservesActiveAndUnresolvedOwnership"],
    "./pkg/rep": [
        "TestRuntimeSourceRecordExactReplaySkipsDownstreamEffects",
        "TestRuntimeSameSecondIdenticalObservationsRemainDistinct",
    ],
    "./pkg/riskengine": [
        "TestAssetSecurityStateExplicitFreshRead",
        "TestGenericInsightRetainsAcknowledgedState",
        "TestGenericInsightRetainsAcknowledgedState/key-0",
        "TestGenericInsightRetainsAcknowledgedState/key-1",
        "TestResolutionDetectorDependencies",
        "TestPodInsightRestorePreservesClusterAndException",
        "TestRuleInsightCarriesClusterIdentity",
        "TestGenericInsightRestoresSoftDeletedRowWithinCluster",
        "TestMalwareMaintenanceSeparatesDuplicatePodUID",
        "TestRuntimeRescoreDebounceSeparatesClusters",
        "TestRuntimeEnrichmentUsesExplicitClusterForDuplicateUID",
        "TestRuntimeInputFailureReachesEvaluators",
        "TestRuntimeInputRejectsCorruptSnapshot",
        "TestRuntimeInputRejectsMalformedBindings",
        "TestClusterAdminBindingForPod",
        "TestClusterAdminBindingForPod/namespace-only",
        "TestClusterAdminBindingForPod/foreign",
        "TestClusterAdminBindingForPod/group",
        "TestClusterAdminBindingForPod/wrong-role-kind",
        "TestEvaluationReportsRuleFailure",
        "TestConfiguredCatalogRejectsPartialAndEmptyLoad",
        "TestCreateOrUpdateInsight_ExceptionPolicyDoesNotCrossCluster",
    ],
    "./pkg/worker": [
        "TestHistoricalRBACNormalizationHandlesNonResourceRulesAndRoleRefs",
        "TestHistoricalRBACNormalizationRejectsMalformedEvidence",
        "TestInsightStatusUpdater_PodPreservedWithoutCollectionCoverage",
        "TestResolutionEvidenceBoundaries",
        "TestResolutionEvidenceBoundaries/old-snapshot-fresh-observation",
        "TestResolutionEvidenceBoundaries/missing-receipt",
        "TestResolutionEvidenceBoundaries/failed-receipt",
        "TestResolutionEvidenceBoundaries/legacy-receipt",
        "TestResolutionEvidenceBoundaries/complete-empty-retained",
        "TestResolutionEvidenceBoundaries/snapshot-not-observed",
        "TestResolutionEvidenceBoundaries/fresh-static",
        "TestResolutionEvidenceBoundaries/fresh-clusterrole",
        "TestResolutionEvidenceBoundaries/foreign-duplicate",
        "TestResolutionEvidenceBoundaries/ambiguous",
        "TestResolutionEvidenceBoundaries/stale",
        "TestResolutionEvidenceBoundaries/future",
        "TestResolutionEvidenceBoundaries/missing",
        "TestResolutionEvidenceBoundaries/foreign-cluster",
        "TestResolutionEvidenceBoundaries/unknown-owner",
        "TestResolutionEvidenceBoundaries/malformed",
        "TestResolutionEvidenceBoundaries/null-rules",
        "TestResolutionEvidenceBoundaries/incomplete-rules",
        "TestResolutionEvidenceBoundaries/before-finding",
        "TestResolutionEvidenceBoundaries/kind-before-finding",
        "TestResolutionEvidenceBoundaries/missing-kind-time",
        "TestResolutionEvidenceBoundaries/runtime",
        "TestResolutionEvidenceBoundaries/collection-error",
        "TestResolutionPreservesConcurrentFindingUpdate",
        "TestResolutionPreservesConcurrentSnapshotChange",
        "TestReconciliationDoesNotEvaluateSameNameReplacement",
        "TestSBOMEventRejectsForeignOwnershipBeforeEffects",
        "TestMalwarePersistenceFailureIsReturned",
        "TestReconciliationPreservesFindingOnRuntimeInputFailure",
        "TestReconciliationAuditRollbackAndCatalogFailure",
        "TestReconciliationPreservesDisabledDetector",
    ],
    "./pkg/policy": [
        "TestPolicyWorker_ProcessViolationEvent_CreatesBaselineInsights",
    ],
    "./internal/api": [
        "TestFullSyncEvaluationCoalescesAndRetainsPendingPass",
        "TestMutationHTTPPermissionAndScope",
        "TestSourceHealthAuthorityReplayFailureAndRestart",
        "TestSourceHealthRejectsUntrustedEvidence",
        "TestSourceHealthRegistryRotationFailsClosed",
        "TestScopedInventoryCollectionHTTPContract",
        "TestScopedInventoryCollectionHTTPContract/empty",
        "TestScopedInventoryCollectionHTTPContract/failed",
        "TestScopedInventoryCollectionHTTPContract/invalid",
        "TestScopedInventoryCollectionHTTPContract/foreign",
        "TestScopedInventoryCollectionHTTPContract/legacy",
        "TestScopedInventoryCollectionHTTPContract/persistence-failure",
        "TestScopedInventoryCollectionHTTPContract/scoped-missing-collection",
        "TestRuntimeCoverageScopedContinuityAndReplay",
        "TestPostRuntimeEventsV2_RejectsProcessableEventWithoutSourceRecordID",
        "TestPostRuntimeEventsV2_ExactReplaySkipsDownstreamEffects",
        "TestAgentStatusMissingSchemaIsUnavailable",
        "TestAgentStatusUsesPersistedIdentityVersionAndHeartbeat",
        "TestSystemMetricsBackingQueryFailureIsUnavailable",
        "TestSystemMetricsCountsDuplicatePodUIDAcrossClustersSeparately",
        "TestDashboardStatsAffectedPodCountUsesActiveInventoryScope",
        "TestClusterInventoryIncludesActiveClusterWithoutPods",
        "TestDashboardStatsSeparatesDuplicatePodUIDAcrossClusters",
        "TestDashboardStatsSelectedClusterRespectsActiveInventory",
        "TestClusterInventoryUnavailableIsDistinctFromEmpty",
        "TestClusterInventoryDatabaseFailureIsRetryable",
        "TestResourceInventoryUnavailableIsDistinctFromEmpty",
        "TestDashboardStatsBackingQueryFailureIsUnavailable",
        "TestDashboardStatsMissingSchemaIsNonRetryable",
        "TestDashboardStatsCatalogFailureIsRetryable",
        "TestDashboardIntegrityClusterQualifiesPodAndSBOMCoverage",
        "TestClusterNodeSurfacesDoNotConvertMissingPodsTableToEmpty",
        "TestCapabilityDetailAndListShareUnavailableSemantics",
        "TestCapabilityCatalogFailureIsRetryable",
        "TestClusterStatsUsesPersistedAgentVersion",
        "TestWorkerMetricsQueryFailureIsUnavailable",
        "TestWorkerMetricsMissingCoreSchemaIsUnavailable",
        "TestDashboardIntegrityMissingCoreSchemaIsNonRetryable",
        "TestDashboardRuntimeHealthReadsPersistedTimestamps",
        "TestDashboardRuntimeHealthQueryFailureIsUnavailable",
        "TestDashboardCatalogHealthQueryFailureIsUnavailable",
        "TestClusterAgentsMissingSchemaIsUnavailable",
        "TestClusterAgentsMissingHeartbeatIsDisconnectedAndNull",
        "TestClusterNodeMissingRiskSchemaIsNonRetryable",
        "TestClusterSecuritySummaryIsClusterQualified",
        "TestClusterSecuritySummaryMissingCapabilitySchemaIsUnavailable",
        "TestPipelineHealthIsClusterScoped",
        "TestPipelineHealthQueryFailureIsUnavailable",
        "TestPodListRiskCountsSeparateDuplicateUIDAcrossClusters",
        "TestPodListRiskQueryFailureIsUnavailable",
        "TestPodListRiskScoreQueryFailureIsUnavailable",
        "TestFailedCoverageGapStartsAtLastAcceptedCoverageEnd",
        "TestRuntimeCoverageRejectsUnsafeWindows",
        "TestRuntimeCoverageRejectsUnsafeWindows/identity-required",
        "TestRuntimeCoverageRejectsUnsafeWindows/historical-window-accepted-but-stale",
        "TestRuntimeCoverageRejectsUnsafeWindows/producer-source-mismatch",
        "TestRuntimeCoverageRejectsUnsafeWindows/zero-duration",
        "TestRuntimeCoverageRejectsUnsafeWindows/complete-with-error",
        "TestRuntimeCoverageRejectsUnsafeWindows/overlap",
        "TestRuntimeCoverageRejectsUnsafeWindows/source-kind-rebind",
        "TestRuntimeCoverageRejectsUnsafeWindows/gap-restarts-continuity",
        "TestRuntimeProducerLifecycleRestartDisableAndLease",
        "TestRuntimeProducerStoppingManifestClosesAllLeases",
        "TestRuntimeProducerHeartbeatPersistsLeaseAndSilenceGaps",
        "TestNonAuthoritativeProducerMayReportCompleteButCannotProveAbsence",
        "TestRuntimeCoverageRejectsUnsafeWindows/lifecycle-required",
        "TestRuntimeCoverageRejectsUnsafeWindows/disabled-producer",
        "TestRuntimeCoverageRejectsUnsafeWindows/stale-lifecycle-lease",
        "TestRuntimeCoverageRejectsUnsafeWindows/stale-observation-recovers-with-fresh-lifecycle",
        "TestRuntimeCoverageRejectsUnsafeWindows/session-mismatch",
        "TestRuntimeIngestTriggersScopedRescore",
        "TestRuntimeIngestTriggersScopedRescore/v2",
        "TestServiceAccountRBACResolution",
        "TestPodRiskReportUsesResolvedRBACScope",
        "TestServiceAccountInventoryScope",
        "TestServiceAccountMutationsProtectIdentity",
        "TestServiceAccountDeleteFailuresPreserveInventory",
        "TestServiceAccountUpdateUsesSelectedClusterRow",
        "TestPodProcessesServeOnlyLatestSnapshot",
        "TestInventoryWorkloadCapabilityScope",
        "TestAgentClusterIdentityIsolation",
        "TestGraphAndRuntimeClusterAliases",
        "TestRetiredRoutesAreNotRegistered",
        "TestVerifyFortunaRouteSecurityContract_DefaultEngine",
        "TestNetworkServiceCacheSeparatesClustersAndCredentials",
        "TestAggregateCacheIsolation",
        "TestRuntimeScopeAndFindingActions",
        "TestBulkRequiresActionPermissionAndNonemptySelection",
        "TestRiskGovernanceAggregateScope",
        "TestRiskExceptionsMutationsRespectOwnership",
        "TestAgentRegisteredRoutesUseScopedIdentityWithoutLegacyFallback",
        "TestScopedSyncRejectsForeignClaimsBeforeDatabaseEffects",
        "TestScopedPodEvidenceOwnership",
        "TestScopedPodEvidenceOwnership/foreign-cluster-claim",
        "TestScopedPodEvidenceOwnership/foreign-pod",
        "TestScopedPodEvidenceOwnership/wrong-namespace",
        "TestScopedPodEventBatchValidatesWholeBatchBeforeHandler",
        "TestScopedPodOwnershipStorageFailureIsUnavailable",
        "TestScopedRuntimeOwnershipValidatesWholeBatch",
        "TestScopedRuntimeOwnershipSupportsLegacyPodUIDAliases",
        "TestScopedRuntimeOwnershipRejectsUnknownAndNamespaceMismatch",
        "TestScopedRuntimeOwnershipRejectsUnknownAndNamespaceMismatch/unknown-pod",
        "TestScopedRuntimeOwnershipRejectsUnknownAndNamespaceMismatch/wrong-namespace",
        "TestScopedRuntimeOwnershipStorageFailureIsUnavailable",
        "TestRuntimeRegisteredRoutesRequireScopedIdentityAndOwnership",
        "TestRuntimeRegisteredRoutesRequireScopedIdentityAndOwnership/_api_v2_runtime_events",
        "TestRuntimeRegisteredRoutesRejectMixedBatchBeforeEffects",
        "TestRuntimeRegisteredRoutesRejectMixedBatchBeforeEffects/_api_v2_runtime_events",
        "TestRuntimeRegisteredRoutesApplyRevocationAndRegistryFailureImmediately",
        "TestRuntimeRegisteredRoutesPreserveExplicitLegacyMode",
        "TestRuntimeEvidenceRoutesRequireScopedLifecycle",
        "TestRuntimeEvidenceRoutesRejectLegacyCompatibilityMode",
    ],
    "./internal/grpc": [
        "TestLegacyGRPCServerQuarantinesWrites",
        "TestGRPCRevocationWhileReceiveBlocked",
        "TestCombinedFindingRetiredWithoutDatabaseEffects",
        "TestSBOMIngestPreservesTrustedClusterIdentity",
        "TestGRPCAgentUnaryInterceptorAuthenticatesVerifiedCertificate",
        "TestGRPCAgentUnaryInterceptorRejectsUnverifiedRevokedAndUnavailableIdentity",
        "TestGRPCAgentStreamReauthenticatesEveryReceivedMessage",
        "TestNewServerRejectsScopedGRPCIdentityWithoutTLS",
        "TestScopedGRPCControlRPCAuthorizationAndClusterBinding",
        "TestScopedGRPCAgentRecordUnavailableFailsClosed",
        "TestScopedGRPCControlRPCSeparatesDuplicateAgentIDs",
        "TestScopedGRPCPodRPCRejectsForeignClaimsAndCanonicalizesCluster",
        "TestScopedGRPCCombinedFindingRequiresOneOwnedResource",
        "TestScopedGRPCCombinedFindingRequiresExactContainerAndDigest",
        "TestScopedGRPCBatchStreamRejectsForeignMessageBeforeHandler",
        "TestScopedGRPCOwnershipStorageFailureIsUnavailable",
        "TestScopedGRPCUnknownMethodFailsClosed",
    ],
    "./pkg/agentidentity": [
        "TestCredentialIdentityIsolation",
        "TestCredentialRotationRevocationAndExpiry",
        "TestCredentialRegistryFailsClosed",
        "TestCredentialRequiresVerifiedTLS",
    ],
    "./pkg/resourceidentity": [
        "TestIdentityRequiresClusterAndUID",
        "TestIdentitySeparatesDuplicateUIDAcrossClusters",
    ],
    "./pkg/models": [
        "TestRuntimeCoverageCoversIntervalRequiresBothBounds",
        "TestRuntimeProducerEffectiveStatusSeparatesActivityFromAuthority",
    ],
    "./migrations": [
        "TestBaselinePodPoliciesCompileAndDetectUnsafeSpec",
        "TestMigration151RepointsOnlyStockBrokenPolicyVersions",
        "TestMigration151RepointsOrphanedLegacyBaselineInstances",
        "TestMigration151PreservesModifiedLegacyTemplate",
        "TestClusterQualifiedPodUniquenessRejectsUnowned",
        "TestClusterResourceIdentityFoundationBackfillsOnlyUnambiguousOwnership",
        "TestClusterResourceIdentityFoundationFailsClosedWithoutPods",
        "TestClusterResourceIdentityFoundationFailsClosedOnMissingRequiredTarget",
        "TestClusterResourceIdentityFoundationFailsClosedOnMissingUIDColumn",
        "TestClusterResourceIdentityFoundationRejectsConflictingExistingOwnership",
    ],
}

POSTGRES_REQUIRED = {
    "./migrations": ["TestClusterResourceIdentityFoundationPostgres", "TestClusterQualifiedPodUniquenessPostgres", "TestAgentCompositeIdentityPostgres", "TestMigrationMetadataUsesCurrentSchemaPostgres", "TestRiskScoreOwnershipQuarantinePostgres", "TestBaselinePodPolicySeedAndRepairPostgres", "TestRuntimeEventIdempotencyPostgres", "TestRuntimeEventIdempotencyRejectsPreexistingDuplicateSourceRecordsPostgres"],
    "./pkg/riskengine": ["TestPodInsightLifecyclePostgres", "TestGenericInsightRestorePostgres", "TestGenericInsightRestorePostgres/key-0", "TestGenericInsightRestorePostgres/key-1"],
    "./internal/repository": ["TestSBOMConcurrentOwnershipPostgres"],
    "./internal/service": ["TestInventoryCollectionPostgres"],
    "./internal/api": ["TestSourceHealthPostgresConcurrencyAndRollback", "TestRuntimeCoveragePostgres", "TestRuntimeCoveragePostgresLegacySchemaUpgrade", "TestRuntimeCoveragePostgresLegacySchemaRejectsUnownedRows", "TestSBOMListClusterScopePostgres", "TestSBOMListDuplicateUIDClusterIsolationPostgres", "TestSBOMListFailClosedPostgres"],
    "./pkg/rep": ["TestRuntimeSourceRecordConcurrentDuplicatePostgres"],
    "./pkg/graph": ["TestAGEScopedTraversalPostgres"],
    "./internal/api/risk": ["TestRiskTrendsAggregationPostgres"],
}

API_REQUIRED = {
    "./collection": [
        "TestRuntimeProducerManifestRequiresCompleteFailClosedRegistry",
        "TestRuntimeCoverageRequiresExecutionSession",
    ],
}

AGENT_REQUIRED = {
    "./internal/config": [
        "TestRuntimePollingDurationsClampNonPositiveValues",
        "TestRuntimeCoverageCadenceIndependentFromPoll",
    ],
    "./cmd": [
        "TestRuntimeProducerDeclarationsAreCompleteAndFailClosed",
        "TestRuntimeProducerDeclarationsNeverInferAuthorityFromEnablement",
        "TestRegisterAgentUsesConfiguredAgentID",
    ],
    "./internal/syncer": ["TestInventoryCollectionAgentReportsEmptyAndFailure",
        "TestInventoryCollectionAgentReportsEmptyAndFailure/empty",
        "TestInventoryCollectionAgentReportsEmptyAndFailure/list-error",
        "TestInventoryCollectionAgentReportsEmptyAndFailure/pagination",
        "TestInventoryCollectionAgentReportsEmptyAndFailure/core-rejected",
        "TestInventoryCollectionRecordsListStartBeforeResponse",
    ],
    "./internal/client": ["TestClientCertificateRotationFailsClosed"],
    "./internal/corehttp": [
        "TestDeliveryBudgetBoundsIsolationAndRetainsUnsent",
        "TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden",
        "TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden/Too_Many_Requests",
        "TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden/Unauthorized",
        "TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden/Forbidden",
        "TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden/Service_Unavailable",
        "TestPostErrorHonorsRetryAfterAndCancelledDelivery",
        "TestAgentRoutePrefersScopedTokenFileOverLegacyToken",
        "TestConfiguredScopedTokenFileFailsClosedWithoutLegacyFallback",
        "TestInvalidScopedTokenFileFailsClosedWithoutLegacyFallback",
        "TestScopedTokenFailureBlocksBearerAndStaleHeaderFallback",
        "TestRuntimeRoutesUseScopedTokenAfterC2Cutover",
        "TestRuntimeRoutesFailClosedWhenScopedSourceUnavailable",
        "TestRuntimeRoutesUseLegacyTokenWhenScopedSourceNotConfigured",
        "TestScopedTokenFileIsRereadForRotation",
    ],
    "./internal/poddetail": [
        "TestEventsCollectorRateLimitStopsFlushAndPreservesQuarantine",
        "TestEventsCollectorQuarantineRetryHasSharedRequestBudget",
        "TestEventsCollectorQuarantineBudgetDoesNotStarveRecoveredEvents",
        "TestEventsCollectorResyncDuringDeliveryPreservesQueueClassAndNewestVersion",
        "TestEventsCollectorResyncDuringDeliveryPreservesQueueClassAndNewestVersion/403",
        "TestEventsCollectorResyncDuringDeliveryPreservesQueueClassAndNewestVersion/429",
        "TestEventsCollectorIsolatesOwnershipRejection",
        "TestEventsCollectorDoesNotSplitOtherForbiddenErrors",
        "TestEventsCollectorDeduplicatesResyncedQuarantine",
    ],
    "./internal/redact": [
        "TestCommandLine",
        "TestText",
    ],
    "./internal/runtime": [
        "TestSourceHealthRelayPreservesSignatureAndRejectsOldSession",
        "TestSourceHealthRelayBackoffStopsSiblingRequests",
        "TestFalcoDurableMixedBatchRestartAndRecovery",
        "TestFalcoDurableBackoffSurvivesRestartAndMissingSource",
        "TestFalcoDurableStateFailsClosedOnCorruptionBindingAndConcurrentWriter",
        "TestFalcoDurableIsolationBudgetAndCapacityKeepEvidence",
        "TestFalcoDurableRotationReplaysPendingBeforeReadingReplacement",
        "TestFalcoDurablePersistenceFailureDoesNotSendOrAdvanceCursor",
        "TestFalcoDurableLargeBacklogDrainsWithinCapacity",
        "TestRuntimeSendersNeverDowngrade",
        "TestRuntimeSendersNeverDowngrade/Not_Found/file",
        "TestRuntimeSendersNeverDowngrade/Not_Found/falco",
        "TestRuntimeSendersNeverDowngrade/Unauthorized/file",
        "TestRuntimeSendersNeverDowngrade/Unauthorized/falco",
        "TestRuntimeSendersNeverDowngrade/Forbidden/file",
        "TestRuntimeSendersNeverDowngrade/Forbidden/falco",
        "TestRuntimeSendersNeverDowngrade/Internal_Server_Error/file",
        "TestRuntimeSendersNeverDowngrade/Internal_Server_Error/falco",
        "TestReaderSend_V2Success_EnrichesCanonicalFieldsAndMetrics",
        "TestRuntimeFileSourceRecordIdentitySurvivesRestartAndSeparatesIdenticalRecords",
        "TestRuntimeReaderStartPerformsImmediateRead",
        "TestRuntimeReaderRetainsOffsetUntilIngestSucceeds",
        "TestRuntimeReaderAdvancesPastInvalidOnlyInput",
        "TestFalcoReaderRetainsCursorAndPartialLineUntilIngestSucceeds",
        "TestFalcoPartialRecordSurvivesReaderRestart",
        "TestFalcoPodUIDResolutionBudgetIsBoundedByPoll",
        "TestCoverageReporterRetriesImmutablePayloadBeforeNewWindow",
        "TestCoverageReporterCoalescesCleanWindowsByCadence",
        "TestCoverageReporterFailureBypassesCadence",
        "TestCoverageReporterFlushForcesCleanBacklog",
        "TestCoverageReporterMarksLossAndErrorsFailed",
        "TestProducerLifecycleReporterRunningAndStopping",
        "TestProducerLifecycleStopSerializesAfterInflightHeartbeat",
        "TestRuntimeFileCoverageEmptyAndInvalid",
        "TestRuntimeFileCoverageEmptyAndInvalid/empty",
        "TestRuntimeFileCoverageEmptyAndInvalid/invalid",
        "TestRuntimeFileCoverageRetainsPartialRecord",
        "TestRuntimeFilePartialRecordSurvivesReaderRestart",
        "TestRuntimeFileCoverageDetectsFileReplacement",
        "TestFalcoCoverageRejectsUnresolvedEventAsDrop",
        "TestFalcoCoverageDetectsFileReplacementAndProcessesNewFile",
    ],
    "./internal/runtime/ebpf": [
        "TestSendBatchToCoreRuntimeEvents",
        "TestFlushLoopRetriesFailedBatchWithoutDroppingIt",
        "TestFlushLoopAccountsRetainedBatchOnShutdownFailure",
        "TestCoverageReportsPendingDeliveryAsFailed",
        "TestCoverageSnapshotDoesNotSplitInflightDelivery",
        "TestEBPFCoverageNeverClaimsCompleteWhileSensorIsNoop",
    ],
}


def validate_events(events, required):
    ran, passed = set(), set()
    problems = []
    for event in events:
        name, action = event.get("Test"), event.get("Action")
        if action == "run":
            ran.add(name)
        elif action == "pass":
            passed.add(name)
        elif action in ("skip", "fail"):
            problems.append(f"{action}: {name or event.get('Package', 'package')}")
    for name in required:
        if name not in ran or name not in passed:
            problems.append(f"required test did not run and pass: {name}")
    return problems


def run_pattern(names):
    # Go's -run matches slash-separated test/subtest components independently.
    # Prefixing the escaped name and allowing an optional descendant suffix runs
    # the required parent plus explicitly named subtests without silently
    # broadening to unrelated tests.
    roots = sorted({name.split("/", 1)[0] for name in names})
    return "^(" + "|".join(re.escape(name) for name in roots) + ")$"


def run_required(module_dir, module_name, required_by_package, errors):
    for package, names in required_by_package.items():
        result = subprocess.run(
            ["go", "test", "-json", "-count=1", "-timeout=2m", "-run", run_pattern(names), package],
            cwd=module_dir, capture_output=True, text=True, check=False,
        )
        events = []
        for line in result.stdout.splitlines():
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                errors.append(f"{module_name}:{package}: unexpected non-JSON test output")
        problems = validate_events(events, names)
        if result.returncode:
            problems.append(f"go test exited {result.returncode}")
        if problems:
            errors.extend(f"{module_name}:{package}: {p}" for p in problems)
            print(result.stderr, file=sys.stderr)
            for event in events:
                if event.get("Action") == "output":
                    print(event.get("Output", ""), end="", file=sys.stderr)
        else:
            print(f"PASS {module_name}:{package}: {len(names)} required regression tests/subtests")


def main():
    repo = Path(__file__).resolve().parents[2]
    errors = []
    if sys.argv[1:] == ["--postgres"]:
        if not os.environ.get("FORTUNA_TEST_POSTGRES_URL"):
            print("isolated FORTUNA_TEST_POSTGRES_URL required", file=sys.stderr)
            return 2
        run_required(repo / "core", "core", POSTGRES_REQUIRED, errors)
    elif not sys.argv[1:]:
        run_required(repo / "core", "core", REQUIRED, errors)
        run_required(repo / "api", "api", API_REQUIRED, errors)
        run_required(repo / "agent", "agent", AGENT_REQUIRED, errors)
    else:
        print("usage: check-security-regressions.py [--postgres]", file=sys.stderr)
        return 2
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
