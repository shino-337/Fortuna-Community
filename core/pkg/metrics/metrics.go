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

	SBOMComponentsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_sbom_components_total",
			Help: "Total number of SBOM components extracted",
		},
	)

	// CVE matching metrics
	CVEMatchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_cve_matches_total",
			Help: "Total number of CVE matches found",
		},
		[]string{"severity"}, // severity: CRITICAL, HIGH, MEDIUM, LOW
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

	// CVE matcher run-level metrics (idempotency + outcomes)
	CVEMatcherRunsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fortuna_cve_matcher_runs_total",
			Help: "Total number of CVE matcher runs by result",
		},
		[]string{"result"}, // result: processed | skipped | duplicate | error
	)

	CVEMatchingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "fortuna_cve_matching_duration_seconds",
			Help:    "CVE matching duration in seconds",
			Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 30, 60},
		},
	)

	// NVD API calls (bottleneck observability; FORTUNA_CVE_MATCHING_ENGINE §13, P1-1)
	NVDQueriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fortuna_nvd_queries_total",
			Help: "Total number of NVD API queries (fallback for heuristic SBOM)",
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
)
