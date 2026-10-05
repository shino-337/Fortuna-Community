package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

// Tests for the Dashboard feature review: exports, notifications, sessions,
// user administration and finding reopen must follow the caller's role and
// cluster scope the same way the list endpoints do.

func reviewDB(t *testing.T, tables ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(tables...))
	return db
}

// reviewRouter injects the principal the way AuthMiddleware does.
func reviewRouter(u *models.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", u)
		c.Set(middleware.CtxPermissions, authorization.PermissionsForUser(u.Role))
		c.Next()
	})
	return r
}

func scopedViewer(clusters ...string) *models.User {
	ids, _ := json.Marshal(clusters)
	return &models.User{ID: 50, Role: models.RoleClusterAdmin, ScopeJSON: `{"cluster_ids":` + string(ids) + `}`, Active: true}
}

func seedTwoClusterFindings(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	for _, cl := range []string{"c1", "c2"} {
		require.NoError(t, db.Create(&models.Cluster{ID: cl, Name: cl}).Error)
		require.NoError(t, db.Create(&models.Pod{UID: "pod-" + cl, Name: "p-" + cl, Namespace: "default", ClusterID: cl}).Error)
		require.NoError(t, db.Create(&models.Insight{
			ClusterID: cl, ResourceType: "Pod", ResourceUID: "pod-" + cl, ResourceName: "=HYPERLINK(\"x\")-" + cl,
			ResourceNamespace: "default", InsightType: "misconfiguration", Severity: "high",
			Title: "finding-" + cl, Status: "active", DetectedAt: now,
		}).Error)
		// A high risk score, so the finding raises a notification and exports with a level.
		require.NoError(t, db.Create(&models.RiskScore{
			ClusterID: cl, ResourceType: "Pod", ResourceUID: "pod-" + cl, TotalScore: 55, ScorerVersion: "v3", CalculatedAt: now,
		}).Error)
	}
}

func TestExportRisksCSVFollowsClusterScope(t *testing.T) {
	db := reviewDB(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{})
	seedTwoClusterFindings(t, db)
	r := reviewRouter(scopedViewer("c1"))
	r.GET("/export", ExportRisksCSV(db))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/export", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	require.Contains(t, body, "finding-c1")
	require.NotContains(t, body, "finding-c2", "a user scoped to c1 must not export c2 findings")
	require.Contains(t, body, `'=HYPERLINK`, "formula cells must be neutralized")

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/export?clusterId=c2", nil))
	require.Equal(t, http.StatusForbidden, w.Code)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/export?format=pdf", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "finding-c2")
}

func TestCSVSafeCell(t *testing.T) {
	for in, want := range map[string]string{
		"":           "",
		"plain":      "plain",
		"=1+1":       "'=1+1",
		"+cmd":       "'+cmd",
		"-2":         "'-2",
		"@SUM(A1)":   "'@SUM(A1)",
		"\tx":        "'\tx",
		"a=b":        "a=b",
		"CVE-2024-1": "CVE-2024-1",
	} {
		require.Equal(t, want, csvSafeCell(in), in)
	}
}

