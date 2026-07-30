package explainability

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoadFactSummaries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.RuntimeBehaviorFact{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	pod := "pod-enrich-1"
	row := models.RuntimeBehaviorFact{
		FactID:     "f-enrich-1",
		PodUID:     pod,
		Namespace:  "ns",
		FactType:   "NETWORK_CONNECT",
		Domain:     "network",
		Attributes: "{}",
		SourceRef:  "{}",
		ObservedAt: now,
		CreatedAt:  now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	out, err := LoadFactSummaries(context.Background(), db, pod, []string{"f-enrich-1", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].FactType != "NETWORK_CONNECT" {
		t.Fatalf("got %+v", out)
	}
}
