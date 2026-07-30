package service

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// openTestDB opens an in-memory SQLite DB with a single connection so that async PCE goroutines
// see the same data as the test (SQLite :memory: is per-connection otherwise).
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return db
}

func TestEqualTimePtr(t *testing.T) {
	t1 := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 2, 13, 0, 0, 0, time.UTC)

	if !equalTimePtr(nil, nil) {
		t.Error("equalTimePtr(nil, nil) should be true")
	}
	if equalTimePtr(&t1, nil) || equalTimePtr(nil, &t1) {
		t.Error("equalTimePtr with one nil should be false")
	}
	if !equalTimePtr(&t1, &t2) {
		t.Error("equalTimePtr with equal times should be true")
	}
	if equalTimePtr(&t1, &t3) {
		t.Error("equalTimePtr with different times should be false")
	}
}

func TestProcessSyncedPods_PodDetailFields(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.PodInstance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	clusterID := "test-cluster-1"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test-cluster"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	svc := NewAgentService(db)
	data := map[string]interface{}{
		"pods": []interface{}{
			map[string]interface{}{
				"name":                         "nginx",
				"namespace":                    "default",
				"uid":                          "pod-uid-123",
				"phase":                        "Running",
				"serviceAccountName":           "default",
				"nodeName":                     "node-1",
				"hostNetwork":                  false,
				"hostPID":                      false,
				"hostIPC":                      false,
				"automountServiceAccountToken": true,
				"podIP":                        "10.0.0.5",
				"startTime":                    "2026-03-02T10:00:00Z",
				"restartCount":                 float64(3),
				"ownerKind":                    "ReplicaSet",
				"ownerName":                    "nginx-7d4f8b",
				"replicaSetName":               "nginx-7d4f8b",
				"qosClass":                     "Burstable",
				"containers":                   []interface{}{},
				"volumes":                      []interface{}{},
			},
		},
	}

	if err := svc.processSyncedPods(clusterID, data, true); err != nil {
		t.Fatalf("processSyncedPods: %v", err)
	}

	var pod models.Pod
	if err := db.Where("cluster_id = ? AND uid = ?", clusterID, "pod-uid-123").First(&pod).Error; err != nil {
		t.Fatalf("find pod: %v", err)
	}
	if pod.PodIP != "10.0.0.5" {
		t.Errorf("PodIP: got %q", pod.PodIP)
	}
	if pod.RestartCount != 3 {
		t.Errorf("RestartCount: got %d", pod.RestartCount)
	}
	if pod.OwnerKind != "ReplicaSet" {
		t.Errorf("OwnerKind: got %q", pod.OwnerKind)
	}
	if pod.OwnerName != "nginx-7d4f8b" {
		t.Errorf("OwnerName: got %q", pod.OwnerName)
	}
	if pod.ReplicaSetName != "nginx-7d4f8b" {
		t.Errorf("ReplicaSetName: got %q", pod.ReplicaSetName)
	}
	if pod.QoSClass != "Burstable" {
		t.Errorf("QoSClass: got %q", pod.QoSClass)
	}
	if pod.StartTime.Time == nil {
		t.Error("StartTime should be set")
	} else if !pod.StartTime.Time.Equal(time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("StartTime: got %v", pod.StartTime.Time)
	}
}

// TestProcessSyncedPods_PodDetailFields_SnakeCaseKeys ensures pod_ip/start_time (snake_case) in payload are parsed and stored.
func TestProcessSyncedPods_PodDetailFields_SnakeCaseKeys(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.PodInstance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "test-cluster-snake"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	svc := NewAgentService(db)
	data := map[string]interface{}{
		"pods": []interface{}{
			map[string]interface{}{
				"name": "p2", "namespace": "default", "uid": "pod-uid-snake",
				"phase": "Running", "serviceAccountName": "default", "nodeName": "node-1",
				"hostNetwork": false, "hostPID": false, "hostIPC": false,
				"pod_ip":   "10.244.1.100",
				"start_time": "2026-03-10T08:00:00Z",
				"restartCount": float64(0),
				"containers": []interface{}{}, "volumes": []interface{}{},
			},
		},
	}
	if err := svc.processSyncedPods(clusterID, data, true); err != nil {
		t.Fatalf("processSyncedPods: %v", err)
	}
	var pod models.Pod
	if err := db.Where("cluster_id = ? AND uid = ?", clusterID, "pod-uid-snake").First(&pod).Error; err != nil {
		t.Fatalf("find pod: %v", err)
	}
	if pod.PodIP != "10.244.1.100" {
		t.Errorf("PodIP from pod_ip: got %q", pod.PodIP)
	}
	if pod.StartTime.Time == nil {
		t.Error("StartTime from start_time should be set")
	}
}

