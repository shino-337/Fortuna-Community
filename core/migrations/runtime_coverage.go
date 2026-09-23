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
	if err := ensureIndex(db, "idx_runtime_producer_identity", "runtime_producer_states", "cluster_id,agent_id,producer_id", true); err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_producer_session", "runtime_producer_states", "session_id", false); err != nil {
		return err
	}
	return ensureIndex(db, "idx_runtime_producer_heartbeat", "runtime_producer_states", "last_heartbeat_at", false)
}
