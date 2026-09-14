package rep

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// persistSynthesizedSignalsFromFacts is REP-B minimal persist path.
// It fills gaps when facts-based synthesis produces signals not already present
// in runtime_signals for the current day window.
//
// Safety: if a signal already exists for (pod_uid, signal_type) in the "today" window,
// we update evidence/confidence (if higher) but we DO NOT increment Count to avoid double-counting.
func persistSynthesizedSignalsFromFacts(
	ctx context.Context,
	db *gorm.DB,
	event *models.RuntimeEvent,
	facts []models.RuntimeBehaviorFact,
	cands []synthesizedSignal,
) error {
	if db == nil || event == nil || event.ID == 0 || strings.TrimSpace(event.PodUID) == "" {
		return nil
	}
	if len(cands) == 0 {
		return nil
	}

	// Keep dedupe semantics aligned with SignalAdapter (it uses "now truncate to day").
	today := time.Now().UTC().Truncate(24 * time.Hour)

	for _, s := range cands {
		// Ignore empty/unknown signal types defensively.
		if strings.TrimSpace(s.SignalType) == "" || strings.EqualFold(strings.TrimSpace(s.SignalType), "UNKNOWN") {
			continue
		}

		factIDs := factIDsForSynthesizedSignal(s.SignalType, facts)
		if len(factIDs) == 0 {
			factIDs = []string{"event:" + itoaUint(event.ID)}
		}

		evidence := map[string]interface{}{
			"source":            "runtime_behavior_facts",
			"evidence_fact_ids": factIDs,
		}
		evidenceJSON, _ := json.Marshal(evidence)
		evidenceRefsJSON, _ := json.Marshal(map[string]interface{}{
			"eventIds": []string{event.EventID},
			"factIds":  factIDs,
		})
		firstSeen := event.CreatedAt.UTC().Format(time.RFC3339Nano)
		lastSeen := event.CreatedAt.UTC().Format(time.RFC3339Nano)

		// Check existing runtime signal for today window.
		var existing models.RuntimeSignal
		err := db.WithContext(ctx).
			Where("pod_uid = ? AND signal_type = ? AND created_at >= ?", event.PodUID, s.SignalType, today).
			First(&existing).Error

		if err != nil {
			if err.Error() == gorm.ErrRecordNotFound.Error() {
				// New signal for today.
				row := &models.RuntimeSignal{
					PodUID:       event.PodUID,
					SignalType:   s.SignalType,
					Category:     s.Category,
					Confidence:   s.Confidence,
					Evidence:     string(evidenceJSON),
					EvidenceRefs: string(evidenceRefsJSON),
					Count:        1,
					FirstSeenAt:  &firstSeen,
					LastSeenAt:   &lastSeen,
					CreatedAt:    event.CreatedAt,
				}
				if err2 := db.WithContext(ctx).Create(row).Error; err2 != nil {
					return err2
				}
				continue
			}
			return err
		}

		// Exists already: update without changing Count to avoid inflation.
		// Update evidence/confidence only when confidence is higher.
		update := map[string]interface{}{
			"evidence":      string(evidenceJSON),
			"evidence_refs": string(evidenceRefsJSON),
			"category":      s.Category,
			"confidence":    existing.Confidence,
			"last_seen_at":  event.CreatedAt,
		}
		if s.Confidence > existing.Confidence {
			update["confidence"] = s.Confidence
		}
		// Always refresh evidence to keep latest facts-based details.
		update["evidence"] = string(evidenceJSON)

		if err2 := db.WithContext(ctx).
			Model(&existing).
			Updates(update).Error; err2 != nil {
			return err2
		}
	}

	return nil
}

func factIDsForSynthesizedSignal(signalType string, facts []models.RuntimeBehaviorFact) []string {
	want := map[string]map[string]struct{}{
		"INTERACTIVE_SHELL_EXEC":        {"INTERACTIVE_SHELL": {}},
		"TMP_BINARY_EXECUTION":          {"TMP_BINARY_EXEC": {}},
		"REMOTE_PAYLOAD_FETCH":          {"REMOTE_TOOL_EXEC": {}},
		"EXTERNAL_EGRESS":               {"EXTERNAL_CONNECT": {}, "NETWORK_CONNECT": {}},
		"SUSPICIOUS_EXEC_FROM_SNAPSHOT": {"INTERACTIVE_SHELL": {}, "TMP_BINARY_EXEC": {}},
		"NETWORK_QUEUE_ANOMALY":         {"EXTERNAL_CONNECT": {}, "NETWORK_CONNECT": {}},
		"SERVICEACCOUNT_TOKEN_READ":     {"SERVICEACCOUNT_TOKEN_READ": {}},
		"HOST_PATH_ACCESS":              {"HOST_PATH_TOUCH": {}},
	}
	set, ok := want[strings.ToUpper(strings.TrimSpace(signalType))]
	if !ok {
		return nil
	}
	out := make([]string, 0, 8)
	for i := range facts {
		ft := strings.ToUpper(strings.TrimSpace(facts[i].FactType))
		if _, ok := set[ft]; ok {
			if strings.TrimSpace(facts[i].FactID) != "" {
				out = append(out, facts[i].FactID)
			}
		}
	}
	return out
}

func itoaUint(id uint) string {
	// small helper to avoid pulling strconv everywhere
	if id == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	n := id
	for n > 0 {
		i--
		b[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(b[i:])
}
