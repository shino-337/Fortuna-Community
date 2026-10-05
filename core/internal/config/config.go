package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	// Database
	DatabaseURL string

	// Server ports
	GRPCPort     string
	HTTPPort     string
	NATSEndpoint string

	// Logging
	LogLevel string

	// Auth
	AuthEnabled          bool
	JWTSecret            string
	TokenExpirationHours int
	// IngestToken is the legacy shared HTTP ingest credential. It is used only when
	// AgentCredentialRegistryPath is empty. Once a registry is configured, HTTP
	// agent and runtime ingest must authenticate through scoped agent credentials
	// instead of silently falling back to this shared token.
	IngestToken string
	// AgentCredentialRegistryPath points to the operator-managed JSON registry used
	// by scoped per-agent/per-cluster HTTP authentication. Empty preserves the
	// legacy shared-token mode during migration.
	AgentCredentialRegistryPath     string
	RuntimeSourceHealthRegistryPath string
	// GRPCAgentCredentialRegistryPath independently enables scoped gRPC agent
	// authentication. It uses the same registry schema and trusted Principal as
	// HTTP, but is a separate migration switch because HTTP token credentials and
	// gRPC client-certificate credentials are provisioned at different times.
	GRPCAgentCredentialRegistryPath string

	// TLS/mTLS Configuration for gRPC
	TLSEnabled    bool
	TLSCACertPath string
	TLSCertPath   string
	TLSKeyPath    string

	// Webhook TLS Configuration (separate from gRPC)
	WebhookTLSCertPath string
	WebhookTLSKeyPath  string

	// PCE Scheduler
	PCESchedulerEnabled  bool
	PCESchedulerInterval time.Duration

	// Pod Detail encryption at rest for process command, binary path and working dir.
	// Base64 of 32 bytes. Empty disables encryption (Core logs a warning); an invalid
	// value stops Core from starting. Previous keys (comma-separated) still decrypt
	// rows written before a rotation.
	PodDetailEncryptionKey          string
	PodDetailEncryptionPreviousKeys string

	// Per-cluster rate limit (Finding #6). When enabled, sync and SBOM ingest are limited per cluster_id.
	RateLimitPerClusterEnabled   bool
	RateLimitSyncPerClusterRPS   float64
	RateLimitSyncPerClusterBurst int
	RateLimitSBOMPerClusterRPS   float64
	RateLimitSBOMPerClusterBurst int
}

func Load(configPath string) (*Config, error) {
	// Use fmt directly to ensure logs appear
	fmt.Fprintf(os.Stdout, "[Config] Load() called - starting config loading...\n")

	tlsEnabledStr := getEnv("TLS_ENABLED", "false")
	tlsEnabled := tlsEnabledStr == "true"

	fmt.Fprintf(os.Stdout, "[Config] TLS_ENABLED env='%s', parsed=%v\n", tlsEnabledStr, tlsEnabled)

	cfg := &Config{
		DatabaseURL:                     getEnv("DATABASE_URL", "postgres://postgres:postgres@postgres:5432/fortuna?sslmode=disable"),
		GRPCPort:                        getEnv("GRPC_PORT", "9090"),
		HTTPPort:                        getEnv("HTTP_PORT", "8080"),
		NATSEndpoint:                    getEnv("NATS_ENDPOINT", "nats://nats.fortuna.svc.cluster.local:4222"),
		LogLevel:                        getEnv("LOG_LEVEL", "info"),
		AuthEnabled:                     getEnv("AUTH_ENABLED", "true") == "true",
		JWTSecret:                       jwtSecretFromEnv(),
		TokenExpirationHours:            parseInt(getEnv("TOKEN_EXPIRATION_HOURS", "24")),
		IngestToken:                     strings.TrimSpace(getEnv("FORTUNA_INGEST_TOKEN", "")),
		AgentCredentialRegistryPath:     strings.TrimSpace(getEnv("FORTUNA_AGENT_CREDENTIAL_REGISTRY", "")),
		RuntimeSourceHealthRegistryPath: strings.TrimSpace(getEnv("FORTUNA_SOURCE_HEALTH_REGISTRY", "")),
		GRPCAgentCredentialRegistryPath: strings.TrimSpace(getEnv("FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY", "")),
		TLSEnabled:                      tlsEnabled,
		TLSCACertPath:                   getEnv("TLS_CA_CERT_PATH", "/etc/fortuna/tls/server/ca.crt"),
		TLSCertPath:                     getEnv("TLS_CERT_PATH", "/etc/fortuna/tls/server/tls.crt"),
		TLSKeyPath:                      getEnv("TLS_KEY_PATH", "/etc/fortuna/tls/server/tls.key"),
		WebhookTLSCertPath:              getEnv("WEBHOOK_TLS_CERT_PATH", "/etc/webhook/certs/tls.crt"),
		WebhookTLSKeyPath:               getEnv("WEBHOOK_TLS_KEY_PATH", "/etc/webhook/certs/tls.key"),
		PCESchedulerEnabled:             getEnv("PCE_SCHEDULER_ENABLED", "true") == "true",
		PCESchedulerInterval:            parseDuration(getEnv("PCE_SCHEDULER_INTERVAL", "6h")),
		PodDetailEncryptionKey:          getEnv("POD_DETAIL_ENCRYPTION_KEY", ""),
		PodDetailEncryptionPreviousKeys: getEnv("POD_DETAIL_ENCRYPTION_KEY_PREVIOUS", ""),
		RateLimitPerClusterEnabled:      getEnv("RATE_LIMIT_PER_CLUSTER_ENABLED", "true") == "true",
		RateLimitSyncPerClusterRPS:      parseFloat(getEnv("RATE_LIMIT_SYNC_PER_CLUSTER_RPS", "10"), 10),
		RateLimitSyncPerClusterBurst:    parseIntEnv(getEnv("RATE_LIMIT_SYNC_PER_CLUSTER_BURST", "20"), 20),
		RateLimitSBOMPerClusterRPS:      parseFloat(getEnv("RATE_LIMIT_SBOM_PER_CLUSTER_RPS", "50"), 50),
		RateLimitSBOMPerClusterBurst:    parseIntEnv(getEnv("RATE_LIMIT_SBOM_PER_CLUSTER_BURST", "100"), 100),
	}

	log.Printf("[Config] Final config: TLSEnabled=%v, TLSCertPath=%s, TLSCACertPath=%s",
		cfg.TLSEnabled, cfg.TLSCertPath, cfg.TLSCACertPath)
	devMode := envEnabled("FORTUNA_DEV_MODE")
	if !cfg.AuthEnabled && !devMode {
		// AUTH_ENABLED=false grants every caller a synthetic admin and opens
		// /api/v1/auth/register; never allow that by accident.
		return nil, fmt.Errorf("AUTH_ENABLED=false requires FORTUNA_DEV_MODE=1 (local development only)")
	}
	if cfg.JWTSecret == "" {
		if !devMode {
			return nil, fmt.Errorf("JWT_SECRET or FORTUNA_JWT_SECRET must be set; set FORTUNA_DEV_MODE=1 only for local development")
		}
		secret, err := randomDevSecret()
		if err != nil {
			return nil, fmt.Errorf("generate development jwt secret: %w", err)
		}
		cfg.JWTSecret = secret
		log.Printf("[Config] ⚠️  FORTUNA_DEV_MODE=1: generated ephemeral JWT secret; sessions expire on restart")
	} else if len(cfg.JWTSecret) < 32 && !devMode {
		return nil, fmt.Errorf("JWT_SECRET or FORTUNA_JWT_SECRET must be at least 32 bytes; current length %d", len(cfg.JWTSecret))
	}
	if _, err := ParsePodDetailEncryptionKeys(cfg.PodDetailEncryptionKey, cfg.PodDetailEncryptionPreviousKeys); err != nil {
		return nil, err
	}
	if cfg.AgentCredentialRegistryPath != "" {
		log.Printf("[Config] FORTUNA_AGENT_CREDENTIAL_REGISTRY is set: HTTP agent/runtime ingest uses scoped agent credentials; shared-token fallback is disabled for scoped ingest routes")
	} else if cfg.IngestToken != "" {
		log.Printf("[Config] FORTUNA_INGEST_TOKEN is set: HTTP agent/runtime ingest routes require ingest token")
	} else if !envEnabled("FORTUNA_ALLOW_UNAUTHED_INGEST") {
		log.Printf("[Config] FORTUNA_INGEST_TOKEN is not set: HTTP agent/runtime ingest routes will fail closed")
	}
	if cfg.GRPCAgentCredentialRegistryPath != "" {
		log.Printf("[Config] FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY is set: gRPC AgentService requires scoped client-certificate identity")
	}

	return cfg, nil
}

