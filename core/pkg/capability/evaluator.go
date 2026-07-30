package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/rbac"
	"github.com/fortuna/core/pkg/riskengine"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Capability struct {
	ID       string
	Group    string
	Severity string
	Evidence map[string]interface{}
	Mitre    []string
}

// EvaluateAndUpsertPod evaluates capabilities for a single pod and persists results.
// expectedSpecHash: when non-empty, used for race protection — only persist if pod.spec_hash still matches;
// after success, set last_evaluated_hash so risk reflects the evaluated spec. Pass "" for legacy (e.g. EvaluateAllPods).
func EvaluateAndUpsertPod(ctx context.Context, db *gorm.DB, pod *models.Pod, expectedSpecHash string) error {
	// Race protection: if caller passed expectedSpecHash, pod must still match (another sync may have updated spec)
	if expectedSpecHash != "" && pod.SpecHash != expectedSpecHash {
		log.Printf("[PCE] Skip stale evaluation for pod %s/%s (spec_hash changed)", pod.Namespace, pod.Name)
		return nil
	}

	// Check if pod instance is active (if pod_instances table exists)
	var isActive bool
	if db.Migrator().HasTable(&models.PodInstance{}) {
		var count int64
		db.Model(&models.PodInstance{}).
			Where("pod_uid = ? AND status = ?", pod.UID, "active").
			Count(&count)
		isActive = count > 0
	} else {
		// Fallback: check if pod is not deleted
		isActive = pod.DeletedAt.Time.IsZero()
	}

	if !isActive {
		log.Printf("[PCE] Skipping evaluation for inactive pod %s/%s (UID: %s)", pod.Namespace, pod.Name, pod.UID)
		return nil
	}

	caps, err := EvaluatePod(ctx, db, pod)
	if err != nil {
		return err
	}

	// Before writing: re-read pod so we don't overwrite with stale result if spec changed meanwhile
	if expectedSpecHash != "" {
		var current models.Pod
		if err := db.WithContext(ctx).Where("cluster_id = ? AND uid = ?", pod.ClusterID, pod.UID).First(&current).Error; err != nil {
			return err
		}
		if current.SpecHash != expectedSpecHash {
			log.Printf("[PCE] Discard result for pod %s/%s (spec_hash changed before write)", pod.Namespace, pod.Name)
			return nil
		}
		pod = &current
	}

	if err := syncPodCapabilities(ctx, db, pod, caps); err != nil {
		return err
	}
	if err := upsertPodRiskProfile(db, *pod, caps); err != nil {
		return err
	}
	if err := syncCapabilityInsights(ctx, db, pod, caps); err != nil {
		return err
	}

	// Phase 2.2 + Layer 3 orchestration:
	// Centralized orchestrator now rebuilds Layer 3 and refreshes V3 score.
	ScheduleAttackPathRebuild(db, pod.UID)

	// Only set last_evaluated_hash when spec_hash still matches (atomic; avoids overwriting after newer sync)
	if expectedSpecHash != "" {
		db.Model(&models.Pod{}).Where("cluster_id = ? AND uid = ? AND spec_hash = ?", pod.ClusterID, pod.UID, expectedSpecHash).
			Update("last_evaluated_hash", expectedSpecHash)
	}
	return nil
}

// EvaluateAllPods evaluates capabilities for all active pods and upserts results.
// Only processes pods with active pod_instances (if table exists).
func EvaluateAllPods(ctx context.Context, db *gorm.DB) error {
	var pods []models.Pod

	// Filter by active pod instances if table exists
	if db.Migrator().HasTable(&models.PodInstance{}) {
		if err := db.Table("pods").
			Joins("INNER JOIN pod_instances ON pods.uid = pod_instances.pod_uid").
			Where("pods.deleted_at IS NULL AND pod_instances.status = ?", "active").
			Find(&pods).Error; err != nil {
			return err
		}
	} else {
		// Fallback: use deleted_at check
		if err := db.Where("deleted_at IS NULL").Find(&pods).Error; err != nil {
			return err
		}
	}

	log.Printf("[PCE] Evaluating %d active pods", len(pods))

	for _, pod := range pods {
		if err := EvaluateAndUpsertPod(ctx, db, &pod, ""); err != nil {
			log.Printf("[PCE] Failed to evaluate/upsert pod %s/%s: %v", pod.Namespace, pod.Name, err)
			continue
		}
	}
	return nil
}

