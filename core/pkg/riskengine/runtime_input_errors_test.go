package riskengine

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRuntimeInputFailureReachesEvaluators(t *testing.T) {
	for _, table := range []string{"runtime_signals", "pod_capabilities", "role_bindings", "cluster_role_bindings"} {
		t.Run(table, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			configureRiskEngineTestDB(t, db)
			if err := db.AutoMigrate(&models.Pod{}, &models.AssetSecurityState{}, &models.RuntimeSignal{}, &models.PodCapability{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.Role{}, &models.ClusterRole{}); err != nil {
				t.Fatal(err)
			}
			const uid = "runtime-input-error-pod"
			if err := db.Create(&models.Pod{UID: uid, Name: "p", ClusterID: "c", Namespace: "ns", ServiceAccount: "sa"}).Error; err != nil {
				t.Fatal(err)
			}
			// A stale clean snapshot must not turn a failed refresh into successful evaluation.
			old := time.Now().Add(-time.Hour)
			if err := db.Create(&models.AssetSecurityState{PodUID: uid, AssetType: "pod", RuntimeSignalsByType: "{}", EffectiveCapabilities: "[]", CreatedAt: old, UpdatedAt: old}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Migrator().DropTable(table); err != nil {
				t.Fatal(err)
			}
			engine := &Engine{db: db}
			yaml := &YAMLEngine{Engine: engine}
			data := map[string]interface{}{"uid": uid}
			for name, evaluate := range map[string]func() error{
				"base":    func() error { _, err := engine.EvaluateResource(context.Background(), "Pod", data); return err },
				"yaml":    func() error { _, err := yaml.EvaluateResource(context.Background(), "Pod", data); return err },
				"runtime": func() error { _, err := yaml.EvaluatePodRuntimeOnly(context.Background(), data); return err },
			} {
				if err := evaluate(); err == nil {
					t.Fatalf("%s accepted unavailable %s", name, table)
				}
			}
			var after models.AssetSecurityState
			if err := db.Where("pod_uid = ?", uid).First(&after).Error; err != nil {
				t.Fatal(err)
			}
			if !after.UpdatedAt.Equal(old) {
				t.Fatal("failed projection replaced previous snapshot")
			}
		})
	}
}

func TestRuntimeInputRejectsCorruptSnapshot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	configureRiskEngineTestDB(t, db)
	if err := db.AutoMigrate(&models.AssetSecurityState{}); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"runtime_signals_by_type", "effective_capabilities"} {
		t.Run(field, func(t *testing.T) {
			state := models.AssetSecurityState{PodUID: field, AssetType: "pod", RuntimeSignalsByType: "{}", EffectiveCapabilities: "[]", UpdatedAt: time.Now()}
			if err := db.Create(&state).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&state).UpdateColumn(field, "broken-json").Error; err != nil {
				t.Fatal(err)
			}
			e := &Engine{db: db}
			if _, err := e.EvaluateResource(context.Background(), "Pod", map[string]interface{}{"uid": field}); err == nil {
				t.Fatal("corrupt snapshot accepted")
			}
		})
	}
}

func TestRuntimeInputRejectsMalformedBindings(t *testing.T) {
	for _, subjects := range []string{"broken-json", `{"kind":"ServiceAccount"}`} {
		t.Run(subjects, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			configureRiskEngineTestDB(t, db)
			if err := db.AutoMigrate(&models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.Role{}, &models.ClusterRole{}); err != nil {
				t.Fatal(err)
			}
			binding := models.RoleBinding{UID: "bad-binding", ClusterID: "c", Namespace: "ns", Name: "bad", Subjects: subjects, RoleRef: `{"kind":"ClusterRole","name":"cluster-admin"}`}
			if err := db.Create(&binding).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := clusterAdminBindingForPod(context.Background(), db, "c", "ns", "sa"); err == nil {
				t.Fatal("malformed binding treated as absent")
			}
		})
	}
}
