package config

import (
	"testing"
	"time"
)

func TestRuntimePollingDurationsClampNonPositiveValues(t *testing.T) {
	t.Setenv("RUNTIME_EVENTS_POLL", "0s")
	t.Setenv("FALCO_EVENTS_POLL", "-1s")
	t.Setenv("EBPF_EVENT_FLUSH_INTERVAL", "0s")
	t.Setenv("RUNTIME_COVERAGE_CADENCE", "0s")

	cfg := LoadConfig()
	if cfg.RuntimeEventsPoll != 5*time.Second {
		t.Fatalf("runtime poll=%s want=5s", cfg.RuntimeEventsPoll)
	}
	if cfg.FalcoEventsPoll != 5*time.Second {
		t.Fatalf("falco poll=%s want=5s", cfg.FalcoEventsPoll)
	}
	if cfg.EBPFEventFlushInterval != 5*time.Second {
		t.Fatalf("ebpf flush=%s want=5s", cfg.EBPFEventFlushInterval)
	}
	if cfg.RuntimeCoverageCadence != 30*time.Second {
		t.Fatalf("coverage cadence=%s want=30s", cfg.RuntimeCoverageCadence)
	}
}

func TestRuntimeCoverageCadenceIndependentFromPoll(t *testing.T) {
	t.Setenv("RUNTIME_EVENTS_POLL", "5s")
	t.Setenv("FALCO_EVENTS_POLL", "7s")
	t.Setenv("RUNTIME_COVERAGE_CADENCE", "45s")

	cfg := LoadConfig()
	if cfg.RuntimeEventsPoll != 5*time.Second || cfg.FalcoEventsPoll != 7*time.Second {
		t.Fatalf("poll intervals unexpectedly changed: runtime=%s falco=%s", cfg.RuntimeEventsPoll, cfg.FalcoEventsPoll)
	}
	if cfg.RuntimeCoverageCadence != 45*time.Second {
		t.Fatalf("coverage cadence=%s want=45s", cfg.RuntimeCoverageCadence)
	}
}
