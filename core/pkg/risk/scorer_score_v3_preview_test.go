package risk

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCalculateScore_AttachesScoreV3PreviewForPod(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Insight{}, &models.Pod{}, &models.AssetSecurityState{}); err != nil {
		t.Fatal(err)
	}
	podUID := "pod-prev-1"
	now := time.Now()
	if err := db.Create(&models.Pod{
		UID: podUID, Name: "p", Namespace: "ns", ClusterID: "c1",
		ServiceAccount: "default", Containers: "[]",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: podUID, ResourceName: "p", ResourceNamespace: "ns",
		InsightType: "runtime", Severity: "high", Title: "t", Status: "active", DetectedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AssetSecurityState{
		AssetType: "pod", PodUID: podUID, Namespace: "ns", ClusterID: "c1",
		HostNetwork: true, HasSuspiciousExec: true,
		RuntimeSignalsByType: "{}", EffectiveCapabilities: "[]",
		UpdatedAt: now, CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	scorer := NewScorer(db)
	score, err := scorer.CalculateScore(context.Background(), podUID)
	if err != nil {
		t.Fatal(err)
	}
	preview, ok := score.Factors["score_v3_preview"].(map[string]interface{})
	if !ok || preview == nil {
		t.Fatalf("expected score_v3_preview map in factors, keys=%v", keysOf(score.Factors))
	}
	if _, ok := preview["byDimension"]; !ok {
		t.Fatalf("preview missing byDimension: %#v", preview)
	}
}

func keysOf(m map[string]interface{}) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	return k
}
