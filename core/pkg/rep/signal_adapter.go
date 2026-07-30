package rep

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// SignalAdapter converts raw runtime events to semantic runtime signals
type SignalAdapter struct {
	db *gorm.DB
}

// NewSignalAdapter creates a new signal adapter
func NewSignalAdapter(db *gorm.DB) *SignalAdapter {
	return &SignalAdapter{db: db}
}

// AdaptEvent converts a runtime event to a runtime signal
func (a *SignalAdapter) AdaptEvent(ctx context.Context, event *models.RuntimeEvent) (*models.RuntimeSignal, error) {
	signalType, category, confidence := classifyEventToSignal(event)

	evidence := map[string]interface{}{
		"syscall":    event.Syscall,
		"target":     event.TargetPath,
		"capability": event.Capability,
	}
	evidenceJSON, _ := json.Marshal(evidence)
	evidenceRefsJSON, _ := json.Marshal(map[string]interface{}{
		"eventIds": []string{event.EventID},
	})
	firstSeen := event.CreatedAt.UTC().Format(time.RFC3339Nano)
	lastSeen := event.CreatedAt.UTC().Format(time.RFC3339Nano)

	signal := &models.RuntimeSignal{
		PodUID:       event.PodUID,
		SignalType:   signalType,
		Category:     category,
		Confidence:   confidence,
		Evidence:     string(evidenceJSON),
		EvidenceRefs: string(evidenceRefsJSON),
		Count:        1,
		FirstSeenAt:  &firstSeen,
		LastSeenAt:   &lastSeen,
		CreatedAt:    event.CreatedAt,
	}

	return signal, nil
}

// AdaptAndPersist converts and persists a runtime event as a signal
func (a *SignalAdapter) AdaptAndPersist(ctx context.Context, event *models.RuntimeEvent) error {
	signal, err := a.AdaptEvent(ctx, event)
	if err != nil {
		return err
	}

	// Check if signal already exists (deduplication by pod_uid + signal_type + same day)
	var existing models.RuntimeSignal
	today := time.Now().Truncate(24 * time.Hour)
	err = a.db.WithContext(ctx).
		Where("pod_uid = ? AND signal_type = ? AND created_at >= ?",
			signal.PodUID, signal.SignalType, today).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// New signal, create it
		return a.db.WithContext(ctx).Create(signal).Error
	} else if err != nil {
		return err
	}

	// Signal exists for today: bump occurrence count and keep best evidence/confidence.
	existing.Count++
	lastSeen := signal.CreatedAt.UTC().Format(time.RFC3339Nano)
	existing.LastSeenAt = &lastSeen
	if existing.FirstSeenAt == nil {
		firstSeen := signal.CreatedAt.UTC().Format(time.RFC3339Nano)
		existing.FirstSeenAt = &firstSeen
	}
	if signal.Confidence > existing.Confidence {
		existing.Confidence = signal.Confidence
		existing.Evidence = signal.Evidence
		existing.EvidenceRefs = signal.EvidenceRefs
	}
	return a.db.WithContext(ctx).Save(&existing).Error
}

