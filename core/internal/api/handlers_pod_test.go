package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func setupPodTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestGetPod_ReturnsPodDetailFields verifies GET /pods/by-id/:id returns POD_DETAIL_SPEC fields.
func TestGetPod_ReturnsPodDetailFields(t *testing.T) {
	db := setupPodTestDB(t)
	clusterID := "c1"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "cluster1"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	start := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	pod := models.Pod{
		ClusterID:      clusterID,
		UID:            "pod-uid-1",
		Name:           "nginx",
		Namespace:      "default",
		ServiceAccount: "default",
		Phase:          "Running",
		PodIP:          "10.0.0.5",
		StartTime:      models.NullTime{Time: &start},
		RestartCount:   3,
		OwnerKind:      "ReplicaSet",
		OwnerName:      "nginx-7d4f8b",
		ReplicaSetName: "nginx-7d4f8b",
		QoSClass:       "Burstable",
	}
	if err := db.Create(&pod).Error; err != nil {
		t.Fatalf("create pod: %v", err)
	}

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.GET("/pods/by-id/:id", GetPod(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/by-id/"+fmt.Sprint(pod.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	for _, key := range []string{"podIP", "startTime", "restartCount", "ownerKind", "ownerName", "replicaSetName", "qosClass"} {
		if _, ok := body[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}
	if body["podIP"] != "10.0.0.5" {
		t.Errorf("podIP: got %v", body["podIP"])
	}
	if body["restartCount"] != float64(3) {
		t.Errorf("restartCount: got %v", body["restartCount"])
	}
	if body["ownerKind"] != "ReplicaSet" {
		t.Errorf("ownerKind: got %v", body["ownerKind"])
	}
	if body["qosClass"] != "Burstable" {
		t.Errorf("qosClass: got %v", body["qosClass"])
	}
	rs, ok := body["risk_signals"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing risk_signals object")
	}
	if _, ok := rs["effective_risk"]; !ok {
		t.Errorf("risk_signals missing effective_risk")
	}
}

// TestGetPodByUID_ReturnsPodDetailFields verifies GET /pods/by-uid/:uid returns POD_DETAIL_SPEC fields.
func TestGetPodByUID_ReturnsPodDetailFields(t *testing.T) {
	db := setupPodTestDB(t)
	clusterID := "c2"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "cluster2"}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	pod := models.Pod{
		ClusterID:      clusterID,
		UID:            "pod-uid-by-uid",
		Name:           "app",
		Namespace:      "default",
		ServiceAccount: "default",
		PodIP:          "10.0.0.10",
		RestartCount:   1,
		OwnerKind:      "Deployment",
		OwnerName:      "app-deploy",
		QoSClass:       "Guaranteed",
	}
	if err := db.Create(&pod).Error; err != nil {
		t.Fatalf("create pod: %v", err)
	}

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.GET("/pods/by-uid/:uid", GetPodByUID(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/by-uid/pod-uid-by-uid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if body["podIP"] != "10.0.0.10" {
		t.Errorf("podIP: got %v", body["podIP"])
	}
	if body["ownerKind"] != "Deployment" {
		t.Errorf("ownerKind: got %v", body["ownerKind"])
	}
	if body["qosClass"] != "Guaranteed" {
		t.Errorf("qosClass: got %v", body["qosClass"])
	}
	rs, ok := body["risk_signals"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing risk_signals object")
	}
	if _, ok := rs["effective_risk"]; !ok {
		t.Errorf("risk_signals missing effective_risk")
	}
}
