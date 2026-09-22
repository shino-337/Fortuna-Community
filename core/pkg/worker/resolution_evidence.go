package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/pkg/inventoryevidence"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"gorm.io/gorm/clause"
)

const resolutionSnapshotMaxAge = collection.MaxAge

// Static Role resolution requires a fresh authenticated inventory observation
// whose digest matches the current snapshot. Runtime coverage remains unknown.
func (u *InsightStatusUpdater) requireResolutionEvidence(ctx context.Context, f *models.Insight) (time.Time, error) {
	if _, err := resourceidentity.New(f.ClusterID, f.ResourceUID); err != nil {
		return time.Time{}, fmt.Errorf("resolution evidence: unknown resource ownership: %w", err)
	}
	if u.yamlEngine == nil || !u.yamlEngine.CanResolveFromRoleSnapshot(f) {
		return time.Time{}, fmt.Errorf("resolution evidence: complete runtime/cross-resource collection coverage is unavailable")
	}
	var snapshot struct {
		UpdatedAt time.Time
		Rules     string
		Name      string
		Namespace string
	}
	table := "roles"
	kind := "roles"
	if f.ResourceType == "ClusterRole" {
		table = "cluster_roles"
		kind = "clusterRoles"
	}
	columns := "updated_at, rules, name, namespace"
	if f.ResourceType == "ClusterRole" {
		columns = "updated_at, rules, name"
	}
	result := u.db.WithContext(ctx).Table(table).Select(columns).Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", f.ClusterID, f.ResourceUID).Take(&snapshot)
	if result.Error != nil {
		return time.Time{}, fmt.Errorf("resolution evidence: exact resource snapshot unavailable: %w", result.Error)
	}
	var count int64
	if err := u.db.WithContext(ctx).Table(table).Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", f.ClusterID, f.ResourceUID).Count(&count).Error; err != nil {
		return time.Time{}, fmt.Errorf("resolution evidence: ownership query failed: %w", err)
	}
	if count != 1 {
		return time.Time{}, fmt.Errorf("resolution evidence: ambiguous resource snapshot")
	}
	now := time.Now()
	var receipt models.InventoryCollection
	// Lock receipt before the resource snapshot, matching inventory transaction order.
	if err := u.db.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("cluster_id = ?", f.ClusterID).First(&receipt).Error; err != nil {
		return time.Time{}, fmt.Errorf("resolution evidence: inventory receipt unavailable: %w", err)
	}
	if receipt.AgentID == "" || receipt.EffectiveStatus(now) != "complete" {
		return time.Time{}, fmt.Errorf("resolution evidence: inventory collection is incomplete, unverified or stale")
	}
	var kindStartedAt map[string]time.Time
	if err := json.Unmarshal([]byte(receipt.KindStartedAt), &kindStartedAt); err != nil {
		return time.Time{}, fmt.Errorf("resolution evidence: per-kind observation metadata unavailable")
	}
	resourceObservedAt, ok := kindStartedAt[kind]
	if !ok || resourceObservedAt.IsZero() || resourceObservedAt.Before(receipt.StartedAt) || resourceObservedAt.After(receipt.ObservedAt) ||
		resourceObservedAt.After(now) || now.Sub(resourceObservedAt) > resolutionSnapshotMaxAge {
		return time.Time{}, fmt.Errorf("resolution evidence: resource observation is missing, invalid or stale")
	}
	if f.DetectedAt.IsZero() || resourceObservedAt.Before(f.DetectedAt) {
		return time.Time{}, fmt.Errorf("resolution evidence: resource observation predates finding")
	}
	if f.ResourceType == "Role" && receipt.Namespace != "" && receipt.Namespace != snapshot.Namespace {
		return time.Time{}, fmt.Errorf("resolution evidence: collection namespace mismatch")
	}
	digest, err := inventoryevidence.RoleDigest(f.ClusterID, f.ResourceType, f.ResourceUID, snapshot.Name, snapshot.Namespace, snapshot.Rules)
	if err != nil {
		return time.Time{}, fmt.Errorf("resolution evidence: malformed static snapshot: %w", err)
	}
	var hashes map[string]string
	if json.Unmarshal([]byte(receipt.RoleDigests), &hashes) != nil || hashes[inventoryevidence.Key(f.ResourceType, f.ResourceUID)] != digest {
		return time.Time{}, fmt.Errorf("resolution evidence: current snapshot was not in the verified collection")
	}

	var rules []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(snapshot.Rules), &rules); err != nil || rules == nil {
		return time.Time{}, fmt.Errorf("resolution evidence: incomplete or malformed role rules")
	}
	for _, r := range rules {
		var verbs []string
		if err := json.Unmarshal(r["verbs"], &verbs); err != nil || len(verbs) == 0 {
			return time.Time{}, fmt.Errorf("resolution evidence: incomplete role verbs")
		}
		for _, verb := range verbs {
			if verb == "" {
				return time.Time{}, fmt.Errorf("resolution evidence: empty role verb")
			}
		}
		for _, key := range []string{"resources", "apiGroups", "resourceNames", "nonResourceURLs"} {
			if raw, ok := r[key]; ok {
				var values []string
				if json.Unmarshal(raw, &values) != nil || values == nil {
					return time.Time{}, fmt.Errorf("resolution evidence: malformed role %s", key)
				}
			}
		}
		target := "resources"
		if _, res := r[target]; !res {
			target = "nonResourceURLs"
		}
		var targets []string
		if json.Unmarshal(r[target], &targets) != nil || len(targets) == 0 {
			return time.Time{}, fmt.Errorf("resolution evidence: incomplete role targets")
		}
		for _, v := range targets {
			if v == "" {
				return time.Time{}, fmt.Errorf("resolution evidence: empty role target")
			}
		}
		if target == "resources" {
			var groups []string
			if json.Unmarshal(r["apiGroups"], &groups) != nil || len(groups) == 0 {
				return time.Time{}, fmt.Errorf("resolution evidence: incomplete API groups")
			}
		}
	}
	return resourceObservedAt, nil
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
