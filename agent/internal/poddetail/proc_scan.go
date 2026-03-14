package poddetail

import (
	"os"
	"path/filepath"
	"strconv"
)

// MaxPID limits how high we scan (avoid reading kernel threads / noise).
const MaxPID = 4194304

// ListPIDs reads procRoot (e.g. /host/proc or /proc) and returns numeric PIDs.
// Skips non-numeric dirs and PIDs above MaxPID.
func ListPIDs(procRoot string) []int {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 || pid > MaxPID {
			continue
		}
		// Optional: skip if /proc/<pid>/cgroup not readable (kernel thread or gone)
		cgroupPath := filepath.Join(procRoot, e.Name(), "cgroup")
		if _, err := os.Stat(cgroupPath); err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}
