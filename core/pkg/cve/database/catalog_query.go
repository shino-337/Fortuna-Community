package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/cve/cvss"
	"github.com/fortuna/core/pkg/cve/loader"
	"gorm.io/gorm"
)

// Severity sources, best first. A finding takes its severity from the best source any of its
// matched advisories offers.
const (
	SeverityTierVendor       = 1 // the distro or GitHub rating on the matched advisory
	SeverityTierAdvisoryCVSS = 2 // CVSS of a matched advisory about this one CVE
	SeverityTierNVD          = 3 // NVD CVSS from the vulnerabilities table
	SeverityTierCVECVSS      = 4 // highest CVSS of another single-CVE advisory for this CVE
	SeverityTierErratumCVSS  = 5 // CVSS of a matched erratum covering several CVEs
	SeverityTierDefault      = 6 // nothing rates it; MEDIUM, as the legacy catalog did
)

var severitySourceNames = map[int]string{
	SeverityTierVendor:       "vendor",
	SeverityTierAdvisoryCVSS: "advisory_cvss",
	SeverityTierNVD:          "nvd",
	SeverityTierCVECVSS:      "cve_cvss",
	SeverityTierErratumCVSS:  "errata_cvss",
	SeverityTierDefault:      "default",
}

// visibleAt restricts a versioned catalog table (alias t) to the rows of generation $gen.
const visibleAt = "%[1]s.valid_from_gen <= @gen AND (%[1]s.valid_to_gen IS NULL OR %[1]s.valid_to_gen > @gen)"

func visible(alias string) string { return fmt.Sprintf(visibleAt, alias) }

// inList matches column against the named list argument: one array parameter on Postgres, an
// expanded IN list elsewhere (the SQLite databases of the tests).
func inList(db *gorm.DB, column, name string) string {
	if db.Dialector.Name() == "postgres" {
		return column + " = ANY(@" + name + ")"
	}
	return column + " IN @" + name
}

// listArg is the value inList expects for values.
func listArg(db *gorm.DB, values []string) interface{} {
	if db.Dialector.Name() == "postgres" {
		return pq.StringArray(values)
	}
	return values
}

// versionedCatalogGeneration returns the active CVE generation when the versioned catalog has
// rows for it, or 0 when no catalog is loaded.
func (m *Manager) versionedCatalogGeneration(ctx context.Context) uint {
	if m.postgresDB == nil || !m.postgresDB.Migrator().HasTable("vuln_affected") {
		return 0
	}
	gen := m.activeCVEGenerationID(ctx)
	if gen == 0 {
		return 0
	}
	var found bool
	if err := m.postgresDB.WithContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM vuln_advisories a WHERE "+visible("a")+")",
		map[string]interface{}{"gen": gen},
	).Scan(&found).Error; err != nil || !found {
		return 0
	}
	return gen
}

type catalogRangeRow struct {
	AdvisoryID     string
	Source         string
	Kind           string
	PackageName    string
	Release        string
	RangeType      string
	Introduced     string
	Fixed          string
	LastAffected   string
	VendorSeverity string
	SourceSeverity string
	Summary        string
	Details        string
	PublishedAt    *time.Time
	ModifiedAt     *time.Time
	CVSSV3Vector   string
	CVSSV3Score    *float64
	CVSSV4Vector   string
	CVSSV4Score    *float64
}

type catalogRefRow struct {
	AdvisoryID string
	RefID      string
	Relation   string
	RefKind    string
}

type cveScoreRow struct {
	VulnID string
	Score  *float64
	Vector string
}

