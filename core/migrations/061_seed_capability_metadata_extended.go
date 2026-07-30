package migrations

import (
	"encoding/json"
	"log"

	"gorm.io/gorm"
)

// Migration061_SeedCapabilityMetadataExtended seeds extended metadata from Capability Specification (MITRE ATT&CK).
// Maps existing capability_id to name, summary, full_description, mitre, kill_chain_stage, impact, mitigations, etc.
func Migration061_SeedCapabilityMetadataExtended(db *gorm.DB) error {
	log.Println("Running migration 061: Seed capability_metadata extended fields")
	if !shouldRunSeedMigrations() {
		log.Println("[Migration 061] seed disabled (FORTUNA_ENABLE_SEED_DATA not set)")
		return nil
	}

	if !db.Migrator().HasTable("capability_metadata") {
		log.Println("[Migration 061] capability_metadata table does not exist, skipping")
		return nil
	}

	// capability_id -> extended metadata (from docs/03-components/podCapabilityEngine/Capability_Specification–MITRE_ATT&C.md)
	seeds := map[string]struct {
		Name                        string
		Summary                     string
		FullDescription             string
		MitreTactic                 string
		MitreTechnique              string
		MitreSubtechnique           string
		KillChainStage              string
		TechnicalIndicators         []string
		Impact                      []string
		RecommendedMitigations      []string
		FalsePositiveConsiderations []string
		References                  []string
	}{
		"ESC_PRIV_POD": {
			Name:                        "Privileged Container Execution",
			Summary:                     "A container is running in privileged mode, granting unrestricted access to host resources.",
			FullDescription:             "Privileged containers disable most isolation mechanisms enforced by the container runtime. An attacker controlling such a container can directly interact with host devices, kernel interfaces, and filesystem paths, effectively bypassing container boundaries and achieving host-level control.",
			MitreTactic:                 "Privilege Escalation (TA0004)",
			MitreTechnique:              "T1611",
			MitreSubtechnique:           "Escape to Host",
			KillChainStage:              "Privilege Escalation",
			TechnicalIndicators:         []string{"Access to /dev, /proc, kernel interfaces"},
			Impact:                      []string{"Full host compromise", "Persistence outside Kubernetes control plane"},
			RecommendedMitigations:      []string{"Remove privileged flag from workload definitions", "Enforce Pod Security Admission (Restricted profile)"},
			FalsePositiveConsiderations: []string{"Common for infrastructure components such as CNI or CSI drivers"},
			References:                  []string{"https://attack.mitre.org/techniques/T1611/"},
		},
		"ESC_HOSTPATH_NODE": {
			Name:                        "Writable HostPath Mount",
			Summary:                     "A container mounts a host filesystem path with write permissions.",
			FullDescription:             "Writable hostPath volumes expose the host filesystem directly to the container. Attackers can modify host binaries, inject malicious files, or tamper with node-level configuration, enabling host takeover or persistence.",
			MitreTactic:                 "Defense Evasion (TA0005)",
			MitreTechnique:              "T1610",
			MitreSubtechnique:           "Escape to Host",
			KillChainStage:              "Defense Evasion",
			TechnicalIndicators:         []string{"Write operations on host-mounted paths"},
			Impact:                      []string{"Host integrity compromise", "Long-term persistence"},
			RecommendedMitigations:      []string{"Remove hostPath usage where possible", "Enforce read-only mounts if unavoidable"},
			FalsePositiveConsiderations: []string{"Some monitoring or storage agents may require hostPath access"},
			References:                  []string{"https://attack.mitre.org/techniques/T1610/"},
		},
		"ESC_HOSTPID_POD": {
			Name:                        "Host PID Namespace Sharing",
			Summary:                     "A container shares the host PID namespace, allowing visibility into host processes.",
			FullDescription:             "When hostPID is enabled, processes inside the container can observe and potentially interfere with processes running on the host, enabling reconnaissance or direct interaction with host-level services.",
			MitreTactic:                 "Privilege Escalation (TA0004)",
			MitreTechnique:              "T1611.001",
			KillChainStage:              "Privilege Escalation",
			TechnicalIndicators:         []string{"Visibility of host process IDs"},
			Impact:                      []string{"Host process manipulation", "Increased attack surface"},
			RecommendedMitigations:      []string{"Disable hostPID unless strictly required"},
			FalsePositiveConsiderations: []string{"Used by certain debugging or monitoring workloads"},
			References:                  []string{"https://attack.mitre.org/techniques/T1611/"},
		},
		"ESC_HOSTIPC_POD": {
			Name:                        "Host IPC Namespace Sharing",
			Summary:                     "A container shares the host IPC namespace, exposing inter-process communication channels.",
			FullDescription:             "Sharing the IPC namespace allows containers to interact with host IPC mechanisms such as shared memory or semaphores, potentially enabling data leakage or process interference.",
			MitreTactic:                 "Privilege Escalation (TA0004)",
			MitreTechnique:              "T1611.002",
			KillChainStage:              "Privilege Escalation",
			TechnicalIndicators:         []string{"Access to host IPC resources"},
			Impact:                      []string{"Data leakage", "Host process interference"},
			RecommendedMitigations:      []string{"Disable hostIPC by default"},
			FalsePositiveConsiderations: []string{"Rare, mostly system-level workloads"},
			References:                  []string{"https://attack.mitre.org/techniques/T1611/"},
		},
		"ID_TOKEN_POD": {
			Name:                        "ServiceAccount Token Access",
			Summary:                     "A container accesses a mounted Kubernetes ServiceAccount token.",
			FullDescription:             "Kubernetes mounts ServiceAccount tokens into pods by default. Attackers can extract these tokens and authenticate to the Kubernetes API, potentially enabling lateral movement or privilege escalation depending on RBAC configuration.",
			MitreTactic:                 "Credential Access (TA0006)",
			MitreTechnique:              "T1552.007",
			MitreSubtechnique:           "Container API Credentials",
			KillChainStage:              "Credential Access",
			TechnicalIndicators:         []string{"Read access to SA token file"},
			Impact:                      []string{"Kubernetes API abuse", "Cross-namespace access"},
			RecommendedMitigations:      []string{"Disable automountServiceAccountToken where unnecessary", "Apply least-privilege RBAC"},
			FalsePositiveConsiderations: []string{"Legitimate controllers may access tokens"},
			References:                  []string{"https://attack.mitre.org/techniques/T1552/007/"},
		},
		"API_RBAC_WRITE_CLUSTER": {
			Name:                        "Excessive Cluster-Level RBAC",
			Summary:                     "A ServiceAccount has cluster-wide write permissions.",
			FullDescription:             "Excessive RBAC permissions allow attackers to create, modify, or bind high-privilege roles, enabling full cluster takeover.",
			MitreTactic:                 "Lateral Movement (TA0008)",
			MitreTechnique:              "T1612",
			MitreSubtechnique:           "Kubernetes RBAC Abuse",
			KillChainStage:              "Lateral Movement",
			TechnicalIndicators:         []string{"Verbs: create/update/delete on clusterroles or bindings"},
			Impact:                      []string{"Full cluster compromise"},
			RecommendedMitigations:      []string{"Audit and reduce RBAC permissions"},
			FalsePositiveConsiderations: []string{"Cluster operators may require elevated access"},
			References:                  []string{"https://attack.mitre.org/techniques/T1612/"},
		},
		"NET_HOSTNETWORK": {
			Name:                        "Host Network Usage",
			Summary:                     "A container runs in the host network namespace.",
			FullDescription:             "Using the host network namespace allows containers to bypass network isolation, enabling traffic sniffing, port scanning, or direct access to node-level services.",
			MitreTactic:                 "Lateral Movement (TA0008)",
			MitreTechnique:              "T1614",
			MitreSubtechnique:           "Container and Cluster Networking Abuse",
			KillChainStage:              "Lateral Movement",
			TechnicalIndicators:         []string{"Access to node network interfaces"},
			Impact:                      []string{"Network reconnaissance", "Service exploitation"},
			RecommendedMitigations:      []string{"Avoid hostNetwork usage", "Enforce network policies"},
			FalsePositiveConsiderations: []string{"Used by certain network plugins"},
			References:                  []string{"https://attack.mitre.org/techniques/T1614/"},
		},
		"CTRL_CONTROL_PLANE_POD": {
			Name:                        "Control Plane Namespace",
			Summary:                     "A pod runs in the control plane namespace (e.g. kube-system).",
			FullDescription:             "Pods in control plane namespaces have proximity to sensitive cluster components. Compromise may enable access to cluster-level secrets or configuration.",
			MitreTactic:                 "Discovery / Impact",
			MitreTechnique:              "T1496",
			KillChainStage:              "Discovery",
			TechnicalIndicators:         []string{"Pod in kube-system or kube-public"},
			Impact:                      []string{"Control plane access", "Cluster-wide impact"},
			RecommendedMitigations:      []string{"Restrict workloads in control plane namespaces"},
			FalsePositiveConsiderations: []string{"System components legitimately run in kube-system"},
			References:                  []string{"https://attack.mitre.org/techniques/T1496/"},
		},
		"ESC_RUNTIME_PROBE": {
			Name:                        "Runtime Escape Probe",
			Summary:                     "Static risk plus runtime signal indicates container escape potential.",
			FullDescription:             "The pod has static escape-prone configuration (e.g. hostPID or sensitive hostPath) and has produced runtime signals (e.g. access to /proc/1/root), indicating active probing toward escape.",
			MitreTactic:                 "Privilege Escalation (TA0004)",
			MitreTechnique:              "T1611",
			KillChainStage:              "Privilege Escalation",
			TechnicalIndicators:         []string{"Runtime access to /proc, /sys, or sensitive paths"},
			Impact:                      []string{"Potential host compromise"},
			RecommendedMitigations:      []string{"Remove hostPID/sensitive hostPath; enforce Pod Security Standards"},
			FalsePositiveConsiderations: []string{"Monitoring or debugging tools may touch /proc"},
			References:                  []string{"https://attack.mitre.org/techniques/T1611/"},
		},
		"ESC_RUNTIME_ACTIVE": {
			Name:                        "Active Runtime Escape",
			Summary:                     "Active runtime escape confirmed with high confidence.",
			FullDescription:             "Multiple or high-severity runtime signals (e.g. proc root pivot, FS escape attempt) confirm that the workload is actively escaping container boundaries, with high confidence.",
			MitreTactic:                 "Privilege Escalation (TA0004)",
			MitreTechnique:              "T1611",
			KillChainStage:              "Privilege Escalation",
			TechnicalIndicators:         []string{"Repeated /proc/1/root access", "mount/pivot_root on sensitive paths"},
			Impact:                      []string{"Full host compromise", "Persistence"},
			RecommendedMitigations:      []string{"Isolate or terminate workload; audit node"},
			FalsePositiveConsiderations: []string{"Rare; high confidence required"},
			References:                  []string{"https://attack.mitre.org/techniques/T1611/"},
		},
		"ESC_RUNTIME_PROC_ROOT": {
			Name:                        "Proc Root Pivot (Runtime Confirmed)",
			Summary:                     "Runtime signal confirms access to /proc/1/root (proc root pivot).",
			FullDescription:             "The workload has been observed accessing /proc/1/root or equivalent, confirming ability to pivot to the host root filesystem from the container.",
			MitreTactic:                 "Privilege Escalation (TA0004)",
			MitreTechnique:              "T1611",
			KillChainStage:              "Privilege Escalation",
			TechnicalIndicators:         []string{"open/openat/readlink on /proc/1/root"},
			Impact:                      []string{"Host filesystem access", "Persistence"},
			RecommendedMitigations:      []string{"Remove hostPath to /proc; restrict capabilities"},
			FalsePositiveConsiderations: []string{"Some monitoring may read /proc"},
			References:                  []string{"https://attack.mitre.org/techniques/T1611/"},
		},
	}

	for capID, data := range seeds {
		ti, _ := json.Marshal(data.TechnicalIndicators)
		imp, _ := json.Marshal(data.Impact)
		rm, _ := json.Marshal(data.RecommendedMitigations)
		fpc, _ := json.Marshal(data.FalsePositiveConsiderations)
		refs, _ := json.Marshal(data.References)

		sql := `
			UPDATE capability_metadata SET
				name = $1,
				summary = $2,
				full_description = $3,
				mitre_tactic = $4,
				mitre_technique = $5,
				mitre_subtechnique = $6,
				kill_chain_stage = $7,
				technical_indicators = COALESCE($8::jsonb, '[]'::jsonb),
				impact = COALESCE($9::jsonb, '[]'::jsonb),
				recommended_mitigations = COALESCE($10::jsonb, '[]'::jsonb),
				false_positive_considerations = COALESCE($11::jsonb, '[]'::jsonb),
				refs = COALESCE($12::jsonb, '[]'::jsonb),
				updated_at = CURRENT_TIMESTAMP
			WHERE capability_id = $13
		`
		err := db.Exec(sql,
			data.Name,
			data.Summary,
			data.FullDescription,
			data.MitreTactic,
			data.MitreTechnique,
			data.MitreSubtechnique,
			data.KillChainStage,
			string(ti),
			string(imp),
			string(rm),
			string(fpc),
			string(refs),
			capID,
		).Error
		if err != nil {
			log.Printf("[Migration 061] Warning updating %s: %v", capID, err)
			continue
		}
		log.Printf("[Migration 061] Updated extended metadata for %s", capID)
	}

	log.Println("[Migration 061] ✅ Completed successfully")
	return nil
}
