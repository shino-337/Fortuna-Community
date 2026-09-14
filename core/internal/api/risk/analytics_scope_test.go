package risk

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func analyticsFixture(t *testing.T) (*gorm.DB, func(string, string) *httptest.ResponseRecorder) {
	t.Helper()
	db := newSupplyChainTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.RiskScore{}, &models.RuntimeEvent{}); err != nil {
		t.Fatal(err)
	}
	must := func(value any) {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	for _, cluster := range []string{"a", "b"} {
		for i := 0; i < 3; i++ {
			uid := fmt.Sprintf("%s-%d", cluster, i)
			ns := fmt.Sprintf("shared-%d", i)
			score := float64(10 + i)
			cvss := float32(5)
			if cluster == "b" {
				score = 90
				cvss = 10
			}
			must(&models.Pod{UID: uid, ClusterID: cluster, Namespace: ns, Name: uid, NodeName: "shared-node"})
			must(&models.RiskScore{ResourceUID: uid, ResourceType: "Pod", ClusterID: cluster, Namespace: ns, TotalScore: score, ScorerVersion: "v3", CalculatedAt: now})
			sbom := models.SBOM{PodUID: uid, PodName: uid, Namespace: ns, ImageName: uid, ImageTag: "latest"}
			must(&sbom)
			count := i + 1
			if cluster == "b" {
				count = 5
			}
			for j := 0; j < count; j++ {
				must(&models.CVEMatch{SBOMID: sbom.ID, CVEID: fmt.Sprintf("CVE-%s-%d-%d", cluster, i, j), Severity: "CRITICAL", CVSS: cvss, PackageName: "pkg"})
			}
			must(&models.RuntimeEvent{PodUID: uid, PodName: uid, Namespace: ns, Severity: "high", ObservedAt: &now, CreatedAt: now})
		}
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		who := c.GetHeader("Test-User")
		if who == "missing" {
			return
		}
		u := &models.User{Role: models.RoleViewer}
		if who == "admin" {
			u.Role = models.RoleAdmin
		} else if who != "unrestricted" {
			u.ScopeJSON = `{"cluster_ids":["` + who + `"]}`
		}
		c.Set("user", u)
	})
	r.GET("/trends", GetRiskTrendsAnalytics(db))
	r.GET("/comparison", GetRiskComparison(db))
	r.GET("/correlation", GetRiskCorrelation(db))
	r.GET("/supply-chain", GetSupplyChainCorrelation(db))
	r.GET("/runtime-cve", GetRuntimeCVECorrelation(db))
	return db, func(path, who string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Test-User", who)
		r.ServeHTTP(w, req)
		return w
	}
}

