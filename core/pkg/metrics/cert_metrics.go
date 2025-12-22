package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Certificate expiry metrics
	CertExpiryTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ksam_cert_expiry_timestamp",
			Help: "Certificate expiry timestamp (Unix time)",
		},
	)

	CertDaysUntilExpiry = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ksam_cert_days_until_expiry",
			Help: "Days until certificate expires",
		},
	)

	CertExpiryWarningTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_cert_expiry_warning_total",
			Help: "Total certificate expiry warnings (<30 days)",
		},
	)

	CertExpiryCriticalTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_cert_expiry_critical_total",
			Help: "Total certificate expiry critical alerts (<7 days)",
		},
	)

	CertExpiredTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_cert_expired_total",
			Help: "Total times certificate has expired",
		},
	)

	// Certificate rotation metrics
	CertRotationTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_cert_rotation_total",
			Help: "Total certificate rotations",
		},
	)

	CertRotationFailureTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ksam_cert_rotation_failure_total",
			Help: "Total certificate rotation failures",
		},
	)

	CertRotationDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ksam_cert_rotation_duration_seconds",
			Help:    "Certificate rotation duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5},
		},
	)

	LastCertRotationTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ksam_last_cert_rotation_timestamp",
			Help: "Timestamp of last certificate rotation (Unix time)",
		},
	)
)

func init() {
	// Register certificate metrics
	prometheus.MustRegister(CertExpiryTime)
	prometheus.MustRegister(CertDaysUntilExpiry)
	prometheus.MustRegister(CertExpiryWarningTotal)
	prometheus.MustRegister(CertExpiryCriticalTotal)
	prometheus.MustRegister(CertExpiredTotal)
	prometheus.MustRegister(CertRotationTotal)
	prometheus.MustRegister(CertRotationFailureTotal)
	prometheus.MustRegister(CertRotationDuration)
	prometheus.MustRegister(LastCertRotationTime)
}



