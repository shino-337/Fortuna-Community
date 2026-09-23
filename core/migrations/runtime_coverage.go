package migrations

import (
	"fmt"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

func ensureModelColumns(db *gorm.DB, model interface{}, columns []string) error {
	for _, column := range columns {
		if db.Migrator().HasColumn(model, column) {
			continue
		}
		if err := db.Migrator().AddColumn(model, column); err != nil {
			return fmt.Errorf("add %T.%s: %w", model, column, err)
		}
	}
	return nil
}

func EnsureRuntimeCoverage(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.RuntimeCoverage{}) {
		if err := db.Migrator().CreateTable(&models.RuntimeCoverage{}); err != nil {
			return err
		}
	}
	// Do not assume a prior experimental runtime_coverages table has the exact
	// shape of the current model. Add every field required by the latest-state
	// projection before validating/querying it.
	if err := ensureModelColumns(db, &models.RuntimeCoverage{}, []string{
		"ClusterID", "AgentID", "ProducerID", "SessionID", "CoverageID",
		"SourceKind", "Status", "WindowStart", "WindowEnd", "ReceivedAt",
		"ContinuousSince", "Emitted", "Delivered", "Dropped", "Invalid",
		"Errors", "Reason",
	}); err != nil {
		return fmt.Errorf("runtime coverage columns: %w", err)
	}
	if err := db.Exec("SELECT cluster_id,agent_id,producer_id,session_id,coverage_id,source_kind,status,window_start,window_end,received_at,continuous_since,emitted,delivered,dropped,invalid,errors,reason FROM runtime_coverages LIMIT 0").Error; err != nil {
		return err
	}
	var unownedCoverage int64
	if err := db.Table("runtime_coverages").
		Where("cluster_id IS NULL OR cluster_id = '' OR agent_id IS NULL OR agent_id = '' OR producer_id IS NULL OR producer_id = ''").
		Count(&unownedCoverage).Error; err != nil {
		return err
	}
	if unownedCoverage != 0 {
		return fmt.Errorf("runtime coverage schema contains %d rows without cluster/agent/producer identity", unownedCoverage)
	}
	// OnConflict(cluster_id,agent_id,producer_id) requires a real uniqueness
	// constraint even when upgrading a table that predates the current PK shape.
	if err := ensureIndex(db, "idx_runtime_coverage_identity", "runtime_coverages", "cluster_id,agent_id,producer_id", true); err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_coverage_cluster_agent", "runtime_coverages", "cluster_id,agent_id", false); err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_coverage_window_end", "runtime_coverages", "window_end", false); err != nil {
		return err
	}

	if !db.Migrator().HasTable(&models.RuntimeCoverageReceipt{}) {
		if err := db.Migrator().CreateTable(&models.RuntimeCoverageReceipt{}); err != nil {
			return err
		}
	}
	if err := ensureModelColumns(db, &models.RuntimeCoverageReceipt{}, []string{
		"ClusterID", "AgentID", "ProducerID", "SessionID", "CoverageID",
		"SourceKind", "Status", "WindowStart", "WindowEnd", "ReceivedAt",
		"ContinuousSince", "Emitted", "Delivered", "Dropped", "Invalid",
		"Errors", "Reason",
	}); err != nil {
		return fmt.Errorf("runtime coverage receipt columns: %w", err)
	}
	if err := db.Exec("SELECT cluster_id,agent_id,producer_id,session_id,coverage_id,source_kind,status,window_start,window_end,received_at,continuous_since,emitted,delivered,dropped,invalid,errors,reason FROM runtime_coverage_receipts LIMIT 0").Error; err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_coverage_receipt_identity", "runtime_coverage_receipts", "cluster_id,agent_id,producer_id,session_id,coverage_id", true); err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_coverage_receipt_window_end", "runtime_coverage_receipts", "window_end", false); err != nil {
		return err
	}
	// Preserve the one latest receipt that may predate the immutable history
	// table. Only rows with a complete evidence identity/window are backfilled;
	// partial legacy rows remain latest-state data and are never fabricated into
	// trusted historical evidence.
	if err := db.Exec(`
		INSERT INTO runtime_coverage_receipts (
			cluster_id,agent_id,producer_id,session_id,coverage_id,source_kind,status,
			window_start,window_end,received_at,continuous_since,
			emitted,delivered,dropped,invalid,errors,reason
		)
		SELECT
			cluster_id,agent_id,producer_id,session_id,coverage_id,source_kind,status,
			window_start,window_end,received_at,continuous_since,
			COALESCE(emitted,0),COALESCE(delivered,0),COALESCE(dropped,0),
			COALESCE(invalid,0),COALESCE(errors,0),COALESCE(reason,'')
		FROM runtime_coverages
		WHERE cluster_id IS NOT NULL AND cluster_id <> ''
		  AND agent_id IS NOT NULL AND agent_id <> ''
		  AND producer_id IS NOT NULL AND producer_id <> ''
		  AND session_id IS NOT NULL AND session_id <> ''
		  AND coverage_id IS NOT NULL AND coverage_id <> ''
		  AND source_kind IS NOT NULL AND source_kind <> ''
		  AND status IS NOT NULL AND status <> ''
		  AND window_start IS NOT NULL
		  AND window_end IS NOT NULL
		  AND received_at IS NOT NULL
		ON CONFLICT (cluster_id,agent_id,producer_id,session_id,coverage_id) DO NOTHING
	`).Error; err != nil {
		return fmt.Errorf("backfill runtime coverage receipt history: %w", err)
	}

	if !db.Migrator().HasTable(&models.RuntimeProducerState{}) {
		if err := db.Migrator().CreateTable(&models.RuntimeProducerState{}); err != nil {
			return err
		}
	}
	if err := ensureModelColumns(db, &models.RuntimeProducerState{}, []string{
		"ClusterID", "AgentID", "ProducerID", "SourceKind", "SessionID",
		"SessionStartedAt", "Enabled", "Authoritative", "State",
		"LastManifestAt", "LastHeartbeatAt", "LastCoverageID",
		"LastCoverageEnd", "GapSince", "GapReason",
	}); err != nil {
		return fmt.Errorf("runtime producer state columns: %w", err)
	}
	if err := db.Exec("SELECT cluster_id,agent_id,producer_id,source_kind,session_id,session_started_at,enabled,authoritative,state,last_manifest_at,last_heartbeat_at,last_coverage_id,last_coverage_end,gap_since,gap_reason FROM runtime_producer_states LIMIT 0").Error; err != nil {
		return err
	}
	var unownedProducer int64
	if err := db.Table("runtime_producer_states").
		Where("cluster_id IS NULL OR cluster_id = '' OR agent_id IS NULL OR agent_id = '' OR producer_id IS NULL OR producer_id = ''").
		Count(&unownedProducer).Error; err != nil {
		return err
	}
	if unownedProducer != 0 {
		return fmt.Errorf("runtime producer schema contains %d rows without cluster/agent/producer identity", unownedProducer)
	}
	if err := ensureIndex(db, "idx_runtime_producer_identity", "runtime_producer_states", "cluster_id,agent_id,producer_id", true); err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_producer_session", "runtime_producer_states", "session_id", false); err != nil {
		return err
	}
	return ensureIndex(db, "idx_runtime_producer_heartbeat", "runtime_producer_states", "last_heartbeat_at", false)
}
