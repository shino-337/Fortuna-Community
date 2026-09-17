package riskengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/evidence"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/fortuna/core/pkg/risk"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func canonicalPodInsightIdentity(insight *models.Insight) (resourceidentity.Identity, error) {
	if insight == nil {
		return resourceidentity.Identity{}, fmt.Errorf("pod insight is nil")
	}
	if !strings.EqualFold(strings.TrimSpace(insight.ResourceType), "pod") {
		return resourceidentity.Identity{}, fmt.Errorf("pod insight requires resource_type=pod")
	}
	id, err := resourceidentity.New(insight.ClusterID, insight.ResourceUID)
	if err != nil {
		return resourceidentity.Identity{}, err
	}
	return id, nil
}

func normalizePodInsightForWrite(insight *models.Insight) {
	insight.CVEID = strings.TrimSpace(insight.CVEID)
	insight.ResourceType = "Pod"
	if insight.Status == "" {
		insight.Status = "active"
	}
	if insight.DetectedAt.IsZero() {
		insight.DetectedAt = time.Now().UTC()
	}
	if strings.TrimSpace(insight.Evidence) == "" {
		insight.Evidence = "{}"
	} else {
		insight.Evidence = evidence.MaskSensitiveInJSON(insight.Evidence)
	}
	if strings.TrimSpace(insight.ViolatedRules) == "" {
		insight.ViolatedRules = "[]"
	} else {
		insight.ViolatedRules = evidence.MaskSensitiveInJSON(insight.ViolatedRules)
	}
	if strings.TrimSpace(insight.Remediation) == "" || !json.Valid([]byte(insight.Remediation)) {
		insight.Remediation = "{}"
	}
}

func scopedPodInsightQuery(tx *gorm.DB, id resourceidentity.Identity, insightType, cveID string) *gorm.DB {
	return tx.Where(
		"cluster_id = ? AND resource_uid = ? AND insight_type = ? AND cve_id = ? AND deleted_at IS NULL",
		id.ClusterID, id.ResourceUID, insightType, cveID,
	)
}

func mergePodInsight(existing, incoming *models.Insight, reactivate bool) {
	existing.ClusterID = incoming.ClusterID
	existing.ResourceType = "Pod"
	existing.ResourceNamespace = incoming.ResourceNamespace
	existing.ResourceName = incoming.ResourceName
	existing.ResourceUID = incoming.ResourceUID
	existing.InsightType = incoming.InsightType
	existing.Severity = incoming.Severity
	existing.Title = incoming.Title
	existing.Description = incoming.Description
	existing.Recommendation = incoming.Recommendation
	existing.CVEID = incoming.CVEID
	existing.AffectedComponent = incoming.AffectedComponent
	existing.AffectedVersion = incoming.AffectedVersion
	existing.FixedVersion = incoming.FixedVersion
	existing.CVSS = incoming.CVSS
	existing.Evidence = incoming.Evidence
	existing.ViolatedRules = incoming.ViolatedRules
	existing.RiskExplanation = incoming.RiskExplanation
	existing.Remediation = incoming.Remediation
	existing.MatchConfidence = incoming.MatchConfidence
	existing.ComponentConfidence = incoming.ComponentConfidence
	existing.SBOMConfidence = incoming.SBOMConfidence
	existing.FinalRiskConfidence = incoming.FinalRiskConfidence
	existing.Degraded = incoming.Degraded
	existing.Sensitivity = incoming.Sensitivity
	if reactivate {
		existing.Status = "active"
		existing.ResolvedAt = nil
		existing.DetectedAt = incoming.DetectedAt
	}
	existing.UpdatedAt = time.Now().UTC()
}

