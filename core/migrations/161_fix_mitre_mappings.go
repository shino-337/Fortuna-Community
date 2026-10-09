package migrations

import (
	"encoding/json"
	"fmt"
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// mitreMetadataFix replaces a wrong MITRE mapping seeded into
// capability_metadata by migration 061. A row is changed only while it still
// holds the wrong technique, so operator edits are kept.
type mitreMetadataFix struct {
	capabilityID   string
	wrongTechnique string
	tactic         string
	technique      string
	subtechnique   string
	killChainStage string
	refs           models.JSONBStringArray
}

var capabilityMitreFixes = []mitreMetadataFix{
	// A hostPath escape is Escape to Host, not Deploy Container (T1610).
	{"ESC_HOSTPATH_NODE", "T1610", "Privilege Escalation (TA0004)", "T1611", "Escape to Host", "Privilege Escalation",
		models.JSONBStringArray{"https://attack.mitre.org/techniques/T1611/"}},
	// T1611 has no sub-techniques.
	{"ESC_HOSTPID_POD", "T1611.001", "Privilege Escalation (TA0004)", "T1611", "", "Privilege Escalation",
		models.JSONBStringArray{"https://attack.mitre.org/techniques/T1611/"}},
	{"ESC_HOSTIPC_POD", "T1611.002", "Privilege Escalation (TA0004)", "T1611", "", "Privilege Escalation",
		models.JSONBStringArray{"https://attack.mitre.org/techniques/T1611/"}},
	// RBAC write is Additional Container Cluster Roles, not Build Image on Host (T1612).
	{"API_RBAC_WRITE_CLUSTER", "T1612", "Privilege Escalation (TA0004)", "T1098.006", "Additional Container Cluster Roles", "Privilege Escalation",
		models.JSONBStringArray{"https://attack.mitre.org/techniques/T1098/006/"}},
	// Running in kube-system is not Resource Hijacking (T1496); no technique applies.
	{"CTRL_CONTROL_PLANE_POD", "T1496", "", "", "", "Discovery", models.JSONBStringArray{}},
}

// riskRuleTagFixes maps rule_id → wrong tag → replacement ("" drops the tag)
// for YAML rules seeded into risk_rules by migration 097.
var riskRuleTagFixes = map[string]map[string]string{
	"pss-host-namespaces":               {"T1611.001": "T1611", "T1611.002": "T1611"},
	"cis-5.1.4":                         {"T1612": "T1610"},
	"cis-5.1.5":                         {"T1612": "T1610"},
	"cis-5.1.6":                         {"T1612": ""},
	"cis-5.1.7":                         {"T1612": "T1136"},
	"cis-5.1.8":                         {"T1612": "T1098.006"},
	"cis-5.1.9":                         {"T1612": "T1098.006"},
	"rbac-escalate-verb":                {"T1612": "T1098.006"},
	"rbac-bind-verb":                    {"T1612": "T1098.006"},
	"rbac-impersonate-verb":             {"T1612": "T1078"},
	"rbac-pods-exec-attach-portforward": {"T1612": "T1609"},
}

// Migration161_FixMitreMappings corrects MITRE ATT&CK IDs already stored in
// capability_metadata and risk_rules: non-existent T1611.001/T1611.002, T1610
// (Deploy Container) used for a filesystem escape, T1612 (Build Image on Host)
// used for RBAC rules, and T1496 (Resource Hijacking) for every kube-system pod.
// pod_capabilities.mitre is rewritten by the next capability evaluation.
func Migration161_FixMitreMappings(db *gorm.DB) error {
	if db.Migrator().HasTable("capability_metadata") {
		for _, f := range capabilityMitreFixes {
			res := db.Table("capability_metadata").
				Where("capability_id = ? AND mitre_technique = ?", f.capabilityID, f.wrongTechnique).
				Updates(map[string]any{
					"mitre_tactic":       f.tactic,
					"mitre_technique":    f.technique,
					"mitre_subtechnique": f.subtechnique,
					"kill_chain_stage":   f.killChainStage,
					"refs":               f.refs,
				})
			if res.Error != nil {
				return fmt.Errorf("[Migration 161] capability_metadata %s: %w", f.capabilityID, res.Error)
			}
		}
	}

	if !db.Migrator().HasTable(&models.RiskRule{}) {
		return nil
	}
	for ruleID, fixes := range riskRuleTagFixes {
		var rule models.RiskRule
		err := db.Where("rule_id = ?", ruleID).Limit(1).Find(&rule).Error
		if err != nil {
			return fmt.Errorf("[Migration 161] load risk rule %s: %w", ruleID, err)
		}
		if rule.ID == 0 || rule.Tags == "" {
			continue
		}
		var tags []string
		if err := json.Unmarshal([]byte(rule.Tags), &tags); err != nil {
			log.Printf("[Migration 161] risk rule %s: tags are not a JSON array, left unchanged", ruleID)
			continue
		}
		fixed, changed := fixMitreTags(tags, fixes)
		if !changed {
			continue
		}
		raw, _ := json.Marshal(fixed)
		if err := db.Model(&models.RiskRule{}).Where("id = ?", rule.ID).Update("tags", string(raw)).Error; err != nil {
			return fmt.Errorf("[Migration 161] update risk rule %s: %w", ruleID, err)
		}
	}
	return nil
}

func fixMitreTags(tags []string, fixes map[string]string) ([]string, bool) {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	changed := false
	for _, t := range tags {
		if repl, ok := fixes[t]; ok {
			changed = true
			t = repl
		}
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out, changed
}
