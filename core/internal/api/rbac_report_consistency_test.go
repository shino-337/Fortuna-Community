package api

import (
	"encoding/json"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestPodRiskReportUsesResolvedRBACScope(t *testing.T) {
	db, _ := serviceAccountScopeFixture(t)
	if err := db.AutoMigrate(&models.Pod{}, &models.Insight{}, &models.RuntimeSignal{}); err != nil {
		t.Fatal(err)
	}
	const ref = `{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"cluster-admin"}`
	for _, row := range []any{
		&models.Pod{UID: "pod", Name: "pod", ClusterID: "a", Namespace: "shared", ServiceAccount: "shared"},
		&models.ClusterRole{UID: "admin-role", ClusterID: "a", Name: "cluster-admin", Rules: `[{"verbs":["*"],"resources":["*"],"apiGroups":["*"]}]`},
		&models.RoleBinding{UID: "rb", ClusterID: "a", Namespace: "shared", Name: "local-admin", Subjects: `[{"kind":"ServiceAccount","name":"shared"}]`, RoleRef: ref},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.GET("/pods/:uid/report", GetPodRiskReport(db))
	check := func(want int) {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/pods/pod/report", nil))
		if w.Code != 200 {
			t.Fatalf("%d: %s", w.Code, w.Body)
		}
		var report PodRiskReport
		if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.Summary["clusterAdminBindings"] != float64(want) {
			t.Fatalf("wrong report: %+v", report)
		}
		for _, binding := range report.Bindings {
			if binding.Kind == "RoleBinding" && binding.IsClusterAdmin {
				t.Fatal("namespaced grant became cluster admin")
			}
		}
		grants, err := buildServiceAccountPermissions(db, &models.ServiceAccount{ClusterID: "a", Namespace: "shared", Name: "shared"})
		if err != nil {
			t.Fatal(err)
		}
		if grants.HasClusterAdminBinding() != (want > 0) {
			t.Fatal("permissions and report disagree")
		}
	}
	check(0)
	if err := db.Create(&models.ClusterRoleBinding{UID: "group", ClusterID: "a", Name: "group-admin", Subjects: `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccounts:shared"}]`, RoleRef: ref}).Error; err != nil {
		t.Fatal(err)
	}
	check(1)
	if err := db.Where("uid = ?", "admin-role").Delete(&models.ClusterRole{}).Error; err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/pods/pod/report", nil))
	if w.Code != 500 {
		t.Fatalf("missing role produced successful report: %d", w.Code)
	}
}
