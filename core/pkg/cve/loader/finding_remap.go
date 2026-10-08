package loader

import (
	"context"
	"fmt"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// RemapFindingsToCanonical moves vulnerability insights and exception policies recorded under
// an advisory ID (GHSA-…, DSA-…, RHSA-…) to the canonical vulnerability IDs the matcher now
// reports (see CanonicalVulnIDs). Triage carries over: a dismissed advisory finding dismisses
// its CVE findings, an assignee is kept, and an exception is copied to every CVE it covered.
// It reads the current catalog and is idempotent; it returns how many advisory IDs it moved.
func RemapFindingsToCanonical(ctx context.Context, db *gorm.DB) (int, error) {
	m := db.Migrator()
	if !m.HasTable("vuln_advisory_refs") || !m.HasTable("insights") || !m.HasTable("exception_policies") {
		return 0, nil
	}
	tx := db.WithContext(ctx)

	var used []string
	if err := tx.Raw(`
SELECT DISTINCT cve_id FROM insights
WHERE insight_type = 'vulnerability' AND cve_id <> '' AND cve_id NOT LIKE 'CVE-%' AND deleted_at IS NULL
UNION
SELECT DISTINCT cve_id FROM exception_policies
WHERE cve_id <> '' AND cve_id NOT LIKE 'CVE-%' AND deleted_at IS NULL`).Scan(&used).Error; err != nil {
		return 0, fmt.Errorf("list advisory IDs in findings: %w", err)
	}
	if len(used) == 0 {
		return 0, nil
	}

	var advisories []struct {
		AdvisoryID string
		Source     string
	}
	if err := tx.Raw(`SELECT advisory_id, source FROM vuln_advisories WHERE valid_to_gen IS NULL AND advisory_id = ANY(?)`,
		pq.StringArray(used)).Scan(&advisories).Error; err != nil {
		return 0, fmt.Errorf("load advisories: %w", err)
	}
	var refRows []struct {
		AdvisoryID string
		RefID      string
		Relation   string
		RefKind    string
	}
	if err := tx.Raw(`SELECT advisory_id, ref_id, relation, ref_kind FROM vuln_advisory_refs WHERE valid_to_gen IS NULL AND advisory_id = ANY(?)`,
		pq.StringArray(used)).Scan(&refRows).Error; err != nil {
		return 0, fmt.Errorf("load advisory refs: %w", err)
	}
	refs := map[string][]AdvisoryRef{}
	for _, r := range refRows {
		refs[r.AdvisoryID] = append(refs[r.AdvisoryID], AdvisoryRef{RefID: r.RefID, Relation: r.Relation, RefKind: r.RefKind})
	}
	var oldIDs, newIDs []string
	moved := 0
	for _, a := range advisories {
		ids := CanonicalVulnIDs(a.AdvisoryID, a.Source, refs[a.AdvisoryID])
		if len(ids) == 1 && ids[0] == a.AdvisoryID {
			continue
		}
		moved++
		for _, id := range ids {
			oldIDs = append(oldIDs, a.AdvisoryID)
			newIDs = append(newIDs, id)
		}
	}
	if moved == 0 {
		return 0, nil
	}

	err := tx.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMP TABLE fortuna_vuln_id_map (old_id VARCHAR(255) NOT NULL, new_id VARCHAR(255) NOT NULL) ON COMMIT DROP`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO fortuna_vuln_id_map SELECT * FROM unnest(?::text[], ?::text[])`,
			pq.StringArray(oldIDs), pq.StringArray(newIDs)).Error; err != nil {
			return err
		}
		stmts := []string{
			// Exceptions: one copy per CVE, then retire the advisory-keyed policy.
			`INSERT INTO exception_policies (cluster_id, resource_uid, cve_id, insight_type, reason, expires_at, created_by, created_at, updated_at)
			 SELECT DISTINCT ON (e.cluster_id, e.resource_uid, e.insight_type, m.new_id)
			   e.cluster_id, e.resource_uid, m.new_id, e.insight_type, e.reason, e.expires_at, e.created_by, e.created_at, now()
			 FROM exception_policies e JOIN fortuna_vuln_id_map m ON m.old_id = e.cve_id
			 WHERE e.deleted_at IS NULL
			   AND NOT EXISTS (SELECT 1 FROM exception_policies x
			                   WHERE x.deleted_at IS NULL AND x.cve_id = m.new_id AND x.insight_type IS NOT DISTINCT FROM e.insight_type
			                     AND x.resource_uid IS NOT DISTINCT FROM e.resource_uid AND x.cluster_id IS NOT DISTINCT FROM e.cluster_id)
			 ORDER BY e.cluster_id, e.resource_uid, e.insight_type, m.new_id, e.expires_at DESC NULLS FIRST, e.id`,
			`UPDATE exception_policies SET deleted_at = now(), updated_at = now()
			 WHERE deleted_at IS NULL AND cve_id IN (SELECT old_id FROM fortuna_vuln_id_map)`,

			// Insights with a single CVE and no CVE-keyed twin keep their row: rename in place.
			`WITH pick AS (
			   SELECT DISTINCT ON (i.cluster_id, i.resource_uid, i.insight_type, m.new_id) i.id, m.new_id
			   FROM insights i JOIN fortuna_vuln_id_map m ON m.old_id = i.cve_id
			   WHERE i.insight_type = 'vulnerability' AND i.deleted_at IS NULL
			     AND (SELECT count(*) FROM fortuna_vuln_id_map c WHERE c.old_id = m.old_id) = 1
			     AND NOT EXISTS (SELECT 1 FROM insights t
			                   WHERE t.cluster_id IS NOT DISTINCT FROM i.cluster_id AND t.resource_uid = i.resource_uid
			                     AND t.insight_type = i.insight_type AND t.cve_id = m.new_id)
			   ORDER BY i.cluster_id, i.resource_uid, i.insight_type, m.new_id, (i.status = 'dismissed') DESC, i.id)
			 UPDATE insights i SET cve_id = p.new_id, title = replace(i.title, i.cve_id, p.new_id), updated_at = now()
			 FROM pick p WHERE i.id = p.id`,

			// The rest: carry triage onto the CVE-keyed twins that exist…
			`UPDATE insights t SET
			   status = CASE WHEN t.deleted_at IS NOT NULL THEN i.status
			                 WHEN i.status = 'dismissed' AND t.status <> 'dismissed' THEN 'dismissed' ELSE t.status END,
			   deleted_at = NULL,
			   assignee_user_id = COALESCE(t.assignee_user_id, i.assignee_user_id),
			   assignee_username = CASE WHEN t.assignee_username = '' THEN i.assignee_username ELSE t.assignee_username END,
			   assigned_at = COALESCE(t.assigned_at, i.assigned_at),
			   updated_at = now()
			 FROM insights i JOIN fortuna_vuln_id_map m ON m.old_id = i.cve_id
			 WHERE i.insight_type = 'vulnerability' AND i.deleted_at IS NULL
			   AND t.cluster_id IS NOT DISTINCT FROM i.cluster_id AND t.resource_uid = i.resource_uid
			   AND t.insight_type = i.insight_type AND t.cve_id = m.new_id`,
			// …create the missing ones…
			`INSERT INTO insights (cluster_id, resource_type, resource_namespace, resource_name, resource_uid, insight_type,
			   severity, title, description, recommendation, status, cve_id, affected_component, affected_version, fixed_version,
			   cvss, evidence, violated_rules, risk_explanation, remediation, match_confidence, component_confidence,
			   sbom_confidence, final_risk_confidence, degraded, sensitivity, assignee_user_id, assignee_username, assigned_at,
			   detected_at, resolved_at, created_at, updated_at)
			 SELECT DISTINCT ON (i.cluster_id, i.resource_uid, i.insight_type, m.new_id)
			   i.cluster_id, i.resource_type, i.resource_namespace, i.resource_name, i.resource_uid, i.insight_type,
			   i.severity, replace(i.title, i.cve_id, m.new_id), i.description, i.recommendation, i.status, m.new_id,
			   i.affected_component, i.affected_version, i.fixed_version, i.cvss, i.evidence, i.violated_rules,
			   i.risk_explanation, i.remediation, i.match_confidence, i.component_confidence, i.sbom_confidence,
			   i.final_risk_confidence, i.degraded, i.sensitivity, i.assignee_user_id, i.assignee_username, i.assigned_at,
			   i.detected_at, i.resolved_at, i.created_at, now()
			 FROM insights i JOIN fortuna_vuln_id_map m ON m.old_id = i.cve_id
			 WHERE i.insight_type = 'vulnerability' AND i.deleted_at IS NULL
			   AND NOT EXISTS (SELECT 1 FROM insights t
			                   WHERE t.cluster_id IS NOT DISTINCT FROM i.cluster_id AND t.resource_uid = i.resource_uid
			                     AND t.insight_type = i.insight_type AND t.cve_id = m.new_id)
			 ORDER BY i.cluster_id, i.resource_uid, i.insight_type, m.new_id, (i.status = 'dismissed') DESC, i.id`,
			// …and retire the advisory-keyed rows.
			`UPDATE insights SET deleted_at = now(), updated_at = now()
			 WHERE insight_type = 'vulnerability' AND deleted_at IS NULL AND cve_id IN (SELECT old_id FROM fortuna_vuln_id_map)`,
		}
		for _, stmt := range stmts {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("remap findings to canonical IDs: %w", err)
	}
	return moved, nil
}