func TestNotificationsFollowClusterScope(t *testing.T) {
	db := notificationTestDB(t)
	seedTwoClusterFindings(t, db)
	r := reviewRouter(scopedViewer("c1"))
	r.GET("/notifications", GetNotifications(db))
	r.PATCH("/notifications/:id/read", MarkNotificationRead(db))
	r.POST("/notifications/read-all", MarkAllNotificationsRead(db))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/notifications", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got struct {
		Notifications []map[string]any `json:"notifications"`
		UnreadCount   int              `json:"unreadCount"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Notifications, 1)
	require.Equal(t, "c1", got.Notifications[0]["clusterId"])
	require.Equal(t, 1, got.UnreadCount)

	var foreign models.Notification
	require.NoError(t, db.Where("cluster_id = ?", "c2").First(&foreign).Error)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/notifications/"+strconv.Itoa(int(foreign.ID))+"/read", nil))
	require.Equal(t, http.StatusOK, w.Code)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var foreignReads int64
	require.NoError(t, db.Model(&models.NotificationRead{}).Where("notification_id = ?", foreign.ID).Count(&foreignReads).Error)
	require.Zero(t, foreignReads, "a scoped user must not change read state of another cluster's notification")
}

func notificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return reviewDB(t, &models.Notification{}, &models.NotificationRead{}, &models.Pod{}, &models.Insight{},
		&models.AttackPath{}, &models.SBOM{}, &models.CVEMatch{}, &models.MalwareMatch{}, &models.Cluster{}, &models.RiskScore{})
}

func TestNotificationReadStateIsPerUser(t *testing.T) {
	db := notificationTestDB(t)
	seedTwoClusterFindings(t, db)
	alice := &models.User{ID: 10, Role: models.RoleAdmin, Active: true}
	bob := &models.User{ID: 11, Role: models.RoleAdmin, Active: true}

	unread := func(u *models.User) (int, []map[string]any) {
		r := reviewRouter(u)
		r.GET("/notifications", GetNotifications(db))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/notifications", nil))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var got struct {
			Notifications []map[string]any `json:"notifications"`
			UnreadCount   int              `json:"unreadCount"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		return got.UnreadCount, got.Notifications
	}
	call := func(u *models.User, method, path string) int64 {
		r := reviewRouter(u)
		r.PATCH("/notifications/:id/read", MarkNotificationRead(db))
		r.POST("/notifications/read-all", MarkAllNotificationsRead(db))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var got struct {
			Updated int64 `json:"updated"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		return got.Updated
	}

	n, list := unread(alice)
	require.Equal(t, 2, n)
	require.Len(t, list, 2)
	firstID := strconv.Itoa(int(list[0]["id"].(float64)))

	require.EqualValues(t, 1, call(alice, http.MethodPatch, "/notifications/"+firstID+"/read"))
	require.EqualValues(t, 0, call(alice, http.MethodPatch, "/notifications/"+firstID+"/read"), "marking twice is a no-op")
	n, list = unread(alice)
	require.Equal(t, 1, n)
	require.NotNil(t, list[0]["readAt"])

	require.EqualValues(t, 1, call(alice, http.MethodPost, "/notifications/read-all"))
	n, _ = unread(alice)
	require.Zero(t, n)

	n, list = unread(bob)
	require.Equal(t, 2, n, "another user's reads must not clear this user's bell")
	require.Nil(t, list[0]["readAt"])
}

func TestRevokeAnotherUsersSessionNeedsRevokeAll(t *testing.T) {
	db := reviewDB(t, &models.UserSession{})
	now := time.Now()
	require.NoError(t, db.Create(&models.UserSession{ID: "admin-session", UserID: 1, IssuedAt: now, ExpiresAt: now.Add(time.Hour), LastActivityAt: now}).Error)

	userAdmin := &models.User{ID: 2, Role: models.RoleUserAdmin, Active: true}
	r := reviewRouter(userAdmin)
	r.DELETE("/sessions/:id", RevokeUserSession(db))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/sessions/admin-session", nil))
	require.Equal(t, http.StatusForbidden, w.Code, "users.read alone must not sign another user out")

	var s models.UserSession
	require.NoError(t, db.First(&s, "id = ?", "admin-session").Error)
	require.Nil(t, s.RevokedAt)
}

func TestUserAdminCannotManageClusterAdmin(t *testing.T) {
	db := reviewDB(t, &models.User{}, &models.Cluster{})
	target := models.User{Username: "ca", Email: "ca@test.local", Password: "x", Role: models.RoleClusterAdmin, Active: true}
	require.NoError(t, db.Create(&target).Error)
	id := strconv.Itoa(int(target.ID))

	r := reviewRouter(&models.User{ID: 999, Role: models.RoleUserAdmin, Active: true})
	r.PATCH("/users/:id", PatchUser(db))
	r.DELETE("/users/:id", DeleteUser(db))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/users/"+id, bytes.NewBufferString(`{"active":false}`)))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/users/"+id, bytes.NewBufferString(`{"role":"operator"}`)))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/users/"+id, nil))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	var after models.User
	require.NoError(t, db.First(&after, target.ID).Error)
	require.True(t, after.Active)
	require.Equal(t, models.RoleClusterAdmin, after.Role)
}

func TestPatchInsightReopen(t *testing.T) {
	db := reviewDB(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{})
	seedTwoClusterFindings(t, db)
	var finding models.Insight
	require.NoError(t, db.Where("cluster_id = ?", "c1").First(&finding).Error)
	resolvedAt := time.Now()
	require.NoError(t, db.Model(&finding).Updates(map[string]any{"status": "resolved", "resolved_at": resolvedAt}).Error)
	path := "/insights/" + strconv.Itoa(int(finding.ID))

	patch := func(u *models.User, body string) int {
		r := reviewRouter(u)
		r.PATCH("/insights/:id", UpdateInsightStatus(db))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}

	// Viewer has no findings.reopen.
	require.Equal(t, http.StatusForbidden, patch(&models.User{ID: 3, Role: models.RoleViewer}, `{"status":"active"}`))
	require.Equal(t, http.StatusOK, patch(&models.User{ID: 4, Role: models.RoleOperator}, `{"status":"active"}`))

	var after models.Insight
	require.NoError(t, db.First(&after, finding.ID).Error)
	require.Equal(t, "active", after.Status)
	require.Nil(t, after.ResolvedAt)

	// An active finding cannot be reopened again.
	require.Equal(t, http.StatusConflict, patch(&models.User{ID: 4, Role: models.RoleOperator}, `{"status":"active"}`))
}
