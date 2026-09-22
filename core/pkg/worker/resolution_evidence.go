package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"gorm.io/gorm/clause"
)

const resolutionSnapshotMaxAge = 10 * time.Minute

// A snapshot update is not a collector coverage receipt. D1 permits only
// self-contained static Role checks; runtime/cross-resource absence stays unknown.
func (u *InsightStatusUpdater) requireResolutionEvidence(ctx context.Context, f *models.Insight) error {
	if _, err := resourceidentity.New(f.ClusterID, f.ResourceUID); err != nil {
		return fmt.Errorf("resolution evidence: unknown resource ownership: %w", err)
	}
	if u.yamlEngine == nil || !u.yamlEngine.CanResolveFromRoleSnapshot(f) {
		return fmt.Errorf("resolution evidence: complete runtime/cross-resource collection coverage is unavailable")
	}
	var snapshot struct {
		UpdatedAt time.Time
		Rules     string
	}
	table := "roles"
	if f.ResourceType == "ClusterRole" {
		table = "cluster_roles"
	}
	result := u.db.WithContext(ctx).Table(table).Select("updated_at, rules").Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", f.ClusterID, f.ResourceUID).Take(&snapshot)
	if result.Error != nil {
		return fmt.Errorf("resolution evidence: exact resource snapshot unavailable: %w", result.Error)
	}
	var count int64
	if err := u.db.WithContext(ctx).Table(table).Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", f.ClusterID, f.ResourceUID).Count(&count).Error; err != nil {
		return fmt.Errorf("resolution evidence: ownership query failed: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("resolution evidence: ambiguous resource snapshot")
	}
	now := time.Now()
	if snapshot.UpdatedAt.IsZero() || snapshot.UpdatedAt.After(now) || now.Sub(snapshot.UpdatedAt) > resolutionSnapshotMaxAge {
		return fmt.Errorf("resolution evidence: static snapshot stale or timestamp invalid")
	}
	if f.DetectedAt.IsZero() || snapshot.UpdatedAt.Before(f.DetectedAt) {
		return fmt.Errorf("resolution evidence: snapshot predates finding")
	}
	var rules []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(snapshot.Rules), &rules); err != nil || rules == nil {
		return fmt.Errorf("resolution evidence: incomplete or malformed role rules")
	}
	for _, r := range rules {
		var verbs []string
		if err := json.Unmarshal(r["verbs"], &verbs); err != nil || len(verbs) == 0 {
			return fmt.Errorf("resolution evidence: incomplete role verbs")
		}
		for _, verb := range verbs {
			if verb == "" {
				return fmt.Errorf("resolution evidence: empty role verb")
			}
		}
		for _, key := range []string{"resources", "apiGroups", "resourceNames", "nonResourceURLs"} {
			if raw, ok := r[key]; ok {
				var values []string
				if json.Unmarshal(raw, &values) != nil || values == nil {
					return fmt.Errorf("resolution evidence: malformed role %s", key)
				}
			}
		}
		target := "resources"
		if _, res := r[target]; !res {
			target = "nonResourceURLs"
		}
		var targets []string
		if json.Unmarshal(r[target], &targets) != nil || len(targets) == 0 {
			return fmt.Errorf("resolution evidence: incomplete role targets")
		}
		for _, v := range targets {
			if v == "" {
				return fmt.Errorf("resolution evidence: empty role target")
			}
		}
		if target == "resources" {
			var groups []string
			if json.Unmarshal(r["apiGroups"], &groups) != nil || len(groups) == 0 {
				return fmt.Errorf("resolution evidence: incomplete API groups")
			}
		}
	}
	return nil
}

// Lock the exact snapshot at commit and compare it with the version preceding
// evaluation. A concurrent sync must not be resolved using an older evaluation.
func (u *InsightStatusUpdater) resolutionSnapshotVersion(ctx context.Context, f *models.Insight, lock bool) (time.Time, error) {
	if f.ResourceType != "Role" && f.ResourceType != "ClusterRole" {
		return time.Time{}, nil
	}
	table := "roles"
	if f.ResourceType == "ClusterRole" {
		table = "cluster_roles"
	}
	var row struct{ UpdatedAt time.Time }
	q := u.db.WithContext(ctx).Table(table).Select("updated_at").Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", f.ClusterID, f.ResourceUID)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.Take(&row).Error
	return row.UpdatedAt, err
}
