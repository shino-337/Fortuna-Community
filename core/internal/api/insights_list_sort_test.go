package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// seedInsightsSortFixture creates six findings on pod-1..pod-6 (all in cluster c1). Scores exist for
// pods 1-4 only; pods 2 and 3 tie on score, so the id tiebreak decides their order.
func seedInsightsSortFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&models.Cluster{ID: "c1", Name: "cluster1"}).Error)
	rows := []struct {
		sev, title string
		detected   int // hours after base
		updated    int
		score      *float64
	}{
		{"low", "Bravo", 1, 9, f64(10)},
		{"critical", "alpha", 2, 8, f64(50)},
		{"info", "Delta", 3, 7, f64(50)},
		{"high", "charlie", 4, 6, f64(90)},
		{"medium", "Echo", 5, 5, nil},
		{"high", "foxtrot", 6, 4, nil},
	}
	for i, r := range rows {
		uid := "pod-" + string(rune('1'+i))
		require.NoError(t, db.Omit("Containers", "ImageDigests", "PodSecurityContext", "ContainerSecurityContexts", "VolumeMounts", "Volumes", "Tolerations", "Affinity").Create(&models.Pod{UID: uid, Name: uid, Namespace: "default", ClusterID: "c1"}).Error)
		require.NoError(t, db.Create(&models.Insight{
			ClusterID: "c1", ResourceType: "Pod", ResourceUID: uid, ResourceName: uid, ResourceNamespace: "default",
			InsightType: "vulnerability", Severity: r.sev, Title: r.title, Description: "d", Status: "active",
			Evidence: "{}", ViolatedRules: "[]", Remediation: "{}",
			DetectedAt: base.Add(time.Duration(r.detected) * time.Hour), CreatedAt: base,
			UpdatedAt: base.Add(time.Duration(r.updated) * time.Hour),
		}).Error)
		if r.score != nil {
			require.NoError(t, db.Create(&models.RiskScore{
				ResourceType: "Pod", ResourceUID: uid, ResourceName: uid, Namespace: "default", ClusterID: "c1",
				TotalScore: *r.score, PriorityLevel: "P1", ScorerVersion: "v3", Factors: "{}", CalculatedAt: base, CreatedAt: base, UpdatedAt: base,
			}).Error)
		}
	}
	// GORM sets UpdatedAt on create; restore the fixture values.
	for i, r := range rows {
		uid := "pod-" + string(rune('1'+i))
		require.NoError(t, db.Model(&models.Insight{}).Where("resource_uid = ?", uid).
			UpdateColumn("updated_at", base.Add(time.Duration(r.updated)*time.Hour)).Error)
	}
}

func f64(v float64) *float64 { return &v }

type sortListOut struct {
	Total    int `json:"total"`
	Insights []struct {
		ID       uint   `json:"id"`
		Title    string `json:"title"`
		Severity string `json:"severity"`
	} `json:"insights"`
	Groups []struct {
		Title       string `json:"title"`
		MaxSeverity string `json:"max_severity"`
	} `json:"groups"`
}

