package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/metrics"
)

func New(cfg *config.Config) (*gorm.DB, error) {
	// Configure connection pool for better performance
	gormConfig := &gorm.Config{}
	gormConfig.Logger = logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	log.Printf("[Storage] SQL logging enabled (LogLevel: %s, ignoreRecordNotFound=true)", cfg.LogLevel)

	// Retry database connection with exponential backoff
	// This helps with network routing issues between nodes
	// Increased retries and delays for cross-node connectivity
	var db *gorm.DB
	var err error
	maxRetries := 20             // Increased from 10 to 20 for network routing issues
	baseDelay := 3 * time.Second // Increased from 2s to 3s

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Add connection timeout to database URL if not present
		dbURL := cfg.DatabaseURL
		if !strings.Contains(dbURL, "connect_timeout") {
			if strings.Contains(dbURL, "?") {
				dbURL += "&connect_timeout=10"
			} else {
				dbURL += "?connect_timeout=10"
			}
		}

		db, err = gorm.Open(postgres.Open(dbURL), gormConfig)
		if err == nil {
			log.Printf("[Storage] Database connection established on attempt %d", attempt)
			break
		}

		if attempt < maxRetries {
			delay := baseDelay * time.Duration(1<<uint(attempt-1)) // Exponential backoff: 3s, 6s, 12s, 24s...
			if delay > 60*time.Second {
				delay = 60 * time.Second // Cap at 60 seconds for network issues
			}
			log.Printf("[Storage] Database connection failed (attempt %d/%d): %v. Retrying in %v...",
				attempt, maxRetries, err, delay)
			time.Sleep(delay)
		} else {
			log.Printf("[Storage] Database connection failed after %d attempts: %v", maxRetries, err)
			return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
		}
	}

	if err != nil {
		return nil, err
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set connection pool settings for performance and reliability
	// Reduced pool size for better stability with network issues
	sqlDB.SetMaxIdleConns(5)                   // Maximum idle connections (reduced for stability)
	sqlDB.SetMaxOpenConns(25)                  // Maximum open connections (reduced for stability)
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // Connection max lifetime (reduced to detect stale connections)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)  // Idle connection timeout (reduced for network issues)

	// Test connection immediately to verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed after connection: %w", err)
	}
	log.Printf("[Storage] Database ping successful")

	// Start connection pool metrics monitoring
	go monitorConnectionPool(context.Background(), sqlDB)

	return db, nil
}

// monitorConnectionPool monitors database connection pool stats and exports as metrics
func monitorConnectionPool(ctx context.Context, sqlDB *sql.DB) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Printf("[Storage] Started connection pool monitoring (interval: 10s)")

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Storage] Stopped connection pool monitoring")
			return
		case <-ticker.C:
			stats := sqlDB.Stats()

			// Update Prometheus metrics
			metrics.DBConnectionsOpen.Set(float64(stats.OpenConnections))
			metrics.DBConnectionsInUse.Set(float64(stats.InUse))
			metrics.DBConnectionsIdle.Set(float64(stats.Idle))
			metrics.DBConnectionsWaitCount.Add(float64(stats.WaitCount))
			metrics.DBConnectionsWaitDuration.Add(float64(stats.WaitDuration.Milliseconds()))

			// Log if approaching connection limit
			utilizationPct := float64(stats.OpenConnections) / float64(stats.MaxOpenConnections) * 100
			if utilizationPct > 80 {
				log.Printf("⚠️  [Storage] High connection pool utilization: %d/%d (%.1f%%)",
					stats.OpenConnections, stats.MaxOpenConnections, utilizationPct)
			}

			// Log if connections are waiting
			if stats.WaitCount > 0 {
				log.Printf("⚠️  [Storage] Connections waiting: %d (total wait duration: %v)",
					stats.WaitCount, stats.WaitDuration)
			}
		}
	}
}

func Migrate(db *gorm.DB) error {
	// Use migration system
	// CRITICAL: This must be called on every startup to ensure schema is up-to-date
	log.Printf("[Storage] Running database migrations...")
	if err := migrations.RunMigrations(db); err != nil {
		log.Printf("[Storage] ❌ Migration failed: %v", err)
		return err
	}
	log.Printf("[Storage] ✅ Database migrations completed successfully")
	return nil
}