// createOrUpdatePodInsightTx owns Pod finding deduplication. The canonical key
// is {cluster_id,resource_uid,cve_id,insight_type}; a Pod UID is never used as a
// globally unique storage identity.
func (m *InsightManager) createOrUpdatePodInsightTx(tx *gorm.DB, insight *models.Insight) (resourceidentity.Identity, error) {
	id, err := canonicalPodInsightIdentity(insight)
	if err != nil {
		return resourceidentity.Identity{}, err
	}
	normalizePodInsightForWrite(insight)

	var existing models.Insight
	q := scopedPodInsightQuery(tx, id, insight.InsightType, insight.CVEID)
	find := q.Limit(1).Find(&existing)
	if find.Error != nil {
		return resourceidentity.Identity{}, find.Error
	}
	if find.RowsAffected > 0 {
		if existing.Status == "dismissed" && isExempted(tx, id.ClusterID, id.ResourceUID, insight.CVEID, insight.InsightType) {
			return id, nil
		}
		reactivate := existing.Status == "resolved" || existing.Status == "dismissed"
		mergePodInsight(&existing, insight, reactivate)
		if !reactivate && strings.TrimSpace(existing.Status) == "" {
			existing.Status = "active"
		}
		if err := tx.Save(&existing).Error; err != nil {
			return resourceidentity.Identity{}, fmt.Errorf("update scoped pod insight: %w", err)
		}
		insight.ID = existing.ID
		return id, nil
	}

	// DoNothing makes concurrent insert races safe on PostgreSQL without putting
	// the transaction into the aborted state. We then load the canonical row and
	// merge if another writer won the race.
	result := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "cluster_id"},
			{Name: "resource_uid"},
			{Name: "cve_id"},
			{Name: "insight_type"},
		},
		DoNothing: true,
	}).Create(insight)
	if result.Error != nil {
		return resourceidentity.Identity{}, fmt.Errorf("create scoped pod insight: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return id, nil
	}

	if err := scopedPodInsightQuery(tx, id, insight.InsightType, insight.CVEID).Limit(1).First(&existing).Error; err != nil {
		return resourceidentity.Identity{}, fmt.Errorf("load concurrent scoped pod insight: %w", err)
	}
	if existing.Status == "dismissed" && isExempted(tx, id.ClusterID, id.ResourceUID, insight.CVEID, insight.InsightType) {
		return id, nil
	}
	reactivate := existing.Status == "resolved" || existing.Status == "dismissed"
	mergePodInsight(&existing, insight, reactivate)
	if err := tx.Save(&existing).Error; err != nil {
		return resourceidentity.Identity{}, fmt.Errorf("merge concurrent scoped pod insight: %w", err)
	}
	insight.ID = existing.ID
	return id, nil
}

func (m *InsightManager) runRiskScoreCalculationForIdentity(ctx context.Context, id resourceidentity.Identity) {
	if m == nil || m.db == nil || id.Validate() != nil {
		return
	}
	scorer := risk.NewUnifiedScorerV3(m.db)
	score, err := scorer.CalculateScoreV3ForIdentity(ctx, id)
	if err != nil {
		return
	}
	_ = scorer.SaveScoreV3ForIdentity(ctx, id, score)
}

// CreateOrUpdatePodInsight is the production write path for Pod findings.
func (m *InsightManager) CreateOrUpdatePodInsight(insight *models.Insight) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("insight manager database is nil")
	}
	var id resourceidentity.Identity
	err := m.db.Transaction(func(tx *gorm.DB) error {
		var err error
		id, err = m.createOrUpdatePodInsightTx(tx, insight)
		return err
	})
	if err != nil {
		return err
	}
	go m.runRiskScoreCalculationForIdentity(context.Background(), id)
	return nil
}

// BatchCreateOrUpdatePodInsights stores a mixed batch only when every finding
// is a cluster-qualified Pod insight. It schedules one V3 calculation per
// canonical Pod identity after commit.
func (m *InsightManager) BatchCreateOrUpdatePodInsights(insights []*models.Insight) error {
	if len(insights) == 0 {
		return nil
	}
	if m == nil || m.db == nil {
		return fmt.Errorf("insight manager database is nil")
	}
	ids := map[string]resourceidentity.Identity{}
	err := m.db.Transaction(func(tx *gorm.DB) error {
		for _, insight := range insights {
			id, err := m.createOrUpdatePodInsightTx(tx, insight)
			if err != nil {
				return err
			}
			key, err := id.Key()
			if err != nil {
				return err
			}
			ids[key] = id
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, id := range ids {
		identity := id
		go m.runRiskScoreCalculationForIdentity(context.Background(), identity)
	}
	return nil
}
