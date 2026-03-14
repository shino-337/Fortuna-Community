package poddetail

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HostProcessItem pairs a process with its pod/container for grouping.
type HostProcessItem struct {
	PodUID        string
	Namespace     string
	ContainerName string
	Process       processPayload
}

// CollectProcessesFromHost scans procRoot (e.g. /host/proc), maps each PID to container via cgroup,
// and returns process list with pod/container info. Group by PodUID in reporter to send per-pod POST.
func CollectProcessesFromHost(procRoot string, containerMap map[string]PodContainerInfo, observedAt string) []HostProcessItem {
	if observedAt == "" {
		observedAt = time.Now().Format(time.RFC3339)
	}
	pids := ListPIDs(procRoot)
	var result []HostProcessItem
	for _, pid := range pids {
		cid := ContainerIDFromCgroup(procRoot, pid)
		if cid == "" {
			continue
		}
		info, ok := containerMap[cid]
		if !ok {
			continue
		}
		comm, ppid := readProcStat(procRoot, pid)
		if comm == "" {
			comm = readProcComm(procRoot, pid)
		}
		if len(comm) > 1024 {
			comm = comm[:1024]
		}
		result = append(result, HostProcessItem{
			PodUID:        info.PodUID,
			Namespace:     info.Namespace,
			ContainerName: info.ContainerName,
			Process: processPayload{
				ContainerName: info.ContainerName,
				PID:           pid,
				PPID:          ppid,
				UserName:      "",
				CPUPercent:    0,
				MemoryPercent: 0,
				Command:       comm,
				BinaryPath:    comm,
				ObservedAt:    observedAt,
			},
		})
	}
	return result
}

// readProcComm reads /proc/<pid>/comm (single line, no newline in kernel).
func readProcComm(procRoot string, pid int) string {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "comm")
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// readProcStat reads /proc/<pid>/stat and returns (comm, ppid).
// Format: pid (comm) state ppid ...
// comm can contain spaces/parens so we find first "(" and last ")".
func readProcStat(procRoot string, pid int) (comm string, ppid int) {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "stat")
	f, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer f.Close()
	rd := bufio.NewReader(f)
	// Read until we have the comm and ppid: pid (comm) state ppid ...
	line, err := rd.ReadString('\n')
	if err != nil && line == "" {
		return "", 0
	}
	line = strings.TrimSpace(line)
	firstParen := strings.Index(line, "(")
	if firstParen < 0 {
		return "", 0
	}
	lastParen := strings.LastIndex(line, ")")
	if lastParen <= firstParen {
		return "", 0
	}
	comm = line[firstParen+1 : lastParen]
	rest := strings.TrimSpace(line[lastParen+1:])
	fields := strings.Fields(rest)
	// state is first, ppid is second (index 1 in rest)
	if len(fields) < 2 {
		return comm, 0
	}
	ppid, _ = strconv.Atoi(fields[1])
	return comm, ppid
}
