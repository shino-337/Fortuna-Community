package migrations

import (
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
)

// Migration146_SeedLegacyFindingCodeRules registers legacy finding codes as
// catalog rules. Historical insights store these codes in insights.cve_id, and
// the Policy Rules activity view matches by exact rule_id = cve_id. These rows
// keep that provenance visible without changing historical findings.
func Migration146_SeedLegacyFindingCodeRules(db *gorm.DB) error {
	log.Println("Running migration 146: seed legacy finding-code catalog rules")

	type legacyRule struct {
		ID          string
		Name        string
		Category    string
		Severity    string
		Description string
		BaseScore   string
		Tags        string
	}

	rules := []legacyRule{
		{
			ID:          "ID_TOKEN_POD",
			Name:        "Pod exposes service account token capability",
			Category:    "pod-security",
			Severity:    "high",
			Description: "Legacy capability finding for pods where a mounted or automounted service account token may allow Kubernetes API access.",
			BaseScore:   "7.5",
			Tags:        `["legacy-finding-code","capability","identity","service-account-token"]`,
		},
		{
			ID:          "ESC_HOSTPATH_NODE",
			Name:        "HostPath mount enables node filesystem access",
			Category:    "pod-security",
			Severity:    "critical",
			Description: "Legacy capability finding for workloads with hostPath exposure that can support node filesystem access or container escape paths.",
			BaseScore:   "9.0",
			Tags:        `["legacy-finding-code","capability","container-escape","hostpath"]`,
		},
		{
			ID:          "ESC_RUNTIME_PROBE",
			Name:        "Runtime behavior indicates escape probing",
			Category:    "runtime-behavior",
			Severity:    "critical",
			Description: "Legacy runtime/capability finding for observed behavior consistent with escape probing or host interaction attempts.",
			BaseScore:   "8.5",
			Tags:        `["legacy-finding-code","runtime","container-escape","attack-path"]`,
		},
		{
			ID:          "API_RBAC_WRITE_CLUSTER",
			Name:        "Cluster-scoped RBAC write capability",
			Category:    "rbac",
			Severity:    "critical",
			Description: "Legacy RBAC finding for identities that can write cluster-scoped RBAC resources or grant broader permissions.",
			BaseScore:   "9.0",
			Tags:        `["legacy-finding-code","rbac","privilege-escalation","cluster-scope"]`,
		},
		{
			ID:          "ESC_HOSTPID_POD",
			Name:        "Pod shares host PID namespace",
			Category:    "pod-security",
			Severity:    "critical",
			Description: "Legacy capability finding for pods using hostPID, which expands process visibility and can support host interaction or escape paths.",
			BaseScore:   "8.5",
			Tags:        `["legacy-finding-code","pod-security","container-escape","hostpid"]`,
		},
		{
			ID:          "CTRL_CONTROL_PLANE_POD",
			Name:        "Control-plane workload exposure",
			Category:    "pod-security",
			Severity:    "high",
			Description: "Legacy control-plane finding for workloads running in or affecting sensitive control-plane context.",
			BaseScore:   "8.0",
			Tags:        `["legacy-finding-code","control-plane","sensitive-workload"]`,
		},
		{
			ID:          "NET_HOSTNETWORK",
			Name:        "Pod uses host network namespace",
			Category:    "pod-security",
			Severity:    "high",
			Description: "Legacy network exposure finding for pods using hostNetwork, increasing node-level network visibility and blast radius.",
			BaseScore:   "7.5",
			Tags:        `["legacy-finding-code","network","hostnetwork","exposure"]`,
		},
		{
			ID:          "ESC_PRIV_POD",
			Name:        "Privileged pod escape capability",
			Category:    "pod-security",
			Severity:    "critical",
			Description: "Legacy capability finding for privileged pods that weaken container isolation and can support host takeover paths.",
			BaseScore:   "9.0",
			Tags:        `["legacy-finding-code","pod-security","container-escape","privileged"]`,
		},
	}

	const conditions = `[{"type":"expression","expression":"false"}]`
	for _, rule := range rules {
		stmt := fmt.Sprintf(`
			INSERT INTO risk_rules (
				rule_id, name, category, severity, description, enabled,
				conditions, aggregation, base_score, tags, created_at, updated_at, deleted_at
			) VALUES (
				%s, %s, %s, %s, %s, true,
				%s, 'AND', %s, %s, NOW(), NOW(), NULL
			)
			ON CONFLICT (rule_id) DO UPDATE SET
				name = EXCLUDED.name,
				category = EXCLUDED.category,
				severity = EXCLUDED.severity,
				description = EXCLUDED.description,
				enabled = true,
				conditions = CASE
					WHEN risk_rules.conditions = '' OR risk_rules.conditions IS NULL THEN EXCLUDED.conditions
					ELSE risk_rules.conditions
				END,
				aggregation = COALESCE(NULLIF(risk_rules.aggregation, ''), EXCLUDED.aggregation),
				base_score = EXCLUDED.base_score,
				tags = EXCLUDED.tags,
				updated_at = NOW(),
				deleted_at = NULL;`,
			sqlQuote(rule.ID),
			sqlQuote(rule.Name),
			sqlQuote(rule.Category),
			sqlQuote(rule.Severity),
			sqlQuote(rule.Description),
			sqlQuote(conditions),
			rule.BaseScore,
			sqlQuote(rule.Tags),
		)
		if err := execDDL(db, stmt); err != nil {
			return fmt.Errorf("migration 146 seed legacy rule %s: %w", rule.ID, err)
		}
	}

	log.Printf("Migration 146 completed: ensured %d legacy finding-code rules", len(rules))
	return nil
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
