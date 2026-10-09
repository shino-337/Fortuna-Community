package rep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/capability"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
)

var ErrRuntimeSourceRecordConflict = errors.New("rep: source-record identity reused with different payload")

type RuntimeEventInput struct {
	AgentID    string
	PodUID     string
	Namespace  string
	Syscall    string
	TargetPath string
	Capability string
	Timestamp  *time.Time
	// Canonical contract fields (P0.1): optional, best-effort.
	EventID         string
	SourceRecordID  string
	ObservedAt      *time.Time
	IngestedAt      *time.Time
	ResolutionState string
	SourceKind      string
	SourceSensorID  string
	SourceRule      string
	PayloadJSON     string
	PayloadHash     string
	// Optional enrichment for R9 correlation/richness.
	PodName        string
	NodeName       string
	Runtime        string
	EventType      string
	Signal         string
	MitreTechnique string
	Severity       string
	// Confidence is required (must be > 0). Sensors must supply explicit confidence.
	Confidence float64
}

type ProcessResult struct {
	Duplicate    bool
	Signal       string
	Mitre        string
	BaseScore    int
	CapabilityID string
	Severity     string
}

func ProcessRuntimeEvent(ctx context.Context, db *gorm.DB, input RuntimeEventInput) (*ProcessResult, error) {
	if strings.TrimSpace(input.PodUID) == "" {
		return nil, fmt.Errorf("rep: pod uid is required")
	}
	// Legacy/direct compatibility callers predate physical source-record identity.
	// Give each direct call an isolated identity rather than falling back to the
	// second-granularity EventID. Production scoped HTTP ingest requires the Agent
	// supplied source_record_id and therefore remains replay-idempotent.
	if strings.TrimSpace(input.SourceRecordID) == "" {
		input.SourceRecordID = fmt.Sprintf("legacy-%020d", time.Now().UnixNano())
	}
	var owners []string
	if err := db.WithContext(ctx).Model(&models.Pod{}).
		Where("uid = ? AND deleted_at IS NULL", input.PodUID).
		Distinct().Order("cluster_id").Pluck("cluster_id", &owners).Error; err != nil {
		return nil, err
	}
	if len(owners) != 1 {
		return nil, fmt.Errorf("rep: cluster-qualified pod identity required: uid=%s owners=%d", input.PodUID, len(owners))
	}
	id, err := resourceidentity.New(owners[0], input.PodUID)
	if err != nil {
		return nil, err
	}
	return ProcessRuntimeEventForIdentity(ctx, db, id, input)
}

func classifySignal(syscall, target, capabilityName, runtimeSource, sourceRule string, db *gorm.DB, ctx context.Context, podUID string) (string, string, int) {
	syscall = strings.ToLower(strings.TrimSpace(syscall))
	target = strings.TrimSpace(target)
	capabilityName = strings.ToUpper(strings.TrimSpace(capabilityName))
	runtimeSource = strings.ToLower(strings.TrimSpace(runtimeSource))

	if isProcRootPivot(syscall, target) {
		return "PROC_ROOT_PIVOT", "T1611", 90
	}
	if isFSEscapeAttempt(syscall, target) {
		return "FS_ESCAPE_ATTEMPT", "T1611", 95
	}
	if isNamespaceEscape(syscall, target) {
		return "NAMESPACE_ESCAPE", "T1055", 85
	}
	if isCapabilityMisuse(syscall, capabilityName, db, ctx, podUID) {
		return "CAPABILITY_MISUSE", "T1611", 60
	}

	// Falco: capability fields may not exist, so map exec/connect into Fortuna runtime-signals
	// via syscall + target heuristics to keep runtime YAML rules usable.
	if runtimeSource == "falco" {
		if strings.EqualFold(syscall, "execve") {
			if isSuspiciousProcessSnapshotExec(syscall, capabilityName, target) {
				return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "T1059", 45
			}
			// Falco provides evt.type=execve + proc.cmdline but not PROCESS_SNAPSHOT_DIFF capability.
			if falcoSuspiciousExecTarget(target) {
				return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "T1059", 48
			}
		}
		if strings.EqualFold(syscall, "connect") {
			t := strings.ToLower(target)
			looksNetwork := strings.Contains(t, ":") ||
				strings.Contains(t, "dst=") ||
				strings.Contains(t, "proto=") ||
				strings.Contains(t, "dport=")
			if looksNetwork {
				return "NETWORK_QUEUE_ANOMALY", "T1046", networkQueueSpikeScore(target)
			}
		}
	}

	// Falco: rule name (and generic falco.alert) after syscall heuristics — fills UNKNOWN gap.
	if runtimeSource == "falco" {
		if sig, ok := ClassifyFalcoRuleToSignal(sourceRule, syscall); ok {
			return sig.SignalType, sig.Mitre, sig.BaseScore
		}
	}

	if isSuspiciousProcessSnapshotExec(syscall, capabilityName, target) {
		return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "T1059", 45
	}
	if isNetworkQueueSpike(syscall, capabilityName) {
		return "NETWORK_QUEUE_ANOMALY", "T1046", networkQueueSpikeScore(target)
	}
	if isEBPFExecTrace(syscall, capabilityName) {
		return "EBPF_EXEC_ACTIVITY", "T1059", 40
	}
	if isEBPFConnectTrace(syscall, capabilityName) {
		return "EBPF_CONNECT_ACTIVITY", "T1046", 30
	}
	return "", "", 0
}