func getSortedInsights(t *testing.T, h gin.HandlerFunc, query string) sortListOut {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk/insights?clusterId=c1&withScores=1&"+query, nil)
	h(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out sortListOut
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func titlesOf(out sortListOut) []string {
	titles := make([]string, 0, len(out.Insights))
	for _, i := range out.Insights {
		titles = append(titles, i.Title)
	}
	return titles
}

func TestInsightsListServerSort(t *testing.T) {
	for name, db := range dialectTestDBs(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{}) {
		t.Run(name, func(t *testing.T) {
			seedInsightsSortFixture(t, db)
			h := GetInsightsList(db)
			cases := []struct {
				query string
				want  []string
			}{
				// Default (and unknown sort) is detected_at DESC, the order before sort existed.
				{"", []string{"foxtrot", "Echo", "charlie", "Delta", "alpha", "Bravo"}},
				{"sort=bogus&order=asc", []string{"foxtrot", "Echo", "charlie", "Delta", "alpha", "Bravo"}},
				{"sort=detected&order=asc", []string{"Bravo", "alpha", "Delta", "charlie", "Echo", "foxtrot"}},
				{"sort=first_seen&order=asc", []string{"Bravo", "alpha", "Delta", "charlie", "Echo", "foxtrot"}},
				{"sort=updated&order=desc", []string{"Bravo", "alpha", "Delta", "charlie", "Echo", "foxtrot"}},
				{"sort=last_seen&order=asc", []string{"foxtrot", "Echo", "charlie", "Delta", "alpha", "Bravo"}},
				// Severity by rank, ties on id in the same direction.
				{"sort=severity&order=desc", []string{"alpha", "foxtrot", "charlie", "Echo", "Bravo", "Delta"}},
				{"sort=severity&order=asc", []string{"Delta", "Bravo", "Echo", "charlie", "foxtrot", "alpha"}},
				// Unknown order falls back to the column default (desc for severity).
				{"sort=severity&order=sideways", []string{"alpha", "foxtrot", "charlie", "Echo", "Bravo", "Delta"}},
				// Score: unscored findings last in both directions; alpha/Delta tie at 50 and break on id.
				{"sort=score&order=desc", []string{"charlie", "Delta", "alpha", "Bravo", "foxtrot", "Echo"}},
				{"sort=score&order=asc", []string{"Bravo", "alpha", "Delta", "charlie", "Echo", "foxtrot"}},
				// Title is case-insensitive.
				{"sort=title&order=asc", []string{"alpha", "Bravo", "charlie", "Delta", "Echo", "foxtrot"}},
				{"sort=title", []string{"alpha", "Bravo", "charlie", "Delta", "Echo", "foxtrot"}},
				{"sort=TITLE&order=DESC", []string{"foxtrot", "Echo", "Delta", "charlie", "Bravo", "alpha"}},
			}
			for _, tc := range cases {
				out := getSortedInsights(t, h, "page=1&pageSize=20&"+tc.query)
				require.Equal(t, 6, out.Total, tc.query)
				require.Equal(t, tc.want, titlesOf(out), tc.query)
			}
		})
	}
}

// Sorting must be global, not per page: paging through score order yields the full sorted list.
func TestInsightsListServerSortAcrossPages(t *testing.T) {
	for name, db := range dialectTestDBs(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{}) {
		t.Run(name, func(t *testing.T) {
			seedInsightsSortFixture(t, db)
			h := GetInsightsList(db)
			var got []string
			for page := 1; page <= 3; page++ {
				out := getSortedInsights(t, h, "sort=score&order=desc&pageSize=2&page="+string(rune('0'+page)))
				require.Equal(t, 6, out.Total)
				got = append(got, titlesOf(out)...)
			}
			require.Equal(t, []string{"charlie", "Delta", "alpha", "Bravo", "foxtrot", "Echo"}, got)
		})
	}
}

func TestInsightsListServerSortGroupView(t *testing.T) {
	for name, db := range dialectTestDBs(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{}) {
		t.Run(name, func(t *testing.T) {
			seedInsightsSortFixture(t, db)
			out := getSortedInsights(t, GetInsightsList(db), "view=group&sort=severity&order=asc")
			require.Len(t, out.Groups, 6)
			require.Equal(t, "Delta", out.Groups[0].Title)
			require.Equal(t, "alpha", out.Groups[5].Title)
		})
	}
}

func TestInsightsListCachedKeyIncludesSort(t *testing.T) {
	oldCache := defaultRisksCache
	defaultRisksCache = NewMemoryRisksCache(time.Minute)
	t.Cleanup(func() { defaultRisksCache = oldCache })

	db := dialectTestDBs(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{})["sqlite"]
	seedInsightsSortFixture(t, db)
	h := GetInsightsListCached(db)

	asc := getSortedInsights(t, h, "sort=title&order=asc")
	desc := getSortedInsights(t, h, "sort=title&order=desc")
	require.Equal(t, "alpha", asc.Insights[0].Title)
	require.Equal(t, "foxtrot", desc.Insights[0].Title, "order must be part of the cache key")
	bySeverity := getSortedInsights(t, h, "sort=severity&order=desc")
	require.Equal(t, "alpha", bySeverity.Insights[0].Title)
	require.Equal(t, "foxtrot", bySeverity.Insights[1].Title, "sort must be part of the cache key")

	// Aliases and unknown values normalize to the same key as their canonical form.
	k1 := BuildRisksListCacheKey("c1", "active", "", "", "", "", "", 0, 1, 20, 1, 0, "", "detected", "asc")
	s, o := normalizeInsightsListSort("first_seen", "ASC")
	require.Equal(t, k1, BuildRisksListCacheKey("c1", "active", "", "", "", "", "", 0, 1, 20, 1, 0, "", s, o))
	s, o = normalizeInsightsListSort("nope", "asc")
	require.Equal(t, "", s)
	require.Equal(t, "", o)
}

// The score sort LEFT JOINs the preferred-score subquery, which keeps one row per
// (cluster_id, resource_uid): several score rows for one resource must not duplicate findings or
// change the total (which is counted before the join).
func TestInsightsListServerSortScoreJoinDoesNotDuplicateRows(t *testing.T) {
	for name, db := range dialectTestDBs(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{}) {
		t.Run(name, func(t *testing.T) {
			seedInsightsSortFixture(t, db)
			base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
			// Extra score rows for pod-1: an older v3 row, a newer non-v3 row and a row for another
			// resource_type. The preferred row stays the newest v3 one (score 10 from the fixture).
			for _, s := range []models.RiskScore{
				{ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "pod-1", Namespace: "default", ClusterID: "c1", TotalScore: 95, ScorerVersion: "v3", CalculatedAt: base.Add(-time.Hour)},
				{ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "pod-1", Namespace: "default", ClusterID: "c1", TotalScore: 99, ScorerVersion: "v2", CalculatedAt: base.Add(time.Hour)},
				{ResourceType: "Deployment", ResourceUID: "pod-1", ResourceName: "pod-1", Namespace: "default", ClusterID: "c1", TotalScore: 97, ScorerVersion: "v3", CalculatedAt: base.Add(-2 * time.Hour)},
			} {
				s.PriorityLevel, s.Factors = "P1", "{}"
				s.CreatedAt, s.UpdatedAt = base, base
				require.NoError(t, db.Create(&s).Error)
			}
			h := GetInsightsList(db)
			for _, order := range []string{"desc", "asc"} {
				out := getSortedInsights(t, h, "sort=score&order="+order+"&page=1&pageSize=20")
				require.Equal(t, 6, out.Total, order)
				require.Len(t, out.Insights, 6, order)
			}
			// Unscored findings still sort last, and pod-1 keeps its preferred score of 10.
			require.Equal(t, []string{"charlie", "Delta", "alpha", "Bravo", "foxtrot", "Echo"},
				titlesOf(getSortedInsights(t, h, "sort=score&order=desc&page=1&pageSize=20")))
		})
	}
}
