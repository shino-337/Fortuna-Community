package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRuntimeScopeAndFindingActions(t *testing.T) {
	db := newSecurityRegressionDB(t)
	if err := db.AutoMigrate(&models.RuntimeSignal{}, &models.RuntimeEvent{}); err != nil {
		t.Fatal(err)
	}
	user := createSecurityUser(t, db, "runtime-scoped", models.RoleOperator, `{"cluster_ids":["cluster-a"]}`, "Password123!")
	token, _ := tokenForUser(t, db, user)
	for _, cluster := range []string{"a", "b"} {
		if err := db.Create(&models.Pod{UID: "pod-" + cluster, Name: "pod-" + cluster, Namespace: "default", ClusterID: "cluster-" + cluster}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.RuntimeSignal{PodUID: "pod-" + cluster, SignalType: "PROC_ROOT_PIVOT", Category: "PROCESS", Evidence: "{}", Confidence: 0.8, CreatedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, uid := range []string{"pod-a", "pod-b"} {
		if err := db.Create(&models.RuntimeEvent{PodUID: uid, Namespace: "default", Syscall: "sendto", Capability: "NETWORK_TXRX_QUEUE_SPIKE", TargetPath: "key=" + uid, CreatedAt: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	a := models.Insight{ResourceUID: "pod-a", ResourceType: "Pod", ResourceName: "pod-a", Title: "finding a", Description: "test", InsightType: "vulnerability", Severity: "high", Status: "active", Recommendation: "Upgrade package"}
	b := a
	b.ResourceUID = "pod-b"
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(middleware.AuthMiddleware(db, securityRegressionJWTSecret))
	r.GET("/signals", api.GetRuntimeSignalsList(db))
	r.GET("/stats", api.GetRuntimeSignalSuppressionStats(db))
	r.POST("/bulk", api.BulkInsightsAction(db))
	r.POST("/findings/:id/ack", api.AcknowledgeInsight(db))
	r.POST("/findings/:id/resolve", api.ResolveInsight(db))
	request := func(method, url, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, url, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	stats := request("GET", "/stats", "")
	var counts struct {
		Emitted int            `json:"emittedEvents"`
		Keys    map[string]int `json:"perKey"`
	}
	if stats.Code != 200 {
		t.Fatalf("stats: %d %s", stats.Code, stats.Body.String())
	}
	if err := json.Unmarshal(stats.Body.Bytes(), &counts); err != nil {
		t.Fatal(err)
	}
	if counts.Emitted != 1 || counts.Keys["pod-b"] != 0 {
		t.Fatalf("stats scope leak: %s", stats.Body.String())
	}
	for _, url := range []string{"/signals", "/signals?clusterId=cluster-a", "/signals?startDate=2026-01-01&endDate=2026-01-01&sinceMinutes=1&search=pivot&sort=confidence_desc&limit=1"} {
		w := request("GET", url, "")
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", url, w.Code, w.Body.String())
		}
		var data struct {
			Signals []models.RuntimeSignal `json:"signals"`
			Total   int                    `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if data.Total != 1 || len(data.Signals) != 1 || data.Signals[0].PodUID != "pod-a" {
			t.Fatalf("scope/filter leak: %s", w.Body.String())
		}
	}
	for _, url := range []string{"/signals?clusterId=cluster-b", "/signals?podUid=pod-b", "/signals?podUid=unknown"} {
		if w := request("GET", url, ""); w.Code != 403 {
			t.Fatalf("%s: %d %s", url, w.Code, w.Body.String())
		}
	}
	w := request("POST", "/bulk", fmt.Sprintf(`{"action":"resolve","insight_ids":["%d","%d"]}`, a.ID, b.ID))
	if w.Code != 403 {
		t.Fatalf("bulk: %d %s", w.Code, w.Body.String())
	}
	db.First(&a, a.ID)
	if a.Status != "active" {
		t.Fatal("bulk wrote before scope validation")
	}
	w = request("POST", fmt.Sprintf("/findings/%d/ack", a.ID), `{}`)
	if w.Code != 200 {
		t.Fatalf("ack: %d %s", w.Code, w.Body.String())
	}
	db.First(&a, a.ID)
	if a.Status != "acknowledged" {
		t.Fatal("acknowledgement not persisted")
	}
	w = request("POST", fmt.Sprintf("/findings/%d/resolve", a.ID), `{"resolution":"Patched and verified"}`)
	if w.Code != 200 {
		t.Fatalf("resolve: %d %s", w.Code, w.Body.String())
	}
	db.First(&a, a.ID)
	if a.Status != "resolved" || a.Recommendation != "Upgrade package" || a.ResolvedAt == nil {
		t.Fatalf("resolution damaged finding: %+v", a)
	}
}

func TestBulkRequiresActionPermissionAndNonemptySelection(t *testing.T) {
	db := newSecurityRegressionDB(t)
	item := models.Insight{ResourceType: "Pod", ResourceUID: "pod-a", ResourceName: "a", InsightType: "vulnerability", Severity: "high", Title: "a", Description: "a", Status: "active"}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, action, ids string
		perms             []authorization.Permission
		want              int
	}{
		{"bulk alone cannot resolve", "resolve", fmt.Sprintf(`["%d"]`, item.ID), []authorization.Permission{authorization.PermissionFindingsBulk}, 403},
		{"ack cannot dismiss", "dismiss", fmt.Sprintf(`["%d"]`, item.ID), []authorization.Permission{authorization.PermissionFindingsBulk, authorization.PermissionFindingsAck}, 403},
		{"blank selection", "acknowledge", `[" "]`, []authorization.Permission{authorization.PermissionFindingsBulk, authorization.PermissionFindingsAck}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/bulk", func(c *gin.Context) { c.Set(middleware.CtxPermissions, tc.perms) }, api.BulkInsightsAction(db))
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/bulk", bytes.NewBufferString(fmt.Sprintf(`{"action":%q,"insight_ids":%s}`, tc.action, tc.ids)))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			var got models.Insight
			db.First(&got, item.ID)
			if got.Status != "active" {
				t.Fatal("unauthorized mutation")
			}
		})
	}
}
