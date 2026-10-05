package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/models"
)

// Tests for the row limits on list endpoints that used to return unbounded
// collections: ?limit is honored, clamped, never disables the bound, and a
// cut result is flagged with "truncated" without changing the response shape.

func getJSON(t *testing.T, h http.Handler, path string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func adminUser() *models.User {
	return &models.User{ID: 1, Username: "admin", Role: models.RoleAdmin, Active: true}
}

func TestGetUsersIsBounded(t *testing.T) {
	db := reviewDB(t, &models.User{})
	for i := 0; i < 5; i++ {
		require.NoError(t, db.Create(&models.User{
			Username: fmt.Sprintf("u%d", i), Email: fmt.Sprintf("u%d@x", i), Password: "x", Role: models.RoleViewer, Active: true,
		}).Error)
	}
	r := reviewRouter(adminUser())
	r.GET("/users", GetUsers(db))

	body := getJSON(t, r, "/users?limit=2")
	require.Len(t, body["users"], 2)
	require.Equal(t, float64(2), body["total"])
	require.Equal(t, true, body["truncated"])

	body = getJSON(t, r, "/users")
	require.Len(t, body["users"], 5)
	require.Equal(t, false, body["truncated"])

	// Invalid / non-positive limits fall back to the default; they never
	// remove the bound (GORM treats a negative LIMIT as "no limit").
	for _, raw := range []string{"-1", "0", "abc"} {
		body = getJSON(t, r, "/users?limit="+raw)
		require.Len(t, body["users"], 5, raw)
	}
}

func TestGetResourcesSharesOneBudgetAcrossKinds(t *testing.T) {
	db := reviewDB(t, &models.Pod{}, &models.ServiceAccount{}, &models.Role{}, &models.ClusterRole{},
		&models.RoleBinding{}, &models.ClusterRoleBinding{})
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&models.Pod{UID: fmt.Sprintf("pod-%d", i), Name: fmt.Sprintf("p%d", i), Namespace: "default", ClusterID: "c1"}).Error)
		require.NoError(t, db.Create(&models.ServiceAccount{UID: fmt.Sprintf("sa-%d", i), Name: fmt.Sprintf("sa%d", i), Namespace: "default", ClusterID: "c1"}).Error)
	}
	r := reviewRouter(adminUser())
	r.GET("/resources", GetResources(db))

	body := getJSON(t, r, "/resources?limit=4")
	require.Len(t, body["resources"], 4)
	// The dashboard requires total == len(resources); truncation is a flag.
	require.Equal(t, float64(4), body["total"])
	require.Equal(t, true, body["truncated"])

	// Budget exhausted exactly by the first kind still reports truncation.
	body = getJSON(t, r, "/resources?limit=3")
	require.Len(t, body["resources"], 3)
	require.Equal(t, true, body["truncated"])

	body = getJSON(t, r, "/resources?limit=6")
	require.Len(t, body["resources"], 6)
	require.Equal(t, false, body["truncated"])
}

func TestGetAgentStatusKeepsExactCountersWhenTruncated(t *testing.T) {
	db := reviewDB(t, &models.Agent{}, &models.Cluster{})
	now := time.Now()
	fresh := now.Add(-time.Minute)
	slow := now.Add(-10 * time.Minute)
	stale := now.Add(-time.Hour)
	for i, seen := range []*time.Time{&fresh, &fresh, &slow, &stale, nil} {
		require.NoError(t, db.Create(&models.Agent{
			ClusterID: "c1", AgentID: fmt.Sprintf("a%d", i), NodeName: fmt.Sprintf("n%d", i), Status: "ready", LastSeenAt: seen,
		}).Error)
	}
	r := reviewRouter(adminUser())
	r.GET("/agents/status", GetAgentStatus(db))

	body := getJSON(t, r, "/agents/status?limit=2")
	require.Len(t, body["agents"], 2)
	require.Equal(t, true, body["truncated"])
	require.Equal(t, float64(5), body["total"])
	require.Equal(t, float64(2), body["healthy"])
	require.Equal(t, float64(1), body["slow"])
	require.Equal(t, float64(2), body["disconnected"])

	body = getJSON(t, r, "/agents/status")
	require.Len(t, body["agents"], 5)
	require.Equal(t, false, body["truncated"])
	require.Equal(t, float64(2), body["healthy"])
	require.Equal(t, float64(1), body["slow"])
	require.Equal(t, float64(2), body["disconnected"])
}

