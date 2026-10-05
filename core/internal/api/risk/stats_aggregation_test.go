package risk

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/fortuna/core/pkg/models"
)

// riskDialectDBs returns SQLite and, when FORTUNA_TEST_POSTGRES_URL is set, PostgreSQL (in a
// throwaway schema), both migrated with RiskScore, so aggregation SQL is checked on both dialects.
func riskDialectDBs(t *testing.T) map[string]*gorm.DB {
	t.Helper()
	out := map[string]*gorm.DB{}
	lite, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	litePool, err := lite.DB()
	require.NoError(t, err)
	litePool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = litePool.Close() })
	out["sqlite"] = lite

	if dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL"); dsn != "" {
		admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		adminSQL, err := admin.DB()
		require.NoError(t, err)
		schema := fmt.Sprintf("risk_stats_%d", time.Now().UnixNano())
		require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
		t.Cleanup(func() {
			_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
			_ = adminSQL.Close()
		})
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		pg, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		pgPool, err := pg.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = pgPool.Close() })
		out["postgres"] = pg
	}
	for _, db := range out {
		require.NoError(t, db.AutoMigrate(&models.RiskScore{}))
	}
	return out
}

func statsRouter(db *gorm.DB) func(path, who string) *httptest.ResponseRecorder {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		who := c.GetHeader("Test-User")
		u := &models.User{Role: models.RoleViewer}
		if who == "admin" {
			u.Role = models.RoleAdmin
		} else {
			u.ScopeJSON = `{"cluster_ids":["` + who + `"]}`
		}
		c.Set("user", u)
	})
	r.GET("/risk/trends", GetRiskTrends(db))
	r.GET("/risk/scores", GetRiskScores(db))
	return func(path, who string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Test-User", who)
		r.ServeHTTP(w, req)
		return w
	}
}

func createScore(t *testing.T, db *gorm.DB, s models.RiskScore) models.RiskScore {
	t.Helper()
	if s.ResourceType == "" {
		s.ResourceType = "Pod"
	}
	if s.ScorerVersion == "" {
		s.ScorerVersion = "v3"
	}
	if s.Factors == "" {
		s.Factors = "{}"
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = s.CalculatedAt
		s.UpdatedAt = s.CalculatedAt
	}
	require.NoError(t, db.Create(&s).Error)
	return s
}

