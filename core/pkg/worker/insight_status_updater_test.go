package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/fortuna/core/pkg/models"
)

func TestInsightStatusUpdater_PodPreservedWithoutCollectionCoverage(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir:", rulesDir, err)
	}
	t.Setenv("FORTUNA_RULES_DIR", rulesDir)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AssetSecurityState{}, &models.RuntimeSignal{}, &models.PodCapability{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.AuditLog{}, &models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RiskScore{}, &models.Role{}, &models.ClusterRole{}); err != nil {
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
		ClusterID: "c-upd", ResourceType: "Pod", ResourceNamespace: "ns", ResourceName: "p", ResourceUID: podUID,
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
	if err := u.UpdateStatusForResolvedRisks(context.Background()); err == nil {
		t.Fatal("a changed Pod field without collection coverage must not prove remediation")
	}

	var st string
	if err := db.Model(&models.Insight{}).Where("id = ?", ins.ID).Select("status").Scan(&st).Error; err != nil {
		t.Fatal(err)
	}
	if st != "active" {
		t.Fatalf("expected finding preserved without collection coverage, got status=%q", st)
	}
}
