package rep

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// correlateAndPersistRuntimeIncidents is REP-C minimal correlator entry point.
// P0 implementation adds first stateful detector: RECON_BURST.
func correlateAndPersistRuntimeIncidents(ctx context.Context, db *gorm.DB, event *models.RuntimeEvent, facts []models.RuntimeBehaviorFact) error {
	if db == nil || event == nil || event.ID == 0 || strings.TrimSpace(event.PodUID) == "" {
		return nil
	}

	if hasAnyFactType(facts, "NETWORK_CONNECT", "EXTERNAL_CONNECT") {
		if err := correlateReconBurst(ctx, db, event, facts); err != nil {
			return err
		}
	}
	if hasAnyFactType(facts, "INTERACTIVE_SHELL", "REMOTE_TOOL_EXEC", "TMP_BINARY_EXEC") {
		if err := correlatePostExploitExecChain(ctx, db, event); err != nil {
			return err
		}
	}
	if hasAnyFactType(facts, "SERVICEACCOUNT_TOKEN_READ", "EXTERNAL_CONNECT") {
		if err := correlateExfilLikeSequence(ctx, db, event); err != nil {
			return err
		}
	}

	return nil
}

func correlateReconBurst(ctx context.Context, db *gorm.DB, event *models.RuntimeEvent, facts []models.RuntimeBehaviorFact) error {
	d := DetectorReconBurst
	since := event.CreatedAt.Add(-d.Window)
	var count int64
	// Count unique fact ids to avoid duplicates across sources/replays.
	if err := db.WithContext(ctx).
		Model(&models.RuntimeBehaviorFact{}).
		Select("COUNT(DISTINCT fact_id)").
		Where("pod_uid = ? AND observed_at >= ? AND fact_type IN ?", event.PodUID, since, d.InputFactTypes).
		Scan(&count).Error; err != nil {
		return err
	}
	if count < d.MinDistinctFacts {
		return nil
	}

	bucket := d.Bucket(event.CreatedAt)
	incidentID := fmt.Sprintf("%s:%s:%d", event.PodUID, d.IncidentType, bucket)

	evidenceRefs := make([]string, 0, len(facts))
	for i := range facts {
		if strings.TrimSpace(facts[i].FactID) != "" {
			evidenceRefs = append(evidenceRefs, facts[i].FactID)
		}
	}
	if len(evidenceRefs) == 0 {
		evidenceRefs = append(evidenceRefs, fmt.Sprintf("event:%d", event.ID))
	}

	evidenceJSON, _ := json.Marshal(evidenceRefs)
	metadataJSON, _ := json.Marshal(map[string]interface{}{
		"detector_id": d.ID,
		"version":     d.Version,
		"window":      d.WindowLabel,
		"fact_count":  count,
	})

	row := models.RuntimeIncident{
		IncidentID:   incidentID,
		PodUID:       event.PodUID,
		Namespace:    event.Namespace,
		IncidentType: d.IncidentType,
		SeverityHint: d.SeverityHint,
		Confidence:   d.Confidence,
		FirstSeenAt:  event.CreatedAt,
		LastSeenAt:   event.CreatedAt,
		Window:       d.WindowLabel,
		EvidenceRefs: string(evidenceJSON),
		Metadata:     string(metadataJSON),
		CreatedAt:    event.CreatedAt,
		UpdatedAt:    event.CreatedAt,
	}

	return upsertOrUpdateLatestIncident(ctx, db, &row, d.Cooldown)
}