// TestGetRiskTrendsPinnedOutput pins GET /risk/trends: daily UTC buckets, average total_score and
// counts per unified level (critical >= 70, high >= 40, medium >= 20, low otherwise), cluster scope.
func TestGetRiskTrendsPinnedOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, db := range riskDialectDBs(t) {
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC()
			d1 := now.AddDate(0, 0, -3)
			d2 := now.AddDate(0, 0, -1)
			for _, s := range []models.RiskScore{
				{ResourceUID: "a1", ClusterID: "a", TotalScore: 70, CalculatedAt: d1},
				{ResourceUID: "a2", ClusterID: "a", TotalScore: 69.99, CalculatedAt: d1},
				{ResourceUID: "a3", ClusterID: "a", TotalScore: 40, CalculatedAt: d1},
				{ResourceUID: "a4", ClusterID: "a", TotalScore: 20, CalculatedAt: d1},
				{ResourceUID: "a5", ClusterID: "a", TotalScore: 19.99, CalculatedAt: d1},
				{ResourceUID: "a6", ClusterID: "a", TotalScore: 0, CalculatedAt: d1},
				{ResourceUID: "a7", ClusterID: "a", TotalScore: 100, CalculatedAt: d2, ScorerVersion: "v2"},
				{ResourceUID: "a8", ClusterID: "a", TotalScore: 35, CalculatedAt: d2},
				{ResourceUID: "old", ClusterID: "a", TotalScore: 90, CalculatedAt: now.AddDate(0, 0, -40)},
				{ResourceUID: "b1", ClusterID: "b", TotalScore: 80, CalculatedAt: d2},
			} {
				createScore(t, db, s)
			}
			deleted := createScore(t, db, models.RiskScore{ResourceUID: "gone", ClusterID: "a", TotalScore: 50, CalculatedAt: d2})
			require.NoError(t, db.Delete(&deleted).Error)

			type point struct {
				Date          string  `json:"date"`
				AvgScore      float64 `json:"avgScore"`
				CriticalCount int64   `json:"criticalCount"`
				HighCount     int64   `json:"highCount"`
				MediumCount   int64   `json:"mediumCount"`
				LowCount      int64   `json:"lowCount"`
			}
			type resp struct {
				Trends     []point `json:"trends"`
				PeriodDays int     `json:"period_days"`
				TotalDays  int     `json:"total_days"`
			}
			day1, day2 := d1.Format("2006-01-02"), d2.Format("2006-01-02")
			cases := []struct {
				path, who string
				want      resp
			}{
				{"/risk/trends", "admin", resp{PeriodDays: 30, TotalDays: 2, Trends: []point{
					{day1, 219.98 / 6, 1, 2, 1, 2},
					{day2, 215.0 / 3, 2, 0, 1, 0},
				}}},
				{"/risk/trends?cluster=b", "admin", resp{PeriodDays: 30, TotalDays: 1, Trends: []point{{day2, 80, 1, 0, 0, 0}}}},
				{"/risk/trends", "a", resp{PeriodDays: 30, TotalDays: 2, Trends: []point{
					{day1, 219.98 / 6, 1, 2, 1, 2},
					{day2, 67.5, 1, 0, 1, 0},
				}}},
				{"/risk/trends?days=2", "a", resp{PeriodDays: 2, TotalDays: 1, Trends: []point{{day2, 67.5, 1, 0, 1, 0}}}},
				{"/risk/trends?days=999", "b", resp{PeriodDays: 30, TotalDays: 1, Trends: []point{{day2, 80, 1, 0, 0, 0}}}},
				{"/risk/trends?cluster=zzz", "admin", resp{PeriodDays: 30, TotalDays: 0, Trends: []point{}}},
			}
			request := statsRouter(db)
			for _, tc := range cases {
				w := request(tc.path, tc.who)
				require.Equal(t, 200, w.Code, w.Body.String())
				var raw map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
				require.Len(t, raw, 3, "response keys")
				var got resp
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
				require.NotNil(t, got.Trends, "trends must be an array, not null")
				require.Equal(t, tc.want.PeriodDays, got.PeriodDays, tc.path)
				require.Equal(t, tc.want.TotalDays, got.TotalDays, tc.path)
				require.Len(t, got.Trends, len(tc.want.Trends), tc.path)
				for i := range tc.want.Trends {
					w, g := tc.want.Trends[i], got.Trends[i]
					require.InDelta(t, w.AvgScore, g.AvgScore, 1e-9, tc.path)
					w.AvgScore, g.AvgScore = 0, 0
					require.Equal(t, w, g, tc.path)
				}
			}
		})
	}
}

