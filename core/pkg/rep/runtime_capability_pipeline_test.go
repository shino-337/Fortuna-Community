package rep

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/capability"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProcessRuntimeEvent_RuntimeFirstCapabilityInitAndPromotion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.RuntimeEvent{},
		&models.RuntimeSignal{},
		&models.RuntimeBehaviorFact{},
		&models.RuntimeIncident{},
		&models.PodRiskProfile{},
		&models.Pod{},
		&models.PodCapability{},
		&models.CapabilityMetadata{},
		&models.PromotionRule{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// SQLite test schema: ensure ON CONFLICT targets exist for upsert paths used in processor/CSC.
	_ = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_risk_profiles_pod_uid_unique ON pod_risk_profiles(pod_uid)").Error
	_ = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_capabilities_pod_uid_capability_id_unique ON pod_capabilities(pod_uid, capability_id)").Error

	// Seed promotion rule to verify CSC promotion path after runtime-first initialize.
	if err := db.Create(&models.PromotionRule{
		CapabilityID:    capability.ESC_RUNTIME_ACTIVE,
		SignalType:      "PROC_ROOT_PIVOT",
		MinOccurrences:  1,
		PromoteTo:       "confirmed",
		ConfidenceBoost: 0.3,
	}).Error; err != nil {
		t.Fatalf("seed promotion rule: %v", err)
	}

	podUID := "capability-runtime-first-pod-1"
	ns := "ns"
	now := time.Now().UTC()

	// /proc/1/root with open syscall maps to PROC_ROOT_PIVOT (score 90 -> ESC_RUNTIME_ACTIVE).
	in := RuntimeEventInput{
		PodUID:     podUID,
		Namespace:  ns,
		Syscall:    "open",
		TargetPath: "/proc/1/root",
		Timestamp:  &now,
	}
	if _, err := ProcessRuntimeEvent(context.Background(), db, in); err != nil {
		t.Fatalf("ProcessRuntimeEvent: %v", err)
	}

	var got struct {
		State      string
		Confidence float64
	}
	if err := db.Table("pod_capabilities").
		Select("state, confidence").
		Where("pod_uid = ? AND capability_id = ?", podUID, capability.ESC_RUNTIME_ACTIVE).
		Scan(&got).Error; err != nil {
		t.Fatalf("query pod capability state/confidence: %v", err)
	}
	if got.State != "confirmed" {
		t.Fatalf("expected state confirmed after promotion rule, got %q", got.State)
	}
	if got.Confidence < 0.8 {
		t.Fatalf("expected confidence >= 0.8 after boost, got %.2f", got.Confidence)
	}
}
