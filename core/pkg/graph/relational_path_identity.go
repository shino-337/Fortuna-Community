package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"gorm.io/gorm"
)

// BuildPathsForPodIdentity computes paths for exactly one cluster-qualified Pod.
func (b *RelationalPathBuilder) BuildPathsForPodIdentity(ctx context.Context, id resourceidentity.Identity, persist bool) ([]AttackPath, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	var pod models.Pod
	if err := b.db.WithContext(ctx).
		Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", id.ClusterID, id.ResourceUID).
		First(&pod).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []AttackPath{}, nil
		}
		return nil, fmt.Errorf("failed to fetch pod: %w", err)
	}
	snap, err := b.loadClusterPathSnapshot(ctx, id.ClusterID)
	if err != nil {
		return nil, err
	}
	return b.buildPathsForPodIdentityWithSnapshot(ctx, id, &pod, snap, persist)
}

func (b *RelationalPathBuilder) buildPathsForPodIdentityWithSnapshot(ctx context.Context, id resourceidentity.Identity, pod *models.Pod, snap *clusterPathSnapshot, persist bool) ([]AttackPath, error) {
	if _, ok := snap.SaByNSName[pod.Namespace+"/"+pod.ServiceAccount]; !ok {
		return []AttackPath{}, nil
	}
	sa := snap.SaByNSName[pod.Namespace+"/"+pod.ServiceAccount]

	observedDestIPs := map[string]time.Time{}
	if b.db.Migrator().HasTable(&models.PodNetworkConnection{}) {
		var conns []models.PodNetworkConnection
		since := time.Now().Add(-attackPathObservedEgressWindowFromEnv())
		if err := b.db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ? AND observed_at >= ?", id.ClusterID, id.ResourceUID, since).
			Find(&conns).Error; err == nil {
			for _, c := range conns {
				ip := strings.TrimSpace(c.DestIP)
				if ip == "" {
					continue
				}
				if prev, ok := observedDestIPs[ip]; !ok || c.ObservedAt.After(prev) {
					observedDestIPs[ip] = c.ObservedAt
				}
			}
		}
	}
	reachCtx := ReachabilityContext{
		ObservedEgressIPs: observedDestIPs,
		DenyCache:         snap.DenyCache,
		Now:               time.Now(),
		FallbackTTL:       attackPathFallbackTTLFromEnv(),
		DenyTTL:           attackPathDenyTTLFromEnv(),
	}

	var podCaps []models.PodCapability
	if b.db.Migrator().HasTable("pod_capabilities") {
		if err := b.db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, id.ResourceUID).
			Find(&podCaps).Error; err != nil {
			log.Printf("[RelationalPathBuilder] warning: failed to load scoped pod_capabilities for %s/%s: %v", id.ClusterID, id.ResourceUID, err)
		}
	}

	var podAttackSteps []models.PodAttackStep
	if b.db.Migrator().HasTable("pod_attack_steps") {
		if err := b.db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, id.ResourceUID).
			Find(&podAttackSteps).Error; err != nil {
			log.Printf("[RelationalPathBuilder] warning: failed to load scoped pod_attack_steps for %s/%s: %v", id.ClusterID, id.ResourceUID, err)
		}
	}

	var paths []AttackPath
	if isServiceAccountPathFeasible(*pod, podCaps) {
		criticalCVE := b.hasCriticalCVEInsightForIdentity(ctx, id)
		highRiskSource := pod.HostNetwork || hasEscapeCapability(podCaps) || criticalCVE
		paths = buildDeterministicPaths(
			*pod, sa,
			snap.ClusterPods,
			snap.SaByNSName,
			snap.RoleBindings, snap.ClusterRoleBindings,
			snap.RoleMap, snap.CrMap,
			podCaps, podAttackSteps,
			reachCtx,
			highRiskSource,
		)
	}

	if persist {
		if err := persistAttackPathsForIdentity(ctx, b.db, id, paths); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func (b *RelationalPathBuilder) hasCriticalCVEInsightForIdentity(ctx context.Context, id resourceidentity.Identity) bool {
	if !b.db.Migrator().HasTable("insights") {
		return false
	}
	var count int64
	_ = b.db.WithContext(ctx).Model(&models.Insight{}).
		Where("cluster_id = ? AND resource_uid = ? AND deleted_at IS NULL AND status = 'active' AND lower(insight_type) = 'vulnerability' AND lower(severity) = 'critical'", id.ClusterID, id.ResourceUID).
		Count(&count).Error
	return count > 0
}

func persistAttackPathsForIdentity(ctx context.Context, db *gorm.DB, id resourceidentity.Identity, paths []AttackPath) error {
	if !db.Migrator().HasTable("attack_paths") {
		return nil
	}
	newByPathID := map[string]AttackPath{}
	for i, p := range paths {
		nodesJSON, _ := json.Marshal(p.Nodes)
		edgesJSON, _ := json.Marshal(p.Edges)
		pathID := fmt.Sprintf("%s-path-%d", id.ResourceUID, i)
		newByPathID[pathID] = p
		record := models.AttackPath{
			ClusterID:       id.ClusterID,
			PodUID:          id.ResourceUID,
			PathID:          pathID,
			Nodes:           string(nodesJSON),
			Edges:           string(edgesJSON),
			TotalRisk:       p.TotalRisk,
			Difficulty:      p.Difficulty,
			Impact:          p.Impact,
			Length:          p.Length,
			Description:     p.Description,
			EnrichedFromPCE: p.EnrichedFromPCE,
		}
		if err := db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ? AND path_id = ?", id.ClusterID, id.ResourceUID, pathID).
			Assign(record).
			FirstOrCreate(&record).Error; err != nil {
			return fmt.Errorf("upsert scoped path %s: %w", pathID, err)
		}
	}

	grace := attackPathHysteresisGraceFromEnv()
	decay := attackPathHysteresisDecayFromEnv()
	if grace <= 0 {
		return nil
	}
	var existing []models.AttackPath
	if err := db.WithContext(ctx).
		Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, id.ResourceUID).
		Find(&existing).Error; err != nil {
		return err
	}
	now := time.Now()
	for _, old := range existing {
		if _, stillPresent := newByPathID[old.PathID]; stillPresent {
			continue
		}
		age := now.Sub(old.UpdatedAt)
		if age > grace {
			if err := db.WithContext(ctx).Where("id = ?", old.ID).Delete(&models.AttackPath{}).Error; err != nil {
				return err
			}
			continue
		}
		old.TotalRisk = clampFloat(old.TotalRisk*decay, 0, 10)
		old.Impact = clampFloat(old.Impact*decay, 0, 1)
		old.Difficulty = clampFloat(1.0-(1.0-old.Difficulty)*decay, 0.1, 1.0)
		old.Description = strings.TrimSpace(old.Description + " [hysteresis]")
		if err := db.WithContext(ctx).Save(&old).Error; err != nil {
			return err
		}
	}
	return nil
}
