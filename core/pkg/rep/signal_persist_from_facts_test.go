package rep

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPersistSynthesizedSignalsFromFacts_DoesNotInflateCount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.RuntimeBehaviorFact{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	podUID := "rep-b-pod-1"

	// Existing runtime signal for today window (Count should not be incremented by facts-based persistence).
	existing := models.RuntimeSignal{
		PodUID:     podUID,
		SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT",
		Category:   "EXECUTION",
		Confidence: 0.1,
		Evidence:   `{"source":"legacy"}`,
		Count:      5,
		CreatedAt:  today.Add(10 * time.Minute),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed existing runtime_signal: %v", err)
	}

	ev := &models.RuntimeEvent{
		ID:        10,
		PodUID:    podUID,
		Namespace: "ns",
		CreatedAt: now,
	}

	facts := []models.RuntimeBehaviorFact{
		{FactID: "f1", FactType: "INTERACTIVE_SHELL", PodUID: podUID, Namespace: "ns"},
		{FactID: "f2", FactType: "TMP_BINARY_EXEC", PodUID: podUID, Namespace: "ns"},
	}

	cands := synthesizeSignalsFromFacts(facts)
	if err := persistSynthesizedSignalsFromFacts(context.Background(), db, ev, facts, cands); err != nil {
		t.Fatalf("persist: %v", err)
	}

	var out models.RuntimeSignal
	if err := db.Where("pod_uid = ? AND signal_type = ? AND created_at >= ?", podUID, "SUSPICIOUS_EXEC_FROM_SNAPSHOT", today).First(&out).Error; err != nil {
		t.Fatalf("query out: %v", err)
	}

	if out.Count != 5 {
		t.Fatalf("expected Count unchanged=5, got %d", out.Count)
	}
	if out.Confidence <= 0.1 {
		t.Fatalf("expected Confidence updated > 0.1, got %f", out.Confidence)
	}
}
