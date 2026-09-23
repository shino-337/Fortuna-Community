package migrations

import (
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

func EnsureRuntimeCoverage(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.RuntimeCoverage{}) {
		if err := db.Migrator().CreateTable(&models.RuntimeCoverage{}); err != nil {
			return err
		}
	}
	for _, column := range []string{"ContinuousSince", "Errors", "SessionID"} {
		if !db.Migrator().HasColumn(&models.RuntimeCoverage{}, column) {
			if err := db.Migrator().AddColumn(&models.RuntimeCoverage{}, column); err != nil {
				return err
			}
		}
	}
	if err := db.Exec("SELECT cluster_id,agent_id,producer_id,session_id,coverage_id,source_kind,status,window_start,window_end,received_at,continuous_since,emitted,delivered,dropped,invalid,errors,reason FROM runtime_coverages LIMIT 0").Error; err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_coverage_cluster_agent", "runtime_coverages", "cluster_id,agent_id", false); err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_coverage_window_end", "runtime_coverages", "window_end", false); err != nil {
		return err
	}
	if !db.Migrator().HasTable(&models.RuntimeProducerState{}) {
		if err := db.Migrator().CreateTable(&models.RuntimeProducerState{}); err != nil {
			return err
		}
	}
	if err := db.Exec("SELECT cluster_id,agent_id,producer_id,source_kind,session_id,session_started_at,enabled,authoritative,state,last_manifest_at,last_heartbeat_at,last_coverage_id,last_coverage_end,gap_since,gap_reason FROM runtime_producer_states LIMIT 0").Error; err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_runtime_producer_session", "runtime_producer_states", "session_id", false); err != nil {
		return err
	}
	return ensureIndex(db, "idx_runtime_producer_heartbeat", "runtime_producer_states", "last_heartbeat_at", false)
}
