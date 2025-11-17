package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ksam_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Database metrics
	DatabaseQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_database_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "table"},
	)

	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ksam_database_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "table"},
	)

	// Agent metrics
	AgentSyncsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_agent_syncs_total",
			Help: "Total number of agent syncs",
		},
		[]string{"cluster_id", "status"},
	)

	AgentSyncDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ksam_agent_sync_duration_seconds",
			Help:    "Agent sync duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
		[]string{"cluster_id"},
	)

	// Resource metrics
	ServiceAccountsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ksam_serviceaccounts_total",
			Help: "Total number of service accounts",
		},
		[]string{"cluster_id", "namespace"},
	)

	ClustersTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ksam_clusters_total",
			Help: "Total number of connected clusters",
		},
	)

	// Cache metrics
	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksam_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type"},
	)
)

