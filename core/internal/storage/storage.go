package storage

import (
	"context"
	"database/sql"
	"log"
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
	// ✅ ALWAYS enable SQL logging to debug risk trends API issue
	// This will help us see the actual SQL queries being executed
	gormConfig.Logger = logger.Default.LogMode(logger.Info) // Log SQL queries
	log.Printf("[Storage] SQL logging enabled (LogLevel: %s)", cfg.LogLevel)
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), gormConfig)
	if err != nil {
		return nil, err
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set connection pool settings for performance
	sqlDB.SetMaxIdleConns(10)                  // Maximum idle connections
	sqlDB.SetMaxOpenConns(100)                 // Maximum open connections
	sqlDB.SetConnMaxLifetime(time.Hour)        // Connection max lifetime
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Idle connection timeout

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
	return migrations.RunMigrations(db)
}

