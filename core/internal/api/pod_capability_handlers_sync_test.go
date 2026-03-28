package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetPodCapabilities_ClassFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.PodCapability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&models.Pod{UID: "pod-cap-ui-1", Name: "pod-cap-ui-1", Namespace: "default", ClusterID: "c1"}).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}
	if err := db.Create(&models.PodCapability{
		PodUID:          "pod-cap-ui-1",
		Namespace:       "default",
		CapabilityID:    "ESC_RUNTIME_ACTIVE",
		CapabilityGroup: "ESC",
		Severity:        "CRITICAL",
		CapabilityClass: "effective",
		State:           "detected",
		Evidence:        `{"source":"runtime"}`,
	}).Error; err != nil {
		t.Fatalf("seed capability effective: %v", err)
	}
	if err := db.Create(&models.PodCapability{
		PodUID:          "pod-cap-ui-1",
		Namespace:       "default",
		CapabilityID:    "OBSERVED_EXECUTION",
		CapabilityGroup: "EXECUTION",
		Severity:        "MEDIUM",
		CapabilityClass: "observed",
		State:           "detected",
		Evidence:        `{"source":"runtime"}`,
	}).Error; err != nil {
		t.Fatalf("seed capability observed: %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{"v1_inventory", "/api/v1/inventory/pods/pod-cap-ui-1/capabilities?class=effective"},
		{"v2_runtime", "/api/v2/runtime/pods/pod-cap-ui-1/capabilities?class=effective"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/api/v1/inventory/pods/:uid/capabilities", GetPodCapabilities(db))
			r.GET("/api/v2/runtime/pods/:uid/capabilities", GetPodCapabilities(db))

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var out map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			items, ok := out["capabilities"].([]interface{})
			if !ok {
				t.Fatalf("capabilities not found in response: %v", out)
			}
			if len(items) != 1 {
				t.Fatalf("expected 1 capability for class=effective, got %d", len(items))
			}
		})
	}
}
