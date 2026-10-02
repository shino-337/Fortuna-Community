package risk

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Opt-in measured PostgreSQL gate: a dedicated isolated test database is required.
func BenchmarkRiskTrends50000(b *testing.B) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		b.Skip("FORTUNA_TEST_POSTGRES_URL required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		b.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		b.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	defer pool.Close()
	name := fmt.Sprintf("trend_benchmark_%d", time.Now().UnixNano())
	if err = db.Exec("CREATE SCHEMA " + name).Error; err != nil {
		b.Fatal(err)
	}
	if err = db.Exec("SET search_path TO " + name).Error; err != nil {
		b.Fatal(err)
	}
	defer func() { db.Exec("SET search_path TO public"); db.Exec("DROP SCHEMA " + name + " CASCADE") }()
	if err = db.AutoMigrate(&models.RiskScore{}); err != nil {
		b.Fatal(err)
	}
	if err = db.Exec(`INSERT INTO risk_scores(resource_type,resource_uid,cluster_id,namespace,total_score,priority_level,calculated_at) SELECT 'Pod','bench-'||n,'benchmark','test',n%101,'P'||(n%4),now()-(n%30)*interval '1 day' FROM generate_series(1,50000) n`).Error; err != nil {
		b.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user", &models.User{Role: models.RoleAdmin}); c.Next() })
	r.GET("/trends", GetRiskTrendsAnalytics(db))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/trends?cluster=benchmark&days=30", nil))
		if w.Code != 200 {
			b.Fatal(w.Body.String())
		}
	}
}
