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

// Only a platform admin manages accounts. user_admin is retired: an account
// still holding it gets no permission, and the handlers refuse it as well.
func TestOnlyAdminCreatesUsers(t *testing.T) {
	db := reviewDB(t, &models.User{}, &models.Cluster{})
	register := func(actorRole, name, role string) int {
		r := reviewRouter(&models.User{ID: 999, Role: actorRole, Active: true})
		r.POST("/register", Register(db, "integration-test-secret-key-32b!!"))
		body, _ := json.Marshal(map[string]string{
			"username": name, "email": name + "@test.local", "password": "AnotherPass12!", "role": role,
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}
	require.Equal(t, http.StatusForbidden, register(models.RoleUserAdmin, "newviewer", models.RoleViewer))
	require.Equal(t, http.StatusBadRequest, register(models.RoleUserAdmin, "newuadmin", models.RoleUserAdmin), "user_admin is no longer a role")
	require.Equal(t, http.StatusForbidden, register(models.RoleClusterAdmin, "newop", models.RoleOperator))
	require.Equal(t, http.StatusBadRequest, register(models.RoleAdmin, "uadmin", models.RoleUserAdmin), "user_admin can no longer be assigned")
	require.Equal(t, http.StatusCreated, register(models.RoleAdmin, "viewer2", models.RoleViewer))

	var n int64
	require.NoError(t, db.Model(&models.User{}).Where("username IN ?", []string{"newviewer", "newuadmin", "newop", "uadmin"}).Count(&n).Error)
	require.Zero(t, n)
}

func TestRetiredUserAdminCannotChangeUsers(t *testing.T) {
	db := reviewDB(t, &models.User{}, &models.Cluster{})
	viewer := models.User{Username: "v", Email: "v@test.local", Password: "x", Role: models.RoleViewer, Active: true}
	op := models.User{Username: "o", Email: "o@test.local", Password: "x", Role: models.RoleOperator, Active: true}
	require.NoError(t, db.Create(&viewer).Error)
	require.NoError(t, db.Create(&op).Error)

	r := reviewRouter(&models.User{ID: 999, Role: models.RoleUserAdmin, Active: true})
	r.PATCH("/users/:id", PatchUser(db))
	r.DELETE("/users/:id", DeleteUser(db))
	send := func(method string, id uint, body string) int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/users/"+strconv.Itoa(int(id)), bytes.NewBufferString(body)))
		return w.Code
	}
	require.Equal(t, http.StatusForbidden, send(http.MethodPatch, viewer.ID, `{"role":"operator"}`))
	require.Equal(t, http.StatusForbidden, send(http.MethodPatch, op.ID, `{"active":false}`))
	require.Equal(t, http.StatusForbidden, send(http.MethodDelete, op.ID, ``))

	var unchanged models.User
	require.NoError(t, db.First(&unchanged, op.ID).Error)
	require.Equal(t, models.RoleOperator, unchanged.Role)
	require.True(t, unchanged.Active)
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
