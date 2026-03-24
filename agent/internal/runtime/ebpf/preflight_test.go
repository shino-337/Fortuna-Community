package ebpf

import "testing"

func TestRunPreflightShape(t *testing.T) {
	res := RunPreflight()
	// Invariant checks independent from host kernel features.
	if res.Ready {
		if !res.Root || !res.HasBTF || !res.HasBPFFS {
			t.Fatalf("ready preflight must imply root+btf+bpffs: %+v", res)
		}
	}
	if res.Reason == "" {
		t.Fatalf("expected non-empty reason")
	}
}