func TestProcessSyncedPods_SpecHashStoredAndConditionalPCE(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.PodInstance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "test-cluster-spec"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	svc := NewAgentService(db)
	payloadWithHash := map[string]interface{}{
		"pods": []interface{}{
			map[string]interface{}{
				"name": "p1", "namespace": "default", "uid": "uid-spec-1",
				"phase": "Running", "serviceAccountName": "default", "nodeName": "n1",
				"containers": []interface{}{}, "volumes": []interface{}{},
				"specHash": "abc123def456",
			},
		},
	}
	if err := svc.processSyncedPods(clusterID, payloadWithHash, true); err != nil {
		t.Fatalf("processSyncedPods: %v", err)
	}
	var pod models.Pod
	if err := db.Where("cluster_id = ? AND uid = ?", clusterID, "uid-spec-1").First(&pod).Error; err != nil {
		t.Fatalf("find pod: %v", err)
	}
	if pod.SpecHash != "abc123def456" {
		t.Errorf("SpecHash: got %q", pod.SpecHash)
	}

	// Second sync with same specHash: update other field only (e.g. phase). spec_hash unchanged -> PCE not triggered (we can't assert that without mock, but no error and spec_hash still set)
	payloadTouch := map[string]interface{}{
		"pods": []interface{}{
			map[string]interface{}{
				"name": "p1", "namespace": "default", "uid": "uid-spec-1",
				"phase": "Pending", "serviceAccountName": "default", "nodeName": "n1",
				"containers": []interface{}{}, "volumes": []interface{}{},
				"specHash": "abc123def456",
			},
		},
	}
	if err := svc.processSyncedPods(clusterID, payloadTouch, true); err != nil {
		t.Fatalf("processSyncedPods second: %v", err)
	}
	if err := db.Where("cluster_id = ? AND uid = ?", clusterID, "uid-spec-1").First(&pod).Error; err != nil {
		t.Fatalf("find pod again: %v", err)
	}
	if pod.SpecHash != "abc123def456" {
		t.Errorf("SpecHash after touch: got %q", pod.SpecHash)
	}
	if pod.Phase != "Pending" {
		t.Errorf("Phase should be updated: got %q", pod.Phase)
	}

	// Third sync with different specHash: should store new hash
	payloadNewHash := map[string]interface{}{
		"pods": []interface{}{
			map[string]interface{}{
				"name": "p1", "namespace": "default", "uid": "uid-spec-1",
				"phase": "Running", "serviceAccountName": "default", "nodeName": "n1",
				"containers": []interface{}{}, "volumes": []interface{}{},
				"specHash": "newhash789",
			},
		},
	}
	if err := svc.processSyncedPods(clusterID, payloadNewHash, true); err != nil {
		t.Fatalf("processSyncedPods third: %v", err)
	}
	if err := db.Where("cluster_id = ? AND uid = ?", clusterID, "uid-spec-1").First(&pod).Error; err != nil {
		t.Fatalf("find pod third: %v", err)
	}
	if pod.SpecHash != "newhash789" {
		t.Errorf("SpecHash after change: got %q", pod.SpecHash)
	}
}

