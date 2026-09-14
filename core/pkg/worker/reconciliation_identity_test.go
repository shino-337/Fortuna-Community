package worker

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func reconciliationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	sqlDB, err := db.DB()
	if err != nil { t.Fatal(err) }
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.Insight{}, &models.ServiceAccount{}, &models.Role{}, &models.ClusterRole{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.Pod{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHistoricalEvaluationReportsQueryFailures(t *testing.T) {
	db := reconciliationTestDB(t)
	// Empty resource tables require no evaluator; this isolates query/error handling.
	e := &HistoricalRiskEvaluator{db: db}
	if err := e.EvaluateAllResources(context.Background()); err != nil { t.Fatal(err) }
	if err := db.Migrator().DropTable(&models.Role{}); err != nil { t.Fatal(err) }
	if err := e.EvaluateAllResources(context.Background()); err == nil { t.Fatal("missing table reported as success") }
}

func TestReconciliationDoesNotEvaluateSameNameReplacement(t *testing.T) {
	db := reconciliationTestDB(t)
	for _, row := range []any{
		&models.ServiceAccount{UID: "other-sa", Name: "shared", Namespace: "shared", ClusterID: "b"},
		&models.Role{UID: "other-role", Name: "shared", Namespace: "shared", ClusterID: "b"},
		&models.ClusterRole{UID: "other-cr", Name: "shared", ClusterID: "b"},
	} {
		if err := db.Create(row).Error; err != nil { t.Fatal(err) }
	}
	// The original UID is absent. The updater must resolve that deleted identity
	// without evaluating a same-name object from another cluster.
	u := &InsightStatusUpdater{db: db}
	for _, kind := range []string{"ServiceAccount", "Role", "ClusterRole"} {
		insight := &models.Insight{ResourceType: kind, ResourceUID: "deleted-"+kind, ResourceName: "shared", ResourceNamespace: "shared", InsightType: "rbac", Status: "active", Title: "old"}
		if err := db.Create(insight).Error; err != nil { t.Fatal(err) }
	}
	if err := u.UpdateStatusForResolvedRisks(context.Background()); err != nil { t.Fatal(err) }
	var rows []models.Insight
	if err := db.Find(&rows).Error; err != nil { t.Fatal(err) }
	for _, row := range rows {
		if row.Status != "resolved" || row.ResolvedAt == nil { t.Fatalf("reconciliation: %+v", row) }
	}
}

func TestReconciliationPreservesUnknownAndFailedResources(t *testing.T) {
	db := reconciliationTestDB(t)
	for _, row := range []models.Insight{
		{ResourceType: "Pod", ResourceUID: "", Status: "active", InsightType: "security", Title: "unknown"},
		{ResourceType: "Role", ResourceUID: "failed", Status: "acknowledged", InsightType: "rbac", Title: "failed"},
	} {
		if err := db.Create(&row).Error; err != nil { t.Fatal(err) }
	}
	if err := db.Migrator().DropTable(&models.Role{}); err != nil { t.Fatal(err) }
	u := &InsightStatusUpdater{db: db}
	if err := u.UpdateStatusForResolvedRisks(context.Background()); err == nil { t.Fatal("incomplete reconciliation reported success") }
	var rows []models.Insight
	if err := db.Find(&rows).Error; err != nil { t.Fatal(err) }
	for _, row := range rows {
		if row.Status == "resolved" || row.ResolvedAt != nil { t.Fatalf("failed resource resolved: %+v", row) }
	}
}
