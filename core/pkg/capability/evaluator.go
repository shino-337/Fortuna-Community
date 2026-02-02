package capability

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/fortuna/core/pkg/models"
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

type roleRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type policyRule struct {
	Verbs []string `json:"verbs"`
}

// EvaluateAndUpsertPod evaluates capabilities for a single pod and persists results.
// Uses CapabilityStateController (CSC) to initialize capabilities with detected state.
func EvaluateAndUpsertPod(ctx context.Context, db *gorm.DB, pod *models.Pod) error {
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
	
	// Use CSC to initialize capabilities
	csc := NewCapabilityStateController(db)
	for _, c := range caps {
		if err := csc.InitializeCapability(ctx, pod.UID, pod.Namespace, c.ID, c.Group, c.Severity, c.Evidence); err != nil {
			log.Printf("[PCE] Failed to initialize capability %s for pod %s/%s: %v", c.ID, pod.Namespace, pod.Name, err)
		}
	}
	
	return upsertPodRiskProfile(db, *pod, caps)
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
	
	csc := NewCapabilityStateController(db)
	for _, pod := range pods {
		caps, err := EvaluatePod(ctx, db, &pod)
		if err != nil {
			log.Printf("[PCE] Failed to evaluate pod %s/%s: %v", pod.Namespace, pod.Name, err)
			continue
		}
		
		// Use CSC to initialize capabilities
		for _, c := range caps {
			if err := csc.InitializeCapability(ctx, pod.UID, pod.Namespace, c.ID, c.Group, c.Severity, c.Evidence); err != nil {
				log.Printf("[PCE] Failed to initialize capability %s for pod %s/%s: %v", c.ID, pod.Namespace, pod.Name, err)
			}
		}
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
	hasWrite, evidence, err := hasAPIWriteAccess(ctx, db, pod)
	if err != nil {
		return caps, err
	}
	if hasWrite {
		caps = append(caps, Capability{
			ID:       API_RBAC_WRITE_CLUSTER,
			Group:    "API",
			Severity: "HIGH",
			Evidence: evidence,
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

func hasAPIWriteAccess(ctx context.Context, db *gorm.DB, pod *models.Pod) (bool, map[string]interface{}, error) {
	if pod.ServiceAccount == "" {
		return false, nil, nil
	}

	roles := make([]roleRef, 0)

	var roleBindings []models.RoleBinding
	if err := db.WithContext(ctx).Where("cluster_id = ? AND namespace = ? AND deleted_at IS NULL", pod.ClusterID, pod.Namespace).
		Find(&roleBindings).Error; err != nil {
		return false, nil, err
	}
	for _, rb := range roleBindings {
		var subs []subject
		if err := json.Unmarshal([]byte(rb.Subjects), &subs); err != nil {
			continue
		}
		for _, s := range subs {
			if s.Kind == "ServiceAccount" && s.Name == pod.ServiceAccount && s.Namespace == pod.Namespace {
				var ref roleRef
				if err := json.Unmarshal([]byte(rb.RoleRef), &ref); err == nil {
					roles = append(roles, ref)
				}
				break
			}
		}
	}

	var clusterRoleBindings []models.ClusterRoleBinding
	if err := db.WithContext(ctx).Where("cluster_id = ? AND deleted_at IS NULL", pod.ClusterID).
		Find(&clusterRoleBindings).Error; err != nil {
		return false, nil, err
	}
	for _, crb := range clusterRoleBindings {
		var subs []subject
		if err := json.Unmarshal([]byte(crb.Subjects), &subs); err != nil {
			continue
		}
		for _, s := range subs {
			if s.Kind == "ServiceAccount" && s.Name == pod.ServiceAccount && s.Namespace == pod.Namespace {
				var ref roleRef
				if err := json.Unmarshal([]byte(crb.RoleRef), &ref); err == nil {
					roles = append(roles, ref)
				}
				break
			}
		}
	}

	for _, r := range roles {
		switch r.Kind {
		case "Role":
			var role models.Role
			if err := db.WithContext(ctx).Where("cluster_id = ? AND name = ? AND namespace = ? AND deleted_at IS NULL",
				pod.ClusterID, r.Name, pod.Namespace).First(&role).Error; err != nil {
				continue
			}
			if hasWriteVerbs(role.Rules) {
				return true, map[string]interface{}{"role": r.Name, "roleKind": r.Kind}, nil
			}
		case "ClusterRole":
			var cr models.ClusterRole
			if err := db.WithContext(ctx).Where("cluster_id = ? AND name = ? AND deleted_at IS NULL",
				pod.ClusterID, r.Name).First(&cr).Error; err != nil {
				continue
			}
			if hasWriteVerbs(cr.Rules) {
				return true, map[string]interface{}{"role": r.Name, "roleKind": r.Kind}, nil
			}
		}
	}

	return false, nil, nil
}

func hasWriteVerbs(rulesJSON string) bool {
	var rules []policyRule
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return false
	}
	for _, r := range rules {
		for _, verb := range r.Verbs {
			v := strings.ToLower(verb)
			if v == "*" || v == "create" || v == "update" || v == "delete" || v == "patch" {
				return true
			}
		}
	}
	return false
}
