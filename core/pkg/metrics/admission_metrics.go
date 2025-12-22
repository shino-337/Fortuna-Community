package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Admission webhook metrics
// Phase 2.7: Metrics for monitoring admission webhook performance

var (
	// AdmissionLatencyMs measures admission webhook latency in milliseconds
	AdmissionLatencyMs = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "admission_latency_ms",
			Help:    "Admission webhook latency in milliseconds",
			Buckets: []float64{10, 25, 50, 100, 200, 500, 1000}, // Buckets for <100ms target
		},
		[]string{"operation"}, // operation: validate, mutate
	)

	// AdmissionDeniedCount counts denied admission requests
	AdmissionDeniedCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "admission_denied_count",
			Help: "Total number of denied admission requests",
		},
		[]string{"reason"}, // reason: policy_violation, parse_error, etc.
	)

	// AdmissionAllowedCount counts allowed admission requests
	AdmissionAllowedCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "admission_allowed_count",
			Help: "Total number of allowed admission requests",
		},
		[]string{"resource_type"}, // resource_type: Pod, Deployment, etc.
	)

	// CELEvaluationMs measures CEL evaluation time in milliseconds
	CELEvaluationMs = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cel_eval_ms",
			Help:    "CEL evaluation time in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100}, // CEL should be <50ms
		},
		[]string{"template_id"}, // template_id: which policy template
	)

	// EventPublishFailCount counts failed event publishes
	EventPublishFailCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "event_publish_fail_count",
			Help: "Total number of failed event publishes to NATS",
		},
		[]string{"event_type"}, // event_type: violation_detected, remediation_applied
	)

	// EventPublishSuccessCount counts successful event publishes
	EventPublishSuccessCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "event_publish_success_count",
			Help: "Total number of successful event publishes to NATS",
		},
		[]string{"event_type"},
	)

	// AdmissionErrorsTotal counts admission webhook errors
	AdmissionErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "admission_errors_total",
			Help: "Total number of admission webhook errors",
		},
		[]string{"error_type"}, // error_type: parse_error, eval_error, timeout
	)
)

func init() {
	// Admission metrics are auto-registered via promauto
	// No explicit registration needed
}