// queryVersionedCatalogBulk returns the ranges of generation gen that affect packages of one
// ecosystem. Each range is reported once per canonical vulnerability of its advisory (see
// loader.CanonicalVulnIDs) with the severity resolved from the best source available.
func (m *Manager) queryVersionedCatalogBulk(ctx context.Context, gen uint, eco string, packages []string) (map[string][]*cve.CVE, error) {
	out := make(map[string][]*cve.CVE, len(packages))
	for _, pkg := range packages {
		out[pkg] = []*cve.CVE{}
	}
	if len(packages) == 0 {
		return out, nil
	}
	db := m.postgresDB.WithContext(ctx)
	args := map[string]interface{}{"gen": gen, "eco": eco, "pkgs": listArg(db, packages)}

	var rows []catalogRangeRow
	if err := db.Raw(`
SELECT f.advisory_id, a.source, a.kind, f.package_name, f.release, f.range_type, f.introduced, f.fixed,
       f.last_affected, f.vendor_severity, a.source_severity, a.summary, a.details,
       a.published_at, a.modified_at, a.cvss_v3_vector, a.cvss_v3_score, a.cvss_v4_vector, a.cvss_v4_score
FROM vuln_affected f
JOIN vuln_advisories a ON a.advisory_id = f.advisory_id AND `+visible("a")+`
WHERE f.ecosystem = @eco AND `+inList(db, "f.package_name", "pkgs")+` AND `+visible("f"), args).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("versioned catalog ranges: %w", err)
	}
	if len(rows) == 0 {
		return out, nil
	}

	advisoryIDs := make([]string, 0, len(rows))
	seenAdvisory := map[string]bool{}
	for _, r := range rows {
		if !seenAdvisory[r.AdvisoryID] {
			seenAdvisory[r.AdvisoryID] = true
			advisoryIDs = append(advisoryIDs, r.AdvisoryID)
		}
	}
	var refRows []catalogRefRow
	if err := db.Raw(`
SELECT r.advisory_id, r.ref_id, r.relation, r.ref_kind
FROM vuln_advisory_refs r
WHERE `+inList(db, "r.advisory_id", "ids")+` AND `+visible("r"),
		map[string]interface{}{"gen": gen, "ids": listArg(db, advisoryIDs)}).Scan(&refRows).Error; err != nil {
		return nil, fmt.Errorf("versioned catalog refs: %w", err)
	}
	refs := make(map[string][]loader.AdvisoryRef, len(advisoryIDs))
	for _, r := range refRows {
		refs[r.AdvisoryID] = append(refs[r.AdvisoryID], loader.AdvisoryRef{RefID: r.RefID, Relation: r.Relation, RefKind: r.RefKind})
	}

	markMalicious(rows, refs)

	vulnIDs := make(map[string][]string, len(advisoryIDs))
	var cveIDs []string
	seenCVE := map[string]bool{}
	for _, r := range rows {
		if _, ok := vulnIDs[r.AdvisoryID]; ok {
			continue
		}
		if r.Kind == "malware" {
			// A malicious package is reported under its own advisory, never a CVE or GHSA alias.
			vulnIDs[r.AdvisoryID] = []string{r.AdvisoryID}
			continue
		}
		ids := loader.CanonicalVulnIDs(r.AdvisoryID, r.Source, refs[r.AdvisoryID])
		vulnIDs[r.AdvisoryID] = ids
		for _, id := range ids {
			if strings.HasPrefix(id, "CVE-") && !seenCVE[id] {
				seenCVE[id] = true
				cveIDs = append(cveIDs, id)
			}
		}
	}
	nvd, others, err := m.cveLevelScores(ctx, gen, cveIDs)
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		introduced := strings.TrimSpace(r.Introduced)
		if introduced == "0" {
			introduced = ""
		}
		fixed := strings.TrimSpace(r.Fixed)
		constraint := buildOSVRangeConstraint(introduced, fixed, strings.TrimSpace(r.LastAffected))
		ids := vulnIDs[r.AdvisoryID]
		for _, id := range ids {
			c := &cve.CVE{
				ID:           id,
				Description:  firstNonEmpty(r.Details, r.Summary),
				Constraint:   constraint,
				FixedVersion: fixed,
				Release:      strings.TrimSpace(r.Release),
				AdvisoryID:   r.AdvisoryID,
				Kind:         r.Kind,
			}
			if r.PublishedAt != nil {
				c.Published = *r.PublishedAt
			}
			if r.ModifiedAt != nil {
				c.Modified = *r.ModifiedAt
			}
			rateCatalogCandidate(c, r, len(ids) > 1, nvd[id], others[id])
			out[r.PackageName] = append(out[r.PackageName], c)
		}
	}
	return out, nil
}

