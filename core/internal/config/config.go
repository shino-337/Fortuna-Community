package config

import (
	"crypto/rand"
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

	// Redis (optional)
	RedisURL string

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
	// agent ingest must authenticate through a scoped agent credential instead of
	// silently falling back to this shared token.
	IngestToken string
	// AgentCredentialRegistryPath points to the operator-managed JSON registry used
	// by scoped per-agent/per-cluster authentication. Empty preserves the legacy
	// shared-token mode during migration.
	AgentCredentialRegistryPath string

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

	// Pod Detail – encryption at-rest for process command/binary_path (Phase 4.2).
	// Base64-encoded 32-byte key. Empty = no encryption. Set POD_DETAIL_ENCRYPTION_KEY (env) or in config file before deploy.
	PodDetailEncryptionKey string

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
		DatabaseURL:                  getEnv("DATABASE_URL", "postgres://postgres:postgres@postgres:5432/fortuna?sslmode=disable"),
		RedisURL:                     getEnv("REDIS_URL", ""),
		GRPCPort:                     getEnv("GRPC_PORT", "9090"),
		HTTPPort:                     getEnv("HTTP_PORT", "8080"),
		NATSEndpoint:                 getEnv("NATS_ENDPOINT", "nats://nats.fortuna.svc.cluster.local:4222"),
		LogLevel:                     getEnv("LOG_LEVEL", "info"),
		AuthEnabled:                  getEnv("AUTH_ENABLED", "true") == "true",
		JWTSecret:                    jwtSecretFromEnv(),
		TokenExpirationHours:         parseInt(getEnv("TOKEN_EXPIRATION_HOURS", "24")),
		IngestToken:                  strings.TrimSpace(getEnv("FORTUNA_INGEST_TOKEN", "")),
		AgentCredentialRegistryPath:  strings.TrimSpace(getEnv("FORTUNA_AGENT_CREDENTIAL_REGISTRY", "")),
		TLSEnabled:                   tlsEnabled,
		TLSCACertPath:                getEnv("TLS_CA_CERT_PATH", "/etc/fortuna/tls/server/ca.crt"),
		TLSCertPath:                  getEnv("TLS_CERT_PATH", "/etc/fortuna/tls/server/tls.crt"),
		TLSKeyPath:                   getEnv("TLS_KEY_PATH", "/etc/fortuna/tls/server/tls.key"),
		WebhookTLSCertPath:           getEnv("WEBHOOK_TLS_CERT_PATH", "/etc/webhook/certs/tls.crt"),
		WebhookTLSKeyPath:            getEnv("WEBHOOK_TLS_KEY_PATH", "/etc/webhook/certs/tls.key"),
		PCESchedulerEnabled:          getEnv("PCE_SCHEDULER_ENABLED", "true") == "true",
		PCESchedulerInterval:         parseDuration(getEnv("PCE_SCHEDULER_INTERVAL", "6h")),
		PodDetailEncryptionKey:       getEnv("POD_DETAIL_ENCRYPTION_KEY", ""),
		RateLimitPerClusterEnabled:   getEnv("RATE_LIMIT_PER_CLUSTER_ENABLED", "true") == "true",
		RateLimitSyncPerClusterRPS:   parseFloat(getEnv("RATE_LIMIT_SYNC_PER_CLUSTER_RPS", "10"), 10),
		RateLimitSyncPerClusterBurst: parseIntEnv(getEnv("RATE_LIMIT_SYNC_PER_CLUSTER_BURST", "20"), 20),
		RateLimitSBOMPerClusterRPS:   parseFloat(getEnv("RATE_LIMIT_SBOM_PER_CLUSTER_RPS", "50"), 50),
		RateLimitSBOMPerClusterBurst: parseIntEnv(getEnv("RATE_LIMIT_SBOM_PER_CLUSTER_BURST", "100"), 100),
	}

	log.Printf("[Config] Final config: TLSEnabled=%v, TLSCertPath=%s, TLSCACertPath=%s",
		cfg.TLSEnabled, cfg.TLSCertPath, cfg.TLSCACertPath)
	devMode := envEnabled("FORTUNA_DEV_MODE")
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
	if cfg.AgentCredentialRegistryPath != "" {
		log.Printf("[Config] FORTUNA_AGENT_CREDENTIAL_REGISTRY is set: HTTP agent ingest uses scoped agent credentials; shared-token fallback is disabled for agent routes")
	} else if cfg.IngestToken != "" {
		log.Printf("[Config] FORTUNA_INGEST_TOKEN is set: HTTP agent/runtime ingest routes require ingest token")
	} else if !envEnabled("FORTUNA_ALLOW_UNAUTHED_INGEST") {
		log.Printf("[Config] FORTUNA_INGEST_TOKEN is not set: HTTP agent/runtime ingest routes will fail closed")
	}

	return cfg, nil
}

func jwtSecretFromEnv() string {
	if v := strings.TrimSpace(os.Getenv("JWT_SECRET")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("FORTUNA_JWT_SECRET"))
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
