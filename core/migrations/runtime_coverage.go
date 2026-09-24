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

func validateRuntimeCoverageSemantics(db *gorm.DB, table string, allowPartial bool) error {
	base := table + " WHERE 1=1"
	if allowPartial {
		base += " AND session_id IS NOT NULL AND session_id <> '' AND coverage_id IS NOT NULL AND coverage_id <> '' AND source_kind IS NOT NULL AND source_kind <> '' AND status IS NOT NULL AND status <> '' AND window_start IS NOT NULL AND window_end IS NOT NULL AND received_at IS NOT NULL"
	}

	var invalid int64
	query := "SELECT COUNT(*) FROM " + base + " AND (" +
		"status IS NULL OR status NOT IN ('complete','failed')" +
		" OR source_kind IS NULL OR received_at IS NULL" +
		" OR window_start IS NULL OR window_end IS NULL OR window_end <= window_start" +
		" OR COALESCE(delivered,0) > COALESCE(emitted,0)" +
		" OR (status = 'complete' AND (COALESCE(dropped,0) <> 0 OR COALESCE(invalid,0) <> 0 OR COALESCE(errors,0) <> 0 OR COALESCE(delivered,0) <> COALESCE(emitted,0)))" +
		" OR (status = 'failed' AND (reason IS NULL OR BTRIM(reason) = ''))" +
		" OR NOT ((source_kind = 'file' AND producer_id = 'runtime-file')" +
		"      OR (source_kind = 'falco' AND producer_id = 'falco')" +
		"      OR (source_kind = 'ebpf' AND producer_id IN ('ebpf-exec','ebpf-connect','ebpf-all')))" +
		" OR (status = 'failed' AND continuous_since IS NOT NULL)" +
		")"
	if err := db.Raw(query).Scan(&invalid).Error; err != nil {
		return fmt.Errorf("validate %s semantic contract: %w", table, err)
	}
	if invalid != 0 {
		return fmt.Errorf("%s contains %d rows violating runtime coverage semantic contract", table, invalid)
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
	if err := validateRuntimeCoverageSemantics(db, "runtime_coverages", true); err != nil {
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
	var unownedReceipt int64
	if err := db.Table("runtime_coverage_receipts").
		Where("cluster_id IS NULL OR cluster_id = '' OR agent_id IS NULL OR agent_id = '' OR producer_id IS NULL OR producer_id = '' OR session_id IS NULL OR session_id = '' OR coverage_id IS NULL OR coverage_id = ''").
		Count(&unownedReceipt).Error; err != nil {
		return err
	}
	if unownedReceipt != 0 {
		return fmt.Errorf("runtime coverage history contains %d rows without immutable evidence identity", unownedReceipt)
	}
	if err := validateRuntimeCoverageSemantics(db, "runtime_coverage_receipts", false); err != nil {
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
		"SessionStartedAt", "Enabled", "Authoritative",
		"SourceHealthStatus", "SourceHealthProofKind", "SourceHealthObservedAt", "SourceHealthContinuousSince", "SourceHealthValidUntil", "State",
		"LastManifestAt", "LastHeartbeatAt", "LastCoverageID",
		"LastCoverageEnd", "GapSince", "GapReason",
	}); err != nil {
		return fmt.Errorf("runtime producer state columns: %w", err)
	}
	if err := db.Exec("SELECT cluster_id,agent_id,producer_id,source_kind,session_id,session_started_at,enabled,authoritative,source_health_status,source_health_proof_kind,source_health_observed_at,source_health_continuous_since,source_health_valid_until,state,last_manifest_at,last_heartbeat_at,last_coverage_id,last_coverage_end,gap_since,gap_reason FROM runtime_producer_states LIMIT 0").Error; err != nil {
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
	if err := ensureIndex(db, "idx_runtime_producer_heartbeat", "runtime_producer_states", "last_heartbeat_at", false); err != nil {
		return err
	}

	if !db.Migrator().HasTable(&models.RuntimeSourceHealth{}) {
		if err := db.Migrator().CreateTable(&models.RuntimeSourceHealth{}); err != nil {
			return err
		}
	}
	if err := ensureModelColumns(db, &models.RuntimeSourceHealth{}, []string{
		"ClusterID", "AgentID", "ProducerID", "SessionID", "SourceKind",
		"Status", "ProofKind", "ObservedAt", "ContinuousSince", "ValidUntil", "ReceivedAt", "Reason",
	}); err != nil {
		return fmt.Errorf("runtime source health columns: %w", err)
	}
	var unownedHealth int64
	if err := db.Table("runtime_source_healths").
		Where("cluster_id IS NULL OR cluster_id = '' OR agent_id IS NULL OR agent_id = '' OR producer_id IS NULL OR producer_id = '' OR session_id IS NULL OR session_id = ''").
		Count(&unownedHealth).Error; err != nil {
		return err
	}
	if unownedHealth != 0 {
		return fmt.Errorf("runtime source health contains %d rows without exact producer/session identity", unownedHealth)
	}
	if err := ensureIndex(db, "idx_runtime_source_health_identity", "runtime_source_healths", "cluster_id,agent_id,producer_id", true); err != nil {
		return err
	}
	return ensureIndex(db, "idx_runtime_source_health_valid_until", "runtime_source_healths", "valid_until", false)
}
