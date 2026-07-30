package worker

import "testing"

func TestShouldProcessSBOMForPhase(t *testing.T) {
	tests := []struct {
		name   string
		phase  string
		policy string
		want   bool
	}{
		{name: "running only - running", phase: "Running", policy: sbomPodPhasePolicyRunningOnly, want: true},
		{name: "running only - pending", phase: "Pending", policy: sbomPodPhasePolicyRunningOnly, want: false},
		{name: "all - pending", phase: "Pending", policy: sbomPodPhasePolicyAll, want: true},
		{name: "all_non_failed - succeeded", phase: "Succeeded", policy: sbomPodPhasePolicyNonFailed, want: true},
		{name: "all_non_failed - failed", phase: "Failed", policy: sbomPodPhasePolicyNonFailed, want: false},
		{name: "all_non_failed - unknown", phase: "Unknown", policy: sbomPodPhasePolicyNonFailed, want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := shouldProcessSBOMForPhase(tc.phase, tc.policy)
			if got != tc.want {
				t.Fatalf("phase=%s policy=%s got=%v want=%v", tc.phase, tc.policy, got, tc.want)
			}
		})
	}
}
