package migrations

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRuntimeEventIdempotencyPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()

	schema := fmt.Sprintf("runtime_event_idem_migration_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")

	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()

	// Simulate the pre-idempotency runtime_events shape.
	require.NoError(t, db.Exec(`
		CREATE TABLE runtime_events (
			id BIGSERIAL PRIMARY KEY,
			cluster_id VARCHAR(255),
			event_id VARCHAR(64),
			pod_uid VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			syscall VARCHAR(100) NOT NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO runtime_events(cluster_id,event_id,pod_uid,namespace,syscall)
		VALUES
		  ('cluster-a','legacy-1','pod-a','ns','execve'),
		  ('cluster-a','legacy-2','pod-a','ns','execve')
	`).Error)

	require.NoError(t, EnsureRuntimeEventIdempotency(db))
	require.NoError(t, EnsureRuntimeEventIdempotency(db))
	require.True(t, db.Migrator().HasColumn(&models.RuntimeEvent{}, "source_record_id"))

	// Multiple legacy rows with no physical identity remain valid and outside the
	// partial uniqueness contract.
	require.NoError(t, db.Exec(`
		INSERT INTO runtime_events(cluster_id,event_id,pod_uid,namespace,syscall,source_record_id)
		VALUES ('cluster-a','legacy-3','pod-a','ns','execve','')
	`).Error)

	sourceID := strings.Repeat("a", 64)
	require.NoError(t, db.Exec(`
		INSERT INTO runtime_events(cluster_id,event_id,pod_uid,namespace,syscall,source_record_id)
		VALUES ('cluster-a','new-1','pod-a','ns','execve',?)
	`, sourceID).Error)
	require.Error(t, db.Exec(`
		INSERT INTO runtime_events(cluster_id,event_id,pod_uid,namespace,syscall,source_record_id)
		VALUES ('cluster-a','new-2','pod-a','ns','execve',?)
	`, sourceID).Error, "same cluster/source record must be unique")

	// The same physical source identifier is isolated by cluster identity.
	require.NoError(t, db.Exec(`
		INSERT INTO runtime_events(cluster_id,event_id,pod_uid,namespace,syscall,source_record_id)
		VALUES ('cluster-b','new-3','pod-b','ns','execve',?)
	`, sourceID).Error)
}

func TestRuntimeEventIdempotencyRejectsPreexistingDuplicateSourceRecordsPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()

	schema := fmt.Sprintf("runtime_event_idem_duplicate_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")

	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()

	require.NoError(t, db.Exec(`
		CREATE TABLE runtime_events (
			id BIGSERIAL PRIMARY KEY,
			cluster_id VARCHAR(255),
			source_record_id VARCHAR(64),
			pod_uid VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			syscall VARCHAR(100) NOT NULL
		)
	`).Error)
	sourceID := strings.Repeat("b", 64)
	require.NoError(t, db.Exec(`
		INSERT INTO runtime_events(cluster_id,source_record_id,pod_uid,namespace,syscall)
		VALUES
		  ('cluster-a',?,'pod-a','ns','execve'),
		  ('cluster-a',?,'pod-a','ns','execve')
	`, sourceID, sourceID).Error)

	err = EnsureRuntimeEventIdempotency(db)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate physical source-record identities")
}