func syncPodCapabilities(ctx context.Context, db *gorm.DB, pod *models.Pod, caps []Capability) error {
	csc := NewCapabilityStateController(db)
	capIDs := make([]string, 0, len(caps))
	for _, c := range caps {
		capIDs = append(capIDs, c.ID)
		if err := csc.InitializeCapability(ctx, pod.UID, pod.Namespace, c.ID, c.Group, c.Severity, c.Evidence); err != nil {
			log.Printf("[PCE] Failed to initialize capability %s for pod %s/%s: %v", c.ID, pod.Namespace, pod.Name, err)
		}
	}

	staleQuery := db.WithContext(ctx).Where("pod_uid = ?", pod.UID)
	if len(capIDs) > 0 {
		staleQuery = staleQuery.Where("capability_id NOT IN ?", capIDs)
	}
	if err := staleQuery.Delete(&models.PodCapability{}).Error; err != nil {
		return fmt.Errorf("delete stale pod_capabilities for pod %s: %w", pod.UID, err)
	}
	return nil
}

func syncCapabilityInsights(ctx context.Context, db *gorm.DB, pod *models.Pod, caps []Capability) error {
	insightMgr := riskengine.NewInsightManager(db)

	capabilityIDs := make([]string, 0, len(caps))
	for _, c := range caps {
		capabilityIDs = append(capabilityIDs, c.ID)
	}

	metadataByID := map[string]models.CapabilityMetadata{}
	if len(capabilityIDs) > 0 {
		var metadataRows []models.CapabilityMetadata
		if err := db.WithContext(ctx).Where("capability_id IN ?", capabilityIDs).Find(&metadataRows).Error; err != nil {
			return fmt.Errorf("load capability metadata: %w", err)
		}
		for _, md := range metadataRows {
			metadataByID[md.CapabilityID] = md
		}
	}

	activeTitles := make([]string, 0, len(caps))
	for _, c := range caps {
		md, hasMetadata := metadataByID[c.ID]
		title := fmt.Sprintf("Capability %s detected", c.ID)
		if hasMetadata && strings.TrimSpace(md.Name) != "" {
			title = fmt.Sprintf("%s detected", strings.TrimSpace(md.Name))
		}
		activeTitles = append(activeTitles, title)

		recommendation := "Review pod security context and harden configuration to remove unnecessary privileges."
		if hasMetadata && len(md.RecommendedMitigations) > 0 && strings.TrimSpace(md.RecommendedMitigations[0]) != "" {
			recommendation = strings.TrimSpace(md.RecommendedMitigations[0])
		}

		description := fmt.Sprintf("Capability %s is detected on pod %s/%s.", c.ID, pod.Namespace, pod.Name)
		if hasMetadata && strings.TrimSpace(md.Description) != "" {
			description = fmt.Sprintf("%s Detected on pod %s/%s.", strings.TrimSpace(md.Description), pod.Namespace, pod.Name)
		}
		evidenceJSON, _ := json.Marshal(c.Evidence)

		// PCE-8: Evaluate false_positive_considerations from capability metadata.
		// When the pod context matches a known FP pattern (e.g., infrastructure
		// components in kube-system), lower the confidence to indicate that the
		// finding may be expected rather than adversarial.
		matchConfidence := "HIGH"
		if hasMetadata && len(md.FalsePositiveConsiderations) > 0 {
			if fpMatch := matchFalsePositiveConsiderations(pod, md.FalsePositiveConsiderations); fpMatch != "" {
				matchConfidence = "LOW"
				description += fmt.Sprintf(" Note: %s", fpMatch)
			}
		}

		insight := &models.Insight{
			ResourceType:      "Pod",
			ResourceNamespace: pod.Namespace,
			ResourceName:      pod.Name,
			ResourceUID:       pod.UID,
			InsightType:       "capability",
			Severity:          strings.ToLower(strings.TrimSpace(c.Severity)),
			Title:             title,
			Description:       description,
			Recommendation:    recommendation,
			AffectedComponent: c.ID,
			Evidence:          string(evidenceJSON),
			ViolatedRules:     "[]",
			Status:            "active",
			DetectedAt:        time.Now(),
			// PCE-8: Set confidence based on FP consideration match.
			MatchConfidence:     matchConfidence,
			FinalRiskConfidence: matchConfidence,
			// CVEID used as logical key for DB unique constraint (resource_uid, cve_id, insight_type)
			// so multiple capability insights per pod are allowed (one per capability ID).
			CVEID: c.ID,
		}
		if err := insightMgr.CreateOrUpdateInsight(insight); err != nil {
			return fmt.Errorf("create/update capability insight for %s on pod %s: %w", c.ID, pod.UID, err)
		}
	}

	resolveQuery := db.WithContext(ctx).
		Model(&models.Insight{}).
		Where("resource_uid = ? AND insight_type = ? AND deleted_at IS NULL AND (status = ? OR status IS NULL)",
			pod.UID, "capability", "active")
	if len(activeTitles) > 0 {
		resolveQuery = resolveQuery.Where("title NOT IN ?", activeTitles)
	}
	if err := resolveQuery.Updates(map[string]interface{}{
		"status":     "resolved",
		"updated_at": time.Now(),
	}).Error; err != nil {
		return fmt.Errorf("resolve stale capability insights for pod %s: %w", pod.UID, err)
	}
	return nil
}