func TestAnalyticsClusterAuthorization(t *testing.T) {
	db, request := analyticsFixture(t)
	for _, path := range []string{"/trends", "/comparison", "/correlation", "/supply-chain", "/runtime-cve"} {
		t.Run(path, func(t *testing.T) {
			for _, tc := range []struct {
				suffix, who string
				code        int
			}{
				{"", "a", 200}, {"", "b", 200}, {"", "admin", 200}, {"", "unrestricted", 200}, {"", "missing", 401},
				{"?cluster=b", "a", 403}, {"?clusterId=b", "a", 403}, {"?cluster=a&clusterId=b", "a", 400},
				{"?cluster=%20a%20", "a", 200}, {"?clusterId=a", "a", 200},
			} {
				w := request(path+tc.suffix, tc.who)
				if w.Code != tc.code {
					t.Fatalf("%+v: %d %s", tc, w.Code, w.Body)
				}
			}
		})
	}
	var trends struct {
		Trends []TrendPoint `json:"trends"`
	}
	decodeAnalytics(t, request("/trends", "a"), &trends)
	if len(trends.Trends) != 1 || trends.Trends[0].Count != 3 || trends.Trends[0].AvgScore != 11 {
		t.Fatalf("trends: %+v", trends)
	}
	decodeAnalytics(t, request("/trends", "admin"), &trends)
	if len(trends.Trends) != 1 || trends.Trends[0].Count != 6 {
		t.Fatalf("admin trends: %+v", trends)
	}
	decodeAnalytics(t, request("/trends?clusterId=a", "admin"), &trends)
	if len(trends.Trends) != 1 || trends.Trends[0].Count != 3 {
		t.Fatalf("explicit trends: %+v", trends)
	}
	var comparison ComparisonResult
	decodeAnalytics(t, request("/comparison", "a"), &comparison)
	if comparison.CurrentPeriod.Count != 3 || comparison.CurrentPeriod.AvgScore != 11 {
		t.Fatalf("comparison: %+v", comparison)
	}
	for _, factor := range []string{"cve_count", "cve_severity"} {
		var correlation CorrelationResult
		decodeAnalytics(t, request("/correlation?factor="+factor, "a"), &correlation)
		if len(correlation.DataPoints) != 3 {
			t.Fatalf("correlation: %+v", correlation)
		}
		for _, p := range correlation.DataPoints {
			if p.Y != p.X+9 || p.X > 3 {
				t.Fatalf("cross-cluster correlation: %+v", correlation)
			}
		}
	}
	// Foreign rows have higher CVSS: filtering after LIMIT would lose the local row.
	var supply SupplyChainSummary
	decodeAnalytics(t, request("/supply-chain?entries=true&limit=1", "a"), &supply)
	if len(supply.Entries) != 1 || !strings.HasPrefix(supply.Entries[0].PodUID, "a-") {
		t.Fatalf("supply: %+v", supply)
	}
	if err := db.Model(&models.RuntimeEvent{}).Where("pod_uid LIKE ?", "a-%").Update("observed_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	var runtime RuntimeCVESummary
	decodeAnalytics(t, request("/runtime-cve?entries=true&limit=1", "a"), &runtime)
	if len(runtime.Entries) != 1 || !strings.HasPrefix(runtime.Entries[0].PodUID, "a-") || runtime.Entries[0].EventObserved.IsZero() {
		t.Fatalf("runtime: %+v", runtime)
	}
	decodeAnalytics(t, request("/runtime-cve?entries=true&pod_uid=b-0", "a"), &runtime)
	if runtime.TotalCorrelations != 0 {
		t.Fatalf("foreign UID: %+v", runtime)
	}
}

func decodeAnalytics(t *testing.T, w *httptest.ResponseRecorder, out any) {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body)
	}
	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyticsHistoricalOwnership(t *testing.T) {
	db, request := analyticsFixture(t)
	if err := db.Where("uid IN ?", []string{"a-0", "b-0"}).Delete(&models.Pod{}).Error; err != nil {
		t.Fatal(err)
	}
	var supply SupplyChainSummary
	decodeAnalytics(t, request("/supply-chain?entries=true&includeHistorical=true", "a"), &supply)
	if supply.TotalPods != 3 {
		t.Fatalf("historical: %+v", supply)
	}
	if err := db.Unscoped().Where("uid = ?", "a-0").Delete(&models.Pod{}).Error; err != nil {
		t.Fatal(err)
	}
	decodeAnalytics(t, request("/supply-chain?entries=true&includeHistorical=true", "a"), &supply)
	if supply.TotalPods != 2 {
		t.Fatalf("orphan included: %+v", supply)
	}
	for _, e := range supply.Entries {
		if !strings.HasPrefix(e.PodUID, "a-") {
			t.Fatalf("foreign historical: %+v", e)
		}
	}
	decodeAnalytics(t, request("/supply-chain?entries=true&includeHistorical=true", "admin"), &supply)
	if supply.TotalPods != 6 {
		t.Fatalf("admin history: %+v", supply)
	}
}

func TestAnalyticsDatabaseErrors(t *testing.T) {
	db, request := analyticsFixture(t)
	if err := db.Migrator().DropTable(&models.RiskScore{}, &models.CVEMatch{}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/trends", "/comparison", "/correlation", "/correlation?factor=cve_severity", "/supply-chain", "/runtime-cve"} {
		w := request(path, "a")
		if w.Code != 500 || strings.Contains(w.Body.String(), "no such table") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}
