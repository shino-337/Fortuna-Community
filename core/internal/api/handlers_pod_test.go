package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPodTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Insight{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestGetPodByUIDScoped_ReturnsPodDetailFields(t *testing.T) {
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
	if err := db.Create(&models.Insight{
		ClusterID: clusterID, ResourceType: "Pod", ResourceUID: pod.UID,
		InsightType: "test", Severity: "medium", Title: "test", Description: "test", Status: "active",
	}).Error; err != nil {
		t.Fatalf("create insight: %v", err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", &models.User{Role: models.RoleAdmin})
		c.Next()
	})
	router.GET("/api/v1/inventory/pods/:uid", middleware.RequirePodUIDClusterScope(db, "uid"), GetPodByUIDScoped(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/pods/"+pod.UID, nil)
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
	if body["podIP"] != "10.0.0.5" || body["restartCount"] != float64(3) || body["ownerKind"] != "ReplicaSet" || body["qosClass"] != "Burstable" {
		t.Fatalf("unexpected pod detail: %v", body)
	}
	if body["riskCount"] != float64(1) {
		t.Fatalf("riskCount: got %v", body["riskCount"])
	}
}

func TestGetPodByUIDScoped_RejectsAmbiguousDuplicateUID(t *testing.T) {
	db := setupPodTestDB(t)
	for _, clusterID := range []string{"c1", "c2"} {
		if err := db.Create(&models.Cluster{ID: clusterID, Name: clusterID}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.Pod{ClusterID: clusterID, UID: "duplicate-uid", Name: "pod-"+clusterID, Namespace: "default"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", &models.User{Role: models.RoleAdmin})
		c.Next()
	})
	router.GET("/api/v1/inventory/pods/:uid", middleware.RequirePodUIDClusterScope(db, "uid"), GetPodByUIDScoped(db))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/pods/duplicate-uid", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for ambiguous UID, got %d body=%s", w.Code, w.Body.String())
	}
}
