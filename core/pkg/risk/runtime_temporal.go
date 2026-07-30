package risk

import (
	"math"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/k8scorroboration"
	"github.com/fortuna/core/pkg/models"
)

const runtimeTemporalHalfLifeHours = 12.0

// runtimeShellAPIPairingMaxGap caps how far apart shell-class and API-class evidence may be
// while still contributing a coherent narrative pairing (spec V.1).
const runtimeShellAPIPairingMaxGap = 20 * time.Minute

// RuntimeTemporalMeta is returned for factors / tuning (spec V).
type RuntimeTemporalMeta struct {
	SequenceBonus           float64 `json:"sequence_bonus"`
	DisorderPenalty         float64 `json:"disorder_penalty"`
	CoherenceMultiplier     float64 `json:"coherence_multiplier"`
	ShellBeforeAPI          *bool   `json:"shell_before_api,omitempty"`
	PairingWindowExceeded   bool    `json:"pairing_window_exceeded,omitempty"`
	PairingGapSeconds       float64 `json:"pairing_gap_seconds,omitempty"`
}

func parseRuntimeSignalTime(rs models.RuntimeSignal) time.Time {
	if rs.LastSeenAt != nil && strings.TrimSpace(*rs.LastSeenAt) != "" {
		if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(*rs.LastSeenAt)); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(*rs.LastSeenAt)); err == nil {
			return t
		}
	}
	if rs.FirstSeenAt != nil && strings.TrimSpace(*rs.FirstSeenAt) != "" {
		if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(*rs.FirstSeenAt)); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(*rs.FirstSeenAt)); err == nil {
			return t
		}
	}
	return rs.CreatedAt
}

func runtimeShellClassSignal(st string) bool {
	switch strings.ToUpper(strings.TrimSpace(st)) {
	case "INTERACTIVE_SHELL_EXEC", "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "EBPF_EXEC_ACTIVITY",
		"TMP_BINARY_EXECUTION":
		return true
	default:
		return false
	}
}

func runtimeAPIClassSignal(rs *models.RuntimeSignal) bool {
	if rs == nil {
		return false
	}
	st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
	if st == "SERVICEACCOUNT_TOKEN_READ" {
		return true
	}
	if st == "NETWORK_QUEUE_ANOMALY" && k8scorroboration.NetworkQueueSignalEvidenceIndicatesK8sAPI(rs) {
		return true
	}
	return false
}

// ComputeRuntimeTemporalCoherence applies ordering / disorder heuristics (spec V.1).
func ComputeRuntimeTemporalCoherence(events []models.RuntimeEvent, signals []models.RuntimeSignal) RuntimeTemporalMeta {
	meta := RuntimeTemporalMeta{CoherenceMultiplier: 1.0}
	var shellT, apiT time.Time
	haveShell, haveAPI := false, false

	for _, rs := range signals {
		t := parseRuntimeSignalTime(rs)
		st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
		if runtimeShellClassSignal(st) {
			if !haveShell || t.Before(shellT) {
				shellT = t
				haveShell = true
			}
		}
		if runtimeAPIClassSignal(&rs) {
			if !haveAPI || t.Before(apiT) {
				apiT = t
				haveAPI = true
			}
		}
	}
	for i := range events {
		e := &events[i]
		if !k8scorroboration.RuntimeEventCorroboratesK8sAPI(e) {
			continue
		}
		t := e.CreatedAt
		if e.ObservedAt != nil && !e.ObservedAt.IsZero() {
			t = *e.ObservedAt
		}
		if !haveAPI || t.Before(apiT) {
			apiT = t
			haveAPI = true
		}
	}

	if !haveShell || !haveAPI {
		return meta
	}
	pairGap := shellT.Sub(apiT)
	if pairGap < 0 {
		pairGap = -pairGap
	}
	meta.PairingGapSeconds = pairGap.Seconds()
	if pairGap > runtimeShellAPIPairingMaxGap {
		meta.PairingWindowExceeded = true
		meta.CoherenceMultiplier = 0.88
		return meta
	}
	shellFirst := shellT.Before(apiT) || shellT.Equal(apiT)
	meta.ShellBeforeAPI = &shellFirst
	if shellFirst {
		meta.SequenceBonus = 0.18
	} else {
		// Shell strictly after API — narrative ordering differs from ideal "foothold then API" story.
		if shellT.After(apiT) {
			meta.DisorderPenalty = 0.14
		}
	}
	meta.CoherenceMultiplier = math.Max(0.86, 1.0+meta.SequenceBonus-meta.DisorderPenalty)
	meta.CoherenceMultiplier = math.Min(1.12, meta.CoherenceMultiplier)
	return meta
}

// EventTemporalDecay returns a multiplier in (0,1] from event age (spec V.2).
func EventTemporalDecay(oldestEventAge time.Duration, halfLifeHours float64) float64 {
	if halfLifeHours <= 0 {
		halfLifeHours = runtimeTemporalHalfLifeHours
	}
	h := oldestEventAge.Hours()
	if h <= 0 {
		return 1.0
	}
	return math.Exp(-h / halfLifeHours)
}