// TestGetRiskScoresPinnedOutput pins GET /risk/scores: latest v3 row per resource, filters,
// finalLevel, every sortBy mode with id tiebreak, pagination and cluster scope.
func TestGetRiskScoresPinnedOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, db := range riskDialectDBs(t) {
		t.Run(name, func(t *testing.T) {
			base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
			at := func(h int) time.Time { return base.Add(time.Duration(h) * time.Hour) }
			for _, s := range []models.RiskScore{
				{ResourceUID: "r1", ClusterID: "a", Namespace: "Beta", ResourceName: "zeta", TotalScore: 80, CalculatedAt: at(1)},
				{ResourceUID: "r2", ClusterID: "a", Namespace: "alpha", ResourceName: "Alpha", TotalScore: 75, CalculatedAt: at(5)},
				{ResourceUID: "r3", ClusterID: "a", Namespace: "alpha", ResourceName: "beta", TotalScore: 75, CalculatedAt: at(5)},
				{ResourceUID: "r4", ClusterID: "a", Namespace: "gamma", ResourceName: "delta", TotalScore: 99, CalculatedAt: at(9), ScorerVersion: "v2"},
				{ResourceUID: "r5", ClusterID: "a", Namespace: "gamma", ResourceName: "epsilon", TotalScore: 45, CalculatedAt: at(3)},
				{ResourceUID: "r5", ClusterID: "a", Namespace: "gamma", ResourceName: "epsilon", TotalScore: 10, CalculatedAt: at(8), ScorerVersion: "v2"},
				{ResourceUID: "r6", ClusterID: "a", Namespace: "alpha", ResourceName: "Alpha", TotalScore: 10, CalculatedAt: at(2)},
				{ResourceUID: "r7", ClusterID: "b", Namespace: "alpha", ResourceName: "omega", TotalScore: 95, CalculatedAt: at(4)},
				{ResourceUID: "r9", ClusterID: "a", Namespace: "beta", ResourceName: "_under", TotalScore: 20, CalculatedAt: at(7)},
				{ResourceUID: "r1", ClusterID: "a", Namespace: "Beta", ResourceName: "zeta", TotalScore: 30, CalculatedAt: at(6)},
				{ResourceUID: "r3", ClusterID: "a", Namespace: "alpha", ResourceName: "beta", TotalScore: 75, CalculatedAt: at(5), ResourceType: "Deployment"},
			} {
				createScore(t, db, s)
			}
			deleted := createScore(t, db, models.RiskScore{ResourceUID: "r8", ClusterID: "a", Namespace: "alpha", ResourceName: "gone", TotalScore: 99, CalculatedAt: at(9)})
			require.NoError(t, db.Delete(&deleted).Error)

			type row struct {
				UID   string
				Type  string
				Score float64
				Level string
			}
			type resp struct {
				Rows     []row
				Total    int
				Page     int
				PageSize int
				SortBy   string
			}
			r := func(uid string, score float64, level string) row { return row{uid, "Pod", score, level} }
			r3dep := row{"r3", "Deployment", 75, "critical"}
			cases := []struct {
				path, who string
				want      resp
			}{
				{"/risk/scores", "admin", resp{[]row{r("r7", 95, "critical"), r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r5", 45, "high"), r("r1", 30, "medium"), r("r9", 20, "medium"), r("r6", 10, "low")}, 8, 1, 50, "score"}},
				{"/risk/scores?sortBy=priority", "admin", resp{[]row{r("r7", 95, "critical"), r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r5", 45, "high"), r("r1", 30, "medium"), r("r9", 20, "medium"), r("r6", 10, "low")}, 8, 1, 50, "priority"}},
				{"/risk/scores?sortBy=name", "admin", resp{[]row{r("r9", 20, "medium"), r("r2", 75, "critical"), r("r6", 10, "low"), r("r3", 75, "critical"), r3dep, r("r5", 45, "high"), r("r7", 95, "critical"), r("r1", 30, "medium")}, 8, 1, 50, "name"}},
				{"/risk/scores?sortBy=namespace", "admin", resp{[]row{r("r7", 95, "critical"), r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r6", 10, "low"), r("r9", 20, "medium"), r("r1", 30, "medium"), r("r5", 45, "high")}, 8, 1, 50, "namespace"}},
				{"/risk/scores?sortBy=calculated", "admin", resp{[]row{r("r9", 20, "medium"), r("r1", 30, "medium"), r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r7", 95, "critical"), r("r5", 45, "high"), r("r6", 10, "low")}, 8, 1, 50, "calculated"}},
				{"/risk/scores?sortBy=bogus", "admin", resp{[]row{r("r7", 95, "critical"), r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r5", 45, "high"), r("r1", 30, "medium"), r("r9", 20, "medium"), r("r6", 10, "low")}, 8, 1, 50, "score"}},
				{"/risk/scores?finalLevel=critical", "admin", resp{[]row{r("r7", 95, "critical"), r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep}, 4, 1, 50, "score"}},
				{"/risk/scores?finalLevel=MEDIUM", "admin", resp{[]row{r("r1", 30, "medium"), r("r9", 20, "medium")}, 2, 1, 50, "score"}},
				{"/risk/scores?finalLevel=bogus", "admin", resp{[]row{}, 0, 1, 50, "score"}},
				{"/risk/scores?minScore=20&maxScore=75", "admin", resp{[]row{r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r5", 45, "high"), r("r1", 30, "medium"), r("r9", 20, "medium")}, 6, 1, 50, "score"}},
				{"/risk/scores?minScore=x&maxScore=40", "admin", resp{[]row{r("r1", 30, "medium"), r("r9", 20, "medium"), r("r6", 10, "low")}, 3, 1, 50, "score"}},
				{"/risk/scores?namespace=alpha&type=Pod", "admin", resp{[]row{r("r7", 95, "critical"), r("r2", 75, "critical"), r("r3", 75, "critical"), r("r6", 10, "low")}, 4, 1, 50, "score"}},
				{"/risk/scores?page=2&pageSize=3", "admin", resp{[]row{r3dep, r("r5", 45, "high"), r("r1", 30, "medium")}, 8, 2, 3, "score"}},
				{"/risk/scores?page=9&pageSize=3", "admin", resp{[]row{}, 8, 9, 3, "score"}},
				{"/risk/scores?page=0&pageSize=0", "admin", resp{[]row{r("r7", 95, "critical"), r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r5", 45, "high"), r("r1", 30, "medium"), r("r9", 20, "medium"), r("r6", 10, "low")}, 8, 1, 50, "score"}},
				{"/risk/scores?pageSize=100000", "admin", resp{[]row{r("r7", 95, "critical"), r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r5", 45, "high"), r("r1", 30, "medium"), r("r9", 20, "medium"), r("r6", 10, "low")}, 8, 1, 500, "score"}},
				{"/risk/scores", "a", resp{[]row{r("r2", 75, "critical"), r("r3", 75, "critical"), r3dep, r("r5", 45, "high"), r("r1", 30, "medium"), r("r9", 20, "medium"), r("r6", 10, "low")}, 7, 1, 50, "score"}},
				{"/risk/scores?cluster=b", "admin", resp{[]row{r("r7", 95, "critical")}, 1, 1, 50, "score"}},
			}
			request := statsRouter(db)
			for _, tc := range cases {
				w := request(tc.path, tc.who)
				require.Equal(t, 200, w.Code, w.Body.String())
				var raw struct {
					Scores []map[string]json.RawMessage `json:"scores"`
					Total  int                          `json:"total"`
					Page   int                          `json:"page"`
					Size   int                          `json:"pageSize"`
					SortBy string                       `json:"sortBy"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
				require.NotNil(t, raw.Scores, "scores must be an array, not null")
				got := resp{Rows: []row{}, Total: raw.Total, Page: raw.Page, PageSize: raw.Size, SortBy: raw.SortBy}
				for _, s := range raw.Scores {
					var gr row
					require.NoError(t, json.Unmarshal(s["resourceUid"], &gr.UID))
					require.NoError(t, json.Unmarshal(s["resourceType"], &gr.Type))
					require.NoError(t, json.Unmarshal(s["totalScore"], &gr.Score))
					require.NoError(t, json.Unmarshal(s["final_level"], &gr.Level))
					for _, key := range []string{"id", "resourceName", "namespace", "clusterId", "baseScore", "severityWeight", "impactMultiplier", "timeDecay", "factors", "insightsCount", "highestSeverity", "calculatedAt", "createdAt", "updatedAt", "deletedAt", "scorerVersion", "risk", "final_score", "meta"} {
						require.Contains(t, s, key, tc.path)
					}
					got.Rows = append(got.Rows, gr)
				}
				require.Equal(t, tc.want, got, tc.path)
			}
		})
	}
}