func jwtSecretFromEnv() string {
	if v := strings.TrimSpace(os.Getenv("JWT_SECRET")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("FORTUNA_JWT_SECRET"))
}

// ParsePodDetailEncryptionKeys decodes the current key and the comma-separated
// previous keys. Each must be base64 of exactly 32 bytes. An empty current key
// means encryption is off. Keys that are set but unusable are an error, so a typo
// never silently turns encryption off.
func ParsePodDetailEncryptionKeys(current, previous string) ([][]byte, error) {
	current = strings.TrimSpace(current)
	previous = strings.TrimSpace(previous)
	if current == "" {
		if previous != "" {
			return nil, fmt.Errorf("POD_DETAIL_ENCRYPTION_KEY_PREVIOUS is set but POD_DETAIL_ENCRYPTION_KEY is empty")
		}
		return nil, nil
	}
	values := []string{current}
	for _, p := range strings.Split(previous, ",") {
		if p = strings.TrimSpace(p); p != "" {
			values = append(values, p)
		}
	}
	keys := make([][]byte, 0, len(values))
	for i, v := range values {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil || len(key) != 32 {
			name := "POD_DETAIL_ENCRYPTION_KEY"
			if i > 0 {
				name = fmt.Sprintf("POD_DETAIL_ENCRYPTION_KEY_PREVIOUS entry %d", i)
			}
			return nil, fmt.Errorf("%s must be base64 of exactly 32 bytes (generate one with: openssl rand -base64 32)", name)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func envEnabled(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func randomDevSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	if result == 0 {
		result = 24 // default
	}
	return result
}

func parseIntEnv(s string, defaultVal int) int {
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil || result <= 0 {
		return defaultVal
	}
	return result
}

func parseFloat(s string, defaultVal float64) float64 {
	var result float64
	if _, err := fmt.Sscanf(s, "%f", &result); err != nil || result < 0 {
		return defaultVal
	}
	return result
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 6 * time.Hour
	}
	if d <= 0 {
		return 6 * time.Hour
	}
	return d
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		// Debug: Log environment variable reads for TLS
		if key == "TLS_ENABLED" {
			log.Printf("[Config] getEnv(%s) = '%s' (default: '%s')", key, value, defaultValue)
		}
		return value
	}
	if key == "TLS_ENABLED" {
		log.Printf("[Config] getEnv(%s) = '%s' (using default)", key, defaultValue)
	}
	return defaultValue
}
