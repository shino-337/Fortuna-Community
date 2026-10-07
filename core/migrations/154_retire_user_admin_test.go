package migrations

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

func TestRetireUserAdminGrantsNoNewAccess(t *testing.T) {
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
	users := []models.User{
		{Username: "ua", Email: "ua@test.local", Password: "x", Role: "user_admin", ScopeJSON: "{}", Active: true},
		{Username: "op", Email: "op@test.local", Password: "x", Role: models.RoleOperator, ScopeJSON: "{}", Active: true},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := Migration154_RetireUserAdmin(db); err != nil {
			t.Fatal(err)
		}
	}

	var ua, op models.User
	db.Where("username = ?", "ua").First(&ua)
	db.Where("username = ?", "op").First(&op)
	if ua.Role != models.RoleViewer {
		t.Fatalf("user_admin must become viewer, got %q", ua.Role)
	}
	if doc := authorization.ParseScopeDocument(ua.ScopeJSON); !doc.GrantsNoCluster() {
		t.Fatalf("former user_admin must see no cluster, got scope %q", ua.ScopeJSON)
	}
	if op.Role != models.RoleOperator || op.ScopeJSON != "{}" {
		t.Fatalf("other accounts must not change: %+v", op)
	}
	var audits int64
	db.Model(&models.SecurityActivityLog{}).Where("action = ? AND resource_id = ?", "user_rbac_change", ua.ID).Count(&audits)
	if audits != 1 {
		t.Fatalf("want one audit row for the retired account, got %d", audits)
	}
}
