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
		if target.extra != "" {
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
		`INSERT INTO risk_scores(resource_uid,resource_type) VALUES ('pod-one','pod'),('pod-one','Node')`,
		`INSERT INTO policy_violations(resource_uid,resource_type) VALUES ('pod-one','Pod')`,
		`INSERT INTO exception_policies(resource_uid) VALUES ('pod-one'),('pod-dup')`,
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

	assertResourceCluster := func(table, resourceType, wantCluster string) {
		t.Helper()
		var cluster sql.NullString
		if err := db.Table(table).Select("cluster_id").Where("resource_uid = ? AND resource_type = ?", "pod-one", resourceType).Scan(&cluster).Error; err != nil {
			t.Fatal(err)
		}
		if wantCluster == "" {
			if cluster.Valid && cluster.String != "" {
				t.Fatalf("%s/%s ownership must not be inferred from pod UID: %q", table, resourceType, cluster.String)
			}
			return
		}
		if !cluster.Valid || cluster.String != wantCluster {
			t.Fatalf("%s/%s cluster=%v, want %s", table, resourceType, cluster, wantCluster)
		}
	}
	assertResourceCluster("insights", "Pod", "cluster-a")
	assertResourceCluster("insights", "ServiceAccount", "")
	assertResourceCluster("risk_scores", "pod", "cluster-a")
	assertResourceCluster("risk_scores", "Node", "")
	assertResourceCluster("policy_violations", "Pod", "cluster-a")
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
	if err == nil || !strings.Contains(err.Error(), "required table runtime_events is missing") {
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
	if err == nil || !strings.Contains(err.Error(), "required column runtime_events.pod_uid is missing") {
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
	if err == nil || !strings.Contains(err.Error(), "runtime_events contains 1 row(s) with cluster ownership inconsistent with pods") {
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
	if err := EnsureClusterResourceIdentityFoundation(db); err != nil {
		t.Fatalf("foundation: %v", err)
	}

	def, exists, err := postgresIndexDefinitionForName(db, "idx_runtime_events_cluster_pod_uid")
	if err != nil || !exists || !def.Valid || def.Unique || def.Table != "runtime_events" || def.Columns != "cluster_id,pod_uid" {
		t.Fatalf("runtime index definition=%+v exists=%v err=%v", def, exists, err)
	}

	// A valid index with the expected name but the wrong definition must not be
	// accepted. ensureIndex must replace it with the exact contract.
	if err := db.Exec(`DROP INDEX CONCURRENTLY idx_runtime_events_cluster_pod_uid`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX CONCURRENTLY idx_runtime_events_cluster_pod_uid ON runtime_events(pod_uid)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := ensureIndex(db, "idx_runtime_events_cluster_pod_uid", "runtime_events", "cluster_id, pod_uid", false); err != nil {
		t.Fatalf("repair mismatched index: %v", err)
	}
	def, exists, err = postgresIndexDefinitionForName(db, "idx_runtime_events_cluster_pod_uid")
	if err != nil || !exists || !def.Valid || def.Unique || def.Table != "runtime_events" || def.Columns != "cluster_id,pod_uid" {
		t.Fatalf("repaired index definition=%+v exists=%v err=%v", def, exists, err)
	}

	// Interrupted CREATE INDEX CONCURRENTLY can leave an invalid object. It must
	// also be removed and rebuilt rather than accepted by name.
	if err := db.Exec(`CREATE TABLE invalid_index_probe (v TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO invalid_index_probe(v) VALUES ('dup'),('dup')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX CONCURRENTLY idx_invalid_probe ON invalid_index_probe(v)`).Error; err == nil {
		t.Fatal("expected duplicate data to fail concurrent unique index build")
	}
	valid, exists, err := postgresIndexValidity(db, "idx_invalid_probe")
	if err != nil || !exists || valid {
		t.Fatalf("expected invalid postgres index: exists=%v valid=%v err=%v", exists, valid, err)
	}
	if err := db.Exec(`TRUNCATE invalid_index_probe`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO invalid_index_probe(v) VALUES ('ok')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := ensureIndex(db, "idx_invalid_probe", "invalid_index_probe", "v", true); err != nil {
		t.Fatalf("recover invalid postgres index: %v", err)
	}
}
