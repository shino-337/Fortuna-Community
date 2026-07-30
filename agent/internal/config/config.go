package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	// Agent identification
	AgentID     string
	NodeID      string
	NodeName    string
	ClusterID   string
	ClusterName string // Display name for dashboard (from kubeconfig or CLUSTER_NAME env; falls back to ClusterID)

	// Core gRPC endpoint
	CoreGRPCEndpoint string
	// Core HTTP endpoint (for full sync)
	CoreHTTPEndpoint string

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

	// Heartbeat interval (Ping to Core); shorter = last_seen_at updated more often for dashboard
	HeartbeatInterval time.Duration

	// Kubeconfig path (optional, uses in-cluster config if empty)
	Kubeconfig string

	// Namespace to watch (empty = all namespaces)
	WatchNamespace string

	// Runtime event ingestion (optional)
	RuntimeEventsEnabled bool
	RuntimeEventsPath    string
	RuntimeEventsPoll    time.Duration

	// Falco JSON output ingestion (optional) (R9 practical source)
	FalcoEventsEnabled bool
	FalcoEventsPath    string
	FalcoEventsPoll    time.Duration

	// eBPF runtime telemetry (R9)
	EBPFEnabled            bool
	EBPFMode               string
	EBPFEventFlushInterval time.Duration
	EBPFEventBufferSize    int
	EBPFSimulate           bool
}

func LoadConfig() *Config {
	// Get node info from environment (set by Kubernetes downward API)
	nodeName := getEnv("NODE_NAME", "unknown-node")
	nodeID := getEnv("NODE_IP", nodeName) // Use IP as node ID if available
	agentID := getEnv("AGENT_ID", nodeName+"-agent")

	cfg := &Config{
		AgentID:                agentID,
		NodeID:                 nodeID,
		NodeName:               nodeName,
		ClusterID:              getEnv("CLUSTER_ID", ""),   // From env or kubeconfig in main; no hardcoded default
		ClusterName:            getEnv("CLUSTER_NAME", ""), // Optional; main uses kubeconfig or CLUSTER_ID when set
		CoreGRPCEndpoint:       getEnv("CORE_GRPC_ENDPOINT", "fortuna-core.fortuna.svc.cluster.local:9090"),
		CoreHTTPEndpoint:       getEnv("CORE_HTTP_ENDPOINT", "http://fortuna-core.fortuna.svc.cluster.local:8080"),
		TLSEnabled:             getEnv("TLS_ENABLED", "true") == "true",
		TLSCertPath:            getEnv("TLS_CERT_PATH", "/etc/fortuna/tls/client/tls.crt"),
		TLSKeyPath:             getEnv("TLS_KEY_PATH", "/etc/fortuna/tls/client/tls.key"),
		TLSCACertPath:          getEnv("TLS_CA_CERT_PATH", "/etc/fortuna/tls/client/ca.crt"),
		BatchSize:              parseInt(getEnv("BATCH_SIZE", "50")),
		BatchTimeoutMS:         parseInt(getEnv("BATCH_TIMEOUT_MS", "5000")),
		SyncInterval:           parseDuration(getEnv("SYNC_INTERVAL", "30s")),
		HeartbeatInterval:      parseDuration(getEnv("HEARTBEAT_INTERVAL", "15s")),
		Kubeconfig:             getEnv("KUBECONFIG", ""),
		WatchNamespace:         getEnv("WATCH_NAMESPACE", ""),
		RuntimeEventsEnabled:   getEnv("RUNTIME_EVENTS_ENABLED", "false") == "true",
		RuntimeEventsPath:      getEnv("RUNTIME_EVENTS_PATH", "/var/log/fortuna/runtime-events.log"),
		RuntimeEventsPoll:      parseDuration(getEnv("RUNTIME_EVENTS_POLL", "5s")),
		FalcoEventsEnabled:     getEnv("FALCO_EVENTS_ENABLED", "false") == "true",
		FalcoEventsPath:        getEnv("FALCO_EVENTS_PATH", "/var/log/falco/events.jsonl"),
		FalcoEventsPoll:        parseDuration(getEnv("FALCO_EVENTS_POLL", "5s")),
		EBPFEnabled:            getEnv("EBPF_ENABLED", "false") == "true",
		EBPFMode:               getEnv("EBPF_MODE", "exec"),
		EBPFEventFlushInterval: parseDuration(getEnv("EBPF_EVENT_FLUSH_INTERVAL", "5s")),
		EBPFEventBufferSize:    parseInt(getEnv("EBPF_EVENT_BUFFER_SIZE", "200")),
		EBPFSimulate:           getEnv("EBPF_SIMULATE", "false") == "true",
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
