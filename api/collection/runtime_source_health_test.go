package collection

import (
	"testing"
	"time"
)

func validRuntimeSourceHealth(now time.Time) RuntimeSourceHealth {
	return RuntimeSourceHealth{
		Version:          RuntimeSourceHealthVersion,
		ID:               "health-contract-000001",
		ProducerID:       "falco",
		SourceKind:       RuntimeSourceFalco,
		SessionID:        "session-contract-000001",
		ProbeKind:        RuntimeSourceHealthProbeFalcoK8sReadiness,
		SourceInstanceID: "falco-pod-uid-0001",
		Status:           RuntimeSourceHealthHealthy,
		ObservedAt:       now,
	}
}

func TestRuntimeSourceHealthContract(t *testing.T) {
	now := time.Now().UTC()
	valid := validRuntimeSourceHealth(now)
	if err := valid.Validate(now); err != nil {
		t.Fatalf("valid source health rejected: %v", err)
	}

	selfAssertedFile := valid
	selfAssertedFile.ProducerID = "runtime-file"
	selfAssertedFile.SourceKind = RuntimeSourceFile
	if err := selfAssertedFile.Validate(now); err == nil {
		t.Fatal("runtime-file accepted unsupported source-health proof")
	}

	missingInstance := valid
	missingInstance.SourceInstanceID = ""
	if err := missingInstance.Validate(now); err == nil {
		t.Fatal("healthy source health without source instance was accepted")
	}

	unhealthy := valid
	unhealthy.Status = RuntimeSourceHealthUnhealthy
	unhealthy.SourceInstanceID = ""
	unhealthy.Reason = "upstream readiness failed"
	if err := unhealthy.Validate(now); err != nil {
		t.Fatalf("unhealthy source-health report rejected: %v", err)
	}

	unhealthy.Reason = ""
	if err := unhealthy.Validate(now); err == nil {
		t.Fatal("unhealthy source health without reason was accepted")
	}

	stale := valid
	stale.ObservedAt = now.Add(-3 * RuntimeProducerLeaseMaxAge)
	if err := stale.Validate(now); err == nil {
		t.Fatal("stale source-health observation was accepted")
	}
}
