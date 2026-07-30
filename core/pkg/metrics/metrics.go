package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Database connection pool metrics
	DBConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fortuna_db_connections_open",
			Help: "Number of open database connections",
		},
	)

	DBConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fortuna_db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	DBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fortuna_db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	DBConnectionsWaitCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_db_connections_wait_count_total",
			Help: "Total number of times a connection had to wait",
		},
	)

	DBConnectionsWaitDuration = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_db_connections_wait_duration_milliseconds_total",
			Help: "Total wait time for database connections in milliseconds",
		},
	)

	// Worker metrics
	WorkerMessagesProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_worker_messages_processed_total",
			Help: "Total number of messages processed by workers",
		},
		[]string{"worker_name", "status"}, // status: success, error
	)

	WorkerProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fortuna_worker_processing_duration_seconds",
			Help:    "Worker message processing duration in seconds",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"worker_name"},
	)

	WorkerErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_worker_errors_total",
			Help: "Total number of worker processing errors",
		},
		[]string{"worker_name", "error_type"}, // error_type: retryable, non_retryable, fatal
	)

	WorkerRetryCountTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_worker_retry_count_total",
			Help: "Total number of worker message retries",
		},
		[]string{"worker_name"},
	)

	WorkerQueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_worker_queue_depth",
			Help: "Current depth of worker message queues",
		},
		[]string{"worker_name"},
	)

	DLQMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_dlq_messages_total",
			Help: "Total number of messages sent to dead letter queue",
		},
		[]string{"worker_name"},
	)

	// DLQThresholdExceededTotal increments when the DLQ stream message count crosses
	// AlertThreshold. A cooldown is applied in the DLQ manager to avoid log/metric spam.
	DLQThresholdExceededTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_dlq_threshold_exceeded_total",
			Help: "Total number of DLQ threshold exceeded alerts",
		},
		[]string{"stream_name"},
	)

	// Backpressure metrics (for worker backpressure.go)
	WorkerBackpressureTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_worker_backpressure_total",
			Help: "Total number of times backpressure was applied",
		},
		[]string{"worker_name"},
	)

	WorkerConcurrentProcessing = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_worker_concurrent_processing",
			Help: "Current number of messages being processed concurrently",
		},
		[]string{"worker_name"},
	)

	WorkerBackpressureDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fortuna_worker_backpressure_duration_seconds",
			Help:    "Duration of backpressure events",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
		[]string{"worker_name"},
	)

	// SBOM metrics
	SBOMProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_sbom_processed_total",
			Help: "Total number of SBOMs processed",
		},
		[]string{"status"}, // status: success, error
	)

	// SBOMValidationTotal tracks SBOM ingestion contract validation outcomes (G1).
	// Labels: result = "accepted" | "rejected"
	SBOMValidationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_sbom_validation_total",
			Help: "Total number of SBOM ingestion contract validations",
		},
		[]string{"result"}, // result: accepted, rejected
	)

	// QueuePressureTotal tracks backpressure events per queue and policy (G2).
	// Labels: queue, policy (drop|retry|defer|block), result (dropped|retried|deferred|blocked)
	QueuePressureTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_queue_pressure_total",
			Help: "Total number of queue pressure events by queue, policy, and result",
		},
		[]string{"queue", "policy", "result"},
	)

	SBOMComponentsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_sbom_components_total",
			Help: "Total number of SBOM components extracted",
		},
	)

	// SBOM_CREATED failed primary publish after retries → DLQ; Core consumes for visibility (log + counter).
	SBOMCreatedDLQConsumedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_sbom_created_dlq_consumed_total",
			Help: "Messages acknowledged from fortuna.sbom.created.dlq (dead-letter pipeline)",
		},
	)

	// Approximate DLQ backlog: StreamInfo.State.Subjects[fortuna.sbom.created.dlq] on fortuna-events (polled).
	SBOMCreatedDLQStreamMessages = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fortuna_sbom_created_dlq_stream_messages",
			Help: "Messages in JetStream fortuna-events for subject fortuna.sbom.created.dlq (StreamInfo.State.Subjects; polled)",
		},
	)

	// Denominator for drift / ingest SLO (one increment per successful UpsertSBOMWithComponents commit).
	SBOMStoreUpsertCommitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_sbom_store_upsert_commits_total",
			Help: "Successful SBOM store upserts (commits); is_new=true for insert, false for update",
		},
		[]string{"is_new"},
	)

	// SBOM determinism / fingerprint drift metrics (Phase 1 - Determinism v1)
	SBOMDriftTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_sbom_drift_total",
			Help: "Total number of SBOM normalized_fingerprint drifts by type",
		},
		[]string{"drift_type", "resolver_version"}, // drift_type: expected_version_changed | unexpected_same_version
	)

	// SBOM coverage metrics (Phase 3)
	SBOMComponentWithVersionRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_sbom_component_with_version_ratio",
			Help: "Ratio of SBOM components with known version (computed at match time)",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)
	SBOMComponentWithEcosystemRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_sbom_component_with_ecosystem_ratio",
			Help: "Ratio of SBOM components with resolvable ecosystem (computed from PURL)",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)

	// Effective coverage: only consider components with component_confidence >= MEDIUM
	SBOMComponentEffectiveDenominatorTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_sbom_component_effective_denominator_total",
			Help: "Effective denominator count for coverage metrics: number of SBOM components with component_confidence >= MEDIUM",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)
	SBOMComponentWithVersionRatioEffective = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_sbom_component_with_version_ratio_effective",
			Help: "Effective ratio of SBOM components with known version among components with component_confidence >= MEDIUM",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)
	SBOMComponentWithEcosystemRatioEffective = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_sbom_component_with_ecosystem_ratio_effective",
			Help: "Effective ratio of SBOM components with resolvable ecosystem among components with component_confidence >= MEDIUM",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)

	SBOMComponentUnknownVersionRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_component_unknown_version_ratio",
			Help: "Ratio of SBOM components with component_version == unknown",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)

	// Unknown version root-cause breakdown (actionable counter).
	SBOMComponentUnknownVersionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_component_unknown_version_total",
			Help: "Total number of SBOM components with unknown version, broken down by reason",
		},
		[]string{"reason", "resolver_version"}, // reason: distroless|inferred|missing_metadata|other
	)

	// Inferred-component ratio (how much system is inferring vs explicit agent fields).
	SBOMComponentInferredRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_component_inferred_ratio",
			Help: "Ratio of SBOM components inferred by core (source_detail != agent-fields)",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)

	// CVE matching ratios (Phase 3)
	CVEMatchRatioRaw = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_cve_match_ratio_raw",
			Help: "Raw ratio of SBOM components that produced any CVE match (before worker severity filter)",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)
	CVEMatchRatioEffective = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_cve_match_ratio_effective",
			Help: "Effective ratio of SBOM components that produced CVE matches after worker severity filter",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)
	// Backward-compat: keep the old metric as an alias of effective ratio.
	CVEMatchRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_cve_match_ratio",
			Help: "Alias of fortuna_cve_match_ratio_effective (kept for backward compatibility)",
		},
		[]string{"sbom_status", "status_reason", "resolver_version"},
	)

	// Risk confidence distribution (Phase 3)
	RiskConfidenceDistributionRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_risk_confidence_distribution_ratio",
			Help: "Last-observed ratio of insights by final risk confidence, split by SBOM status and ecosystem (computed at insight creation)",
		},
		[]string{"level", "sbom_status", "status_reason", "ecosystem", "resolver_version"}, // level: HIGH|MEDIUM|LOW|VERY_LOW
	)

	// CVE matching metrics
	CVEMatchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_cve_matches_total",
			Help: "Total number of CVE matches found",
		},
		[]string{"severity"}, // severity: CRITICAL, HIGH, MEDIUM, LOW
	)

	// Component-level CVE coverage counters.
	// For each SBOM component, result=matched means it produced at least one CVE match (raw, before worker severity filter).
	CVEMatchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_cve_match_total",
			Help: "Total number of SBOM components with/without CVE matches (raw)",
		},
		[]string{"result", "confidence_level", "resolver_version"}, // result: matched|not_matched, confidence_level: HIGH|MEDIUM|LOW|NONE
	)

	// Best confidence per component:
	// increment once per SBOM component (raw matched), using the MAX match confidence level among its CVE matches.
	// This avoids the "min-confidence floor hides good signals" problem.
	CVEMatchBestConfidenceTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_cve_match_best_confidence_total",
			Help: "Total number of SBOM components with CVE matches by best (max) match confidence level (raw)",
		},
		[]string{"confidence_level", "resolver_version"}, // confidence_level: HIGH|MEDIUM|LOW
	)

	// Matcher trust/selection metrics (noise control)
	MatcherComponentsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_components_total",
			Help: "Total number of components considered by the matcher (after snapshot/DB load)",
		},
	)
	MatcherComponentsSkippedLowTrustTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_components_skipped_low_trust_total",
			Help: "Total number of components skipped due to low trust when non-low components exist",
		},
	)
	MatcherComponentsFallbackModeTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_components_fallback_mode_total",
			Help: "Total number of matcher invocations that ran in low-trust fallback mode (all components low trust)",
		},
	)
	MatcherComponentsFallbackLimitedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_components_fallback_limited_total",
			Help: "Total number of components dropped due to fallback-mode safety limit",
		},
	)

	// Last-observed ratios (operational signals). These are not per-SBOM to avoid high cardinality.
	MatcherFallbackRatio = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fortuna_matcher_fallback_ratio",
			Help: "Ratio of matcher invocations running in fallback mode (last observed value)",
		},
	)
	MatcherLowTrustRatio = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fortuna_matcher_low_trust_ratio",
			Help: "Ratio of low-trust components among candidates (last observed value)",
		},
	)

	// Time-aware (cumulative) signals for alerting on deltas / rates.
	MatcherInvocationsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_invocations_total",
			Help: "Total number of matcher invocations (including fallback mode)",
		},
	)
	MatcherFallbackInvocationsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_fallback_invocations_total",
			Help: "Total number of matcher invocations that ran in fallback mode",
		},
	)
	MatcherCandidatesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_candidate_components_total",
			Help: "Total number of candidate components considered by the resolver",
		},
	)
	MatcherCandidatesLowTrustTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_candidate_components_low_trust_total",
			Help: "Total number of low-trust candidate components considered by the resolver",
		},
	)

	MatcherVulnerabilityCandidatesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_vulnerability_candidates_total",
			Help: "Total number of vulnerability records returned for candidate packages before version filtering",
		},
		[]string{"ecosystem", "resolver_version"},
	)
	MatcherVulnerabilityVersionMatchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_vulnerability_version_matches_total",
			Help: "Total number of vulnerability candidates whose version constraint matched the installed component version",
		},
		[]string{"ecosystem", "resolver_version"},
	)
	MatcherVulnerabilitySkipsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_vulnerability_skips_total",
			Help: "Total number of vulnerability matching skips by reason",
		},
		[]string{"reason", "ecosystem", "resolver_version"}, // reason: no_candidate | version_compare_error | not_vulnerable | no_constraint | arch_mismatch | cap_reached | low_confidence_unknown_version
	)

	MatcherComponentsShadowedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_matcher_components_shadowed_total",
			Help: "Total number of components shadowed/dropped by the resolver, by reason",
		},
		[]string{"reason"}, // reason: priority | conflict | invalid | duplicate
	)

	// CVE matcher run-level metrics (idempotency + outcomes)
	CVEMatcherRunsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_cve_matcher_runs_total",
			Help: "Total number of CVE matcher runs by result and resolver_version",
		},
		[]string{"result", "resolver_version"}, // result: processed | skipped | duplicate | replay | error
	)

	CVEMatchingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "fortuna_cve_matching_duration_seconds",
			Help:    "CVE matching duration in seconds",
			Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 30, 60},
		},
	)

	// OSV mirror metrics (Go ecosystem SBOM matching)
	OSVMirrorQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_osv_mirror_queries_total",
			Help: "Total number of OSV mirror bulk queries (by ecosystem)",
		},
		[]string{"ecosystem"},
	)

	OSVMirrorCacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_osv_mirror_cache_hits_total",
			Help: "Total number of CVE cache hits for OSV mirror-backed lookups (by ecosystem)",
		},
		[]string{"ecosystem"},
	)

	// Insight metrics
	InsightsCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_insights_created_total",
			Help: "Total number of insights created",
		},
		[]string{"insight_type", "severity"},
	)

	InsightsActiveTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fortuna_insights_active_total",
			Help: "Total number of active insights",
		},
		[]string{"insight_type", "severity"},
	)

	// Risk/insights batch metrics (Phase 3)
	RiskEvaluationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "fortuna_risk_evaluation_duration_seconds",
			Help:    "Risk/insights batch evaluation duration in seconds",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
	)
	InsightsBatchSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "fortuna_insights_batch_size",
			Help:    "Number of insights in a single batch (create/update or list)",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		},
	)

	// Risk scoring metrics
	RiskScoresCalculatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_risk_scores_calculated_total",
			Help: "Total number of risk scores calculated",
		},
	)

	RiskScoreDistribution = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fortuna_risk_score_distribution",
			Help:    "Distribution of risk scores",
			Buckets: []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		},
		[]string{"resource_type"},
	)

	// EPSS (FIRST.org) enrichment — RISK-1
	EPSSLookupsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_epss_lookups_total",
			Help: "EPSS API lookups (excluding cache hits)",
		},
	)
	EPSSLookupErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_epss_lookup_errors_total",
			Help: "EPSS API lookup failures (HTTP/parse)",
		},
	)
	EPSSCacheHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_epss_cache_hits_total",
			Help: "EPSS lookups served from in-process cache",
		},
	)

	// CISA KEV catalog (RISK-1+)
	KEVRefreshTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_kev_catalog_refresh_total",
			Help: "Successful refreshes of the CISA KEV CVE set",
		},
	)
	KEVRefreshErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_kev_catalog_refresh_errors_total",
			Help: "Failed KEV feed downloads or parses",
		},
	)
	KEVCatalogSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fortuna_kev_catalog_cve_entries",
			Help: "Number of CVE IDs in the last successful KEV catalog load",
		},
	)
)
