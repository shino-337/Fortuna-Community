package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration101_AddPodRuntimeMetricsNetDev adds netns counters to pod_runtime_metrics.
// These counters come from /proc/<pid>/net/dev (host mode) and are stored as part of
// the runtime metrics stream for observability/throughput analysis.
func Migration101_AddPodRuntimeMetricsNetDev(db *gorm.DB) error {
	log.Println("Running migration 101: Add netns counters to pod_runtime_metrics")
	cols := []struct {
		name string
		typ  string
	}{
		{"net_rx_bytes", "BIGINT DEFAULT 0"},
		{"net_tx_bytes", "BIGINT DEFAULT 0"},
		{"net_rx_packets", "BIGINT DEFAULT 0"},
		{"net_tx_packets", "BIGINT DEFAULT 0"},
	}
	for _, c := range cols {
		var exists int
		if err := db.Raw(`
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_name = 'pod_runtime_metrics' AND column_name = ?
		`, c.name).Scan(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			if err := db.Exec("ALTER TABLE pod_runtime_metrics ADD COLUMN " + c.name + " " + c.typ).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