// classifyEventToSignal maps runtime event to semantic signal
func classifyEventToSignal(event *models.RuntimeEvent) (signalType, category string, confidence float64) {
	syscall := event.Syscall
	target := event.TargetPath
	runtimeSource := strings.ToLower(strings.TrimSpace(event.Runtime))

	// PROC_ROOT_PIVOT detection
	if (syscall == "openat" || syscall == "open" || syscall == "stat" || syscall == "readlink") &&
		(target == "/proc/1/root" ||
			target == "/proc/self/exe" ||
			target == "/proc/1/exe" ||
			(len(target) > 6 && target[:6] == "/proc/" && (target[6:] == "1/root" || target[6:] == "self/exe"))) {
		return "PROC_ROOT_PIVOT", "ESCAPE", 0.9
	}

	// FS_ESCAPE_ATTEMPT detection
	if (syscall == "mount" || syscall == "pivot_root") &&
		(target == "/proc" || target == "/sys" || target == "/dev" ||
			target == "/run" || target == "/var/run" ||
			len(target) >= 5 && (target[:5] == "/proc" || target[:5] == "/sys/" || target[:4] == "/dev")) {
		return "FS_ESCAPE_ATTEMPT", "ESCAPE", 0.95
	}

	// NAMESPACE_ESCAPE detection
	if (syscall == "setns" || syscall == "unshare" || syscall == "clone") &&
		len(target) > 6 && target[:6] == "/proc/" && len(target) > 10 && target[6:10] == "ns/" {
		return "NAMESPACE_ESCAPE", "ESCAPE", 0.85
	}

	// CAPABILITY_MISUSE detection
	if (syscall == "mount" || syscall == "setns" || syscall == "pivot_root") &&
		event.Capability == "SYS_ADMIN" {
		return "CAPABILITY_MISUSE", "ESCAPE", 0.6
	}

	// SUSPICIOUS_EXEC_FROM_SNAPSHOT (R6 heuristic extension)
	if strings.EqualFold(syscall, "execve") {
		// For Falco: capability might be missing; still map to suspicious exec when the target looks like
		// "bash/sh/nc/curl/wget/..." or comes from writable locations (/tmp,/dev/shm).
		allowFalco := runtimeSource == "falco"
		allowSnapshot := strings.EqualFold(event.Capability, "PROCESS_SNAPSHOT_DIFF")
		if allowFalco || allowSnapshot {
			t := strings.ToLower(strings.TrimSpace(target))
			if t != "" {
				keywords := []string{
					"bash", "sh", "nc", "netcat", "ncat", "socat",
					"curl", "wget", "python", "perl", "ruby",
				}
				for _, k := range keywords {
					if strings.Contains(t, k) {
						return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "EXECUTION", 0.7
					}
				}
				if strings.HasPrefix(t, "/tmp/") || strings.HasPrefix(t, "/dev/shm/") {
					return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "EXECUTION", 0.7
				}
			}
		}
	}
	// NETWORK_QUEUE_ANOMALY (R5 phase-2)
	if strings.EqualFold(syscall, "connect") {
		allowNetworkSpike := strings.EqualFold(event.Capability, "NETWORK_TXRX_QUEUE_SPIKE")
		// For Falco: also map connect() to queue anomaly when the target looks like network-ish evidence
		// (ip:port, dst=..., proto=...). This keeps runtime-signals consistent for existing runtime YAML rules.
		allowFalco := runtimeSource == "falco" &&
			(strings.Contains(strings.ToLower(target), ":") ||
				strings.Contains(strings.ToLower(target), "dst=") ||
				strings.Contains(strings.ToLower(target), "proto=") ||
				strings.Contains(strings.ToLower(target), "dport="))
		if allowNetworkSpike || allowFalco {
			return "NETWORK_QUEUE_ANOMALY", "NETWORK", 0.65
		}
	}

	// R9 eBPF enriched mappings
	if strings.EqualFold(syscall, "execve") && strings.EqualFold(event.Capability, "EBPF_EXEC_TRACE") {
		return "EBPF_EXEC_ACTIVITY", "EXECUTION", 0.75
	}
	if strings.EqualFold(syscall, "connect") && strings.EqualFold(event.Capability, "EBPF_CONNECT_TRACE") {
		return "EBPF_CONNECT_ACTIVITY", "NETWORK", 0.65
	}

	// Falco: map rule name when syscall is generic (falco.alert) or heuristics missed.
	if runtimeSource == "falco" {
		if sig, ok := ClassifyFalcoRuleToSignal(event.SourceRule, event.Syscall); ok {
			return sig.SignalType, sig.Category, sig.Confidence
		}
	}

	// Default: unknown signal
	return "UNKNOWN", "UNKNOWN", 0.3
}
