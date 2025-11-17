package storage

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/ksam/core/internal/config"
	"github.com/ksam/core/migrations"
)

func New(cfg *config.Config) (*gorm.DB, error) {
	// Configure connection pool for better performance
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
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

	return db, nil
}

func Migrate(db *gorm.DB) error {
	// Use migration system
	return migrations.RunMigrations(db)
}

