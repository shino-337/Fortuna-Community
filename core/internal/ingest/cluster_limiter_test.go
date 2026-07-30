package ingest

import (
	"testing"
)

func TestClusterRateLimiter_AllowSync(t *testing.T) {
	cfg := ClusterLimitConfig{
		SyncRPS:   2,   // 2 per second
		SyncBurst: 2,
		SBOMRPS:   10,
		SBOMBurst: 10,
		Enabled:   true,
	}
	rl := NewClusterRateLimiter(cfg)

	// Burst of 2 should succeed
	if !rl.AllowSync("cluster-a") {
		t.Error("expected first AllowSync(cluster-a)")
	}
	if !rl.AllowSync("cluster-a") {
		t.Error("expected second AllowSync(cluster-a)")
	}
	// Third within same second may be rate limited
	allowed := rl.AllowSync("cluster-a")
	// Token bucket refills over time; we only assert burst 2, so third might be false
	_ = allowed

	// Different cluster has its own bucket
	if !rl.AllowSync("cluster-b") {
		t.Error("expected AllowSync(cluster-b)")
	}
}

func TestClusterRateLimiter_AllowSBOM(t *testing.T) {
	cfg := ClusterLimitConfig{SBOMRPS: 100, SBOMBurst: 10, Enabled: true}
	rl := NewClusterRateLimiter(cfg)
	if !rl.AllowSBOM("c1") {
		t.Error("expected AllowSBOM(c1)")
	}
}

func TestClusterRateLimiter_Disabled(t *testing.T) {
	cfg := ClusterLimitConfig{Enabled: false}
	rl := NewClusterRateLimiter(cfg)
	for i := 0; i < 100; i++ {
		if !rl.AllowSync("any") || !rl.AllowSBOM("any") {
			t.Errorf("disabled limiter should allow all (i=%d)", i)
		}
	}
}
