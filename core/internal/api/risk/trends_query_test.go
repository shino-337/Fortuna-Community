package risk

import (
	"fmt"
	"gorm.io/driver/postgres"
	"os"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRiskTrendsBoundedAggregationScopeAndCalendar(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()
	testTrendAggregation(t, db)
}

func TestRiskTrendsAggregationPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	defer pool.Close()
	schema := fmt.Sprintf("trend_calendar_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE SCHEMA "+schema).Error)
	require.NoError(t, db.Exec("SET search_path TO "+schema).Error)
	defer func() { db.Exec("SET search_path TO public"); db.Exec("DROP SCHEMA " + schema + " CASCADE") }()
	testTrendAggregation(t, db)
}

func testTrendAggregation(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&models.RiskScore{}))

	parse := func(value string) time.Time {
		when, err := time.Parse(time.RFC3339, value)
		require.NoError(t, err)
		return when
	}
	rows := []models.RiskScore{{ResourceUID: "sunday", ClusterID: "a", Namespace: "one", TotalScore: 90, PriorityLevel: "P0", CalculatedAt: parse("2026-01-04T23:00:00Z")}, {ResourceUID: "monday", ClusterID: "a", Namespace: "one", TotalScore: 10, PriorityLevel: "P3", CalculatedAt: parse("2026-01-05T00:00:00Z")}, {ResourceUID: "tuesday", ClusterID: "a", Namespace: "one", TotalScore: 30, PriorityLevel: "P2", CalculatedAt: parse("2026-01-06T00:00:00Z")}, {ResourceUID: "february", ClusterID: "a", Namespace: "one", TotalScore: 70, PriorityLevel: "P1", CalculatedAt: parse("2026-02-11T00:00:00Z")}, {ResourceUID: "foreign", ClusterID: "b", Namespace: "one", TotalScore: 100, PriorityLevel: "P0", CalculatedAt: parse("2026-01-05T00:00:00Z")}, {ResourceUID: "namespace", ClusterID: "a", Namespace: "two", TotalScore: 100, PriorityLevel: "P0", CalculatedAt: parse("2026-01-05T00:00:00Z")}, {ResourceUID: "old", ClusterID: "a", Namespace: "one", TotalScore: 100, PriorityLevel: "P0", CalculatedAt: parse("2025-12-31T23:59:59Z")}, {ResourceUID: "deleted", ClusterID: "a", Namespace: "one", TotalScore: 100, PriorityLevel: "P0", CalculatedAt: parse("2026-01-05T00:00:00Z"), DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}}
	for _, row := range rows {
		row.ResourceType = "Pod"
		require.NoError(t, db.Create(&row).Error)
	}
	scope := analyticsScope{restricted: true, clusterIDs: []string{"a"}}
	for _, period := range []string{"daily", "weekly", "monthly", "yearly"} {
		t.Run(period, func(t *testing.T) {
			points, err := loadRiskTrendPoints(db, scope, period, "one", parse("2026-01-01T00:00:00Z"))
			require.NoError(t, err)
			var total, p0 int64
			for _, p := range points {
				total += p.Count
				p0 += p.P0Count
			}
			require.EqualValues(t, 4, total)
			require.EqualValues(t, 1, p0)
			switch period {
			case "daily":
				require.Len(t, points, 4)
				require.Equal(t, "2026-01-04", points[0].Date)
			case "weekly":
				require.Len(t, points, 3)
				require.Equal(t, "2025-12-29", points[0].Date)
				require.Equal(t, "2026-01-05", points[1].Date)
				require.Equal(t, float64(20), points[1].AvgScore)
				require.EqualValues(t, 2, points[1].Count)
			case "monthly":
				require.Len(t, points, 2)
				require.Equal(t, "2026-01", points[0].Date)
				require.InDelta(t, float64(130)/3, points[0].AvgScore, 0.0001)
			case "yearly":
				require.Len(t, points, 1)
				require.Equal(t, float64(50), points[0].AvgScore)
				require.Equal(t, float64(10), points[0].MinScore)
				require.Equal(t, float64(90), points[0].MaxScore)
			}
		})
	}
	require.NoError(t, db.Migrator().DropTable(&models.RiskScore{}))
	_, err := loadRiskTrendPoints(db, scope, "daily", "one", time.Now())
	require.Error(t, err, "unavailable trend evidence must not become an empty successful result")
}