// isProcRootPivot matches accesses that indicate a container escape through
// procfs or kernel usermode-helper hooks:
//   - /proc/<pid>/root (or below) of another process, e.g. /proc/1/root: with
//     hostPID this is the host root filesystem;
//   - opening or writing /proc/sys/kernel/core_pattern, /proc/sys/kernel/modprobe,
//     a cgroup release_agent or /sys/kernel/uevent_helper, which make the
//     kernel run a chosen binary on the host.
//
// /proc/self/exe, /proc/<pid>/exe and /proc/self/root are routine (runtimes,
// language launchers, shells) and are not pivots.
func isProcRootPivot(syscall, target string) bool {
	t := path.Clean(strings.TrimSpace(target))
	switch syscall {
	case "open", "openat", "openat2", "write", "pwrite64":
		if isKernelHelperHook(t) {
			return true
		}
	}
	switch syscall {
	case "open", "openat", "openat2", "stat", "lstat", "newfstatat", "readlink", "readlinkat", "chdir", "chroot":
		return isOtherProcessRoot(t)
	}
	return false
}

func isKernelHelperHook(p string) bool {
	switch {
	case p == "/proc/sys/kernel/core_pattern", p == "/proc/sys/kernel/modprobe",
		p == "/sys/kernel/uevent_helper", path.Base(p) == "release_agent":
		return true
	}
	return false
}

