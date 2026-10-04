package metrics

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AddrFromEnv returns the listen address for the Prometheus endpoint, or "" when
// FORTUNA_METRICS_ADDR is unset and the endpoint is disabled.
func AddrFromEnv() string {
	return strings.TrimSpace(os.Getenv("FORTUNA_METRICS_ADDR"))
}

// NewServer serves the default Prometheus registry at /metrics on its own
// listener, separate from the authenticated API, so it can be kept off the
// public Service and scraped only from inside the cluster.
func NewServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
