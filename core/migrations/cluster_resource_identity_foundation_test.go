package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
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

func createClusterIdentityFoundationSchema(t *testing.T, db *gorm.DB, omittedTable string) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE pods (uid TEXT NOT NULL, cluster_id TEXT NOT NULL)`).Error; err != nil {
		t.Fatalf("create pods: %v", err)
	}
	for _, target := range clusterOwnedPodTables {
		if target.table == omittedTable {
			continue
		}
		columns := target.uidColumn + " TEXT"
		if target.table == "insights" {
			columns += ", resource_type TEXT NOT NULL"
		}
		stmt := fmt.Sprintf("CREATE TABLE %s (%s)", target.table, columns)
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create %s: %v", target.table, err)
		}
	}
}

func TestClusterResourceIdentityFoundationBackfillsOnlyUnambiguousOwnership(t *testing.T) {
	db := openClusterIdentityTestDB(t)
	createClusterIdentityFoundationSchema(t, db, "")
	for _, stmt := range []string{
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
		ClusterID sql.NullString
	}
	var events []row
	if err := db.Table("runtime_events").Select("pod_uid, cluster_id").Order("pod_uid").Scan(&events).Error; err != nil {
		t.Fatal(err)
	}
	got := map[string]sql.NullString{}
	for _, event := range events {
		got[event.PodUID] = event.ClusterID
	}
	if !got["pod-one"].Valid || got["pod-one"].String != "cluster-a" {
		t.Fatalf("unambiguous owner=%v, want cluster-a", got["pod-one"])
	}
	if got["pod-dup"].Valid && got["pod-dup"].String != "" {
		t.Fatalf("ambiguous duplicate UID ownership was guessed: %q", got["pod-dup"].String)
	}

	var podInsightCluster sql.NullString
	if err := db.Table("insights").Select("cluster_id").Where("resource_uid = ? AND resource_type = ?", "pod-one", "Pod").Scan(&podInsightCluster).Error; err != nil {
		t.Fatal(err)
	}
	if !podInsightCluster.Valid || podInsightCluster.String != "cluster-a" {
		t.Fatalf("pod insight cluster=%v, want cluster-a", podInsightCluster)
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

func TestClusterResourceIdentityFoundationFailsClosedOnMissingRequiredTarget(t *testing.T) {
	db := openClusterIdentityTestDB(t)
	createClusterIdentityFoundationSchema(t, db, "runtime_events")
	err := EnsureClusterResourceIdentityFoundation(db)
	if err == nil {
		t.Fatal("foundation succeeded with required runtime_events table missing")
	}
	if !strings.Contains(err.Error(), "required table runtime_events is missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClusterResourceIdentityFoundationFailsClosedOnMissingUIDColumn(t *testing.T) {
	db := openClusterIdentityTestDB(t)
	createClusterIdentityFoundationSchema(t, db, "runtime_events")
	if err := db.Exec(`CREATE TABLE runtime_events (wrong_uid TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	err := EnsureClusterResourceIdentityFoundation(db)
	if err == nil {
		t.Fatal("foundation succeeded with runtime_events.pod_uid missing")
	}
	if !strings.Contains(err.Error(), "required column runtime_events.pod_uid is missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClusterResourceIdentityFoundationRejectsConflictingExistingOwnership(t *testing.T) {
	db := openClusterIdentityTestDB(t)
	createClusterIdentityFoundationSchema(t, db, "")
	if err := db.Exec(`ALTER TABLE runtime_events ADD COLUMN cluster_id TEXT`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO pods(uid,cluster_id) VALUES ('pod-one','cluster-a')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO runtime_events(pod_uid,cluster_id) VALUES ('pod-one','cluster-b')`).Error; err != nil {
		t.Fatal(err)
	}

	err := EnsureClusterResourceIdentityFoundation(db)
	if err == nil {
		t.Fatal("foundation accepted cluster ownership inconsistent with authoritative pods")
	}
	if !strings.Contains(err.Error(), "runtime_events contains 1 row(s) with cluster ownership inconsistent with pods") {
		t.Fatalf("unexpected error: %v", err)
	}

	var cluster string
	if scanErr := db.Table("runtime_events").Select("cluster_id").Scan(&cluster).Error; scanErr != nil {
		t.Fatal(scanErr)
	}
	if cluster != "cluster-b" {
		t.Fatalf("foundation silently rewrote conflicting ownership to %q", cluster)
	}
}

func TestClusterResourceIdentityFoundationPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	defer sqlDB.Close()

	schema := fmt.Sprintf("cluster_identity_%d", time.Now().UnixNano())
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() {
		_ = db.Exec("SET search_path TO public").Error
		_ = db.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
	}()
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	createClusterIdentityFoundationSchema(t, db, "")
	for _, stmt := range []string{
		`INSERT INTO pods(uid,cluster_id) VALUES ('pod-one','cluster-a')`,
		`INSERT INTO pods(uid,cluster_id) VALUES ('pod-dup','cluster-a')`,
		`INSERT INTO pods(uid,cluster_id) VALUES ('pod-dup','cluster-b')`,
		`INSERT INTO runtime_events(pod_uid) VALUES ('pod-one'),('pod-dup')`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("postgres setup %q: %v", stmt, err)
		}
	}

	for i := 0; i < 2; i++ {
		if err := EnsureClusterResourceIdentityFoundation(db); err != nil {
			t.Fatalf("postgres foundation pass %d: %v", i+1, err)
		}
	}

	var uniqueOwner sql.NullString
	if err := db.Table("runtime_events").Select("cluster_id").Where("pod_uid = ?", "pod-one").Scan(&uniqueOwner).Error; err != nil {
		t.Fatal(err)
	}
	if !uniqueOwner.Valid || uniqueOwner.String != "cluster-a" {
		t.Fatalf("postgres unambiguous owner=%v, want cluster-a", uniqueOwner)
	}
	var ambiguousOwner sql.NullString
	if err := db.Table("runtime_events").Select("cluster_id").Where("pod_uid = ?", "pod-dup").Scan(&ambiguousOwner).Error; err != nil {
		t.Fatal(err)
	}
	if ambiguousOwner.Valid && ambiguousOwner.String != "" {
		t.Fatalf("postgres ambiguous duplicate UID ownership was guessed: %q", ambiguousOwner.String)
	}

	var indexCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?`, "idx_runtime_events_cluster_pod_uid").Scan(&indexCount).Error; err != nil {
		t.Fatal(err)
	}
	if indexCount != 1 {
		t.Fatalf("postgres cluster-qualified runtime_events index count=%d, want 1", indexCount)
	}
}
