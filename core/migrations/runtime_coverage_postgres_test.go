package migrations

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRuntimeCoverageFoundationPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	sqlDB, err := db.DB()
	if err != nil { t.Fatal(err) }
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	defer sqlDB.Close()

	schema := fmt.Sprintf("runtime_coverage_%d", time.Now().UnixNano())
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil { t.Fatal(err) }
	defer func() {
		_ = db.Exec("SET search_path TO public").Error
		_ = db.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
	}()
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil { t.Fatal(err) }

	// Simulate the early #51 scaffold so the final ensure path proves in-place
	// upgrade of continuous_since rather than assuming a clean database.
	if err := db.Exec(`CREATE TABLE runtime_coverages (
		cluster_id VARCHAR(255) NOT NULL,
		agent_id VARCHAR(255) NOT NULL,
		producer_id VARCHAR(128) NOT NULL,
		coverage_id VARCHAR(128),
		source_kind VARCHAR(64),
		status VARCHAR(32),
		window_start TIMESTAMPTZ,
		window_end TIMESTAMPTZ,
		received_at TIMESTAMPTZ,
		emitted BIGINT DEFAULT 0,
		delivered BIGINT DEFAULT 0,
		dropped BIGINT DEFAULT 0,
		invalid BIGINT DEFAULT 0,
		reason VARCHAR(512),
		PRIMARY KEY(cluster_id,agent_id,producer_id)
	)`).Error; err != nil { t.Fatal(err) }

	if err := EnsureRuntimeCoverage(db); err != nil { t.Fatalf("coverage ensure: %v", err) }
	if err := EnsureRuntimeCoverage(db); err != nil { t.Fatalf("coverage rerun: %v", err) }
	if !db.Migrator().HasColumn(&models.RuntimeCoverage{}, "continuous_since") {
		t.Fatal("continuous_since not installed")
	}

	if err := EnsureRuntimeEventIngestClaims(db); err != nil { t.Fatalf("claim ensure: %v", err) }
	if err := EnsureRuntimeEventIngestClaims(db); err != nil { t.Fatalf("claim rerun: %v", err) }

	first := models.RuntimeEventIngestClaim{
		ClusterID: "cluster-a", EventID: "event-1", PayloadSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AcceptedAt: time.Now().UTC(),
	}
	if err := db.Create(&first).Error; err != nil { t.Fatal(err) }
	duplicate := first
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate {cluster_id,event_id} claim was accepted")
	}
	otherCluster := first
	otherCluster.ClusterID = "cluster-b"
	if err := db.Create(&otherCluster).Error; err != nil {
		t.Fatalf("same event_id in another cluster must remain independent: %v", err)
	}

	var count int64
	if err := db.Model(&models.RuntimeEventIngestClaim{}).Count(&count).Error; err != nil { t.Fatal(err) }
	if count != 2 { t.Fatalf("claim rows=%d want 2", count) }

	def, exists, err := postgresIndexDefinitionForName(db, "idx_runtime_event_ingest_claim_accepted")
	if err != nil || !exists || !def.Valid || def.Unique || def.Table != "runtime_event_ingest_claims" || def.Columns != "accepted_at" {
		t.Fatalf("claim accepted index=%+v exists=%v err=%v", def, exists, err)
	}
}