func correlatePostExploitExecChain(ctx context.Context, db *gorm.DB, event *models.RuntimeEvent) error {
	d := DetectorPostExploitExecChain
	since := event.CreatedAt.Add(-d.Window)
	required := d.InputFactTypes

	type rows struct {
		FactType string
	}
	var got []rows
	if err := db.WithContext(ctx).
		Model(&models.RuntimeBehaviorFact{}).
		Select("DISTINCT fact_type").
		Where("pod_uid = ? AND observed_at >= ? AND fact_type IN ?", event.PodUID, since, required).
		Scan(&got).Error; err != nil {
		return err
	}
	if len(got) < len(required) {
		return nil
	}

	// Collect recent evidence refs for explainability.
	var refs []models.RuntimeBehaviorFact
	_ = db.WithContext(ctx).
		Where("pod_uid = ? AND observed_at >= ? AND fact_type IN ?", event.PodUID, since, required).
		Order("observed_at DESC").
		Limit(12).
		Find(&refs).Error
	evidenceRefs := make([]string, 0, len(refs))
	for i := range refs {
		if strings.TrimSpace(refs[i].FactID) != "" {
			evidenceRefs = append(evidenceRefs, refs[i].FactID)
		}
	}
	if len(evidenceRefs) == 0 {
		evidenceRefs = append(evidenceRefs, fmt.Sprintf("event:%d", event.ID))
	}

	evidenceJSON, _ := json.Marshal(evidenceRefs)
	metadataJSON, _ := json.Marshal(map[string]interface{}{
		"detector_id": d.ID,
		"version":     d.Version,
		"window":      d.WindowLabel,
		"required":    required,
	})

	bucket := d.Bucket(event.CreatedAt)
	incidentID := fmt.Sprintf("%s:%s:%d", event.PodUID, d.IncidentType, bucket)

	row := models.RuntimeIncident{
		IncidentID:   incidentID,
		PodUID:       event.PodUID,
		Namespace:    event.Namespace,
		IncidentType: d.IncidentType,
		SeverityHint: d.SeverityHint,
		Confidence:   d.Confidence,
		FirstSeenAt:  event.CreatedAt,
		LastSeenAt:   event.CreatedAt,
		Window:       d.WindowLabel,
		EvidenceRefs: string(evidenceJSON),
		Metadata:     string(metadataJSON),
		CreatedAt:    event.CreatedAt,
		UpdatedAt:    event.CreatedAt,
	}
	return upsertOrUpdateLatestIncident(ctx, db, &row, d.Cooldown)
}

func correlateExfilLikeSequence(ctx context.Context, db *gorm.DB, event *models.RuntimeEvent) error {
	d := DetectorExfilLikeSequence
	since := event.CreatedAt.Add(-d.Window)

	var tokenRead models.RuntimeBehaviorFact
	err := db.WithContext(ctx).
		Where("pod_uid = ? AND observed_at >= ? AND fact_type = ?", event.PodUID, since, "SERVICEACCOUNT_TOKEN_READ").
		Order("observed_at DESC").
		Limit(1).
		Find(&tokenRead).Error
	if err != nil {
		return err
	}
	if tokenRead.ID == 0 {
		return nil
	}

	var extConnect models.RuntimeBehaviorFact
	err = db.WithContext(ctx).
		Where("pod_uid = ? AND observed_at >= ? AND fact_type = ?", event.PodUID, since, "EXTERNAL_CONNECT").
		Order("observed_at DESC").
		Limit(1).
		Find(&extConnect).Error
	if err != nil {
		return err
	}
	if extConnect.ID == 0 {
		return nil
	}

	// Best-effort ordering to reduce false positives.
	if extConnect.ObservedAt.Before(tokenRead.ObservedAt) {
		return nil
	}

	evidenceRefs := []string{}
	if strings.TrimSpace(tokenRead.FactID) != "" {
		evidenceRefs = append(evidenceRefs, tokenRead.FactID)
	}
	if strings.TrimSpace(extConnect.FactID) != "" {
		evidenceRefs = append(evidenceRefs, extConnect.FactID)
	}
	if len(evidenceRefs) == 0 {
		evidenceRefs = append(evidenceRefs, fmt.Sprintf("event:%d", event.ID))
	}

	evidenceJSON, _ := json.Marshal(evidenceRefs)
	metadataJSON, _ := json.Marshal(map[string]interface{}{
		"detector_id": d.ID,
		"version":     d.Version,
		"window":      d.WindowLabel,
		"ordered":     true,
	})

	bucket := d.Bucket(event.CreatedAt)
	incidentID := fmt.Sprintf("%s:%s:%d", event.PodUID, d.IncidentType, bucket)

	row := models.RuntimeIncident{
		IncidentID:   incidentID,
		PodUID:       event.PodUID,
		Namespace:    event.Namespace,
		IncidentType: d.IncidentType,
		SeverityHint: d.SeverityHint,
		Confidence:   d.Confidence,
		FirstSeenAt:  event.CreatedAt,
		LastSeenAt:   event.CreatedAt,
		Window:       d.WindowLabel,
		EvidenceRefs: string(evidenceJSON),
		Metadata:     string(metadataJSON),
		CreatedAt:    event.CreatedAt,
		UpdatedAt:    event.CreatedAt,
	}
	return upsertOrUpdateLatestIncident(ctx, db, &row, d.Cooldown)
}

