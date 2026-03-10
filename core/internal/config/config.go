package config

import (
	"fmt"
	"log"
	"os"
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
}

func Load(configPath string) (*Config, error) {
	// Use fmt directly to ensure logs appear
	fmt.Fprintf(os.Stdout, "[Config] Load() called - starting config loading...\n")

	tlsEnabledStr := getEnv("TLS_ENABLED", "false")
	tlsEnabled := tlsEnabledStr == "true"

	fmt.Fprintf(os.Stdout, "[Config] TLS_ENABLED env='%s', parsed=%v\n", tlsEnabledStr, tlsEnabled)

	cfg := &Config{
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@postgres:5432/fortuna?sslmode=disable"),
		RedisURL:             getEnv("REDIS_URL", ""),
		GRPCPort:             getEnv("GRPC_PORT", "9090"),
		HTTPPort:             getEnv("HTTP_PORT", "8080"),
		NATSEndpoint:         getEnv("NATS_ENDPOINT", "nats://nats.fortuna.svc.cluster.local:4222"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		AuthEnabled:          getEnv("AUTH_ENABLED", "true") == "true",
		JWTSecret:            getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		TokenExpirationHours: parseInt(getEnv("TOKEN_EXPIRATION_HOURS", "24")),
		TLSEnabled:           tlsEnabled,
		TLSCACertPath:        getEnv("TLS_CA_CERT_PATH", "/etc/fortuna/tls/server/ca.crt"),
		TLSCertPath:          getEnv("TLS_CERT_PATH", "/etc/fortuna/tls/server/tls.crt"),
		TLSKeyPath:           getEnv("TLS_KEY_PATH", "/etc/fortuna/tls/server/tls.key"),
		WebhookTLSCertPath:   getEnv("WEBHOOK_TLS_CERT_PATH", "/etc/webhook/certs/tls.crt"),
		WebhookTLSKeyPath:    getEnv("WEBHOOK_TLS_KEY_PATH", "/etc/webhook/certs/tls.key"),
		PCESchedulerEnabled:      getEnv("PCE_SCHEDULER_ENABLED", "true") == "true",
		PCESchedulerInterval:     parseDuration(getEnv("PCE_SCHEDULER_INTERVAL", "6h")),
		PodDetailEncryptionKey:   getEnv("POD_DETAIL_ENCRYPTION_KEY", ""),
	}

	log.Printf("[Config] Final config: TLSEnabled=%v, TLSCertPath=%s, TLSCACertPath=%s",
		cfg.TLSEnabled, cfg.TLSCertPath, cfg.TLSCACertPath)

	return cfg, nil
}

func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	if result == 0 {
		result = 24 // default
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
