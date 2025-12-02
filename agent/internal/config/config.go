package config

import (
	"os"
	"time"
)

type Config struct {
	// Core Controller endpoint
	CoreEndpoint string

	// Cluster identifier
	ClusterID string

	// Authentication token
	AuthToken string

	// Sync interval
	SyncInterval time.Duration

	// Kubeconfig path (optional, uses in-cluster config if empty)
	Kubeconfig string

	// Namespace to watch (empty = all namespaces)
	WatchNamespace string

	// TLS/mTLS Configuration
	TLSEnabled      bool
	TLSCACertPath   string
	TLSCertPath     string
	TLSKeyPath      string
}

func Load(configPath string) (*Config, error) {
	cfg := &Config{
		CoreEndpoint:   getEnv("KSAM_CORE_ENDPOINT", "core.ksam.svc.cluster.local:9090"),
		ClusterID:      getEnv("KSAM_CLUSTER_ID", ""),
		AuthToken:      getEnv("KSAM_AUTH_TOKEN", ""),
		SyncInterval:   parseDuration(getEnv("KSAM_SYNC_INTERVAL", "30s")),
		Kubeconfig:     getEnv("KSAM_KUBECONFIG", ""),
		WatchNamespace: getEnv("KSAM_WATCH_NAMESPACE", ""),
		TLSEnabled:    getEnv("TLS_ENABLED", "false") == "true",
		TLSCACertPath: getEnv("TLS_CA_CERT_PATH", "/etc/ksam/certs/ca.crt"),
		TLSCertPath:   getEnv("TLS_CERT_PATH", "/etc/ksam/certs/tls.crt"),
		TLSKeyPath:    getEnv("TLS_KEY_PATH", "/etc/ksam/certs/tls.key"),
	}

	if cfg.ClusterID == "" {
		// Try to get from node name or generate
		cfg.ClusterID = getEnv("NODE_NAME", "default-cluster")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

