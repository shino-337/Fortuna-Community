package riskengine

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	puresqlite "github.com/glebarez/sqlite"
	cgosqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAssetSecurityStateRuntimeTimestamp(t *testing.T) {
	for _, driver := range []struct {
		name string
		open func(string) gorm.Dialector
	}{
		{"purego", puresqlite.Open},
		{"cgo", cgosqlite.Open},
	} {
		t.Run(driver.name, func(t *testing.T) {
			for _, scenario := range []string{"no signals", "created fallback", "last seen", "query failure"} {
				t.Run(scenario, func(t *testing.T) {
					db, err := gorm.Open(driver.open(":memory:"), &gorm.Config{})
					if err != nil {
						t.Fatal(err)
					}
					configureRiskEngineTestDB(t, db)
					if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeSignal{}, &models.AssetSecurityState{}, &models.PodCapability{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.Role{}, &models.ClusterRole{}); err != nil {
						t.Fatal(err)
					}
					const uid = "timestamp-pod"
					if err := db.Create(&models.Pod{UID: uid, Name: "test", Namespace: "ns", ClusterID: "c1", ServiceAccount: "sa"}).Error; err != nil {
						t.Fatal(err)
					}
					now := time.Now().UTC().Truncate(time.Second)
					want := now.Add(-time.Hour)
					if scenario == "created fallback" || scenario == "last seen" {
						signal := models.RuntimeSignal{PodUID: uid, SignalType: "NETWORK_QUEUE_ANOMALY", Category: "NETWORK", Evidence: "{}", CreatedAt: want}
						if scenario == "last seen" {
							want = now.Add(-time.Minute)
							seen := want.In(time.FixedZone("UTC+7", 7*3600)).Format(time.RFC3339)
							signal.LastSeenAt = &seen
						}
						if err := db.Create(&signal).Error; err != nil {
							t.Fatal(err)
						}
					}
					if scenario == "query failure" {
						if err := db.Migrator().DropTable(&models.RuntimeSignal{}); err != nil {
							t.Fatal(err)
						}
					}
					engine := &Engine{db: db}
					err = engine.UpsertAssetSecurityState(context.Background(), uid)
					if scenario == "query failure" {
						if err == nil {
							t.Fatal("expected query error")
						}
						var count int64
						if err := db.Model(&models.AssetSecurityState{}).Count(&count).Error; err != nil {
							t.Fatal(err)
						}
						if count != 0 {
							t.Fatal("persisted incomplete security state")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					var state models.AssetSecurityState
					if err := db.Where("pod_uid = ?", uid).First(&state).Error; err != nil {
						t.Fatal(err)
					}
					if scenario == "no signals" {
						if state.LastRuntimeActivityAt != nil {
							t.Fatalf("expected nil activity, got %v", state.LastRuntimeActivityAt)
						}
					} else if state.LastRuntimeActivityAt == nil || !state.LastRuntimeActivityAt.Equal(want) {
						t.Fatalf("activity=%v, want %v", state.LastRuntimeActivityAt, want)
					}
				})
			}
		})
	}
}