// markMalicious marks as malware the advisories that describe a malicious package without
// being one of its MAL-* entries: those a MAL advisory aliases (a PYSEC or GHSA about a
// compromised release) and those pointing to a MAL advisory (a Go typosquat entry). They are
// the same finding as the MAL advisory, not a vulnerability of the package.
func markMalicious(rows []catalogRangeRow, refs map[string][]loader.AdvisoryRef) {
	malicious := map[string]bool{}
	for _, r := range rows {
		if r.Kind != "malware" {
			continue
		}
		for _, ref := range refs[r.AdvisoryID] {
			if ref.Relation == "alias" {
				malicious[ref.RefID] = true
			}
		}
	}
	for id, rs := range refs {
		for _, ref := range rs {
			if strings.HasPrefix(ref.RefID, "MAL-") {
				malicious[id] = true
			}
		}
	}
	for i := range rows {
		if malicious[rows[i].AdvisoryID] {
			rows[i].Kind = "malware"
		}
	}
}

// rateCatalogCandidate sets the severity of one range from the best source available to it.
func rateCatalogCandidate(c *cve.CVE, r catalogRangeRow, erratum bool, nvd, other cveScoreRow) {
	set := func(tier int, severity string, score float64, vector string) {
		c.SeverityTier = tier
		c.SeveritySource = severitySourceNames[tier]
		c.Severity = reportedSeverity(severity)
		c.CVSSScore = score
		c.CVSSVector = vector
	}
	ownScore, ownVector := advisoryScore(r)
	if rating := firstNonEmpty(cvss.NormalizeRating(r.VendorSeverity), cvss.NormalizeRating(r.SourceSeverity)); rating != "" {
		set(SeverityTierVendor, rating, ownScore, ownVector)
		return
	}
	if ownScore > 0 && !erratum {
		set(SeverityTierAdvisoryCVSS, cvss.SeverityFromScore(ownScore), ownScore, ownVector)
		return
	}
	if nvd.Score != nil && *nvd.Score > 0 {
		set(SeverityTierNVD, cvss.SeverityFromScore(*nvd.Score), *nvd.Score, nvd.Vector)
		return
	}
	if other.Score != nil && *other.Score > 0 {
		set(SeverityTierCVECVSS, cvss.SeverityFromScore(*other.Score), *other.Score, other.Vector)
		return
	}
	if ownScore > 0 {
		set(SeverityTierErratumCVSS, cvss.SeverityFromScore(ownScore), ownScore, ownVector)
		return
	}
	set(SeverityTierDefault, cvss.Medium, 0, "")
}

// reportedSeverity keeps findings on the four levels the rest of Fortuna knows: a negligible
// rating (Debian "unimportant", CVSS 0) is reported as LOW.
func reportedSeverity(s string) string {
	if s == cvss.Negligible {
		return cvss.Low
	}
	return s
}

func advisoryScore(r catalogRangeRow) (float64, string) {
	if r.CVSSV3Score != nil && *r.CVSSV3Score > 0 {
		return *r.CVSSV3Score, r.CVSSV3Vector
	}
	if r.CVSSV4Score != nil && *r.CVSSV4Score > 0 {
		return *r.CVSSV4Score, r.CVSSV4Vector
	}
	return 0, ""
}

