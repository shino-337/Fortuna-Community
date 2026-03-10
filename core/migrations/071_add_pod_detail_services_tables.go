package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration071_AddPodDetailServicesTables adds tables for Pod Detail extended services:
// - pod_runtime_metrics: CPU/memory and container state (Runtime Service)
// - pod_processes: process list snapshot (Process Service)
// - pod_network_connections: network connections (Network Service)
// - k8s_events: Kubernetes cluster events (Event Service)
func Migration071_AddPodDetailServicesTables(db *gorm.DB) error {
	log.Println("Running migration 071: Add pod detail services tables (pod_runtime_metrics, pod_processes, pod_network_connections, k8s_events)")

	// 1. pod_runtime_metrics: per-container CPU/memory and state
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pod_runtime_metrics (
			id BIGSERIAL PRIMARY KEY,
			pod_uid VARCHAR(255) NOT NULL,
			cluster_id VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			container_name VARCHAR(255) NOT NULL,
			cpu_usage_millicore INTEGER DEFAULT 0,
			memory_usage_bytes BIGINT DEFAULT 0,
			memory_limit_bytes BIGINT DEFAULT 0,
			restart_count INTEGER DEFAULT 0,
			state VARCHAR(32) DEFAULT 'Running',
			last_observed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_pod_runtime_metrics_pod_uid ON pod_runtime_metrics(pod_uid);
		CREATE INDEX IF NOT EXISTS idx_pod_runtime_metrics_last_observed ON pod_runtime_metrics(last_observed_at);
	`).Error; err != nil {
		return err
	}

	// 2. pod_processes: process snapshot per pod
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pod_processes (
			id BIGSERIAL PRIMARY KEY,
			pod_uid VARCHAR(255) NOT NULL,
			cluster_id VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			container_name VARCHAR(255) NOT NULL,
			pid INTEGER NOT NULL,
			ppid INTEGER DEFAULT 0,
			user_name VARCHAR(255) DEFAULT '',
			cpu_percent REAL DEFAULT 0,
			memory_percent REAL DEFAULT 0,
			command TEXT DEFAULT '',
			binary_path VARCHAR(1024) DEFAULT '',
			started_at TIMESTAMP WITH TIME ZONE,
			observed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_pod_processes_pod_uid ON pod_processes(pod_uid);
		CREATE INDEX IF NOT EXISTS idx_pod_processes_observed_at ON pod_processes(observed_at);
	`).Error; err != nil {
		return err
	}

	// 3. pod_network_connections: connection snapshot per pod
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pod_network_connections (
			id BIGSERIAL PRIMARY KEY,
			pod_uid VARCHAR(255) NOT NULL,
			cluster_id VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			container_name VARCHAR(255) DEFAULT '',
			source_ip VARCHAR(45) DEFAULT '',
			source_port INTEGER DEFAULT 0,
			dest_ip VARCHAR(45) DEFAULT '',
			dest_port INTEGER DEFAULT 0,
			protocol VARCHAR(16) DEFAULT 'tcp',
			state VARCHAR(32) DEFAULT '',
			bytes_sent BIGINT DEFAULT 0,
			bytes_recv BIGINT DEFAULT 0,
			observed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_pod_network_connections_pod_uid ON pod_network_connections(pod_uid);
		CREATE INDEX IF NOT EXISTS idx_pod_network_connections_observed_at ON pod_network_connections(observed_at);
	`).Error; err != nil {
		return err
	}

	// 4. k8s_events: Kubernetes Events (Warning/Normal)
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS k8s_events (
			id BIGSERIAL PRIMARY KEY,
			cluster_id VARCHAR(255) NOT NULL,
			event_uid VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			event_name VARCHAR(255) NOT NULL,
			involved_kind VARCHAR(64) NOT NULL,
			involved_uid VARCHAR(255) NOT NULL,
			involved_name VARCHAR(255) NOT NULL,
			reason VARCHAR(128) NOT NULL,
			message TEXT DEFAULT '',
			event_type VARCHAR(32) DEFAULT 'Normal',
			count INTEGER DEFAULT 1,
			first_timestamp TIMESTAMP WITH TIME ZONE,
			last_timestamp TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_k8s_events_uid ON k8s_events(cluster_id, event_uid);
		CREATE INDEX IF NOT EXISTS idx_k8s_events_involved_uid ON k8s_events(involved_uid);
		CREATE INDEX IF NOT EXISTS idx_k8s_events_last_timestamp ON k8s_events(last_timestamp);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 071] Completed successfully")
	return nil
}
