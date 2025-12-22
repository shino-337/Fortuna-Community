package config

import (
	"fmt"
	"log"
	"os"
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
}

func Load(configPath string) (*Config, error) {
	// Use fmt directly to ensure logs appear
	fmt.Fprintf(os.Stdout, "[Config] Load() called - starting config loading...\n")

	tlsEnabledStr := getEnv("TLS_ENABLED", "false")
	tlsEnabled := tlsEnabledStr == "true"

	fmt.Fprintf(os.Stdout, "[Config] TLS_ENABLED env='%s', parsed=%v\n", tlsEnabledStr, tlsEnabled)

	cfg := &Config{
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@postgres:5432/ksam?sslmode=disable"),
		RedisURL:             getEnv("REDIS_URL", ""),
		GRPCPort:             getEnv("GRPC_PORT", "9090"),
		HTTPPort:             getEnv("HTTP_PORT", "8080"),
		NATSEndpoint:         getEnv("NATS_ENDPOINT", "nats://nats.ksam.svc.cluster.local:4222"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		AuthEnabled:          getEnv("AUTH_ENABLED", "true") == "true",
		JWTSecret:            getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		TokenExpirationHours: parseInt(getEnv("TOKEN_EXPIRATION_HOURS", "24")),
		TLSEnabled:           tlsEnabled,
		TLSCACertPath:        getEnv("TLS_CA_CERT_PATH", "/etc/ksam/ca-cert/ca.crt"),
		TLSCertPath:          getEnv("TLS_CERT_PATH", "/etc/ksam/certs/tls.crt"),
		TLSKeyPath:           getEnv("TLS_KEY_PATH", "/etc/ksam/certs/tls.key"),
		WebhookTLSCertPath:   getEnv("WEBHOOK_TLS_CERT_PATH", "/etc/webhook/certs/tls.crt"),
		WebhookTLSKeyPath:    getEnv("WEBHOOK_TLS_KEY_PATH", "/etc/webhook/certs/tls.key"),
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
