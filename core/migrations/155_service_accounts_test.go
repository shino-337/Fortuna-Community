package migrations

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

func serviceAccountsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.User{}, &models.SecurityActivityLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSystemUserLosesAdminOnlyWhenAnotherAdminExists(t *testing.T) {
	db := serviceAccountsDB(t)
	system := models.User{Username: "system", Email: "system@fortuna.local", Password: "x", Role: models.RoleAdmin, Active: true}
	if err := db.Create(&system).Error; err != nil {
		t.Fatal(err)
	}
	// Alone, it keeps admin so the platform stays administrable.
	if err := Migration155_SystemUserHasNoRole(db); err != nil {
		t.Fatal(err)
	}
	db.First(&system, system.ID)
	if system.Role != models.RoleAdmin {
		t.Fatalf("only active admin must be left unchanged, got %q", system.Role)
	}

	human := models.User{Username: "admin", Email: "admin@fortuna.local", Password: "x", Role: models.RoleAdmin, Active: true}
	if err := db.Create(&human).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migration155_SystemUserHasNoRole(db); err != nil {
		t.Fatal(err)
	}
	db.First(&system, system.ID)
	if system.Role != models.RoleSystem || system.Active {
		t.Fatalf("system user must lose admin and be disabled, got role %q active %t", system.Role, system.Active)
	}
	if perms := authorization.PermissionsForUser(system.Role); len(perms) != 0 {
		t.Fatalf("system role must have no permissions, got %v", perms)
	}
	db.First(&human, human.ID)
	if human.Role != models.RoleAdmin || !human.Active {
		t.Fatal("the human admin must not change")
	}
}

func TestEnsureRiskEvaluatorAccount(t *testing.T) {
	db := serviceAccountsDB(t)
	t.Setenv("FORTUNA_RISK_EVALUATOR_PASSWORD", "")
	if err := EnsureRiskEvaluatorAccount(db); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&models.User{}).Count(&n)
	if n != 0 {
		t.Fatal("no account without FORTUNA_RISK_EVALUATOR_PASSWORD")
	}

	t.Setenv("FORTUNA_RISK_EVALUATOR_PASSWORD", "Fr1-ServiceAccountPass12345")
	if err := EnsureRiskEvaluatorAccount(db); err != nil {
		t.Fatal(err)
	}
	var u models.User
	if err := db.Where("username = ?", DefaultRiskEvaluatorUsername).First(&u).Error; err != nil {
		t.Fatal(err)
	}
	perms := authorization.PermissionsForUser(u.Role)
	if len(perms) != 2 || !authorization.HasPermission(perms, authorization.PermissionRiskEvaluate) {
		t.Fatalf("risk evaluator must hold auth.session and risk.evaluate only, got %v", perms)
	}
	if authorization.ParseScopeDocument(u.ScopeJSON).RestrictsClusters() {
		t.Fatal("global evaluation needs an unrestricted scope")
	}

	// A rotated password is synced; the hash is compared, not replaced each start.
	t.Setenv("FORTUNA_RISK_EVALUATOR_PASSWORD", "Fr1-RotatedServicePass12345")
	if err := EnsureRiskEvaluatorAccount(db); err != nil {
		t.Fatal(err)
	}
	db.First(&u, u.ID)
	if !auth.CheckPasswordHash("Fr1-RotatedServicePass12345", u.Password) {
		t.Fatal("rotated password must be synced")
	}

	// An account of another role with that name is never taken over.
	db.Model(&models.User{}).Where("id = ?", u.ID).Update("role", models.RoleOperator)
	t.Setenv("FORTUNA_RISK_EVALUATOR_PASSWORD", "Fr1-ThirdServicePass12345")
	if err := EnsureRiskEvaluatorAccount(db); err != nil {
		t.Fatal(err)
	}
	db.First(&u, u.ID)
	if u.Role != models.RoleOperator || auth.CheckPasswordHash("Fr1-ThirdServicePass12345", u.Password) {
		t.Fatal("a human account with the service name must be left alone")
	}
}
