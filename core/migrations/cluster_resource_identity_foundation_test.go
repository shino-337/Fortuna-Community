package migrations

import (
	"database/sql"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openClusterIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestClusterResourceIdentityFoundationBackfillsOnlyUnambiguousOwnership(t *testing.T) {
	db := openClusterIdentityTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE pods (id INTEGER PRIMARY KEY, uid TEXT NOT NULL, cluster_id TEXT NOT NULL, deleted_at DATETIME)`,
		`CREATE TABLE runtime_events (id INTEGER PRIMARY KEY, pod_uid TEXT NOT NULL)`,
		`CREATE TABLE insights (id INTEGER PRIMARY KEY, resource_uid TEXT NOT NULL, resource_type TEXT NOT NULL)`,
		`INSERT INTO pods(uid,cluster_id) VALUES ('pod-one','cluster-a')`,
		`INSERT INTO pods(uid,cluster_id) VALUES ('pod-dup','cluster-a')`,
		`INSERT INTO pods(uid,cluster_id) VALUES ('pod-dup','cluster-b')`,
		`INSERT INTO runtime_events(pod_uid) VALUES ('pod-one'),('pod-dup')`,
		`INSERT INTO insights(resource_uid,resource_type) VALUES ('pod-one','Pod'),('pod-dup','Pod'),('pod-one','ServiceAccount')`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	for i := 0; i < 2; i++ {
		if err := EnsureClusterResourceIdentityFoundation(db); err != nil {
			t.Fatalf("foundation pass %d: %v", i+1, err)
		}
	}

	type row struct {
		PodUID    string
		ClusterID string
	}
	var events []row
	if err := db.Table("runtime_events").Select("pod_uid, cluster_id").Order("pod_uid").Scan(&events).Error; err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, event := range events {
		got[event.PodUID] = event.ClusterID
	}
	if got["pod-one"] != "cluster-a" {
		t.Fatalf("unambiguous owner=%q, want cluster-a", got["pod-one"])
	}
	if got["pod-dup"] != "" {
		t.Fatalf("ambiguous duplicate UID ownership was guessed: %q", got["pod-dup"])
	}

	var podInsightCluster string
	if err := db.Table("insights").Select("cluster_id").Where("resource_uid = ? AND resource_type = ?", "pod-one", "Pod").Scan(&podInsightCluster).Error; err != nil {
		t.Fatal(err)
	}
	if podInsightCluster != "cluster-a" {
		t.Fatalf("pod insight cluster=%q, want cluster-a", podInsightCluster)
	}
	var nonPodCluster sql.NullString
	if err := db.Table("insights").Select("cluster_id").Where("resource_uid = ? AND resource_type = ?", "pod-one", "ServiceAccount").Scan(&nonPodCluster).Error; err != nil {
		t.Fatal(err)
	}
	if nonPodCluster.Valid && nonPodCluster.String != "" {
		t.Fatalf("non-Pod insight ownership must not be guessed from pod UID: %q", nonPodCluster.String)
	}
}

func TestClusterResourceIdentityFoundationFailsClosedWithoutPods(t *testing.T) {
	db := openClusterIdentityTestDB(t)
	if err := EnsureClusterResourceIdentityFoundation(db); err == nil {
		t.Fatal("foundation succeeded without authoritative pods table")
	}
}
