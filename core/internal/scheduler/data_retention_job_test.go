package scheduler

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func newRetentionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.K8sEvent{}, &models.UserSession{},
		&models.AuditLog{}, &models.InvestigationCase{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newTestRetentionJob(db *gorm.DB, now time.Time) *DataRetentionJob {
	ctx, cancel := context.WithCancel(context.Background())
	return &DataRetentionJob{db: db, rules: DefaultRetentionRules(), interval: time.Hour,
		now: func() time.Time { return now }, ctx: ctx, cancel: cancel}
}

func TestDataRetentionDeletesOnlyExpiredRows(t *testing.T) {
	db := newRetentionTestDB(t)
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	old, recent := now.AddDate(0, 0, -45), now.AddDate(0, 0, -2)

	for _, at := range []time.Time{old, recent} {
		at := at
		db.Create(&models.RuntimeEvent{PodUID: "p", Namespace: "n", Syscall: "execve", ObservedAt: &at})
	}
	// No observed_at: falls back to created_at.
	db.Create(&models.RuntimeEvent{PodUID: "p", Namespace: "n", Syscall: "execve", CreatedAt: old})
	db.Create(&models.K8sEvent{ClusterID: "c", EventUID: "e1", InvolvedUID: "u", LastTimestamp: &old})
	db.Create(&models.UserSession{ID: "s-old", UserID: 1, ExpiresAt: old})
	db.Create(&models.UserSession{ID: "s-new", UserID: 1, ExpiresAt: now.Add(time.Hour)})
	db.Create(&models.AuditLog{Action: "update", CreatedAt: now.AddDate(0, 0, -100)})
	db.Create(&models.AuditLog{Action: "update", CreatedAt: recent})

	newTestRetentionJob(db, now).RunOnce()

	var n int64
	db.Model(&models.RuntimeEvent{}).Count(&n)
	if n != 1 {
		t.Fatalf("runtime_events left = %d, want 1", n)
	}
	db.Model(&models.K8sEvent{}).Count(&n)
	if n != 0 {
		t.Fatalf("k8s_events left = %d, want 0", n)
	}
	var sessions []models.UserSession
	db.Find(&sessions)
	if len(sessions) != 1 || sessions[0].ID != "s-new" {
		t.Fatalf("sessions left = %+v, want only s-new", sessions)
	}
	db.Model(&models.AuditLog{}).Count(&n)
	if n != 1 {
		t.Fatalf("audit_logs left = %d, want 1", n)
	}
}

func TestDataRetentionPurgesArchivedCasesPastDeadlineOnly(t *testing.T) {
	db := newRetentionTestDB(t)
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.AddDate(1, 0, 0)

	db.Create(&models.InvestigationCase{ID: "archived-expired", Title: "a", RetentionUntil: &past})
	db.Create(&models.InvestigationCase{ID: "archived-kept", Title: "b", RetentionUntil: &future})
	db.Create(&models.InvestigationCase{ID: "open", Title: "c", RetentionUntil: &past})
	db.Where("id IN ?", []string{"archived-expired", "archived-kept"}).Delete(&models.InvestigationCase{})

	newTestRetentionJob(db, now).RunOnce()

	var ids []string
	db.Unscoped().Model(&models.InvestigationCase{}).Order("id").Pluck("id", &ids)
	if len(ids) != 2 || ids[0] != "archived-kept" || ids[1] != "open" {
		t.Fatalf("cases left = %v, want [archived-kept open]", ids)
	}
}

func TestDataRetentionScrubsDeletedUserPasswords(t *testing.T) {
	db := newRetentionTestDB(t)
	db.Create(&models.User{Username: "gone", Email: "g@x", Password: "hash1", Role: models.RoleViewer})
	db.Create(&models.User{Username: "live", Email: "l@x", Password: "hash2", Role: models.RoleViewer})
	db.Where("username = ?", "gone").Delete(&models.User{})

	newTestRetentionJob(db, time.Now()).RunOnce()

	var gone, live models.User
	db.Unscoped().Where("username = ?", "gone").First(&gone)
	db.Where("username = ?", "live").First(&live)
	if gone.Password != "" || live.Password != "hash2" {
		t.Fatalf("passwords = %q / %q, want empty / hash2", gone.Password, live.Password)
	}
}

func TestRetentionEnvOverride(t *testing.T) {
	t.Setenv("FORTUNA_RETENTION_RUNTIME_DAYS", "0")
	t.Setenv("FORTUNA_RETENTION_AUDIT_LOG_DAYS", "365")
	j := NewDataRetentionJob(nil)
	defer j.Stop()
	for _, r := range j.rules {
		switch r.Table {
		case "runtime_events":
			if r.Days != 0 {
				t.Fatalf("runtime days = %d, want 0 (disabled)", r.Days)
			}
		case "audit_logs":
			if r.Days != 365 {
				t.Fatalf("audit days = %d, want 365", r.Days)
			}
		}
	}
}
