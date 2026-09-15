package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func reconciliationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.AuditLog{}, &models.Insight{}, &models.ServiceAccount{}, &models.Role{}, &models.ClusterRole{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.Pod{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHistoricalEvaluationReportsQueryFailures(t *testing.T) {
	db := reconciliationTestDB(t)
	// Empty resource tables require no evaluator; this isolates query/error handling.
	e := &HistoricalRiskEvaluator{db: db}
	if err := e.EvaluateAllResources(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&models.Role{}); err != nil {
		t.Fatal(err)
	}
	if err := e.EvaluateAllResources(context.Background()); err == nil {
		t.Fatal("missing table reported as success")
	}
}

func TestReconciliationDoesNotEvaluateSameNameReplacement(t *testing.T) {
	db := reconciliationTestDB(t)
	for _, row := range []any{
		&models.ServiceAccount{UID: "other-sa", Name: "shared", Namespace: "shared", ClusterID: "b"},
		&models.Role{UID: "other-role", Name: "shared", Namespace: "shared", ClusterID: "b"},
		&models.ClusterRole{UID: "other-cr", Name: "shared", ClusterID: "b"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// The original UID is absent. The updater must resolve that deleted identity
	// without evaluating a same-name object from another cluster.
	u := &InsightStatusUpdater{db: db}
	for _, kind := range []string{"ServiceAccount", "Role", "ClusterRole"} {
		insight := &models.Insight{ResourceType: kind, ResourceUID: "deleted-" + kind, ResourceName: "shared", ResourceNamespace: "shared", InsightType: "rbac", Status: "active", Title: "old"}
		if err := db.Create(insight).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := u.UpdateStatusForResolvedRisks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rows []models.Insight
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Status != "resolved" || row.ResolvedAt == nil {
			t.Fatalf("reconciliation: %+v", row)
		}
	}
}

func TestReconciliationPreservesUnknownAndFailedResources(t *testing.T) {
	db := reconciliationTestDB(t)
	for _, row := range []models.Insight{
		{ResourceType: "Pod", ResourceUID: "", Status: "active", InsightType: "security", Title: "unknown"},
		{ResourceType: "Role", ResourceUID: "failed", Status: "acknowledged", InsightType: "rbac", Title: "failed"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrator().DropTable(&models.Role{}); err != nil {
		t.Fatal(err)
	}
	u := &InsightStatusUpdater{db: db}
	if err := u.UpdateStatusForResolvedRisks(context.Background()); err == nil {
		t.Fatal("incomplete reconciliation reported success")
	}
	var rows []models.Insight
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Status == "resolved" || row.ResolvedAt != nil {
			t.Fatalf("failed resource resolved: %+v", row)
		}
	}
}

func TestReconciliationAuditRollbackAndCatalogFailure(t *testing.T) {
	db := reconciliationTestDB(t)
	row := models.Insight{ResourceType: "Role", ResourceUID: "deleted-role", Status: "active", InsightType: "rbac", Title: "old risk"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	u := &InsightStatusUpdater{db: db}
	if err := u.UpdateStatusForResolvedRisks(context.Background()); err == nil {
		t.Fatal("audit failure must abort resolution")
	}
	var current models.Insight
	if err := db.First(&current, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "active" || current.ResolvedAt != nil {
		t.Fatalf("failed transaction changed finding: %+v", current)
	}
	t.Setenv("FORTUNA_RULES_DIR", t.TempDir())
	if err := NewInsightStatusUpdater(db).UpdateStatusForResolvedRisks(context.Background()); err == nil {
		t.Fatal("empty catalog should prevent reconciliation")
	}
	if err := NewHistoricalRiskEvaluator(db).EvaluateAllResources(context.Background()); err == nil {
		t.Fatal("empty catalog should prevent historical evaluation")
	}
}

func TestReconciliationPreservesDisabledDetector(t *testing.T) {
	db := reconciliationTestDB(t)
	dir := t.TempDir()
	t.Setenv("FORTUNA_RULES_DIR", dir)
	rule := []byte("id: disabled\nname: Disabled detector\ncategory: rbac\nseverity: high\nenabled: false\nbase_score: 5\nconditions:\n  - type: expression\n    expression: 'false'\n")
	if err := os.WriteFile(filepath.Join(dir, "rule.yaml"), rule, 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Role{UID: "existing", Name: "role", Namespace: "ns", ClusterID: "a", Rules: "[]"}).Error; err != nil {
		t.Fatal(err)
	}
	finding := models.Insight{ResourceType: "Role", ResourceUID: "existing", ResourceName: "role", ResourceNamespace: "ns", CVEID: "disabled", Title: "Disabled detector", InsightType: "rbac", Status: "active"}
	if err := db.Create(&finding).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewInsightStatusUpdater(db).UpdateStatusForResolvedRisks(context.Background()); err == nil {
		t.Fatal("disabled detector reported a clean evaluation")
	}
	var actual models.Insight
	if err := db.First(&actual, finding.ID).Error; err != nil {
		t.Fatal(err)
	}
	if actual.Status != "active" || actual.ResolvedAt != nil {
		t.Fatal("disabled detector resolved finding")
	}
}

func TestReconciliationPreservesFindingOnRuntimeInputFailure(t *testing.T) {
	db := reconciliationTestDB(t)
	dir := t.TempDir()
	t.Setenv("FORTUNA_RULES_DIR", dir)
	rule := []byte("id: runtime-input-test\nname: Runtime input test\ncategory: runtime\nseverity: high\nenabled: true\nbase_score: 5\nconditions:\n  - type: expression\n    expression: 'false'\n")
	if err := os.WriteFile(filepath.Join(dir, "rule.yaml"), rule, 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Pod{UID: "existing-pod", Name: "pod", Namespace: "ns", ClusterID: "a", ServiceAccount: "sa", Containers: "[]"}).Error; err != nil {
		t.Fatal(err)
	}
	finding := models.Insight{ResourceType: "Pod", ResourceUID: "existing-pod", ResourceName: "pod", ResourceNamespace: "ns", CVEID: "runtime-input-test", Title: "Runtime input test", InsightType: "runtime", Status: "active"}
	if err := db.Create(&finding).Error; err != nil {
		t.Fatal(err)
	}
	// The detector is enabled and would return no findings. Required runtime tables
	// are absent, so reconciliation must fail before treating that as remediation.
	if err := NewInsightStatusUpdater(db).UpdateStatusForResolvedRisks(context.Background()); err == nil {
		t.Fatal("unavailable runtime evidence reported success")
	}
	var actual models.Insight
	if err := db.First(&actual, finding.ID).Error; err != nil {
		t.Fatal(err)
	}
	if actual.Status != "active" || actual.ResolvedAt != nil {
		t.Fatal("runtime failure resolved finding")
	}
	var audits int64
	if err := db.Model(&models.AuditLog{}).Where("action = ?", "auto_resolve").Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if audits != 0 {
		t.Fatal("runtime failure produced resolution audit")
	}
}
