package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Database connection pool metrics
	DBConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ksam_db_connections_open",
			Help: "Number of open database connections",
		},
	)

	DBConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ksam_db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	DBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ksam_db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	DBConnectionsWaitCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_db_connections_wait_count_total",
			Help: "Total number of times a connection had to wait",
		},
	)

	DBConnectionsWaitDuration = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_db_connections_wait_duration_milliseconds_total",
			Help: "Total wait time for database connections in milliseconds",
		},
	)

	// Worker metrics
	WorkerMessagesProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_worker_messages_processed_total",
			Help: "Total number of messages processed by workers",
		},
		[]string{"worker_name", "status"}, // status: success, error
	)

	WorkerProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ksam_worker_processing_duration_seconds",
			Help:    "Worker message processing duration in seconds",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"worker_name"},
	)

	WorkerErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_worker_errors_total",
			Help: "Total number of worker processing errors",
		},
		[]string{"worker_name", "error_type"}, // error_type: retryable, non_retryable, fatal
	)

	WorkerRetryCountTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_worker_retry_count_total",
			Help: "Total number of worker message retries",
		},
		[]string{"worker_name"},
	)

	WorkerQueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ksam_worker_queue_depth",
			Help: "Current depth of worker message queues",
		},
		[]string{"worker_name"},
	)

	DLQMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_dlq_messages_total",
			Help: "Total number of messages sent to dead letter queue",
		},
		[]string{"worker_name"},
	)

	// Backpressure metrics (for worker backpressure.go)
	WorkerBackpressureTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_worker_backpressure_total",
			Help: "Total number of times backpressure was applied",
		},
		[]string{"worker_name"},
	)

	WorkerConcurrentProcessing = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ksam_worker_concurrent_processing",
			Help: "Current number of messages being processed concurrently",
		},
		[]string{"worker_name"},
	)

	WorkerBackpressureDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ksam_worker_backpressure_duration_seconds",
			Help:    "Duration of backpressure events",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
		[]string{"worker_name"},
	)

	// SBOM metrics
	SBOMProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_sbom_processed_total",
			Help: "Total number of SBOMs processed",
		},
		[]string{"status"}, // status: success, error
	)

	SBOMComponentsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_sbom_components_total",
			Help: "Total number of SBOM components extracted",
		},
	)

	// CVE matching metrics
	CVEMatchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_cve_matches_total",
			Help: "Total number of CVE matches found",
		},
		[]string{"severity"}, // severity: CRITICAL, HIGH, MEDIUM, LOW
	)

	CVEMatchingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ksam_cve_matching_duration_seconds",
			Help:    "CVE matching duration in seconds",
			Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 30, 60},
		},
	)

	// Insight metrics
	InsightsCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_insights_created_total",
			Help: "Total number of insights created",
		},
		[]string{"insight_type", "severity"},
	)

	InsightsActiveTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ksam_insights_active_total",
			Help: "Total number of active insights",
		},
		[]string{"insight_type", "severity"},
	)

	// Risk scoring metrics
	RiskScoresCalculatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_risk_scores_calculated_total",
			Help: "Total number of risk scores calculated",
		},
	)

	RiskScoreDistribution = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ksam_risk_score_distribution",
			Help:    "Distribution of risk scores",
			Buckets: []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		},
		[]string{"resource_type"},
	)
)
