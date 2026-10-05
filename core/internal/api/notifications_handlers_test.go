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
		&models.NotificationRead{},
		&models.Pod{},
		&models.Insight{},
		&models.AttackPath{},
		&models.SBOM{},
		&models.CVEMatch{},
		&models.MalwareMatch{},
		&models.RiskScore{},
	))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_dedupe_key_unique ON notifications(dedupe_key) WHERE dedupe_key <> '' AND deleted_at IS NULL`).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: 1, Role: models.RoleAdmin, Active: true})
		c.Next()
	})
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
	require.NoError(t, db.Create(&models.RiskScore{
		ClusterID: "cluster-a", ResourceType: "Pod", ResourceUID: "pod-core-uid", TotalScore: 55, ScorerVersion: "v3", CalculatedAt: now,
	}).Error)
	require.NoError(t, db.Create(&models.Insight{
		ClusterID:         "cluster-a",
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
		TotalRisk: 9.2,
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
	require.Regexp(t, `^/risks/\d+$`, notificationByCategory(first.Notifications, "risk").Route, "a finding alert opens that finding")
	require.Equal(t, "high", notificationByCategory(first.Notifications, "risk").Severity, "score 55 is the high risk level")
	require.Equal(t, "critical", notificationByCategory(first.Notifications, "attack-path").Severity, "9.2/10 is a critical path")
	require.Equal(t, "/resources/pods/uid/pod-core-uid?tab=sbom", notificationByCategory(first.Notifications, "cve").Route)
	require.Equal(t, "/resources/pods/uid/pod-core-uid?tab=sbom", notificationByCategory(first.Notifications, "malware").Route)
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
	Severity     string `json:"severity"`
	Title        string `json:"title"`
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

// A finding's alert must carry the same level as the finding page it opens: the
// band of its resource's risk score, not the rule severity.
func TestFindingNotificationsUseRiskLevel(t *testing.T) {
	r, db := setupNotificationsTestRouter(t)
	now := time.Now().UTC().Add(-5 * time.Minute)
	pod := func(uid string) {
		require.NoError(t, db.Create(&models.Pod{ClusterID: "c1", UID: uid, Name: uid, Namespace: "ns"}).Error)
	}
	finding := func(uid, severity, title string) models.Insight {
		i := models.Insight{
			ClusterID: "c1", ResourceType: "Pod", ResourceUID: uid, ResourceName: uid, ResourceNamespace: "ns",
			InsightType: "vulnerability", Severity: severity, Title: title, Description: title, Status: "active", DetectedAt: now,
		}
		require.NoError(t, db.Create(&i).Error)
		return i
	}
	score := func(uid string, total float64, at time.Time) {
		require.NoError(t, db.Create(&models.RiskScore{
			ClusterID: "c1", ResourceType: "Pod", ResourceUID: uid, TotalScore: total, ScorerVersion: "v3", CalculatedAt: at,
		}).Error)
	}
	for _, uid := range []string{"low-pod", "crit-pod", "drop-pod"} {
		pod(uid)
	}
	finding("low-pod", "critical", "rule says critical")     // score 25: medium risk
	escalated := finding("crit-pod", "low", "rule says low") // score 75: critical risk
	dropping := finding("drop-pod", "high", "will drop")     // score 50, then 10
	score("low-pod", 25, now)
	score("crit-pod", 75, now)
	score("drop-pod", 50, now)

	byTitle := func(resp notificationsTestResponse) map[string]notificationTestItem {
		out := map[string]notificationTestItem{}
		for _, n := range resp.Notifications {
			if n.Category == "risk" {
				out[n.Route] = n
			}
		}
		return out
	}
	first := byTitle(getNotificationsForTest(t, r))
	require.Len(t, first, 2, "the critical-rule finding on a medium-risk pod raises no alert")
	crit := first["/risks/"+strconv.FormatUint(uint64(escalated.ID), 10)]
	require.Equal(t, "critical", crit.Severity)
	require.Contains(t, crit.Title, "Critical risk finding")
	require.Contains(t, crit.Message, "rule severity low")
	require.Equal(t, "high", first["/risks/"+strconv.FormatUint(uint64(dropping.ID), 10)].Severity)

	// The pod is rescored to low: its stored alert goes away instead of staying "high".
	score("drop-pod", 10, now.Add(time.Minute))
	// The escalated pod drops to high: its alert follows.
	score("crit-pod", 45, now.Add(time.Minute))
	second := byTitle(getNotificationsForTest(t, r))
	require.Len(t, second, 1)
	require.Equal(t, "high", second["/risks/"+strconv.FormatUint(uint64(escalated.ID), 10)].Severity)
	require.Contains(t, second["/risks/"+strconv.FormatUint(uint64(escalated.ID), 10)].Title, "High risk finding")

	// Resolving the finding removes its alert.
	require.NoError(t, db.Model(&models.Insight{}).Where("id = ?", escalated.ID).Update("status", "resolved").Error)
	require.Empty(t, byTitle(getNotificationsForTest(t, r)))
}

// Alerts stored before this rule (rule severity, search link) are corrected on the next read.
func TestStoredFindingNotificationIsReconciled(t *testing.T) {
	r, db := setupNotificationsTestRouter(t)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Pod{ClusterID: "c1", UID: "p", Name: "p", Namespace: "ns"}).Error)
	i := models.Insight{ClusterID: "c1", ResourceType: "Pod", ResourceUID: "p", ResourceName: "p", ResourceNamespace: "ns",
		InsightType: "vulnerability", Severity: "critical", Title: "t", Description: "t", Status: "active", DetectedAt: now}
	require.NoError(t, db.Create(&i).Error)
	require.NoError(t, db.Create(&models.RiskScore{ClusterID: "c1", ResourceType: "Pod", ResourceUID: "p", TotalScore: 15, ScorerVersion: "v3", CalculatedAt: now}).Error)
	require.NoError(t, db.Create(&models.Notification{
		Title: "Critical finding: t", Severity: "critical", Category: "risk", Source: "risk-engine",
		Route: "/risks/findings?search=p", DedupeKey: "insight:" + strconv.FormatUint(uint64(i.ID), 10), ClusterID: "c1", CreatedAt: now,
	}).Error)

	resp := getNotificationsForTest(t, r)
	require.Zero(t, resp.Total, "a stored critical alert for a low-risk finding is removed")
}