// EvaluatePod evaluates pod capability rules.
func EvaluatePod(ctx context.Context, db *gorm.DB, pod *models.Pod) ([]Capability, error) {
	caps := make([]Capability, 0)

	automount := true
	if pod.AutomountServiceAccountToken != nil {
		automount = *pod.AutomountServiceAccountToken
	}

	// CTRL_CONTROL_PLANE_POD (standardized ID - keep as is)
	if pod.Namespace == "kube-system" {
		caps = append(caps, Capability{
			ID:       CTRL_CONTROL_PLANE_POD,
			Group:    "CTRL",
			Severity: "MEDIUM",
			Evidence: map[string]interface{}{"namespace": pod.Namespace},
			Mitre:    []string{"T1496"},
		})
	}

	// NET_HOSTNETWORK (standardized ID)
	if pod.HostNetwork {
		caps = append(caps, Capability{
			ID:       NET_HOSTNETWORK,
			Group:    "NET",
			Severity: "MEDIUM",
			Evidence: map[string]interface{}{"hostNetwork": true},
			Mitre:    []string{"T1046", "T1595"},
		})
	}

	// ESC_HOSTPID_POD (standardized ID - split from ESC_KERNEL)
	if pod.HostPID {
		caps = append(caps, Capability{
			ID:       ESC_HOSTPID_POD,
			Group:    "ESC",
			Severity: "HIGH",
			Evidence: map[string]interface{}{"hostPID": true},
			Mitre:    []string{"T1611", "T1068"},
		})
	}

	// ESC_HOSTIPC_POD (standardized ID - split from ESC_KERNEL)
	if pod.HostIPC {
		caps = append(caps, Capability{
			ID:       ESC_HOSTIPC_POD,
			Group:    "ESC",
			Severity: "HIGH",
			Evidence: map[string]interface{}{"hostIPC": true},
			Mitre:    []string{"T1611", "T1068"},
		})
	}

	// ESC_PRIV_POD (standardized ID)
	if hasPrivilegedContainer(pod.ContainerSecurityContexts) {
		caps = append(caps, Capability{
			ID:       ESC_PRIV_POD,
			Group:    "ESC",
			Severity: "CRITICAL",
			Evidence: map[string]interface{}{"securityContext.privileged": true},
			Mitre:    []string{"T1611"},
		})
	}

	// ESC_HOSTPATH_NODE (standardized ID)
	if hasHostPathMount(pod.Volumes, pod.VolumeMounts) {
		caps = append(caps, Capability{
			ID:       ESC_HOSTPATH_NODE,
			Group:    "ESC",
			Severity: "CRITICAL",
			Evidence: map[string]interface{}{"hostPath": true},
			Mitre:    []string{"T1611"},
		})
	}

	// ESC_RUNTIME_PROBE (standardized ID - host artifact access: /proc, /sys, /run)
	if sensitive, evidence := hasSensitiveHostPathMount(pod.Volumes, pod.VolumeMounts); pod.HostPID || sensitive {
		evidence["hostPID"] = pod.HostPID
		evidence["hostIPC"] = pod.HostIPC
		caps = append(caps, Capability{
			ID:       ESC_RUNTIME_PROBE,
			Group:    "ESC",
			Severity: "HIGH",
			Evidence: evidence,
			Mitre:    []string{"T1611", "T1068"},
		})
	}

	// ID_TOKEN_POD (standardized ID)
	if automount {
		caps = append(caps, Capability{
			ID:       ID_TOKEN_POD,
			Group:    "ID",
			Severity: "MEDIUM",
			Evidence: map[string]interface{}{"automountServiceAccountToken": true},
			Mitre:    []string{"T1528"},
		})
	}

	// API_RBAC_WRITE_CLUSTER (standardized ID)
	// Use the shared RBAC analyzer for consistent classification across PCE,
	// Attack Path Builder, and Risk Engine.
	rbacAnalysis, err := rbac.AnalyzePod(ctx, db, pod)
	if err != nil {
		return caps, err
	}
	if rbacAnalysis.HasWrite {
		caps = append(caps, Capability{
			ID:       API_RBAC_WRITE_CLUSTER,
			Group:    "API",
			Severity: "HIGH",
			Evidence: rbacAnalysis.Evidence,
			Mitre:    []string{"T1609"},
		})
	}

	return caps, nil
}

