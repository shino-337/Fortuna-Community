package risk

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
)

func TestRiskScoreAPIsScopeAndLatest(t *testing.T) {
	db, request := analyticsFixture(t)
	now := time.Now().UTC()
	if err := db.Model(&models.RiskScore{}).Where("cluster_id = ?", "a").Update("priority_level", "P3").Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []models.RiskScore{
		{ResourceType: "Pod", ResourceUID: "a-0", ClusterID: "a", Namespace: "shared-0", TotalScore: 100, PriorityLevel: "P0", ScorerVersion: "v3", CalculatedAt: now.Add(-time.Hour)},
		{ResourceType: "Pod", ResourceUID: "a-1", ClusterID: "a", Namespace: "shared-1", TotalScore: 100, PriorityLevel: "P0", ScorerVersion: "v2", CalculatedAt: now},
	} {
		if err := db.Create(&row).Error; err != nil { t.Fatal(err) }
	}
	for _, path := range []string{"/scores", "/priorities", "/top", "/grouped?includeRisks=true", "/legacy-trends"} {
		for _, who := range []string{"a", "b", "admin", "unrestricted", "empty"} {
			if w := request(path, who); w.Code != 200 { t.Fatalf("%s %s: %d %s", path, who, w.Code, w.Body) }
		}
		sep := "?"
		if strings.Contains(path, "?") { sep = "&" }
		for _, suffix := range []string{"cluster=b", "clusterId=b"} {
			if w := request(path+sep+suffix, "a"); w.Code != 403 { t.Fatalf("scope bypass %s: %d %s", path, w.Code, w.Body) }
		}
		if w := request(path, "missing"); w.Code != 401 { t.Fatalf("unauthenticated %s: %d", path, w.Code) }
	}
	var scores struct {
		Scores []models.RiskScore
		Total int
		Page int
		PageSize int
	}
	decodeAnalytics(t, request("/scores", "a"), &scores)
	if scores.Total != 3 { t.Fatalf("scores: %+v", scores) }
	for _, score := range scores.Scores {
		if score.ClusterID != "a" || score.TotalScore > 12 { t.Fatalf("foreign or stale score: %+v", score) }
	}
	decodeAnalytics(t, request("/scores?minScore=80", "a"), &scores)
	if scores.Total != 0 { t.Fatalf("old score resurrected by filter: %+v", scores) }
	decodeAnalytics(t, request("/scores?clusterId=a", "admin"), &scores)
	if scores.Total != 3 { t.Fatalf("explicit filter: %+v", scores) }
	decodeAnalytics(t, request("/scores?page=9223372036854775807&pageSize=500", "a"), &scores)
	if len(scores.Scores) != 0 { t.Fatal("huge page should be empty") }
	decodeAnalytics(t, request("/scores?page=0&pageSize=-1", "a"), &scores)
	if scores.Page != 1 || scores.PageSize != 50 { t.Fatalf("pagination: %+v", scores) }

	var top struct { Risks []models.RiskScore }
	decodeAnalytics(t, request("/top?limit=1", "a"), &top)
	if len(top.Risks) != 1 || top.Risks[0].ClusterID != "a" || top.Risks[0].TotalScore != 12 {
		t.Fatalf("top: %+v", top)
	}
	decodeAnalytics(t, request("/top?priority=P0", "a"), &top)
	if len(top.Risks) != 0 { t.Fatalf("historical priority resurfaced: %+v", top) }
	var stats struct { Total int; Priorities map[string]PriorityStats }
	decodeAnalytics(t, request("/priorities", "a"), &stats)
	if stats.Total != 3 || stats.Priorities["P3"].Count != 3 || stats.Priorities["P3"].Percentage != 100 {
		t.Fatalf("stats: %+v", stats)
	}
	for _, by := range []string{"cluster", "namespace", "type", "priority"} {
		var grouped struct { Grouped map[string]GroupedRisk }
		decodeAnalytics(t, request("/grouped?includeRisks=true&by="+by, "a"), &grouped)
		var count int64
		for _, group := range grouped.Grouped {
			count += group.Count
			for _, row := range group.Risks {
				if row.ClusterID != "a" || row.TotalScore > 12 { t.Fatalf("group entry: %+v", row) }
			}
		}
		if count != 3 { t.Fatalf("group count %s: %d", by, count) }
	}
}

func TestRiskScoreAPIDatabaseErrors(t *testing.T) {
	db, request := analyticsFixture(t)
	if err := db.Migrator().DropTable(&models.RiskScore{}); err != nil { t.Fatal(err) }
	for _, path := range []string{"/scores", "/priorities", "/top", "/grouped?includeRisks=true", "/legacy-trends", "/sync"} {
		w := request(path, "a")
		if w.Code != 500 || strings.Contains(w.Body.String(), "no such table") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}

func TestRiskSyncUsesAuthorizedSelection(t *testing.T) {
	_, db := setupCalculateRiskScoreSQLite(t)
	sqlDB, err := db.DB()
	if err != nil { t.Fatal(err) }
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	for _, cluster := range []string{"a", "b"} {
		uid := "sync-"+cluster
		if err := db.Create(&models.Pod{UID: uid, Name: uid, Namespace: "shared", ClusterID: cluster, Containers: "[]"}).Error; err != nil { t.Fatal(err) }
		if err := db.Create(&models.Insight{ResourceUID: uid, ResourceType: "Pod", Status: "acknowledged", Severity: "high", Title: "test", InsightType: "security"}).Error; err != nil { t.Fatal(err) }
	}
	scope := analyticsScope{restricted: true, clusterIDs: []string{"a"}}
	uids, err := scope.syncUIDs(db)
	if err != nil || len(uids) != 1 || uids[0] != "sync-a" { t.Fatalf("selection=%v err=%v", uids, err) }
	syncSelectedRiskScores(context.Background(), db, uids)
	var rows []models.RiskScore
	if err := db.Find(&rows).Error; err != nil { t.Fatal(err) }
	if len(rows) != 1 || rows[0].ResourceUID != "sync-a" { t.Fatalf("sync wrote outside selection: %+v", rows) }
	all, err := (analyticsScope{}).syncUIDs(db)
	if err != nil || len(all) != 2 { t.Fatalf("global selection=%v err=%v", all, err) }
	explicit, err := (analyticsScope{clusterID: "b"}).syncUIDs(db)
	if err != nil || len(explicit) != 1 || explicit[0] != "sync-b" { t.Fatalf("explicit selection=%v err=%v", explicit, err) }
}

func TestRiskSyncRejectsForeignClusterBeforeEnqueue(t *testing.T) {
	_, request := analyticsFixture(t)
	for _, suffix := range []string{"?cluster=b", "?clusterId=b"} {
		w := request("/sync"+suffix, "a")
		if w.Code != 403 { t.Fatalf("sync scope: %d %s", w.Code, w.Body) }
	}
}

