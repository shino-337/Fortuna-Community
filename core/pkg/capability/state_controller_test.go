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
	if err := db.Session(&gorm.Session{DisableForeignKeyConstraintWhenMigrating: true}).AutoMigrate(&models.Pod{}); err != nil {
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
			category TEXT NOT NULL,
			confidence REAL DEFAULT 0.5,
			evidence TEXT,
			evidence_refs TEXT,
			count INTEGER DEFAULT 1,
			first_seen_at TIMESTAMP,
			last_seen_at TIMESTAMP,
			created_at TIMESTAMP
		);
	`).Error; err != nil {
		t.Fatalf("Failed to create runtime_signals table: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE capability_metadata (
			capability_id TEXT PRIMARY KEY,
			domain TEXT NOT NULL,
			category TEXT NOT NULL,
			description TEXT NOT NULL,
			severity_base TEXT NOT NULL,
			confidence_base REAL DEFAULT 0.5,
			preconditions TEXT,
			produces_attack_steps TEXT,
			expires_with_instance INTEGER DEFAULT 1,
			supports_runtime_promotion INTEGER DEFAULT 1,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		);
	`).Error; err != nil {
		t.Fatalf("Failed to create capability_metadata table: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE pod_attack_steps (
			id INTEGER PRIMARY KEY,
			pod_uid TEXT NOT NULL,
			step_id TEXT NOT NULL,
			description TEXT,
			category TEXT NOT NULL,
			confidence REAL DEFAULT 0.5,
			evidence TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			UNIQUE(pod_uid, step_id)
		);
	`).Error; err != nil {
		t.Fatalf("Failed to create pod_attack_steps table: %v", err)
	}

	return db
}

func TestCapabilityStateController_InitializeCapability(t *testing.T) {
	db := setupTestDB(t)
	csc := NewCapabilityStateController(db)
	ctx := context.Background()

	t.Run("Initialize new capability", func(t *testing.T) {
		err := csc.InitializeCapability(ctx, "pod-123", "namespace-1", "ESC_PRIV_POD", "ESCAPE", "CRITICAL", map[string]interface{}{
			"hostPID": true,
		})

		assert.NoError(t, err)

		var cap models.PodCapability
		err = db.Where("pod_uid = ? AND capability_id = ?", "pod-123", "ESC_PRIV_POD").First(&cap).Error
		assert.NoError(t, err)
		assert.Equal(t, "detected", cap.State)
		assert.Equal(t, 0.5, cap.Confidence)
		assert.Equal(t, "pod-123", cap.PodUID)
		assert.Equal(t, "ESC_PRIV_POD", cap.CapabilityID)
	})

	t.Run("Initialize existing capability updates", func(t *testing.T) {
		existing := models.PodCapability{
			PodUID:       "pod-123",
			Namespace:    "namespace-1",
			CapabilityID: "ESC_PRIV_POD",
			State:        "detected",
			Confidence:   0.5,
		}
		db.Create(&existing)

		err := csc.InitializeCapability(ctx, "pod-123", "namespace-1", "ESC_PRIV_POD", "ESCAPE", "CRITICAL", map[string]interface{}{
			"hostPID": true,
		})

		assert.NoError(t, err)

		var cap models.PodCapability
		db.Where("pod_uid = ? AND capability_id = ?", "pod-123", "ESC_PRIV_POD").First(&cap)
		assert.Equal(t, "detected", cap.State)
	})
}

func TestCapabilityStateController_PromoteCapability(t *testing.T) {
	db := setupTestDB(t)
	csc := NewCapabilityStateController(db)
	ctx := context.Background()

	cap := models.PodCapability{
		PodUID:       "pod-123",
		Namespace:    "namespace-1",
		CapabilityID: "ESC_HOSTPATH_NODE",
		State:        "detected",
		Confidence:   0.5,
	}
	db.Create(&cap)

	rule := models.PromotionRule{
		CapabilityID:    "ESC_HOSTPATH_NODE",
		SignalType:      "PROC_ROOT_PIVOT",
		MinOccurrences:  1,
		PromoteTo:       "confirmed",
		ConfidenceBoost: 0.2,
	}
	db.Create(&rule)

	signal := models.RuntimeSignal{
		PodUID:     "pod-123",
		SignalType: "PROC_ROOT_PIVOT",
		Category:   "ESCAPE",
		Confidence: 0.9,
		Evidence:   `{"syscall": "openat", "path": "/proc/1/root"}`,
	}
	db.Create(&signal)

	t.Run("Promote capability with matching rule", func(t *testing.T) {
		err := csc.PromoteCapability(ctx, "pod-123", "ESC_HOSTPATH_NODE", "PROC_ROOT_PIVOT", 0.9)

		assert.NoError(t, err)

		var updated models.PodCapability
		db.Where("pod_uid = ? AND capability_id = ?", "pod-123", "ESC_HOSTPATH_NODE").First(&updated)
		assert.Equal(t, "confirmed", updated.State)
		assert.Greater(t, updated.Confidence, 0.5)
	})

	t.Run("Promote capability without matching rule", func(t *testing.T) {
		cap2 := models.PodCapability{
			PodUID:       "pod-456",
			Namespace:    "namespace-1",
			CapabilityID: "ESC_PRIV_POD",
			State:        "detected",
			Confidence:   0.5,
		}
		db.Create(&cap2)

		err := csc.PromoteCapability(ctx, "pod-456", "ESC_PRIV_POD", "UNKNOWN_SIGNAL", 0.9)
		assert.NoError(t, err)

		var updated models.PodCapability
		db.Where("pod_uid = ? AND capability_id = ?", "pod-456", "ESC_PRIV_POD").First(&updated)
		assert.Equal(t, "detected", updated.State)
	})

	t.Run("Promote non-existent capability", func(t *testing.T) {
		err := csc.PromoteCapability(ctx, "pod-999", "NON_EXISTENT", "PROC_ROOT_PIVOT", 0.9)
		assert.NoError(t, err)
	})
}

func TestCapabilityStateController_PromoteCapability_MinOccurrences(t *testing.T) {
	db := setupTestDB(t)
	csc := NewCapabilityStateController(db)
	ctx := context.Background()

	cap := models.PodCapability{
		PodUID:       "pod-123",
		Namespace:    "namespace-1",
		CapabilityID: "ESC_HOSTPATH_NODE",
		State:        "detected",
		Confidence:   0.5,
	}
	db.Create(&cap)

	rule := models.PromotionRule{
		CapabilityID:    "ESC_HOSTPATH_NODE",
		SignalType:      "PROC_ROOT_PIVOT",
		MinOccurrences:  2,
		PromoteTo:       "confirmed",
		ConfidenceBoost: 0.2,
	}
	db.Create(&rule)

	signal1 := models.RuntimeSignal{
		PodUID:     "pod-123",
		SignalType: "PROC_ROOT_PIVOT",
		Category:   "ESCAPE",
		Confidence: 0.9,
	}
	db.Create(&signal1)

	t.Run("Promote with insufficient occurrences", func(t *testing.T) {
		err := csc.PromoteCapability(ctx, "pod-123", "ESC_HOSTPATH_NODE", "PROC_ROOT_PIVOT", 0.9)
		assert.NoError(t, err)

		var updated models.PodCapability
		db.Where("pod_uid = ? AND capability_id = ?", "pod-123", "ESC_HOSTPATH_NODE").First(&updated)
		assert.Equal(t, "detected", updated.State)
	})

	signal2 := models.RuntimeSignal{
		PodUID:     "pod-123",
		SignalType: "PROC_ROOT_PIVOT",
		Category:   "ESCAPE",
		Confidence: 0.9,
	}
	db.Create(&signal2)

	t.Run("Promote with sufficient occurrences", func(t *testing.T) {
		err := csc.PromoteCapability(ctx, "pod-123", "ESC_HOSTPATH_NODE", "PROC_ROOT_PIVOT", 0.9)
		assert.NoError(t, err)

		var updated models.PodCapability
		db.Where("pod_uid = ? AND capability_id = ?", "pod-123", "ESC_HOSTPATH_NODE").First(&updated)
		assert.Equal(t, "confirmed", updated.State)
	})
}

func TestCapabilityStateController_PromoteCapability_StateProgression(t *testing.T) {
	db := setupTestDB(t)
	csc := NewCapabilityStateController(db)
	ctx := context.Background()

	cap := models.PodCapability{
		PodUID:       "pod-123",
		Namespace:    "namespace-1",
		CapabilityID: "ESC_HOSTPATH_NODE",
		State:        "detected",
		Confidence:   0.5,
	}
	db.Create(&cap)

	for i := 0; i < 3; i++ {
		signal := models.RuntimeSignal{
			PodUID:     "pod-123",
			SignalType: "PROC_ROOT_PIVOT",
			Category:   "ESCAPE",
			Confidence: 0.9,
		}
		db.Create(&signal)
	}

	rule1 := models.PromotionRule{
		CapabilityID:    "ESC_HOSTPATH_NODE",
		SignalType:      "PROC_ROOT_PIVOT",
		MinOccurrences:  1,
		PromoteTo:       "confirmed",
		ConfidenceBoost: 0.2,
	}
	db.Create(&rule1)

	rule2 := models.PromotionRule{
		CapabilityID:    "ESC_HOSTPATH_NODE",
		SignalType:      "PROC_ROOT_PIVOT",
		MinOccurrences:  3,
		PromoteTo:       "exploited",
		ConfidenceBoost: 0.3,
	}
	db.Create(&rule2)

	t.Run("State progression: detected -> confirmed -> exploited", func(t *testing.T) {
		db.Model(&models.PodCapability{}).
			Where("pod_uid = ? AND capability_id = ?", "pod-123", "ESC_HOSTPATH_NODE").
			Update("state", "detected")
		db.Where("pod_uid = ?", "pod-123").Delete(&models.RuntimeSignal{})

		signal1 := models.RuntimeSignal{
			PodUID:     "pod-123",
			SignalType: "PROC_ROOT_PIVOT",
			Category:   "ESCAPE",
			Confidence: 0.9,
		}
		db.Create(&signal1)

		err := csc.PromoteCapability(ctx, "pod-123", "ESC_HOSTPATH_NODE", "PROC_ROOT_PIVOT", 0.9)
		assert.NoError(t, err)

		var updated models.PodCapability
		db.Where("pod_uid = ? AND capability_id = ?", "pod-123", "ESC_HOSTPATH_NODE").First(&updated)
		assert.Equal(t, "confirmed", updated.State)

		signal2 := models.RuntimeSignal{
			PodUID:     "pod-123",
			SignalType: "PROC_ROOT_PIVOT",
			Category:   "ESCAPE",
			Confidence: 0.9,
		}
		db.Create(&signal2)
		signal3 := models.RuntimeSignal{
		PodUID:     "pod-123",
		SignalType: "PROC_ROOT_PIVOT",
		Category:   "ESCAPE",
		Confidence: 0.9,
	}
	db.Create(&signal3)

		err = csc.PromoteCapability(ctx, "pod-123", "ESC_HOSTPATH_NODE", "PROC_ROOT_PIVOT", 0.9)
		assert.NoError(t, err)

		db.Where("pod_uid = ? AND capability_id = ?", "pod-123", "ESC_HOSTPATH_NODE").First(&updated)
		assert.Equal(t, "exploited", updated.State)
	})
}

func TestCapabilityStateController_GetCapabilityState(t *testing.T) {
	db := setupTestDB(t)
	csc := NewCapabilityStateController(db)
	ctx := context.Background()

	cap := models.PodCapability{
		PodUID:       "pod-123",
		Namespace:    "namespace-1",
		CapabilityID: "ESC_PRIV_POD",
		State:        "confirmed",
		Confidence:   0.7,
	}
	db.Create(&cap)

	t.Run("Get existing capability state", func(t *testing.T) {
		state, err := csc.GetCapabilityState(ctx, "pod-123", "ESC_PRIV_POD")
		assert.NoError(t, err)
		assert.Equal(t, "confirmed", state)
	})

	t.Run("Get non-existent capability state", func(t *testing.T) {
		state, err := csc.GetCapabilityState(ctx, "pod-999", "NON_EXISTENT")
		assert.Error(t, err)
		assert.Equal(t, "", state)
	})
}
