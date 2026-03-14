package service

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
)

// TestProcessSyncedPods_EmptyPayloadNoDelete ensures full sync with data["pods"] missing or empty
// does NOT delete existing pods (Finding #3: avoid wipe on transient collection failure).
func TestProcessSyncedPods_EmptyPayloadNoDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.PodInstance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "sync-empty-pods"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	// Pre-seed one pod
	if err := db.Create(&models.Pod{ClusterID: clusterID, UID: "pod-1", Name: "p", Namespace: "default", ServiceAccount: "default"}).Error; err != nil {
		t.Fatalf("create pod: %v", err)
	}

	svc := NewAgentService(db)

	// Case 1: key missing (nil)
	dataNil := map[string]interface{}{}
	if err := svc.processSyncedPods(clusterID, dataNil, true); err != nil {
		t.Fatalf("processSyncedPods(nil): %v", err)
	}
	var count int64
	if err := db.Model(&models.Pod{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count pods: %v", err)
	}
	if count != 1 {
		t.Errorf("after nil payload: expected 1 pod, got %d (should not delete)", count)
	}

	// Case 2: key present but empty slice
	dataEmpty := map[string]interface{}{"pods": []interface{}{}}
	if err := svc.processSyncedPods(clusterID, dataEmpty, true); err != nil {
		t.Fatalf("processSyncedPods(empty): %v", err)
	}
	if err := db.Model(&models.Pod{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count pods: %v", err)
	}
	if count != 1 {
		t.Errorf("after empty pods slice: expected 1 pod, got %d (should not delete)", count)
	}
}

// TestProcessSyncedServiceAccounts_EmptyPayloadNoDelete ensures full sync with data["serviceAccounts"] missing or empty does NOT delete existing SAs.
func TestProcessSyncedServiceAccounts_EmptyPayloadNoDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.ServiceAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "sync-empty-sa"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if err := db.Create(&models.ServiceAccount{ClusterID: clusterID, UID: "sa-1", Name: "default", Namespace: "default"}).Error; err != nil {
		t.Fatalf("create SA: %v", err)
	}

	svc := NewAgentService(db)

	dataNil := map[string]interface{}{}
	if err := svc.processSyncedServiceAccounts(clusterID, dataNil, true, false); err != nil {
		t.Fatalf("processSyncedServiceAccounts(nil): %v", err)
	}
	var count int64
	if err := db.Model(&models.ServiceAccount{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count SAs: %v", err)
	}
	if count != 1 {
		t.Errorf("after nil payload: expected 1 SA, got %d", count)
	}

	dataEmpty := map[string]interface{}{"serviceAccounts": []interface{}{}}
	if err := svc.processSyncedServiceAccounts(clusterID, dataEmpty, true, false); err != nil {
		t.Fatalf("processSyncedServiceAccounts(empty): %v", err)
	}
	if err := db.Model(&models.ServiceAccount{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count SAs: %v", err)
	}
	if count != 1 {
		t.Errorf("after empty serviceAccounts slice: expected 1 SA, got %d", count)
	}
}

// TestProcessSyncedRoles_EmptyPayloadNoDelete ensures full sync with data["roles"] missing or empty does NOT delete existing roles.
func TestProcessSyncedRoles_EmptyPayloadNoDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.Role{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "sync-empty-roles"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if err := db.Create(&models.Role{ClusterID: clusterID, UID: "role-1", Name: "r1", Namespace: "default", Rules: "[]"}).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	svc := NewAgentService(db)

	dataNil := map[string]interface{}{}
	if err := svc.processSyncedRoles(clusterID, dataNil); err != nil {
		t.Fatalf("processSyncedRoles(nil): %v", err)
	}
	var count int64
	if err := db.Model(&models.Role{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 1 {
		t.Errorf("after nil payload: expected 1 role, got %d", count)
	}

	dataEmpty := map[string]interface{}{"roles": []interface{}{}}
	if err := svc.processSyncedRoles(clusterID, dataEmpty); err != nil {
		t.Fatalf("processSyncedRoles(empty): %v", err)
	}
	if err := db.Model(&models.Role{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 1 {
		t.Errorf("after empty roles slice: expected 1 role, got %d", count)
	}
}

// TestProcessSyncedClusterRoles_EmptyPayloadNoDelete ensures full sync with data["clusterRoles"] missing or empty does NOT delete existing cluster roles.
func TestProcessSyncedClusterRoles_EmptyPayloadNoDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.ClusterRole{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "sync-empty-cr"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if err := db.Create(&models.ClusterRole{ClusterID: clusterID, UID: "cr-1", Name: "admin", Rules: "[]"}).Error; err != nil {
		t.Fatalf("create cluster role: %v", err)
	}

	svc := NewAgentService(db)

	dataNil := map[string]interface{}{}
	if err := svc.processSyncedClusterRoles(clusterID, dataNil); err != nil {
		t.Fatalf("processSyncedClusterRoles(nil): %v", err)
	}
	var count int64
	if err := db.Model(&models.ClusterRole{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count cluster roles: %v", err)
	}
	if count != 1 {
		t.Errorf("after nil payload: expected 1 cluster role, got %d", count)
	}

	dataEmpty := map[string]interface{}{"clusterRoles": []interface{}{}}
	if err := svc.processSyncedClusterRoles(clusterID, dataEmpty); err != nil {
		t.Fatalf("processSyncedClusterRoles(empty): %v", err)
	}
	if err := db.Model(&models.ClusterRole{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count cluster roles: %v", err)
	}
	if count != 1 {
		t.Errorf("after empty clusterRoles slice: expected 1 cluster role, got %d", count)
	}
}

// TestProcessSyncedRoleBindings_EmptyPayloadNoDelete ensures full sync with data["roleBindings"] missing or empty does NOT delete existing role bindings.
func TestProcessSyncedRoleBindings_EmptyPayloadNoDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.RoleBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "sync-empty-rb"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if err := db.Create(&models.RoleBinding{ClusterID: clusterID, UID: "rb-1", Name: "rb1", Namespace: "default", RoleRef: "{}", Subjects: "[]"}).Error; err != nil {
		t.Fatalf("create role binding: %v", err)
	}

	svc := NewAgentService(db)

	dataNil := map[string]interface{}{}
	if err := svc.processSyncedRoleBindings(clusterID, dataNil); err != nil {
		t.Fatalf("processSyncedRoleBindings(nil): %v", err)
	}
	var count int64
	if err := db.Model(&models.RoleBinding{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count role bindings: %v", err)
	}
	if count != 1 {
		t.Errorf("after nil payload: expected 1 role binding, got %d", count)
	}

	dataEmpty := map[string]interface{}{"roleBindings": []interface{}{}}
	if err := svc.processSyncedRoleBindings(clusterID, dataEmpty); err != nil {
		t.Fatalf("processSyncedRoleBindings(empty): %v", err)
	}
	if err := db.Model(&models.RoleBinding{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count role bindings: %v", err)
	}
	if count != 1 {
		t.Errorf("after empty roleBindings slice: expected 1 role binding, got %d", count)
	}
}

// TestProcessSyncedClusterRoleBindings_EmptyPayloadNoDelete ensures full sync with data["clusterRoleBindings"] missing or empty does NOT delete existing cluster role bindings.
func TestProcessSyncedClusterRoleBindings_EmptyPayloadNoDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.ClusterRoleBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "sync-empty-crb"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if err := db.Create(&models.ClusterRoleBinding{ClusterID: clusterID, UID: "crb-1", Name: "admin-binding", RoleRef: "{}", Subjects: "[]"}).Error; err != nil {
		t.Fatalf("create cluster role binding: %v", err)
	}

	svc := NewAgentService(db)

	dataNil := map[string]interface{}{}
	if err := svc.processSyncedClusterRoleBindings(clusterID, dataNil); err != nil {
		t.Fatalf("processSyncedClusterRoleBindings(nil): %v", err)
	}
	var count int64
	if err := db.Model(&models.ClusterRoleBinding{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count cluster role bindings: %v", err)
	}
	if count != 1 {
		t.Errorf("after nil payload: expected 1 cluster role binding, got %d", count)
	}

	dataEmpty := map[string]interface{}{"clusterRoleBindings": []interface{}{}}
	if err := svc.processSyncedClusterRoleBindings(clusterID, dataEmpty); err != nil {
		t.Fatalf("processSyncedClusterRoleBindings(empty): %v", err)
	}
	if err := db.Model(&models.ClusterRoleBinding{}).Where("cluster_id = ?", clusterID).Count(&count).Error; err != nil {
		t.Fatalf("count cluster role bindings: %v", err)
	}
	if count != 1 {
		t.Errorf("after empty clusterRoleBindings slice: expected 1 cluster role binding, got %d", count)
	}
}