func TestListExceptionsIsBounded(t *testing.T) {
	db := reviewDB(t, &models.ExceptionPolicy{})
	for i := 0; i < 4; i++ {
		require.NoError(t, db.Create(&models.ExceptionPolicy{ClusterID: "c1", ResourceUID: fmt.Sprintf("r%d", i)}).Error)
	}
	r := reviewRouter(adminUser())
	r.GET("/exceptions", ListExceptions(db))

	body := getJSON(t, r, "/exceptions?limit=3")
	require.Len(t, body["exceptions"], 3)
	require.Equal(t, true, body["truncated"])

	// Above the hard maximum is clamped, not rejected.
	body = getJSON(t, r, fmt.Sprintf("/exceptions?limit=%d", exceptionsMaxLimit*10))
	require.Len(t, body["exceptions"], 4)
	require.Equal(t, false, body["truncated"])
}

func TestInvestigationTimelineKeepsNewestEntriesOldestFirst(t *testing.T) {
	db := reviewDB(t, &models.InvestigationCase{}, &models.InvestigationActivityLog{})
	require.NoError(t, db.Create(&models.InvestigationCase{ID: "case-1", Title: "t", Status: "open"}).Error)
	base := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		require.NoError(t, db.Create(&models.InvestigationActivityLog{
			CaseID: "case-1", EventID: fmt.Sprintf("ev-%d", i), EventType: fmt.Sprintf("e%d", i), Summary: fmt.Sprintf("s%d", i), CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}).Error)
	}
	r := reviewRouter(adminUser())
	r.GET("/investigations/:id/timeline", ListInvestigationTimeline(db))

	body := getJSON(t, r, "/investigations/case-1/timeline?limit=2")
	items := body["items"].([]any)
	require.Len(t, items, 2)
	require.Equal(t, true, body["truncated"])
	require.Equal(t, "s3", items[0].(map[string]any)["summary"])
	require.Equal(t, "s4", items[1].(map[string]any)["summary"])
}

func TestCapGraphDataLeavesNoDanglingLinks(t *testing.T) {
	nodes := []map[string]interface{}{}
	links := []map[string]interface{}{}
	for i := 0; i < 10; i++ {
		risk := "low"
		if i < 3 {
			risk = "critical"
		}
		nodes = append(nodes, map[string]interface{}{"id": fmt.Sprintf("n%d", i), "risk": risk})
		if i > 0 {
			links = append(links, map[string]interface{}{"source": fmt.Sprintf("n%d", i-1), "target": fmt.Sprintf("n%d", i), "type": "x", "value": float64(i)})
		}
	}
	data := map[string]interface{}{"nodes": nodes, "links": links}
	require.Equal(t, data, capGraphData(data, 10, 100), "within caps the payload is untouched")

	out := capGraphData(data, 3, 100)
	require.Equal(t, true, out["truncated"])
	require.Equal(t, 10, out["totalNodes"])
	kept := map[string]bool{}
	for _, n := range out["nodes"].([]map[string]interface{}) {
		require.Equal(t, "critical", n["risk"], "highest-risk nodes are kept first")
		kept[n["id"].(string)] = true
	}
	require.Len(t, kept, 3)
	for _, l := range out["links"].([]map[string]interface{}) {
		require.True(t, kept[l["source"].(string)] && kept[l["target"].(string)])
	}
}

func TestCapAttackPathsKeepsReferencedPathsAndOrder(t *testing.T) {
	paths := make([]graph.AttackPath, 6)
	for i := range paths {
		paths[i] = graph.AttackPath{PathID: fmt.Sprintf("p%d", i), TotalRisk: float64(i)}
	}
	out, truncated := capAttackPaths(paths, 3, map[string]struct{}{"p0": {}})
	require.True(t, truncated)
	ids := []string{}
	for _, p := range out {
		ids = append(ids, p.PathID)
	}
	require.Equal(t, []string{"p0", "p4", "p5"}, ids)

	out, truncated = capAttackPaths(paths, 10, nil)
	require.False(t, truncated)
	require.Len(t, out, 6)
}

func TestListInvestigationCasesIsBounded(t *testing.T) {
	db := reviewDB(t, &models.InvestigationCase{})
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&models.InvestigationCase{ID: fmt.Sprintf("case-%d", i), Title: "t", Status: "open"}).Error)
	}
	r := reviewRouter(adminUser())
	r.GET("/investigations", ListInvestigationCases(db))

	body := getJSON(t, r, "/investigations?limit=2")
	require.Len(t, body["items"], 2)
	require.Equal(t, float64(2), body["total"])
	require.Equal(t, true, body["truncated"])

	body = getJSON(t, r, "/investigations")
	require.Len(t, body["items"], 3)
	require.Equal(t, false, body["truncated"])
}
