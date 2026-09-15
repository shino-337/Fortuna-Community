package riskengine

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func TestClusterAdminBindingForPod_Positive(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Cluster{ID: "c1", Name: "c1"}).Error; err != nil {
		t.Fatal(err)
	}
	rb := models.RoleBinding{
		ClusterID: "c1", Name: "sa-admin", Namespace: "app", UID: "rb-uid-1",
		RoleRef:  `{"kind":"ClusterRole","name":"cluster-admin"}`,
		Subjects: `[{"kind":"ServiceAccount","name":"workload-sa","namespace":"app"}]`,
	}
	if err := db.Create(&rb).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if matched, err := clusterAdminBindingForPod(ctx, db, "c1", "app", "workload-sa"); err != nil || !matched {
		t.Fatal("expected cluster-admin binding")
	}
	if matched, err := clusterAdminBindingForPod(ctx, db, "c1", "app", "other-sa"); err != nil || matched {
		t.Fatal("unexpected match for other SA")
	}
}

func TestClusterAdminBindingForPod_RequiresClusterRoleKind(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Cluster{ID: "c1", Name: "c1"}).Error; err != nil {
		t.Fatal(err)
	}
	rb := models.RoleBinding{
		ClusterID: "c1", Name: "bad", Namespace: "app", UID: "rb-2",
		RoleRef:  `{"kind":"Role","name":"cluster-admin"}`,
		Subjects: `[{"kind":"ServiceAccount","name":"x","namespace":"app"}]`,
	}
	if err := db.Create(&rb).Error; err != nil {
		t.Fatal(err)
	}
	if matched, err := clusterAdminBindingForPod(context.Background(), db, "c1", "app", "x"); err != nil || matched {
		t.Fatal("namespaced Role named cluster-admin must not count as cluster ClusterRole binding")
	}
}