// cveLevelScores loads, per CVE, the NVD score from the vulnerabilities table and the highest
// score among the advisories that describe only that CVE.
func (m *Manager) cveLevelScores(ctx context.Context, gen uint, cveIDs []string) (map[string]cveScoreRow, map[string]cveScoreRow, error) {
	nvd := map[string]cveScoreRow{}
	others := map[string]cveScoreRow{}
	if len(cveIDs) == 0 {
		return nvd, others, nil
	}
	db := m.postgresDB.WithContext(ctx)
	args := map[string]interface{}{"gen": gen, "ids": listArg(db, cveIDs)}

	var nvdRows []cveScoreRow
	if err := db.Raw(`
SELECT vuln_id,
       CAST(COALESCE(NULLIF(nvd_cvss_v3_score, 0), nvd_cvss_v4_score) AS DOUBLE PRECISION) AS score,
       CASE WHEN COALESCE(nvd_cvss_v3_score, 0) > 0 THEN nvd_cvss_v3_vector ELSE nvd_cvss_v4_vector END AS vector
FROM vulnerabilities WHERE `+inList(db, "vuln_id", "ids"), args).Scan(&nvdRows).Error; err != nil {
		return nil, nil, fmt.Errorf("vulnerabilities NVD scores: %w", err)
	}
	for _, r := range nvdRows {
		nvd[r.VulnID] = r
	}

	var otherRows []cveScoreRow
	if err := db.Raw(`
WITH about AS (
  SELECT r.ref_id AS vuln_id, r.advisory_id
  FROM vuln_advisory_refs r
  WHERE `+inList(db, "r.ref_id", "ids")+` AND r.ref_kind = 'cve' AND r.relation IN ('alias', 'upstream') AND `+visible("r")+`
    AND NOT EXISTS (
      SELECT 1 FROM vuln_advisory_refs x
      WHERE x.advisory_id = r.advisory_id AND x.ref_kind = 'cve' AND x.relation IN ('alias', 'upstream')
        AND x.ref_id <> r.ref_id AND `+visible("x")+`)
  UNION
  SELECT a.advisory_id, a.advisory_id FROM vuln_advisories a WHERE `+inList(db, "a.advisory_id", "ids")+` AND `+visible("a")+`
), scored AS (
  SELECT b.vuln_id,
         CAST(COALESCE(NULLIF(a.cvss_v3_score, 0), a.cvss_v4_score) AS DOUBLE PRECISION) AS score,
         CASE WHEN COALESCE(a.cvss_v3_score, 0) > 0 THEN a.cvss_v3_vector ELSE a.cvss_v4_vector END AS vector
  FROM about b JOIN vuln_advisories a ON a.advisory_id = b.advisory_id AND `+visible("a")+`
)
SELECT vuln_id, score, vector
FROM scored WHERE score > 0
ORDER BY vuln_id, score DESC, vector`, args).Scan(&otherRows).Error; err != nil {
		return nil, nil, fmt.Errorf("per-CVE advisory scores: %w", err)
	}
	// Rows come best first per CVE; keep the first.
	for _, r := range otherRows {
		if _, ok := others[r.VulnID]; !ok {
			others[r.VulnID] = r
		}
	}
	return nvd, others, nil
}

// getFromVersionedCatalog serves GetVulnerabilitiesForPackages from the versioned catalog of
// generation gen.
func (m *Manager) getFromVersionedCatalog(ctx context.Context, gen uint, ecosystem, eco string, packages []string) (map[string][]*cve.CVE, error) {
	result := make(map[string][]*cve.CVE, len(packages))
	cacheKey := func(pkg string) string { return fmt.Sprintf("%s:%s:vcat2-%d", ecosystem, pkg, gen) }
	uncached := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if cached, ok := m.cache.Get(cacheKey(pkg)); ok {
			result[pkg] = cached
			continue
		}
		uncached = append(uncached, pkg)
	}
	if len(uncached) == 0 {
		return result, nil
	}
	found, err := m.queryVersionedCatalogBulk(ctx, gen, eco, uncached)
	if err != nil {
		return nil, err
	}
	for _, pkg := range uncached {
		cves := found[pkg]
		if cves == nil {
			cves = []*cve.CVE{}
		}
		m.cache.Set(cacheKey(pkg), cves)
		result[pkg] = cves
	}
	m.logger.Printf("✅ Bulk query returned CVEs for %d packages (versioned catalog generation=%d ecosystem=%s, cached %d)",
		len(packages), gen, eco, len(packages)-len(uncached))
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
