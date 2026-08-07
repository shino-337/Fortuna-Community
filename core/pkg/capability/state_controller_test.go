package capability

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// SQLite :memory: databases are connection-local. Keep a single connection so
	// transactions and asynchronous attack-path rebuilds observe the same schema.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get SQL database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	// The promotion path may rebuild attack paths and read the pod inventory.
	// Use the real model schema rather than silently relying on a missing table.
	if err := db.AutoMigrate(&models.Pod{}); err != nil {
		t.Fatalf("Failed to create pods table: %v", err)
	}

	// Create tables
	if err := db.Exec(`
		CREATE TABLE pod_capabilities (
			id INTEGER PRIMARY KEY,
			pod_uid TEXT NOT NULL,
			namespace TEXT NOT NULL,
			capability_id TEXT NOT NULL,
			capability_group TEXT NOT NULL,
			severity TEXT NOT NULL,
			state TEXT DEFAULT 'detected',
			confidence REAL DEFAULT 0.5,
			capability_class TEXT DEFAULT 'effective',
			derived_from TEXT,
			first_seen_at TIMESTAMP,
			last_seen_at TIMESTAMP,
			evidence TEXT,
			mitre TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			UNIQUE(pod_uid, capability_id)
		);
	`).Error; err != nil {
		t.Fatalf("Failed to create pod_capabilities table: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE promotion_rules (
			id INTEGER PRIMARY KEY,
			capability_id TEXT NOT NULL,
			signal_type TEXT NOT NULL,
			min_occurrences INTEGER DEFAULT 1,
			required_capabilities TEXT,
			promote_to TEXT NOT NULL,
			confidence_boost REAL DEFAULT 0.1,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			UNIQUE(capability_id, signal_type, promote_to)
		);
	`).Error; err != nil {
		t.Fatalf("Failed to create promotion_rules table: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE runtime_signals (
			id INTEGER PRIMARY KEY,
			pod_uid TEXT NOT NULL,
			signal_type TEXT NOT NULL,
			severity TEXT NOT NULL,
			evidence TEXT,
			observed_at TIMESTAMP,
			created_at TIMESTAMP,
			UNIQUE(pod_uid, signal_type, observed_at)
		);
	`).Error; err != nil {
		t.Fatalf("Failed to create runtime_signals table: %v", err)
	}

	return db
}

func TestCapabilityStateController_PromoteCapability(t *testing.T) {
	db := setupTestDB(t)
	controller := NewCapabilityStateController(db)

	ctx := context.Background()

	// Insert test data
	_, err := controller.PromoteCapability(ctx, "pod-123", "CAP_NET_RAW", "runtime_signal")
	assert.NoError(t, err)

	var state string
	err = db.Raw("SELECT state FROM pod_capabilities WHERE pod_uid = ? AND capability_id = ?", "pod-123", "CAP_NET_RAW").Scan(&state).Error
	assert.NoError(t, err)
	assert.Equal(t, "promoted", state)
}
