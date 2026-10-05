package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func statsRequest(t *testing.T, h gin.HandlerFunc, user *models.User, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if user != nil {
		c.Set("user", user)
	}
	h(c)
	return w
}

// createStatsPod inserts a pod without its JSON columns (PostgreSQL rejects empty strings as jsonb).
func createStatsPod(t *testing.T, db *gorm.DB, pod models.Pod) models.Pod {
	t.Helper()
	if pod.ServiceAccount == "" {
		pod.ServiceAccount = "default"
	}
	require.NoError(t, db.Omit("Containers", "ImageDigests", "PodSecurityContext", "ContainerSecurityContexts",
		"VolumeMounts", "Volumes", "Tolerations", "Affinity").Create(&pod).Error)
	return pod
}

// TestGetInvestigationCaseStatsPinnedOutput pins GET /investigations/stats: open cases and overdue
// remediation actions over the cases the caller can access (creator or owner, cluster scope).
func TestGetInvestigationCaseStatsPinnedOutput(t *testing.T) {
	for name, db := range dialectTestDBs(t, &models.InvestigationCase{}) {
		t.Run(name, func(t *testing.T) {
			past := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
			pastOffset := time.Now().Add(-72 * time.Hour).In(time.FixedZone("x", 2*3600)).Format(time.RFC3339)
			future := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
			str := func(s string) *string { return &s }
			rem := func(items ...string) string { return "[" + strings.Join(items, ",") + "]" }
			item := func(status, due string) string {
				if due == "" {
					return fmt.Sprintf(`{"id":"x","status":%q}`, status)
				}
				return fmt.Sprintf(`{"id":"x","status":%q,"dueAt":%q}`, status, due)
			}
			cases := []models.InvestigationCase{
				{ID: "k1", Status: "OPEN", CreatedByUserID: 2, Owner: "someone",
					RemediationJSON: rem(item("open", past), item("done", past), item("open", future), item("open", "garbage"), item("open", ""))},
				{ID: "k2", Status: "closed", CreatedByUserID: 1, Owner: "Alice", RemediationJSON: rem(item("open", past))},
				{ID: "k3", Status: "triaged", CreatedByUserID: 3, ClusterID: str("c2"), RemediationJSON: rem(item("in_progress", past))},
				{ID: "k4", Status: "RESOLVED", CreatedByUserID: 3, ClusterID: str("c1"), RemediationJSON: rem(item("open", pastOffset))},
				{ID: "k5", Status: "archived", CreatedByUserID: 1, RemediationJSON: `[{"status":5,"dueAt":"2000-01-01T00:00:00Z"}]`},
				{ID: "k7", Status: "", CreatedByUserID: 1, Owner: "bob", ClusterID: str(" "), RemediationJSON: rem(item("open", past), item("blocked", past))},
				{ID: "k8", Status: "contained", CreatedByUserID: 1, Owner: "carol", RemediationJSON: `{"not":"a list"}`},
				{ID: "k9", Status: "open", CreatedByUserID: 1, Owner: "dave", RemediationJSON: ""},
			}
			for _, k := range cases {
				k.Title = k.ID
				k.EntitiesJSON, k.NotesJSON, k.CollaborationJSON = "[]", "[]", "{}"
				if k.RemediationJSON == "" {
					require.NoError(t, db.Omit("RemediationJSON").Create(&k).Error)
					continue
				}
				require.NoError(t, db.Create(&k).Error)
			}
			gone := models.InvestigationCase{ID: "k6", Title: "k6", Status: "open", CreatedByUserID: 2,
				EntitiesJSON: "[]", NotesJSON: "[]", CollaborationJSON: "{}", RemediationJSON: rem(item("open", past))}
			require.NoError(t, db.Create(&gone).Error)
			require.NoError(t, db.Delete(&gone).Error)

			admin := &models.User{ID: 1, Username: "root", Role: models.RoleAdmin}
			alice := &models.User{ID: 2, Username: "alice", Role: models.RoleOperator}
			bob := &models.User{ID: 3, Username: "bob", Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["c1"]}`}
			badScope := &models.User{ID: 3, Username: "bob", Role: models.RoleOperator, ScopeJSON: `{"namespaces":["x"]}`}
			for _, tc := range []struct {
				user          *models.User
				open, overdue int
			}{
				{admin, 5, 7},
				{alice, 1, 2},
				{bob, 1, 3},
				{badScope, 1, 2},
				{&models.User{ID: 99, Username: "nobody", Role: models.RoleViewer}, 0, 0},
			} {
				w := statsRequest(t, GetInvestigationCaseStats(db), tc.user, "/api/v1/investigations/stats")
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.JSONEq(t, fmt.Sprintf(`{"openCases":%d,"overdueRemediation":%d}`, tc.open, tc.overdue), w.Body.String(), tc.user.Username+" "+tc.user.ScopeJSON)
			}
			w := statsRequest(t, GetInvestigationCaseStats(db), nil, "/api/v1/investigations/stats")
			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

type accessSignal struct {
	Code     string         `json:"code"`
	Severity string         `json:"severity"`
	UserID   uint           `json:"userId"`
	Username string         `json:"username"`
	Role     string         `json:"role"`
	Detail   map[string]any `json:"detail"`
}

func getAccessReview(t *testing.T, db *gorm.DB) (signals []accessSignal, truncated bool, total int) {
	t.Helper()
	w := statsRequest(t, GetGovernanceAccessReview(db), nil, "/api/v1/governance/access-review")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Signals          []accessSignal `json:"signals"`
		SignalsTruncated bool           `json:"signalsTruncated"`
		UserTotal        int            `json:"userTotal"`
		GeneratedAt      string         `json:"generatedAt"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.NotEmpty(t, out.GeneratedAt)
	return out.Signals, out.SignalsTruncated, out.UserTotal
}

func createAccessUser(t *testing.T, db *gorm.DB, u models.User) models.User {
	t.Helper()
	u.Email = u.Username + "@example.test"
	u.Password = "x"
	if u.ScopeJSON == "" {
		u.ScopeJSON = "{}"
	}
	active := u.Active
	require.NoError(t, db.Create(&u).Error)
	// Active has a DB default of true, so GORM skips an explicit false on create.
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", u.ID).Update("active", active).Error)
	return u
}

// TestGetGovernanceAccessReviewPinnedOutput pins the per-user signals of GET /governance/access-review.
func TestGetGovernanceAccessReviewPinnedOutput(t *testing.T) {
	for name, db := range dialectTestDBs(t, &models.User{}, &models.UserSession{}) {
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC()
			days := func(n int) time.Time { return now.Add(-time.Duration(n) * 24 * time.Hour) }
			nine := `{"clusters":["1","2","3","4","5","6","7","8","9"]}`
			eight := `{"cluster_ids":["1","2","3","4","5","6","7","8"]}`
			users := []models.User{
				{Username: "dormant-admin", Role: "admin", Active: true, LastLogin: days(100), CreatedAt: days(400)},
				{Username: "never-admin", Role: "admin", Active: true, CreatedAt: days(200)},
				{Username: "stale-op", Role: "operator", Active: true, LastLogin: days(200), CreatedAt: days(400)},
				{Username: "old-viewer", Role: "viewer", Active: true, LastLogin: days(400), CreatedAt: days(500)},
				{Username: "inactive-op", Role: "operator", Active: false, LastLogin: days(400), CreatedAt: days(500)},
				{Username: "wide-scope", Role: "operator", Active: true, LastLogin: days(1), CreatedAt: days(10), ScopeJSON: nine},
				{Username: "eight-scope", Role: "operator", Active: true, LastLogin: days(1), CreatedAt: days(10), ScopeJSON: eight},
				{Username: "spaced-admin", Role: " Admin ", Active: true, LastLogin: days(120), CreatedAt: days(400)},
				{Username: "inactive-admin", Role: "admin", Active: false, LastLogin: days(300), CreatedAt: days(400)},
				{Username: "new-admin", Role: "admin", Active: true, CreatedAt: days(5)},
				{Username: "busy-admin", Role: "admin", Active: true, LastLogin: days(1), CreatedAt: days(400)},
				{Username: "wide-stale", Role: "cluster_admin", Active: true, LastLogin: days(181), CreatedAt: days(400), ScopeJSON: nine},
			}
			ids := map[string]uint{}
			for _, u := range users {
				created := createAccessUser(t, db, u)
				ids[u.Username] = created.ID
			}
			gone := createAccessUser(t, db, models.User{Username: "deleted-admin", Role: "admin", Active: true, LastLogin: days(300), CreatedAt: days(400)})
			require.NoError(t, db.Delete(&gone).Error)
			require.NoError(t, db.Create(&models.UserSession{ID: "s-idle", UserID: ids["stale-op"], IssuedAt: days(40), ExpiresAt: now.Add(time.Hour), LastActivityAt: days(35)}).Error)
			require.NoError(t, db.Create(&models.UserSession{ID: "s-live", UserID: ids["stale-op"], IssuedAt: days(1), ExpiresAt: now.Add(time.Hour), LastActivityAt: days(1)}).Error)

			signals, truncated, total := getAccessReview(t, db)
			require.False(t, truncated)
			require.Equal(t, 12, total)
			type sig struct{ Code, Severity, Username, Role, Detail string }
			got := make([]sig, 0, len(signals))
			for _, s := range signals {
				detail := ""
				if s.Detail != nil {
					delete(s.Detail, "lastLogin")
					b, _ := json.Marshal(s.Detail)
					detail = string(b)
				}
				if s.Username != "" {
					require.Equal(t, ids[s.Username], s.UserID)
				}
				got = append(got, sig{s.Code, s.Severity, s.Username, s.Role, detail})
			}
			want := []sig{
				{"DORMANT_ADMIN", "CRITICAL", "dormant-admin", "admin", "{}"},
				{"DORMANT_ADMIN", "CRITICAL", "never-admin", "admin", "{}"},
				{"STALE_ACCOUNT", "HIGH", "never-admin", "admin", ""},
				{"STALE_ACCOUNT", "HIGH", "stale-op", "operator", ""},
				{"EXCESSIVE_SCOPE", "MEDIUM", "wide-scope", "", `{"clusterCount":9}`},
				{"DORMANT_ADMIN", "CRITICAL", "spaced-admin", " Admin ", "{}"},
				{"STALE_ACCOUNT", "HIGH", "wide-stale", "cluster_admin", ""},
				{"EXCESSIVE_SCOPE", "MEDIUM", "wide-stale", "", `{"clusterCount":9}`},
				{"PRIVILEGE_CONCENTRATION", "HIGH", "", "", `{"adminCount":6}`},
				{"INACTIVE_SESSIONS", "MEDIUM", "", "", `{"approxCount":1}`},
			}
			require.Equal(t, want, got)
		})
	}
}

// Truncation keeps aggregate signals first and then per-user signals in user order.
func TestGetGovernanceAccessReviewTruncation(t *testing.T) {
	for name, db := range dialectTestDBs(t, &models.User{}) {
		t.Run(name, func(t *testing.T) {
			old := time.Now().UTC().Add(-400 * 24 * time.Hour)
			var users []models.User
			for i := 0; i < accessReviewMaxSignals+5; i++ {
				users = append(users, models.User{Username: fmt.Sprintf("u%04d", i), Email: fmt.Sprintf("u%04d@x", i), Password: "x",
					Role: "operator", Active: true, LastLogin: old, CreatedAt: old, ScopeJSON: "{}"})
			}
			for i := 0; i < 6; i++ {
				users = append(users, models.User{Username: fmt.Sprintf("admin%d", i), Email: fmt.Sprintf("admin%d@x", i), Password: "x",
					Role: "admin", Active: false, LastLogin: old, CreatedAt: old, ScopeJSON: "{}"})
			}
			require.NoError(t, db.CreateInBatches(users, 200).Error)
			signals, truncated, total := getAccessReview(t, db)
			require.True(t, truncated)
			require.Equal(t, accessReviewMaxSignals+11, total)
			require.Len(t, signals, accessReviewMaxSignals)
			require.Equal(t, "PRIVILEGE_CONCENTRATION", signals[0].Code)
			names := make([]string, 0, len(signals)-1)
			for _, s := range signals[1:] {
				require.Equal(t, "STALE_ACCOUNT", s.Code)
				names = append(names, s.Username)
			}
			require.True(t, sort.StringsAreSorted(names))
			require.Equal(t, "u0000", names[0])
			require.Equal(t, fmt.Sprintf("u%04d", accessReviewMaxSignals-2), names[len(names)-1])
		})
	}
}

// TestGetPodCapabilitiesTrendPinnedOutput pins GET /pod-capabilities/trends: one zero-filled point per
// UTC day, counts per severity, live pods only, filters and cluster scope.
func TestGetPodCapabilitiesTrendPinnedOutput(t *testing.T) {
	for name, db := range dialectTestDBs(t, &models.Cluster{}, &models.Pod{}, &models.PodCapability{}) {
		t.Run(name, func(t *testing.T) {
			today := time.Now().UTC().Truncate(24 * time.Hour)
			require.NoError(t, db.Create(&models.Cluster{ID: "c1", Name: "c1"}).Error)
			require.NoError(t, db.Create(&models.Cluster{ID: "c2", Name: "c2"}).Error)
			createStatsPod(t, db, models.Pod{UID: "p1", Name: "p1", Namespace: "ns1", ClusterID: "c1"})
			createStatsPod(t, db, models.Pod{UID: "p2", Name: "p2", Namespace: "ns2", ClusterID: "c1"})
			createStatsPod(t, db, models.Pod{UID: "p3", Name: "p3", Namespace: "ns1", ClusterID: "c2"})
			dead := createStatsPod(t, db, models.Pod{UID: "p4", Name: "p4", Namespace: "ns1", ClusterID: "c1"})
			require.NoError(t, db.Delete(&dead).Error)
			n := 0
			capability := func(pod, cluster, ns, sev string, at time.Time) {
				n++
				pc := models.PodCapability{ClusterID: cluster, PodUID: pod, Namespace: ns, CapabilityID: fmt.Sprintf("CAP_%d", n),
					CapabilityGroup: "g", Severity: sev, State: "detected", Confidence: 0.5, Evidence: "{}", DerivedFrom: "[]",
					CreatedAt: at, UpdatedAt: at}
				require.NoError(t, db.Create(&pc).Error)
			}
			capability("p1", "c1", "ns1", "critical", today.Add(time.Second))
			capability("p1", "c1", "ns1", "HIGH", today.Add(23*time.Hour))
			capability("p1", "c1", "ns1", "info", today.Add(time.Hour))
			capability("p2", "c1", "ns2", "medium", today.Add(-12*time.Hour))
			capability("p2", "c1", "ns2", "Low", today.Add(-24*time.Hour-time.Second))
			capability("p1", "c1", "ns1", "low", today.Add(-48*time.Hour))
			capability("p1", "c1", "ns1", "critical", today.Add(-72*time.Hour))
			capability("p1", "c1", "ns1", "critical", today.Add(24*time.Hour))
			capability("p3", "c2", "ns1", "high", today.Add(-time.Hour))
			capability("p4", "c1", "ns1", "critical", today)
			capability("p1", "c2", "ns1", "critical", today) // pod identity is cluster-qualified: no c2/p1 pod

			type point struct {
				Date                        string
				Critical, High, Medium, Low int
			}
			day := func(offset int) string { return today.AddDate(0, 0, offset).Format("2006-01-02") }
			admin := &models.User{Role: models.RoleAdmin}
			scoped := &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["c2"]}`}
			for _, tc := range []struct {
				user  *models.User
				query string
				want  []point
			}{
				{admin, "days=3", []point{{day(-2), 0, 0, 0, 2}, {day(-1), 0, 1, 1, 0}, {day(0), 1, 1, 0, 0}}},
				{admin, "days=4&clusterId=c1", []point{{day(-3), 1, 0, 0, 0}, {day(-2), 0, 0, 0, 2}, {day(-1), 0, 0, 1, 0}, {day(0), 1, 1, 0, 0}}},
				{admin, "days=2&namespace=ns2", []point{{day(-1), 0, 0, 1, 0}, {day(0), 0, 0, 0, 0}}},
				{admin, "days=3&podUid=p2", []point{{day(-2), 0, 0, 0, 1}, {day(-1), 0, 0, 1, 0}, {day(0), 0, 0, 0, 0}}},
				{admin, "days=3&capabilityId=CAP_1", []point{{day(-2), 0, 0, 0, 0}, {day(-1), 0, 0, 0, 0}, {day(0), 1, 0, 0, 0}}},
				{scoped, "days=2", []point{{day(-1), 0, 1, 0, 0}, {day(0), 0, 0, 0, 0}}},
				{admin, "days=0", nil},
			} {
				w := statsRequest(t, GetPodCapabilitiesTrend(db), tc.user, "/api/v1/inventory/pod-capabilities/trends?"+tc.query)
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				var out struct {
					Points []point `json:"points"`
					Total  int     `json:"total"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
				if tc.want == nil {
					require.Len(t, out.Points, 7, "invalid days falls back to 7")
					require.Equal(t, 7, out.Total)
					continue
				}
				require.Equal(t, tc.want, out.Points, tc.query)
				require.Equal(t, len(tc.want), out.Total)
			}
		})
	}
}
