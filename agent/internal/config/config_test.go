package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Set test environment variables
	os.Setenv("KSAM_CORE_ENDPOINT", "http://test:8080")
	os.Setenv("KSAM_CLUSTER_ID", "test-cluster")
	os.Setenv("KSAM_SYNC_INTERVAL", "60s")
	defer os.Clearenv()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.CoreEndpoint != "http://test:8080" {
		t.Errorf("Expected CoreEndpoint to be 'http://test:8080', got '%s'", cfg.CoreEndpoint)
	}

	if cfg.ClusterID != "test-cluster" {
		t.Errorf("Expected ClusterID to be 'test-cluster', got '%s'", cfg.ClusterID)
	}

	if cfg.SyncInterval != 60*time.Second {
		t.Errorf("Expected SyncInterval to be 60s, got %v", cfg.SyncInterval)
	}
}