func upsertCapabilities(db *gorm.DB, pod models.Pod, caps []Capability) error {
	if len(caps) == 0 {
		return nil
	}
	now := time.Now()
	records := make([]models.PodCapability, 0, len(caps))
	for _, c := range caps {
		evidenceJSON, _ := json.Marshal(c.Evidence)
		records = append(records, models.PodCapability{
			PodUID:          pod.UID,
			Namespace:       pod.Namespace,
			CapabilityID:    c.ID,
			CapabilityGroup: c.Group,
			Severity:        c.Severity,
			CapabilityClass: "effective",
			DerivedFrom:     `{}`,
			Evidence:        string(evidenceJSON),
			MitreTechniques: c.Mitre,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pod_uid"}, {Name: "capability_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"capability_group", "severity", "evidence", "mitre", "updated_at"}),
	}).Create(&records).Error
}

func upsertPodRiskProfile(db *gorm.DB, pod models.Pod, caps []Capability) error {
	staticRisk := ComputeStaticRisk(&pod)
	capIDs := make([]string, 0, len(caps))
	for _, c := range caps {
		capIDs = append(capIDs, c.ID)
	}
	now := time.Now()
	profile := models.PodRiskProfile{
		PodUID:       pod.UID,
		Namespace:    pod.Namespace,
		StaticRisk:   staticRisk,
		RuntimeScore: 0,
		Capabilities: pq.StringArray(capIDs),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pod_uid"}},
		DoUpdates: clause.AssignmentColumns([]string{"namespace", "static_risk", "capabilities", "updated_at"}),
	}).Create(&profile).Error
}

// ComputeStaticRisk derives a static risk score from pod spec signals.
func ComputeStaticRisk(pod *models.Pod) int {
	risk := 0
	if pod.HostPID || pod.HostIPC {
		risk += 40
	}
	if sensitive, _ := hasSensitiveHostPathMount(pod.Volumes, pod.VolumeMounts); sensitive {
		risk += 40
	}
	if risk > 50 {
		risk = 50
	}
	return risk
}

func hasPrivilegedContainer(containerSecurityContexts string) bool {
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
		if priv, ok := cm["privileged"].(bool); ok && priv {
			return true
		}
	}
	return false
}

func hasHostPathMount(volumesJSON, volumeMountsJSON string) bool {
	var volumes []map[string]interface{}
	if err := json.Unmarshal([]byte(volumesJSON), &volumes); err != nil {
		return false
	}
	var mounts []map[string]interface{}
	if err := json.Unmarshal([]byte(volumeMountsJSON), &mounts); err != nil {
		return false
	}

	hostPathVolumes := map[string]string{}
	for _, v := range volumes {
		name, _ := v["name"].(string)
		hostPath, ok := v["hostPath"].(map[string]interface{})
		if !ok || name == "" {
			continue
		}
		path, _ := hostPath["path"].(string)
		if path != "" {
			hostPathVolumes[name] = path
		}
	}

	for _, m := range mounts {
		name, _ := m["name"].(string)
		mountPath, _ := m["mountPath"].(string)
		if name == "" || mountPath == "" {
			continue
		}
		if hostPath, ok := hostPathVolumes[name]; ok {
			if mountPath == "/" || strings.HasPrefix(hostPath, "/") {
				return true
			}
		}
	}
	return false
}

