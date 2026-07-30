package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func setupResourcesDetailTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Role{}, &models.RoleBinding{}, &models.ClusterRole{}, &models.ClusterRoleBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestGetResourceDetailRoleBindingResolvesNamespacedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupResourcesDetailTestDB(t)
	if err := db.Create(&models.Role{
		ClusterID: "c1",
		Name:      "pod-reader",
		Namespace: "ns1",
		UID:       "role-uid",
		Rules:     `[{"verbs":["get","list"],"apiGroups":[""],"resources":["pods"]}]`,
	}).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := db.Create(&models.RoleBinding{
		ClusterID: "c1",
		Name:      "bind-reader",
		Namespace: "ns1",
		UID:       "binding-uid",
		RoleRef:   `{"kind":"Role","name":"pod-reader","apiGroup":"rbac.authorization.k8s.io"}`,
		Subjects:  `[{"kind":"ServiceAccount","name":"reader","namespace":"ns1"}]`,
	}).Error; err != nil {
		t.Fatalf("create role binding: %v", err)
	}

	r := gin.New()
	r.GET("/resources/:kind/:uid", GetResourceDetail(db))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/resources/RoleBinding/binding-uid", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rules, _ := body["rules"].([]interface{})
	subjects, _ := body["subjects"].([]interface{})
	if len(rules) != 1 || len(subjects) != 1 {
		t.Fatalf("expected resolved rule and subject, body=%s", w.Body.String())
	}
}

func TestGetResourceDetailRoleReferencesOnlyRoleBindings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupResourcesDetailTestDB(t)
	if err := db.Create(&models.Role{
		ClusterID: "c1",
		Name:      "shared-name",
		Namespace: "ns1",
		UID:       "role-uid",
		Rules:     `[{"verbs":["get"],"apiGroups":[""],"resources":["pods"]}]`,
	}).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	bindings := []models.RoleBinding{
		{
			ClusterID: "c1",
			Name:      "role-binding",
			Namespace: "ns1",
			UID:       "binding-role-uid",
			RoleRef:   `{"kind":"Role","name":"shared-name","apiGroup":"rbac.authorization.k8s.io"}`,
			Subjects:  `[]`,
		},
		{
			ClusterID: "c1",
			Name:      "cluster-role-binding",
			Namespace: "ns1",
			UID:       "binding-cluster-role-uid",
			RoleRef:   `{"kind":"ClusterRole","name":"shared-name","apiGroup":"rbac.authorization.k8s.io"}`,
			Subjects:  `[]`,
		},
	}
	if err := db.Create(&bindings).Error; err != nil {
		t.Fatalf("create bindings: %v", err)
	}

	r := gin.New()
	r.GET("/resources/:kind/:uid", GetResourceDetail(db))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/resources/Role/role-uid", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	referencedBy, _ := body["referencedBy"].([]interface{})
	if len(referencedBy) != 1 {
		t.Fatalf("expected only direct Role reference, body=%s", w.Body.String())
	}
}
