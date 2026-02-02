package rep

import (
	"context"
	"encoding/json"
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
		"syscall":   event.Syscall,
		"target":    event.TargetPath,
		"capability": event.Capability,
	}
	evidenceJSON, _ := json.Marshal(evidence)

	signal := &models.RuntimeSignal{
		PodUID:     event.PodUID,
		SignalType: signalType,
		Category:   category,
		Confidence: confidence,
		Evidence:   string(evidenceJSON),
		CreatedAt:  event.CreatedAt,
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

	// Signal exists, update confidence if higher
	if signal.Confidence > existing.Confidence {
		existing.Confidence = signal.Confidence
		existing.Evidence = signal.Evidence
		return a.db.WithContext(ctx).Save(&existing).Error
	}

	return nil
}

// classifyEventToSignal maps runtime event to semantic signal
func classifyEventToSignal(event *models.RuntimeEvent) (signalType, category string, confidence float64) {
	syscall := event.Syscall
	target := event.TargetPath

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

	// Default: unknown signal
	return "UNKNOWN", "UNKNOWN", 0.3
}
