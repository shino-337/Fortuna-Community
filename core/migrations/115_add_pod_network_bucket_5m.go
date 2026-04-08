package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration115_AddPodNetworkBucket5m adds 5-minute bucketing + unique signature for upsert on pod_network_connections.
//
// Transaction context: RunMigrations (migrations.go) invokes each migration as migration(db) with no outer
// db.Transaction wrapper — each db.Exec here is typically auto-commit. That means CREATE INDEX CONCURRENTLY
// is valid here (PostgreSQL forbids CONCURRENTLY inside an explicit transaction block).
//
// Sizing (order of magnitude, after upsert + 24h retention): rows ≈ O(active connection signatures × buckets in window).
// Worst-case upper bound per pod ≈ 288 buckets/24h × distinct signatures per bucket; cluster totals often 10³–10⁵,
// or 10⁶+ with many pods and chatty ephemeral ports. Pre-bucket legacy tables could be much larger (append per poll);
// backfill UPDATE + dedupe DELETE may be slow on millions of rows — run during a maintenance window if needed.
func Migration115_AddPodNetworkBucket5m(db *gorm.DB) error {
	log.Println("Running migration 115: pod_network_connections bucket_5m + unique index + query indexes")

	if err := db.Exec(`
ALTER TABLE pod_network_connections
ADD COLUMN IF NOT EXISTS bucket_5m TIMESTAMP WITH TIME ZONE;
`).Error; err != nil {
		return err
	}

	// NULL observed_at would leave bucket_5m NULL and fail SET NOT NULL below
	if err := db.Exec(`
UPDATE pod_network_connections
SET observed_at = COALESCE(created_at, CURRENT_TIMESTAMP)
WHERE observed_at IS NULL;
`).Error; err != nil {
		return err
	}

	// Backfill from observed_at (epoch buckets = UTC semantics for timestamptz)
	if err := db.Exec(`
UPDATE pod_network_connections
SET bucket_5m = TIMESTAMPTZ 'epoch' + (FLOOR(EXTRACT(EPOCH FROM observed_at) / 300) * 300) * INTERVAL '1 second'
WHERE bucket_5m IS NULL;
`).Error; err != nil {
		return err
	}

	// Normalize NULL text fields so the unique key matches ingest (empty string)
	if err := db.Exec(`
UPDATE pod_network_connections SET
  container_name = COALESCE(container_name, ''),
  source_ip = COALESCE(source_ip, ''),
  dest_ip = COALESCE(dest_ip, ''),
  protocol = COALESCE(protocol, 'tcp'),
  state = COALESCE(state, '')
WHERE container_name IS NULL OR source_ip IS NULL OR dest_ip IS NULL OR protocol IS NULL OR state IS NULL;
`).Error; err != nil {
		return err
	}

	// Canonical protocol case for signature + upsert (TCP vs tcp)
	if err := db.Exec(`
UPDATE pod_network_connections SET protocol = LOWER(TRIM(protocol));
`).Error; err != nil {
		return err
	}

	// Dedupe historical rows per signature+bucket (keep latest observed_at, then highest id)
	if err := db.Exec(`
DELETE FROM pod_network_connections
WHERE id IN (
  SELECT id FROM (
    SELECT id,
           ROW_NUMBER() OVER (
             PARTITION BY cluster_id, pod_uid, namespace, container_name, source_ip, source_port, dest_ip, dest_port, protocol, state, bucket_5m
             ORDER BY observed_at DESC, id DESC
           ) AS rn
    FROM pod_network_connections
  ) sub WHERE rn > 1
);
`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
ALTER TABLE pod_network_connections
ALTER COLUMN bucket_5m SET NOT NULL;
`).Error; err != nil {
		return err
	}

	// CONCURRENTLY: avoids long write-blocking index builds on large tables (requires running outside a txn; see header).
	if err := db.Exec(`
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uidx_pnc_bucket_sig
ON pod_network_connections (
  cluster_id, pod_uid, namespace, container_name,
  source_ip, source_port, dest_ip, dest_port,
  protocol, state, bucket_5m
);
`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_pnc_pod_bucket_obs
ON pod_network_connections (pod_uid, bucket_5m DESC, observed_at DESC);
`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_pnc_cluster_bucket_obs
ON pod_network_connections (cluster_id, bucket_5m DESC, observed_at DESC);
`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_pnc_cluster_ns_bucket
ON pod_network_connections (cluster_id, namespace, bucket_5m DESC);
`).Error; err != nil {
		return err
	}

	log.Println("[Migration 115] Completed successfully")
	return nil
}