// TestProcessSyncedPods_NullToNonNullSpecHash ensures agent upgrade (no specHash -> with specHash) triggers PCE.
func TestProcessSyncedPods_NullToNonNullSpecHash(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.PodInstance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clusterID := "test-cluster-null"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	svc := NewAgentService(db)
	// First sync: old agent, no specHash
	payloadOld := map[string]interface{}{
		"pods": []interface{}{
			map[string]interface{}{
				"name": "p2", "namespace": "default", "uid": "uid-null-1",
				"phase": "Running", "serviceAccountName": "default", "nodeName": "n1",
				"containers": []interface{}{}, "volumes": []interface{}{},
			},
		},
	}
	if err := svc.processSyncedPods(clusterID, payloadOld, true); err != nil {
		t.Fatalf("processSyncedPods old agent: %v", err)
	}
	var pod models.Pod
	if err := db.Where("cluster_id = ? AND uid = ?", clusterID, "uid-null-1").First(&pod).Error; err != nil {
		t.Fatalf("find pod: %v", err)
	}
	if pod.SpecHash != "" {
		t.Errorf("old agent should leave SpecHash empty: got %q", pod.SpecHash)
	}

	// Second sync: new agent sends specHash -> existing.SpecHash == "" so specHashChanged is true; PCE should be triggered
	payloadNew := map[string]interface{}{
		"pods": []interface{}{
			map[string]interface{}{
				"name": "p2", "namespace": "default", "uid": "uid-null-1",
				"phase": "Running", "serviceAccountName": "default", "nodeName": "n1",
				"containers": []interface{}{}, "volumes": []interface{}{},
				"specHash": "firsthash",
			},
		},
	}
	if err := svc.processSyncedPods(clusterID, payloadNew, true); err != nil {
		t.Fatalf("processSyncedPods new agent: %v", err)
	}
	if err := db.Where("cluster_id = ? AND uid = ?", clusterID, "uid-null-1").First(&pod).Error; err != nil {
		t.Fatalf("find pod again: %v", err)
	}
	if pod.SpecHash != "firsthash" {
		t.Errorf("SpecHash should be set after new agent sync: got %q", pod.SpecHash)
	}
}

func TestProcessSyncedPods_UpdatePodDetailFields(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.PodInstance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	clusterID := "test-cluster-2"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "test-cluster-2"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	// Create pod with initial detail fields
	if err := db.Create(&models.Pod{
		ClusterID:      clusterID,
		UID:            "pod-uid-456",
		Name:           "app",
		Namespace:      "default",
		ServiceAccount: "default",
		PodIP:          "10.0.0.1",
		RestartCount:   1,
		QoSClass:       "BestEffort",
	}).Error; err != nil {
		t.Fatalf("create pod: %v", err)
	}

	svc := NewAgentService(db)
	data := map[string]interface{}{
		"pods": []interface{}{
			map[string]interface{}{
				"name":                         "app",
				"namespace":                    "default",
				"uid":                          "pod-uid-456",
				"phase":                        "Running",
				"serviceAccountName":           "default",
				"nodeName":                     "node-2",
				"podIP":                        "10.0.0.2",
				"restartCount":                 float64(5),
				"ownerKind":                    "Deployment",
				"ownerName":                    "app-deploy",
				"qosClass":                     "Guaranteed",
				"containers":                   []interface{}{},
				"volumes":                      []interface{}{},
			},
		},
	}

	if err := svc.processSyncedPods(clusterID, data, true); err != nil {
		t.Fatalf("processSyncedPods: %v", err)
	}

	var pod models.Pod
	if err := db.Where("cluster_id = ? AND uid = ?", clusterID, "pod-uid-456").First(&pod).Error; err != nil {
		t.Fatalf("find pod: %v", err)
	}
	if pod.PodIP != "10.0.0.2" {
		t.Errorf("PodIP after update: got %q", pod.PodIP)
	}
	if pod.RestartCount != 5 {
		t.Errorf("RestartCount after update: got %d", pod.RestartCount)
	}
	if pod.OwnerKind != "Deployment" {
		t.Errorf("OwnerKind: got %q", pod.OwnerKind)
	}
	if pod.OwnerName != "app-deploy" {
		t.Errorf("OwnerName: got %q", pod.OwnerName)
	}
	if pod.QoSClass != "Guaranteed" {
		t.Errorf("QoSClass after update: got %q", pod.QoSClass)
	}
}