func upsertIncident(ctx context.Context, db *gorm.DB, row *models.RuntimeIncident) error {
	if row == nil {
		return nil
	}
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "incident_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"last_seen_at":  row.LastSeenAt,
				"confidence":    row.Confidence,
				"severity_hint": row.SeverityHint,
				"evidence_refs": row.EvidenceRefs,
				"metadata":      row.Metadata,
				"updated_at":    row.UpdatedAt,
			}),
		}).
		Create(&row).Error
}

// upsertOrUpdateLatestIncident enforces minimal suppression:
// - If there is an existing incident of the same type for the pod whose LastSeenAt is within cooldown,
//   update that latest incident instead of inserting a new one (avoids incident spam across buckets).
// - Otherwise, upsert deterministically by incident_id (pod_uid:type:bucket).
func upsertOrUpdateLatestIncident(ctx context.Context, db *gorm.DB, row *models.RuntimeIncident, cooldown time.Duration) error {
	if row == nil || db == nil {
		return nil
	}
	var latest models.RuntimeIncident
	err := db.WithContext(ctx).
		Where("pod_uid = ? AND incident_type = ?", row.PodUID, row.IncidentType).
		Order("last_seen_at DESC").
		Limit(1).
		Find(&latest).Error
	if err != nil {
		return err
	}

	if latest.ID != 0 && row.LastSeenAt.Sub(latest.LastSeenAt) < cooldown {
		// Suppress: update latest instead of creating new bucket incident.
		prevN := evidenceRefCountJSON(latest.EvidenceRefs)
		newN := evidenceRefCountJSON(row.EvidenceRefs)
		mergedConf := mergeIncidentConfidenceOnSuppress(latest.Confidence, row.Confidence, prevN, newN)
		update := map[string]interface{}{
			"last_seen_at":  row.LastSeenAt,
			"confidence":    mergedConf,
			"severity_hint": row.SeverityHint,
			"evidence_refs": row.EvidenceRefs,
			"metadata":      row.Metadata,
			"updated_at":    row.UpdatedAt,
		}
		return db.WithContext(ctx).Model(&latest).Updates(update).Error
	}

	// Not suppressed: normal deterministic upsert by incident_id.
	return upsertIncident(ctx, db, row)
}

func hasAnyFactType(facts []models.RuntimeBehaviorFact, factTypes ...string) bool {
	if len(facts) == 0 || len(factTypes) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, t := range factTypes {
		k := strings.ToUpper(strings.TrimSpace(t))
		if k != "" {
			set[k] = struct{}{}
		}
	}
	for i := range facts {
		if _, ok := set[strings.ToUpper(strings.TrimSpace(facts[i].FactType))]; ok {
			return true
		}
	}
	return false
}

func countFactTypes(facts []models.RuntimeBehaviorFact, factTypes ...string) int64 {
	if len(facts) == 0 || len(factTypes) == 0 {
		return 0
	}
	set := map[string]struct{}{}
	for _, t := range factTypes {
		k := strings.ToUpper(strings.TrimSpace(t))
		if k != "" {
			set[k] = struct{}{}
		}
	}
	var n int64
	for i := range facts {
		if _, ok := set[strings.ToUpper(strings.TrimSpace(facts[i].FactType))]; ok {
			n++
		}
	}
	return n
}
