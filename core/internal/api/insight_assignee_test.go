package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// assigneeDB seeds findings 1 (cluster c1) and 2 (c2) and these accounts:
// 10 admin, 11 operator scoped to c1, 12 operator scoped to c2, 13 viewer, 14 disabled operator.
func assigneeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := reviewDB(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{}, &models.User{}, &models.SecurityActivityLog{})
	seedTwoClusterFindings(t, db)
	users := []models.User{
		{ID: 10, Username: "root", Email: "root@x", Password: "x", Role: models.RoleAdmin},
		{ID: 11, Username: "mai", Email: "mai@x", Password: "x", Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["c1"]}`},
		{ID: 12, Username: "an", Email: "an@x", Password: "x", Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["c2"]}`},
		{ID: 13, Username: "vy", Email: "vy@x", Password: "x", Role: models.RoleViewer},
		{ID: 14, Username: "old", Email: "old@x", Password: "x", Role: models.RoleOperator},
	}
	for i := range users {
		users[i].Active = true
		require.NoError(t, db.Create(&users[i]).Error)
	}
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", 14).Update("active", false).Error)
	return db
}

func operatorIn(id uint, name string, clusters string) *models.User {
	return &models.User{ID: id, Username: name, Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":` + clusters + `}`, Active: true}
}

func putAssignee(t *testing.T, r http.Handler, insightID string, body string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/risk/insights/"+insightID+"/assignee", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func assigneeRouter(db *gorm.DB, u *models.User) http.Handler {
	r := reviewRouter(u)
	r.Use(func(c *gin.Context) { c.Set("userID", u.ID) })
	r.PUT("/risk/insights/:id/assignee", AssignInsight(db))
	r.GET("/risk/insights/:id/assignees", ListInsightAssignees(db))
	r.GET("/risk/insights", GetInsightsList(db))
	return r
}

func TestAssignInsightToSelfAndTeammate(t *testing.T) {
	db := assigneeDB(t)
	r := assigneeRouter(db, operatorIn(11, "mai", `["c1"]`))

	code, out := putAssignee(t, r, "1", `{"userId": 11}`)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "mai", out["assignee"].(map[string]any)["username"])

	var f models.Insight
	require.NoError(t, db.First(&f, 1).Error)
	require.NotNil(t, f.AssigneeUserID)
	require.Equal(t, uint(11), *f.AssigneeUserID)
	require.Equal(t, "mai", f.AssigneeUsername)
	require.NotNil(t, f.AssignedAt)

	// The admin can triage every cluster, so they can take it over.
	code, _ = putAssignee(t, r, "1", `{"userId": 10}`)
	require.Equal(t, http.StatusOK, code)

	code, _ = putAssignee(t, r, "1", `{"userId": null}`)
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, db.First(&f, 1).Error)
	require.Nil(t, f.AssigneeUserID)
	require.Equal(t, "", f.AssigneeUsername)

	var events int64
	require.NoError(t, db.Model(&models.SecurityActivityLog{}).Where("action = ?", "findings.assignee.set").Count(&events).Error)
	require.Equal(t, int64(3), events, "every change is audited")
}

func TestAssignInsightRefusesUsersWhoCannotTriageIt(t *testing.T) {
	db := assigneeDB(t)
	r := assigneeRouter(db, operatorIn(11, "mai", `["c1"]`))

	for name, body := range map[string]string{
		"other cluster": `{"userId": 12}`,
		"viewer":        `{"userId": 13}`,
		"disabled":      `{"userId": 14}`,
		"unknown":       `{"userId": 999}`,
	} {
		code, out := putAssignee(t, r, "1", body)
		require.Equal(t, http.StatusUnprocessableEntity, code, name)
		require.Equal(t, "this user cannot be assigned the finding", out["error"], name)
	}
}

func TestAssignInsightHidesFindingsOutsideScope(t *testing.T) {
	db := assigneeDB(t)
	r := assigneeRouter(db, operatorIn(11, "mai", `["c1"]`))

	code, _ := putAssignee(t, r, "2", `{"userId": 11}`)
	require.Equal(t, http.StatusNotFound, code)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/risk/insights/2/assignees", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAssignInsightRefusesClosedFindings(t *testing.T) {
	db := assigneeDB(t)
	require.NoError(t, db.Model(&models.Insight{}).Where("id = ?", 1).Update("status", "resolved").Error)
	r := assigneeRouter(db, adminUser())

	code, _ := putAssignee(t, r, "1", `{"userId": 10}`)
	require.Equal(t, http.StatusConflict, code)
}

func TestListInsightAssigneesOnlyNamesPeopleWhoCanTriage(t *testing.T) {
	db := assigneeDB(t)
	r := assigneeRouter(db, adminUser())

	body := getJSON(t, r, "/risk/insights/1/assignees")
	var names []string
	for _, it := range body["items"].([]any) {
		names = append(names, it.(map[string]any)["username"].(string))
	}
	require.Equal(t, []string{"mai", "root"}, names, "c2 operator, viewer and disabled account are left out")
}

func TestInsightsListAssigneeFilter(t *testing.T) {
	db := assigneeDB(t)
	mai := uint(11)
	require.NoError(t, db.Model(&models.Insight{}).Where("id = ?", 1).Updates(map[string]any{"assignee_user_id": mai, "assignee_username": "mai"}).Error)

	r := assigneeRouter(db, operatorIn(11, "mai", `["c1","c2"]`))
	mine := getJSON(t, r, "/risk/insights?assignee=me")
	require.Equal(t, float64(1), mine["total"])
	first := mine["insights"].([]any)[0].(map[string]any)
	require.Equal(t, "mai", first["assignee"])

	none := getJSON(t, r, "/risk/insights?assignee=none")
	require.Equal(t, float64(1), none["total"])

	// "open" keeps a finding the owner acknowledged; the default (needs triage) drops it.
	require.NoError(t, db.Model(&models.Insight{}).Where("id = ?", 1).Update("status", "acknowledged").Error)
	if defaultRisksCache != nil {
		defaultRisksCache.ClearByPrefix("risks:list:")
	}
	require.Equal(t, float64(0), getJSON(t, r, "/risk/insights?assignee=me")["total"])
	require.Equal(t, float64(1), getJSON(t, r, "/risk/insights?assignee=me&status=open")["total"])

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/risk/insights?assignee=someone", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}
