package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func setupNotificationsTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Notification{},
		&models.Pod{},
		&models.Insight{},
		&models.AttackPath{},
		&models.SBOM{},
		&models.CVEMatch{},
		&models.MalwareMatch{},
	))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_dedupe_key_unique ON notifications(dedupe_key) WHERE dedupe_key <> '' AND deleted_at IS NULL`).Error)

	r := gin.New()
	r.GET("/notifications", GetNotifications(db))
	r.PATCH("/notifications/:id/read", MarkNotificationRead(db))
	r.POST("/notifications/read-all", MarkAllNotificationsRead(db))
	return r, db
}

func TestNotificationsSynthesizesSecurityEventsAndReadState(t *testing.T) {
	r, db := setupNotificationsTestRouter(t)
	now := time.Now().UTC().Add(-5 * time.Minute)

	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a",
		UID:       "pod-core-uid",
		Name:      "fortuna-core",
		Namespace: "fortuna",
	}).Error)
	require.NoError(t, db.Create(&models.Insight{
		ResourceType:      "Pod",
		ResourceNamespace: "fortuna",
		ResourceName:      "fortuna-core",
		ResourceUID:       "pod-core-uid",
		InsightType:       "misconfiguration",
		Severity:          "high",
		Title:             "Privileged container",
		Description:       "test finding",
		Status:            "active",
		DetectedAt:        now,
	}).Error)
	require.NoError(t, db.Create(&models.AttackPath{
		PodUID:    "pod-core-uid",
		PathID:    "path-core-admin",
		Nodes:     "[]",
		Edges:     "[]",
		TotalRisk: 86,
		Length:    4,
		UpdatedAt: now,
	}).Error)
	require.NoError(t, db.Create(&models.SBOM{
		ImageName:   "fortuna-core",
		ImageTag:    "test",
		ImageDigest: "sha256:test",
		PodUID:      "pod-core-uid",
		GeneratedAt: now,
	}).Error)
	require.NoError(t, db.Create(&models.CVEMatch{
		SBOMID:         1,
		PodUID:         "pod-core-uid",
		CVEID:          "CVE-2099-0001",
		PackageName:    "openssl",
		PackageVersion: "1.0.0",
		Severity:       "critical",
		MatchedAt:      now,
	}).Error)
	require.NoError(t, db.Create(&models.MalwareMatch{
		SBOMID:         1,
		PodUID:         "pod-core-uid",
		Namespace:      "fortuna",
		PackageName:    "evil-package",
		PackageVersion: "9.9.9",
		Reason:         "MALWARE",
		MatchedAt:      now,
	}).Error)

	first := getNotificationsForTest(t, r)
	require.EqualValues(t, 4, first.UnreadCount)
	require.EqualValues(t, 4, first.Total)
	require.Len(t, first.Notifications, 4)
	require.ElementsMatch(t, []string{"attack-path", "cve", "malware", "risk"}, notificationCategories(first.Notifications))
	for _, n := range first.Notifications {
		require.Equal(t, "fortuna/fortuna-core", n.ResourceName)
		require.NotContains(t, n.Message, "pod-core-uid")
	}
	require.Contains(t, notificationByCategory(first.Notifications, "risk").Route, "search=fortuna-core")
	require.Contains(t, notificationByCategory(first.Notifications, "cve").Route, "search=fortuna-core")
	require.Contains(t, notificationByCategory(first.Notifications, "malware").Route, "search=fortuna-core")
	require.Equal(t,
		"/risks/findings?search=fortuna-core",
		humanizeNotificationRoute("/risks/findings?search=fortuna%2Ffortuna-core", "pod-core-uid", "fortuna/fortuna-core"),
	)

	second := getNotificationsForTest(t, r)
	require.EqualValues(t, 4, second.Total, "second GET must not duplicate derived notifications")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/notifications/"+first.Notifications[0].IDString()+"/read", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	afterOneRead := getNotificationsForTest(t, r)
	require.EqualValues(t, 3, afterOneRead.UnreadCount)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	afterReadAll := getNotificationsForTest(t, r)
	require.EqualValues(t, 0, afterReadAll.UnreadCount)
	require.EqualValues(t, 4, afterReadAll.Total)
}

type notificationsTestResponse struct {
	Notifications []notificationTestItem `json:"notifications"`
	Total         int64                  `json:"total"`
	UnreadCount   int64                  `json:"unreadCount"`
}

type notificationTestItem struct {
	ID           uint   `json:"id"`
	Category     string `json:"category"`
	Route        string `json:"route"`
	Message      string `json:"message"`
	ResourceName string `json:"resourceName"`
}

func (n notificationTestItem) IDString() string {
	return strconv.FormatUint(uint64(n.ID), 10)
}

func getNotificationsForTest(t *testing.T, r *gin.Engine) notificationsTestResponse {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/notifications?limit=20", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp notificationsTestResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	for _, n := range resp.Notifications {
		require.NotEmpty(t, n.Route)
	}
	return resp
}

func notificationCategories(items []notificationTestItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Category)
	}
	return out
}

func notificationByCategory(items []notificationTestItem, category string) notificationTestItem {
	for _, item := range items {
		if item.Category == category {
			return item
		}
	}
	return notificationTestItem{}
}