// isOtherProcessRoot matches /proc/<numeric pid>/root[/...].
func isOtherProcessRoot(p string) bool {
	if !strings.HasPrefix(p, "/proc/") {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(p, "/proc/"), "/", 3)
	if len(parts) < 2 || parts[1] != "root" || parts[0] == "" {
		return false
	}
	for _, r := range parts[0] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isFSEscapeAttempt(syscall, target string) bool {
	if syscall != "mount" && syscall != "pivot_root" {
		return false
	}
	return strings.HasPrefix(target, "/proc") ||
		strings.HasPrefix(target, "/sys") ||
		strings.HasPrefix(target, "/dev") ||
		strings.HasPrefix(target, "/run") ||
		strings.HasPrefix(target, "/var/run")
}

func isNamespaceEscape(syscall, target string) bool {
	if syscall != "setns" && syscall != "unshare" && syscall != "clone" {
		return false
	}
	return strings.Contains(target, "/proc/") && strings.Contains(target, "/ns/")
}

func isCapabilityMisuse(syscall, capName string, db *gorm.DB, ctx context.Context, podUID string) bool {
	if syscall != "mount" && syscall != "setns" && syscall != "pivot_root" {
		return false
	}
	if capName != "SYS_ADMIN" {
		return false
	}
	var pod models.Pod
	if err := db.WithContext(ctx).Where("uid = ?", podUID).First(&pod).Error; err != nil {
		return true
	}
	return !podAllowsCapability(pod.ContainerSecurityContexts, "SYS_ADMIN")
}

func isSuspiciousProcessSnapshotExec(syscall, capName, target string) bool {
	if strings.ToLower(strings.TrimSpace(syscall)) != "execve" {
		return false
	}
	if strings.ToUpper(strings.TrimSpace(capName)) != "PROCESS_SNAPSHOT_DIFF" {
		return false
	}
	return falcoSuspiciousExecTarget(target)
}

// suspiciousExecBinaries are shells, network relays, downloaders and script
// interpreters, matched on the executable's basename.
var suspiciousExecBinaries = map[string]bool{
	"sh": true, "bash": true, "dash": true, "zsh": true, "ash": true, "ksh": true, "mksh": true,
	"csh": true, "tcsh": true, "fish": true,
	"nc": true, "ncat": true, "netcat": true, "socat": true,
	"curl": true, "wget": true,
	"perl": true, "ruby": true,
}

// falcoSuspiciousExecTarget classifies an execve target (an executable path
// or a proc.cmdline) by the basename of its executable, so that ssh, flush,
// sync or launch no longer match "sh"/"nc". Binaries run from /tmp or
// /dev/shm are suspicious too.
func falcoSuspiciousExecTarget(target string) bool {
	fields := strings.Fields(strings.ToLower(target))
	if len(fields) == 0 {
		return false
	}
	exe := fields[0]
	if strings.HasPrefix(exe, "/tmp/") || strings.HasPrefix(exe, "/dev/shm/") {
		return true
	}
	base := path.Base(exe)
	if base == "busybox" && len(fields) > 1 {
		base = path.Base(fields[1])
	}
	if suspiciousExecBinaries[base] {
		return true
	}
	// python, python3, python3.12, ...
	return strings.HasPrefix(base, "python") && strings.Trim(strings.TrimPrefix(base, "python"), "0123456789.") == ""
}

func isNetworkQueueSpike(syscall, capabilityName string) bool {
	if strings.ToLower(strings.TrimSpace(syscall)) != "connect" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(capabilityName), "NETWORK_TXRX_QUEUE_SPIKE")
}

func networkQueueSpikeScore(target string) int {
	ratio := parseTargetFloatKV(target, "ratio")
	// Default for backward compatibility when old events don't carry ratio.
	if ratio <= 0 {
		return 35
	}
	switch {
	case ratio >= 12:
		return 65
	case ratio >= 8:
		return 55
	case ratio >= 5:
		return 45
	default:
		return 35
	}
}

func parseTargetFloatKV(target, key string) float64 {
	if target == "" || key == "" {
		return 0
	}
	for _, token := range strings.Fields(target) {
		if !strings.HasPrefix(token, key+"=") {
			continue
		}
		v := strings.TrimPrefix(token, key+"=")
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

func isEBPFExecTrace(syscall, capabilityName string) bool {
	return strings.EqualFold(strings.TrimSpace(syscall), "execve") &&
		strings.EqualFold(strings.TrimSpace(capabilityName), "EBPF_EXEC_TRACE")
}

func isEBPFConnectTrace(syscall, capabilityName string) bool {
	return strings.EqualFold(strings.TrimSpace(syscall), "connect") &&
		strings.EqualFold(strings.TrimSpace(capabilityName), "EBPF_CONNECT_TRACE")
}

func podAllowsCapability(containerSecurityContexts, capName string) bool {
	if containerSecurityContexts == "" {
		return false
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(containerSecurityContexts), &m); err != nil {
		return false
	}
	for _, v := range m {
		cm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		caps, ok := cm["capabilities"].(map[string]interface{})
		if !ok {
			continue
		}
		add, ok := caps["add"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range add {
			if s, ok := c.(string); ok && strings.EqualFold(s, capName) {
				return true
			}
		}
	}
	return false
}

func scoreToCapability(score int) (string, string) {
	if score >= 90 {
		return capability.ESC_RUNTIME_ACTIVE, "CRITICAL"
	}
	if score >= 60 {
		return capability.ESC_RUNTIME_PROBE, "HIGH"
	}
	return "", ""
}