func hasSensitiveHostPathMount(volumesJSON, volumeMountsJSON string) (bool, map[string]interface{}) {
	evidence := map[string]interface{}{
		"hostPathMatches": []map[string]interface{}{},
	}
	var volumes []map[string]interface{}
	if err := json.Unmarshal([]byte(volumesJSON), &volumes); err != nil {
		return false, evidence
	}
	var mounts []map[string]interface{}
	if err := json.Unmarshal([]byte(volumeMountsJSON), &mounts); err != nil {
		return false, evidence
	}

	hostPathVolumes := map[string]string{}
	for _, v := range volumes {
		name, _ := v["name"].(string)
		hostPath, ok := v["hostPath"].(map[string]interface{})
		if !ok || name == "" {
			continue
		}
		path, _ := hostPath["path"].(string)
		if path != "" {
			hostPathVolumes[name] = path
		}
	}

	sensitivePrefixes := []string{"/proc", "/sys", "/run", "/var/run", "/dev"}
	matches := make([]map[string]interface{}, 0)
	for _, m := range mounts {
		name, _ := m["name"].(string)
		mountPath, _ := m["mountPath"].(string)
		readOnly, _ := m["readOnly"].(bool)
		if name == "" {
			continue
		}
		hostPath, ok := hostPathVolumes[name]
		if !ok {
			continue
		}
		for _, prefix := range sensitivePrefixes {
			if strings.HasPrefix(hostPath, prefix) {
				matches = append(matches, map[string]interface{}{
					"volume":    name,
					"hostPath":  hostPath,
					"mountPath": mountPath,
					"readOnly":  readOnly,
				})
				break
			}
		}
	}
	if len(matches) == 0 {
		return false, evidence
	}
	evidence["hostPathMatches"] = matches
	return true, evidence
}

// isTableMissingError returns true if err indicates the table does not exist (e.g. SQLite "no such table", PostgreSQL "does not exist").
// Used so PCE evaluation does not fail in test or minimal DBs that omit role_bindings / cluster_role_bindings.
func isTableMissingError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no such table") || strings.Contains(s, "does not exist")
}

// matchFalsePositiveConsiderations checks whether the pod context matches any
// of the false-positive consideration strings seeded in capability_metadata.
// PCE-8: When a match is found, the returned string describes the matching
// consideration so it can be included in the insight description and the
// confidence can be lowered.
//
// Pattern matching heuristics:
//   - "infrastructure components" / "cni" / "csi" / "dns" → matches pods in kube-system / kube-node-lease
//   - "monitoring" / "prometheus" / "logging" / "fluentd" → matches pods in monitoring / observability namespaces
//   - "init container" → matches if pod name contains "init" pattern
func matchFalsePositiveConsiderations(pod *models.Pod, fpConsiderations []string) string {
	ns := strings.ToLower(strings.TrimSpace(pod.Namespace))
	name := strings.ToLower(strings.TrimSpace(pod.Name))

	for _, fp := range fpConsiderations {
		fpLower := strings.ToLower(strings.TrimSpace(fp))
		if fpLower == "" {
			continue
		}

		// Infrastructure pattern: CNI/CSI/DNS/system components commonly run in kube-system
		if (strings.Contains(fpLower, "infrastructure") ||
			strings.Contains(fpLower, "cni") ||
			strings.Contains(fpLower, "csi") ||
			strings.Contains(fpLower, "dns") ||
			strings.Contains(fpLower, "system component")) &&
			(ns == "kube-system" || ns == "kube-node-lease" || ns == "kube-public") {
			return fp
		}

		// Monitoring/observability pattern
		if (strings.Contains(fpLower, "monitoring") ||
			strings.Contains(fpLower, "prometheus") ||
			strings.Contains(fpLower, "logging") ||
			strings.Contains(fpLower, "fluentd") ||
			strings.Contains(fpLower, "fluentbit") ||
			strings.Contains(fpLower, "datadog")) &&
			(ns == "monitoring" || ns == "observability" || ns == "kube-system" ||
				strings.Contains(name, "prometheus") ||
				strings.Contains(name, "fluent") ||
				strings.Contains(name, "datadog")) {
			return fp
		}

		// Init container pattern
		if strings.Contains(fpLower, "init container") &&
			strings.Contains(name, "init") {
			return fp
		}

		// Storage driver pattern
		if (strings.Contains(fpLower, "storage driver") ||
			strings.Contains(fpLower, "volume plugin")) &&
			(ns == "kube-system" ||
				strings.Contains(name, "csi-") ||
				strings.Contains(name, "ebs-") ||
				strings.Contains(name, "nfs-")) {
			return fp
		}
	}
	return ""
}
