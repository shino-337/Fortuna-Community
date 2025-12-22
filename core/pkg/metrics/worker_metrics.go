package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Worker processing metrics
	WorkerMessagesProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_worker_messages_processed_total",
			Help: "Total number of messages processed by workers",
		},
		[]string{"worker_type", "status"}, // status: success, error, backpressure
	)

	WorkerProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ksam_worker_processing_duration_seconds",
			Help:    "Time taken to process a message",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"worker_type"},
	)

	WorkerQueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ksam_worker_queue_depth",
			Help: "Current queue depth for workers",
		},
		[]string{"worker_type"},
	)

	WorkerErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_worker_errors_total",
			Help: "Total number of errors in workers",
		},
		[]string{"worker_type", "error_type"}, // error_type: retryable, non_retryable, fatal
	)

	WorkerConcurrentProcessing = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ksam_worker_concurrent_processing",
			Help: "Current number of messages being processed concurrently",
		},
		[]string{"worker_type"},
	)

	// Backpressure metrics
	WorkerBackpressureTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_worker_backpressure_total",
			Help: "Total number of times backpressure was applied (messages NAKed due to overload)",
		},
		[]string{"worker_type"},
	)

	WorkerBackpressureDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ksam_worker_backpressure_duration_seconds",
			Help:    "Duration of backpressure events (time until load decreases)",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
		[]string{"worker_type"},
	)

	WorkerRetryCountTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_worker_retry_count_total",
			Help: "Total number of retry attempts",
		},
		[]string{"worker_type"},
	)

	DLQMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_dlq_messages_total",
			Help: "Total number of messages sent to Dead Letter Queue",
		},
		[]string{"worker_type"},
	)
)

func init() {
	// Register worker metrics
	prometheus.MustRegister(WorkerMessagesProcessedTotal)
	prometheus.MustRegister(WorkerProcessingDuration)
	prometheus.MustRegister(WorkerQueueDepth)
	prometheus.MustRegister(WorkerErrorsTotal)
	prometheus.MustRegister(WorkerConcurrentProcessing)
	prometheus.MustRegister(WorkerBackpressureTotal)
	prometheus.MustRegister(WorkerBackpressureDuration)
	prometheus.MustRegister(WorkerRetryCountTotal)
	prometheus.MustRegister(DLQMessagesTotal)
}

