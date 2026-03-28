package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/fortuna/core/pkg/models"
)

// TestRiskWorker_Process_NormalizedPodMessage_EndToEnd verifies the same path as JetStream
// delivery: JSON body → RiskWorker.Process → YAMLEngine.EvaluateResource → insights persisted.
// JetStream transport is orthogonal; this is the contract integration test for GAP "NATS message shape → insights".
func TestRiskWorker_Process_NormalizedPodMessage_EndToEnd(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir:", rulesDir, err)
	}
	t.Setenv("FORTUNA_RULES_DIR", rulesDir)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RiskScore{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Cluster{ID: "c-pipe", Name: "c-pipe"}).Error; err != nil {
		t.Fatal(err)
	}
	podUID := "e2e11111-e2e1-e2e1-e2e1-e2e111111111"
	if err := db.Create(&models.Pod{
		ClusterID: "c-pipe", Name: "hostpod", Namespace: "ns1", ServiceAccount: "default",
		UID: podUID, HostNetwork: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	w := NewRiskWorker(nil, db, nil)
	defer w.Stop()
	if w.yamlEngine == nil {
		t.Fatal("expected YAMLEngine when FORTUNA_RULES_DIR is set")
	}

	raw := `{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hostpod","namespace":"ns1","uid":"` + podUID + `"},"spec":{"hostNetwork":true,"containers":[{"name":"c","image":"nginx"}]}}`
	normalized := map[string]interface{}{
		"kind":       "Pod",
		"uid":        podUID,
		"name":       "hostpod",
		"namespace":  "ns1",
		"cluster_id": "c-pipe",
		"raw_json":   raw,
	}
	body, _ := json.Marshal(normalized)
	if err := w.Process(context.Background(), &nats.Msg{Data: body}); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := db.Model(&models.Insight{}).
		Where("resource_uid = ? AND deleted_at IS NULL AND status = ?", podUID, "active").
		Where("title = ?", "Pod uses host namespaces (hostNetwork, hostPID, and/or hostIPC)").
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active PSS host-namespaces insight, got %d", count)
	}
}

func TestRiskWorker_Process_IdempotentInsightDedup(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir:", rulesDir, err)
	}
	t.Setenv("FORTUNA_RULES_DIR", rulesDir)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RiskScore{}); err != nil {
		t.Fatal(err)
	}
	_ = db.Create(&models.Cluster{ID: "c-ded", Name: "c-ded"})
	podUID := "e2e22222-e2e2-e2e2-e2e2-e2e222222222"
	_ = db.Create(&models.Pod{
		ClusterID: "c-ded", Name: "p", Namespace: "ns", ServiceAccount: "default",
		UID: podUID, HostNetwork: true,
	})

	w := NewRiskWorker(nil, db, nil)
	defer w.Stop()
	raw := `{"kind":"Pod","metadata":{"namespace":"ns","name":"p","uid":"` + podUID + `"},"spec":{"hostNetwork":true,"containers":[]}}`
	normalized := map[string]interface{}{
		"kind": "Pod", "uid": podUID, "name": "p", "namespace": "ns", "cluster_id": "c-ded", "raw_json": raw,
	}
	body, _ := json.Marshal(normalized)
	msg := &nats.Msg{Data: body}
	if err := w.Process(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if err := w.Process(context.Background(), msg); err != nil {
		t.Fatal(err)
	}

	var n int64
	_ = db.Model(&models.Insight{}).Where("resource_uid = ? AND deleted_at IS NULL", podUID).Count(&n)
	if n != 1 {
		t.Fatalf("expected single insight row after duplicate Process (dedup), got %d", n)
	}
}

func TestInsightStatusUpdater_PodResolvesWhenPSSNoLongerApplies(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir:", rulesDir, err)
	}
	t.Setenv("FORTUNA_RULES_DIR", rulesDir)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RiskScore{}); err != nil {
		t.Fatal(err)
	}
	_ = db.Create(&models.Cluster{ID: "c-upd", Name: "c-upd"})
	podUID := "e2e33333-e2e3-e2e3-e2e3-e2e333333333"
	_ = db.Create(&models.Pod{
		ClusterID: "c-upd", Name: "p", Namespace: "ns", ServiceAccount: "default",
		UID: podUID, HostNetwork: true,
	})
	now := time.Now()
	title := "Pod uses host namespaces (hostNetwork, hostPID, and/or hostIPC)"
	ins := models.Insight{
		ResourceType: "Pod", ResourceNamespace: "ns", ResourceName: "p", ResourceUID: podUID,
		InsightType: "pod-security", Severity: "high", Title: title,
		Description: title + ": test", Status: "active",
		DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&ins).Error; err != nil {
		t.Fatal(err)
	}

	// Simulate correlator: host namespaces cleared on node / spec sync
	if err := db.Model(&models.Pod{}).Where("uid = ?", podUID).Update("host_network", false).Error; err != nil {
		t.Fatal(err)
	}

	u := NewInsightStatusUpdater(db)
	if err := u.UpdateStatusForResolvedRisks(context.Background()); err != nil {
		t.Fatal(err)
	}

	var st string
	if err := db.Model(&models.Insight{}).Where("id = ?", ins.ID).Select("status").Scan(&st).Error; err != nil {
		t.Fatal(err)
	}
	if st != "resolved" {
		t.Fatalf("expected insight resolved after Pod re-eval without host namespaces, got status=%q", st)
	}
}
