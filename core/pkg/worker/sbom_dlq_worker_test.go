package worker

import (
	"os"
	"testing"
)

func TestSBOMDLQReplayMaxAttempts_DefaultAndInvalid(t *testing.T) {
	t.Setenv("FORTUNA_SBOM_DLQ_REPLAY_MAX_ATTEMPTS", "")
	if got := sbomDLQReplayMaxAttempts(); got != 5 {
		t.Fatalf("default max attempts = %d, want 5", got)
	}

	_ = os.Setenv("FORTUNA_SBOM_DLQ_REPLAY_MAX_ATTEMPTS", "invalid")
	if got := sbomDLQReplayMaxAttempts(); got != 5 {
		t.Fatalf("invalid env max attempts = %d, want 5", got)
	}

	_ = os.Setenv("FORTUNA_SBOM_DLQ_REPLAY_MAX_ATTEMPTS", "9")
	if got := sbomDLQReplayMaxAttempts(); got != 9 {
		t.Fatalf("configured max attempts = %d, want 9", got)
	}
}
