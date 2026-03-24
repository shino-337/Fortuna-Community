package poddetail

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"regexp"
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

const defaultClockTicksPerSecond = 100.0
const defaultCPUCoreCount = 1

// CollectProcessesFromHost scans procRoot (e.g. /host/proc), maps each PID to container via cgroup,
// and returns process list with pod/container info. Group by PodUID in reporter to send per-pod POST.
func CollectProcessesFromHost(procRoot string, containerMap map[string]PodContainerInfo, observedAt string) []HostProcessItem {
	if observedAt == "" {
		observedAt = time.Now().Format(time.RFC3339)
	}
	pids := ListPIDs(procRoot)
	approxCPUFromProc := enableApproxCPUFromProc()
	cpuCoreCount := readHostCPUCoreCount(procRoot)
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
		cmdline := readProcCmdline(procRoot, pid)
		if cmdline == "" {
			cmdline = comm
		}
		if len(cmdline) > 1024 {
			cmdline = cmdline[:1024]
		}
		uid, gid, capEff := readProcStatusMeta(procRoot, pid)
		cwd := readProcCwd(procRoot, pid)
		binaryPath := inferBinaryPath(cmdline, comm)
		cpuPercent := 0.0
		if approxCPUFromProc {
			cpuPercent = readProcCPUPercent(procRoot, pid, cpuCoreCount)
		}
		memPercent := readProcMemoryPercent(procRoot, pid)
		result = append(result, HostProcessItem{
			PodUID:        info.PodUID,
			Namespace:     info.Namespace,
			ContainerName: info.ContainerName,
			Process: processPayload{
				ContainerName: info.ContainerName,
				PID:           pid,
				PPID:          ppid,
				UserName:      "",
				UserID:        uid,
				GroupID:       gid,
				CPUPercent:    cpuPercent,
				MemoryPercent: memPercent,
				Command:       cmdline,
				BinaryPath:    binaryPath,
				WorkingDir:    truncate(cwd, 1024),
				CapEff:        capEff,
				ObservedAt:    observedAt,
			},
		})
	}
	return result
}

func enableApproxCPUFromProc() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("POD_DETAIL_PROCESS_CPU_APPROX_FROM_PROC")))
	// default ON to preserve behavior without extra env.
	if v == "" {
		return true
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
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

// readProcCmdline reads /proc/<pid>/cmdline and converts NUL-delimited args to a printable string.
func readProcCmdline(procRoot string, pid int) string {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "cmdline")
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return ""
	}
	parts := strings.Split(string(b), "\x00")
	args := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		args = append(args, p)
	}
	return strings.Join(args, " ")
}

// readProcStatusMeta reads Uid/Gid/CapEff from /proc/<pid>/status.
func readProcStatusMeta(procRoot string, pid int) (uid int, gid int, capEff string) {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "status")
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, ""
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "Uid:"):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid, _ = strconv.Atoi(fields[1])
			}
		case strings.HasPrefix(line, "Gid:"):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				gid, _ = strconv.Atoi(fields[1])
			}
		case strings.HasPrefix(line, "CapEff:"):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				capEff = fields[1]
			}
		}
	}
	return uid, gid, truncate(capEff, 128)
}

// readProcCwd resolves /proc/<pid>/cwd symlink.
func readProcCwd(procRoot string, pid int) string {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "cwd")
	cwd, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return cwd
}

func inferBinaryPath(cmdline, comm string) string {
	first := strings.TrimSpace(cmdline)
	if first != "" {
		fields := strings.Fields(first)
		if len(fields) > 0 {
			return truncate(fields[0], 1024)
		}
	}
	return truncate(comm, 1024)
}

// readProcCPUPercent returns approximate process CPU percent from /proc:
// %CPU ~= 100 * (utime+stime)/HZ / (uptime - starttime/HZ)
// This is a lifetime average (not instant), but removes always-0 blind spot in host mode.
func readProcCPUPercent(procRoot string, pid int, cpuCoreCount int) float64 {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "stat")
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	procJiffies, startJiffies, ok := parseProcStatCPUFields(string(b))
	if !ok {
		return 0
	}
	upB, err := os.ReadFile(filepath.Join(procRoot, "uptime"))
	if err != nil {
		return 0
	}
	uptimeFields := strings.Fields(string(upB))
	if len(uptimeFields) == 0 {
		return 0
	}
	uptimeSec, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil || uptimeSec <= 0 {
		return 0
	}
	elapsedSec := uptimeSec - (startJiffies / defaultClockTicksPerSecond)
	if elapsedSec <= 0 {
		return 0
	}
	cpuSec := procJiffies / defaultClockTicksPerSecond
	if cpuCoreCount <= 0 {
		cpuCoreCount = defaultCPUCoreCount
	}
	// Normalize by node CPU cores to avoid inflated values on multi-core nodes.
	v := (cpuSec / elapsedSec) * 100.0 / float64(cpuCoreCount)
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func readHostCPUCoreCount(procRoot string) int {
	b, err := os.ReadFile(filepath.Join(procRoot, "stat"))
	if err != nil {
		return defaultCPUCoreCount
	}
	re := regexp.MustCompile(`^cpu[0-9]+\s`)
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if re.MatchString(line) {
			n++
		}
	}
	if n <= 0 {
		return defaultCPUCoreCount
	}
	return n
}

// parseProcStatCPUFields parses utime/stime/starttime from /proc/<pid>/stat.
func parseProcStatCPUFields(stat string) (procJiffies float64, startJiffies float64, ok bool) {
	line := strings.TrimSpace(stat)
	firstParen := strings.Index(line, "(")
	lastParen := strings.LastIndex(line, ")")
	if firstParen < 0 || lastParen <= firstParen {
		return 0, 0, false
	}
	rest := strings.TrimSpace(line[lastParen+1:])
	fields := strings.Fields(rest)
	// In "rest", index 11=utime, 12=stime, 19=starttime.
	if len(fields) <= 19 {
		return 0, 0, false
	}
	utime, err1 := strconv.ParseFloat(fields[11], 64)
	stime, err2 := strconv.ParseFloat(fields[12], 64)
	start, err3 := strconv.ParseFloat(fields[19], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, false
	}
	return utime + stime, start, true
}

func readProcMemoryPercent(procRoot string, pid int) float64 {
	statusB, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "status"))
	if err != nil {
		return 0
	}
	vmRssKB := parseVmRssKB(string(statusB))
	if vmRssKB <= 0 {
		return 0
	}
	memInfoB, err := os.ReadFile(filepath.Join(procRoot, "meminfo"))
	if err != nil {
		return 0
	}
	memTotalKB := parseMemTotalKB(string(memInfoB))
	if memTotalKB <= 0 {
		return 0
	}
	v := (float64(vmRssKB) / float64(memTotalKB)) * 100.0
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func parseVmRssKB(status string) int64 {
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					return v
				}
			}
			return 0
		}
	}
	return 0
}

func parseMemTotalKB(meminfo string) int64 {
	for _, line := range strings.Split(meminfo, "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					return v
				}
			}
			return 0
		}
	}
	return 0
}
