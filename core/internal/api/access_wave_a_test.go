package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

// A user admin has no security-data permission and cannot set cluster scope, so it must not be
// able to mint an account that can change data (operator and above get every cluster by default).
func TestUserAdminCanOnlyCreateViewerOrUserAdmin(t *testing.T) {
	db := reviewDB(t, &models.User{}, &models.Cluster{})
	r := reviewRouter(&models.User{ID: 999, Role: models.RoleUserAdmin, Active: true})
	r.POST("/register", Register(db, "integration-test-secret-key-32b!!"))

	register := func(name, role string) int {
		body, _ := json.Marshal(map[string]string{
			"username": name, "email": name + "@test.local", "password": "AnotherPass12!", "role": role,
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}
	require.Equal(t, http.StatusForbidden, register("newop", models.RoleOperator))
	require.Equal(t, http.StatusForbidden, register("legacy", "user"), "legacy user role is operator")
	require.Equal(t, http.StatusCreated, register("newviewer", models.RoleViewer))
	require.Equal(t, http.StatusCreated, register("newuadmin", models.RoleUserAdmin))

	var n int64
	require.NoError(t, db.Model(&models.User{}).Where("username IN ?", []string{"newop", "legacy"}).Count(&n).Error)
	require.Zero(t, n)
}

func TestUserAdminCannotPromoteToOperatorButCanDemote(t *testing.T) {
	db := reviewDB(t, &models.User{}, &models.Cluster{})
	viewer := models.User{Username: "v", Email: "v@test.local", Password: "x", Role: models.RoleViewer, Active: true}
	op := models.User{Username: "o", Email: "o@test.local", Password: "x", Role: models.RoleOperator, Active: true}
	require.NoError(t, db.Create(&viewer).Error)
	require.NoError(t, db.Create(&op).Error)

	r := reviewRouter(&models.User{ID: 999, Role: models.RoleUserAdmin, Active: true})
	r.PATCH("/users/:id", PatchUser(db))
	patch := func(id uint, body string) int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/users/"+strconv.Itoa(int(id)), bytes.NewBufferString(body)))
		return w.Code
	}
	require.Equal(t, http.StatusForbidden, patch(viewer.ID, `{"role":"operator"}`))
	require.Equal(t, http.StatusOK, patch(op.ID, `{"role":"viewer"}`))

	var stillViewer, demoted models.User
	require.NoError(t, db.First(&stillViewer, viewer.ID).Error)
	require.Equal(t, models.RoleViewer, stillViewer.Role)
	require.NoError(t, db.First(&demoted, op.ID).Error)
	require.Equal(t, models.RoleViewer, demoted.Role)
}

// Rules are global, but their matches are findings and follow the caller's cluster scope.
func TestPolicyRuleMatchesFollowClusterScope(t *testing.T) {
	db := reviewDB(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{})
	seedTwoClusterFindings(t, db)
	require.NoError(t, db.Model(&models.Insight{}).Where("1 = 1").Update("cve_id", "RULE-X").Error)

	get := func(u *models.User, path string) map[string]any {
		r := reviewRouter(u)
		r.GET("/rules/uid/:uid/matches", GetRuleMatches(db))
		r.GET("/rules/uid/:uid/metrics", GetRuleMetrics(db))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var out map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out
	}

	scoped := scopedViewer("c1")
	matches := get(scoped, "/rules/uid/RULE-X/matches")
	require.EqualValues(t, 1, matches["count"])
	body, _ := json.Marshal(matches)
	require.Contains(t, string(body), "finding-c1")
	require.NotContains(t, string(body), "finding-c2", "a user scoped to c1 must not see c2 findings")

	metrics := get(scoped, "/rules/uid/RULE-X/metrics")
	require.EqualValues(t, 1, metrics["totalMatches"])

	admin := &models.User{ID: 1, Role: models.RoleAdmin, Active: true}
	require.EqualValues(t, 2, get(admin, "/rules/uid/RULE-X/matches")["count"])
	require.EqualValues(t, 2, get(admin, "/rules/uid/RULE-X/metrics")["totalMatches"])
}

func TestAuditOnSuccessRecordsOnlySuccessfulWrites(t *testing.T) {
	db := reviewDB(t, &models.SecurityActivityLog{})
	r := reviewRouter(&models.User{ID: 7, Username: "op", Role: models.RoleOperator, Active: true})
	r.DELETE("/rules/:id", auditOnSuccess(db, securityaudit.ActionRiskRuleDelete, "risk_rule", "id"), func(c *gin.Context) {
		if c.Param("id") == "missing" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for _, id := range []string{"r1", "missing"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/rules/"+id, nil))
	}

	var rows []models.SecurityActivityLog
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, securityaudit.ActionRiskRuleDelete, rows[0].Action)
	require.Equal(t, "r1", rows[0].ResourceID)
	require.Equal(t, "op", rows[0].ActorUsername)
}
