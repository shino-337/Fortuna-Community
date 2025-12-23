package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	// Agent identification
	AgentID  string
	NodeID   string
	NodeName string

	// Core gRPC endpoint
	CoreGRPCEndpoint string

	// TLS/mTLS Configuration
	TLSEnabled    bool
	TLSCertPath   string
	TLSKeyPath    string
	TLSCACertPath string

	// Batch configuration
	BatchSize      int
	BatchTimeoutMS int

	// Sync interval
	SyncInterval time.Duration

	// Kubeconfig path (optional, uses in-cluster config if empty)
	Kubeconfig string

	// Namespace to watch (empty = all namespaces)
	WatchNamespace string
}

func LoadConfig() *Config {
	// Get node info from environment (set by Kubernetes downward API)
	nodeName := getEnv("NODE_NAME", "unknown-node")
	nodeID := getEnv("NODE_IP", nodeName) // Use IP as node ID if available
	agentID := getEnv("AGENT_ID", nodeName+"-agent")

	cfg := &Config{
		AgentID:          agentID,
		NodeID:           nodeID,
		NodeName:         nodeName,
		CoreGRPCEndpoint: getEnv("CORE_GRPC_ENDPOINT", "fortuna-core.fortuna.svc.cluster.local:9090"),
		TLSEnabled:       getEnv("TLS_ENABLED", "true") == "true",
		TLSCertPath:      getEnv("TLS_CERT_PATH", "/etc/fortuna/tls/client/tls.crt"),
		TLSKeyPath:       getEnv("TLS_KEY_PATH", "/etc/fortuna/tls/client/tls.key"),
		TLSCACertPath:    getEnv("TLS_CA_CERT_PATH", "/etc/fortuna/tls/client/ca.crt"),
		BatchSize:        parseInt(getEnv("BATCH_SIZE", "50")),
		BatchTimeoutMS:   parseInt(getEnv("BATCH_TIMEOUT_MS", "5000")),
		SyncInterval:     parseDuration(getEnv("SYNC_INTERVAL", "30s")),
		Kubeconfig:       getEnv("KUBECONFIG", ""),
		WatchNamespace:   getEnv("WATCH_NAMESPACE", ""),
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	var result int
	_, _ = fmt.Sscanf(s, "%d", &result)
	if result == 0 {
		result = 50 // default
	}
	return result
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second
	}
	return d
}
