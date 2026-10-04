package api

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// A demotion that passed the early count must still be rolled back when a
// concurrent change removed the other admin before this write commits.
func TestSaveUserKeepingAnAdminRollsBackWhenNoActiveAdminRemains(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	a := models.User{Username: "a", Email: "a@test.local", Password: "x", Role: models.RoleAdmin, Active: true}
	b := models.User{Username: "b", Email: "b@test.local", Password: "x", Role: models.RoleAdmin, Active: true}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	before := a
	// Early check saw two active admins; meanwhile b is demoted.
	if err := db.Model(&b).Update("role", models.RoleViewer).Error; err != nil {
		t.Fatal(err)
	}
	a.Role = models.RoleViewer
	err = saveUserKeepingAnAdmin(db, before, func(tx *gorm.DB) error { return tx.Save(&a).Error })
	if !errors.Is(err, errLastActiveAdmin) {
		t.Fatalf("want errLastActiveAdmin, got %v", err)
	}
	var stored models.User
	if err := db.First(&stored, a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Role != models.RoleAdmin {
		t.Fatalf("demotion must be rolled back, role is %q", stored.Role)
	}

	// A change that does not touch an active admin is never blocked.
	viewer := models.User{Username: "v", Email: "v@test.local", Password: "x", Role: models.RoleViewer, Active: true}
	if err := db.Create(&viewer).Error; err != nil {
		t.Fatal(err)
	}
	beforeViewer := viewer
	viewer.Active = false
	if err := saveUserKeepingAnAdmin(db, beforeViewer, func(tx *gorm.DB) error { return tx.Save(&viewer).Error }); err != nil {
		t.Fatalf("non-admin change: %v", err)
	}
}
