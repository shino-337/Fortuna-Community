package poddetail

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestListPIDs(t *testing.T) {
	dir := t.TempDir()
	// Create a few numeric dirs (PIDs) with cgroup file
	for _, pid := range []int{1, 2, 100} {
		pidDir := filepath.Join(dir, strconv.Itoa(pid))
		if err := os.MkdirAll(pidDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pidDir, "cgroup"), []byte("0::/\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Non-numeric dir should be skipped
	if err := os.MkdirAll(filepath.Join(dir, "net"), 0755); err != nil {
		t.Fatal(err)
	}
	pids := ListPIDs(dir)
	if len(pids) != 3 {
		t.Errorf("expected 3 PIDs, got %d: %v", len(pids), pids)
	}
}
