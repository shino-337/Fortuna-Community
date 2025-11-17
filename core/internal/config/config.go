package config

import (
	"fmt"
	"os"
)

type Config struct {
	// Database
	DatabaseURL string

	// Redis (optional)
	RedisURL string

	// Server ports
	GRPCPort string
	HTTPPort string

	// Logging
	LogLevel string

	// Auth
	AuthEnabled          bool
	JWTSecret            string
	TokenExpirationHours int
}

func Load(configPath string) (*Config, error) {
	cfg := &Config{
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ksam?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", ""),
		GRPCPort:            getEnv("GRPC_PORT", "9090"),
		HTTPPort:            getEnv("HTTP_PORT", "8080"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		AuthEnabled:         getEnv("AUTH_ENABLED", "true") == "true",
		JWTSecret:           getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		TokenExpirationHours: parseInt(getEnv("TOKEN_EXPIRATION_HOURS", "24")),
	}

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
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

