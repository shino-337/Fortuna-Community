package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/mirror/osv"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Manager manages CVE database access: Postgres (OSV / package_vulnerabilities).
// Current SBOM path uses Postgres only; Trivy is not used.
type Manager struct {
	postgresDB *gorm.DB
	source     string // "postgres" (OSV in DB)
	cache      *CVECache
	logger     *log.Logger
}

// nvdEnrichTarget pairs an in-memory CVE (from OSV mirror) with a CVE-* id for batch enrichment from the central cves table.
type nvdEnrichTarget struct {
	cveID string
	out   *cve.CVE
}

// NewPostgresManager creates a manager that queries PostgreSQL tables
// populated by the OSV loader (cves + package_vulnerabilities).
func NewPostgresManager(db *gorm.DB) *Manager {
	return &Manager{
		postgresDB: db,
		source:     "postgres",
		cache:      NewCVECache(),
		logger:     log.New(log.Writer(), "[CVEDatabaseManager] ", log.LstdFlags),
	}
}

type osvMirrorRow struct {
	VulnID       string
	Summary      string
	Details      string
	Severity     string
	CVSSScore    float64
	Aliases      string
	PackageName  string
	RangeType    string
	Introduced   string
	Fixed        string
	LastAffected string
}

// GetGoStdlibVulns returns CVEs for Go stdlib based on OSV mirror (package_name = "stdlib").
func (m *Manager) GetGoStdlibVulns(ctx context.Context) ([]*cve.CVE, error) {
	pkgMap, err := m.GetVulnerabilitiesForPackages(ctx, "go", []string{"stdlib"})
	if err != nil {
		return nil, err
	}
	return pkgMap["stdlib"], nil
}

// GetVulnerabilitiesForPackage gets CVEs for a package (no options).
func (m *Manager) GetVulnerabilitiesForPackage(
	ctx context.Context,
	ecosystem string,
	name string,
	version string,
) ([]*cve.CVE, error) {
	// 1. Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%s", ecosystem, name, version)
	if cached, ok := m.cache.Get(cacheKey); ok {
		m.logger.Printf("✅ Cache hit for %s:%s@%s", ecosystem, name, version)
		return cached, nil
	}

	if m.source == "postgres" && m.postgresDB != nil {
		cves, err := m.queryPostgres(ctx, ecosystem, name)
		if err != nil {
			m.logger.Printf("⚠️  PostgreSQL CVE query failed: %v", err)
			return nil, err
		}

		m.cache.Set(cacheKey, cves)
		m.logger.Printf("✅ Found %d candidate CVEs (postgres) for %s:%s@%s", len(cves), ecosystem, name, version)
		return cves, nil
	}

	return nil, fmt.Errorf("CVE source must be postgres (OSV)")
}

// GetVulnsByPackageFromOSVMirror returns OSV-based vulnerabilities for a given ecosystem/package
// using osv_vulnerabilities + osv_packages + osv_ranges mirror tables (P2-7, Go ecosystem first).
// Version filtering is NOT applied here; caller (matcher) must apply semver logic using VersionComparator.
func (m *Manager) GetVulnsByPackageFromOSVMirror(
	ctx context.Context,
	ecosystem string,
	packageName string,
) ([]models.OSVVulnerability, []models.OSVRange, error) {
	if m.postgresDB == nil {
		return nil, nil, fmt.Errorf("postgres required for OSV mirror queries")
	}
	eco := strings.ToLower(strings.TrimSpace(ecosystem))
	pkg := strings.TrimSpace(packageName)
	if eco == "" || pkg == "" {
		return nil, nil, nil
	}

	var vulns []models.OSVVulnerability
	var ranges []models.OSVRange
	activeGenerationID := m.activeCVEGenerationID(ctx)
	useGenerationScope := activeGenerationID > 0 && m.hasOSVMirrorForGeneration(ctx, activeGenerationID)

	// Join osv_packages -> osv_vulnerabilities to get vuln metadata.
	vulnQuery := m.postgresDB.WithContext(ctx).
		Joins("JOIN osv_packages p ON p.vuln_id = osv_vulnerabilities.id").
		Where("p.ecosystem = ? AND p.package_name = ?", eco, pkg)
	if useGenerationScope {
		vulnQuery = vulnQuery.Where("p.catalog_generation_id = ? AND osv_vulnerabilities.catalog_generation_id = ?", activeGenerationID, activeGenerationID)
	}
	if err := vulnQuery.Find(&vulns).Error; err != nil {
		return nil, nil, fmt.Errorf("query OSV mirror vulnerabilities: %w", err)
	}

	if len(vulns) == 0 {
		return nil, nil, nil
	}

	// Fetch all ranges for these packages.
	var pkgs []models.OSVPackage
	pkgQuery := m.postgresDB.WithContext(ctx).Where("ecosystem = ? AND package_name = ?", eco, pkg)
	if useGenerationScope {
		pkgQuery = pkgQuery.Where("catalog_generation_id = ?", activeGenerationID)
	}
	if err := pkgQuery.Find(&pkgs).Error; err != nil {
		return nil, nil, fmt.Errorf("query OSV mirror packages: %w", err)
	}
	if len(pkgs) == 0 {
		return vulns, nil, nil
	}

	pkgIDs := make([]uint, 0, len(pkgs))
	for _, p := range pkgs {
		pkgIDs = append(pkgIDs, p.ID)
	}

	rangeQuery := m.postgresDB.WithContext(ctx).Where("package_id IN ?", pkgIDs)
	if useGenerationScope {
		rangeQuery = rangeQuery.Where("catalog_generation_id = ?", activeGenerationID)
	}
	if err := rangeQuery.Find(&ranges).Error; err != nil {
		return nil, nil, fmt.Errorf("query OSV mirror ranges: %w", err)
	}

	return vulns, ranges, nil
}

// GetVulnerabilitiesForPackages gets CVEs for multiple packages in bulk (OPTIMIZATION)
// Returns a map of package name -> CVEs.
// Cache key includes mirror_state version when using OSV mirror so that sync bumps version → cache miss automatically.
func (m *Manager) GetVulnerabilitiesForPackages(
	ctx context.Context,
	ecosystem string,
	packages []string, // Package names only
) (map[string][]*cve.CVE, error) {
	if m.source != "postgres" || m.postgresDB == nil {
		return nil, fmt.Errorf("postgres required for bulk CVE query (SBOM flow)")
	}

	eco := strings.ToLower(strings.TrimSpace(ecosystem))
	useOSVMirror := m.hasOSVMirrorTables() && useOSVMirrorForEcosystem(eco)
	cacheSuffix := "*"
	if useOSVMirror {
		if ver := m.getMirrorVersion(ctx, "osv"); ver != "" {
			cacheSuffix = ver
		}
	}
	if nvdVer := m.getMirrorVersion(ctx, "nvd"); nvdVer != "" {
		cacheSuffix += ":" + nvdVer
	}
	activeGenerationID := m.activeCVEGenerationID(ctx)
	if activeGenerationID > 0 && m.hasPackageVulnsForGeneration(ctx, activeGenerationID) {
		cacheSuffix += fmt.Sprintf(":cvegen-%d", activeGenerationID)
	}

	// Check cache first
	result := make(map[string][]*cve.CVE)
	uncachedPackages := make([]string, 0, len(packages))

	for _, pkg := range packages {
		cacheKey := fmt.Sprintf("%s:%s:%s", ecosystem, pkg, cacheSuffix)
		if cached, ok := m.cache.Get(cacheKey); ok {
			result[pkg] = cached
			metrics.OSVMirrorCacheHitsTotal.WithLabelValues(eco).Inc()
		} else {
			uncachedPackages = append(uncachedPackages, pkg)
		}
	}

	if len(uncachedPackages) == 0 {
		m.logger.Printf("✅ Bulk cache hit for all %d packages", len(packages))
		return result, nil
	}

	// Bulk query PostgreSQL for uncached packages

	// P2-7: OSV mirror for Go ecosystem (preferred for go modules) when tables exist.
	if useOSVMirror {
		metrics.OSVMirrorQueriesTotal.WithLabelValues(eco).Inc()
		packageCVEs, err := m.queryOSVMirrorBulk(ctx, eco, uncachedPackages)
		if err != nil {
			return nil, err
		}
		if pvCVEs, err := m.queryPostgresPackageVulnsBulk(ctx, []string{eco}, uncachedPackages); err != nil {
			m.logger.Printf("⚠️  package_vulnerabilities fallback merge: %v", err)
		} else {
			for _, pkg := range uncachedPackages {
				packageCVEs[pkg] = mergeCVEByIDUnique(packageCVEs[pkg], pvCVEs[pkg])
			}
		}
		// CPE-scoped supplemental rows live under ecosystem "nvd" in package_vulnerabilities. Distro OSV
		// queries (alpine, debian, …) would otherwise see 0 CVEs when osv_packages is empty and never merge these.
		if isDistroPackageEcosystemForCPESupplement(eco) {
			if nvdSupp, err := m.queryPostgresPackageVulnsBulk(ctx, []string{"nvd"}, uncachedPackages); err != nil {
				m.logger.Printf("⚠️  CPE supplement merge (package_vulnerabilities): %v", err)
			} else {
				for _, pkg := range uncachedPackages {
					packageCVEs[pkg] = mergeCVEByIDUnique(packageCVEs[pkg], nvdSupp[pkg])
				}
			}
		}
		for _, pkg := range uncachedPackages {
			cves := packageCVEs[pkg]
			if cves == nil {
				cves = []*cve.CVE{}
			}
			cacheKey := fmt.Sprintf("%s:%s:%s", ecosystem, pkg, cacheSuffix)
			m.cache.Set(cacheKey, cves)
			result[pkg] = cves
		}
		m.logger.Printf("✅ Bulk query returned CVEs for %d packages (OSV mirror ecosystem=%s)", len(packages), eco)
		return result, nil
	}

	packageCVEs, err := m.queryPostgresPackageVulnsBulk(ctx, []string{eco}, uncachedPackages)
	if err != nil {
		return nil, fmt.Errorf("bulk query postgres: %w", err)
	}

	// Cache and add to result (non-OSV path: cache suffix "*")
	for _, pkg := range uncachedPackages {
		cves := packageCVEs[pkg]
		if cves == nil {
			cves = []*cve.CVE{} // Empty slice for packages with no CVEs
		}

		cacheKey := fmt.Sprintf("%s:%s:*", ecosystem, pkg)
		m.cache.Set(cacheKey, cves)
		result[pkg] = cves
	}

	m.logger.Printf("✅ Bulk query returned CVEs for %d packages (queried %d, cached %d)",
		len(packages), len(uncachedPackages), len(packages)-len(uncachedPackages))

	return result, nil
}

// hasOSVMirrorTables reports whether OSV mirror tables exist.
func (m *Manager) hasOSVMirrorTables() bool {
	if m.postgresDB == nil {
		return false
	}
	// Works for Postgres and sqlite test DBs.
	return m.postgresDB.Migrator().HasTable("osv_vulnerabilities") &&
		m.postgresDB.Migrator().HasTable("osv_packages") &&
		m.postgresDB.Migrator().HasTable("osv_ranges")
}

// ResolveGoModuleAlias returns the canonical Go module path for OSV lookup (exact match only).
// If name is in go_module_alias (e.g. github.com/coreos/etcd), returns canonical (e.g. go.etcd.io/etcd); otherwise returns name unchanged.
func (m *Manager) ResolveGoModuleAlias(ctx context.Context, name string) string {
	candidates := m.ResolveGoModuleAliasCandidates(ctx, name)
	if len(candidates) == 0 {
		return name
	}
	// Prefer exact canonical; otherwise first candidate (canonical base)
	return candidates[0]
}

// ResolveGoModuleAliasCandidates returns all names to try for OSV lookup: exact match canonical + prefix-match canonicals.
// E.g. github.com/coreos/etcd/client/v3 with alias github.com/coreos/etcd -> go.etcd.io/etcd yields [go.etcd.io/etcd, go.etcd.io/etcd/client/v3]
// so that OSV mirror (which may list only go.etcd.io/etcd) is queried correctly.
func (m *Manager) ResolveGoModuleAliasCandidates(ctx context.Context, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" || m.postgresDB == nil {
		return nil
	}
	if !m.postgresDB.Migrator().HasTable("go_module_alias") {
		return nil
	}
	var rows []models.GoModuleAlias
	if err := m.postgresDB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil
	}
	// Exact match
	for _, r := range rows {
		if r.Alias == name && r.Canonical != "" {
			return []string{r.Canonical}
		}
	}
	// Prefix match: alias is a prefix of name -> add canonical and canonical+suffix
	var out []string
	for _, r := range rows {
		if r.Alias == "" || r.Canonical == "" {
			continue
		}
		if name == r.Alias || strings.HasPrefix(name, r.Alias+"/") {
			suffix := strings.TrimPrefix(name, r.Alias)
			out = append(out, r.Canonical, r.Canonical+suffix)
			break // longest match first would require sort; one alias match is enough for etcd
		}
	}
	return out
}

func (m *Manager) queryOSVMirrorBulk(ctx context.Context, ecosystem string, packages []string) (map[string][]*cve.CVE, error) {
	if m.postgresDB == nil {
		return nil, fmt.Errorf("postgres required for OSV mirror bulk query")
	}
	if !m.hasOSVMirrorTables() {
		out := make(map[string][]*cve.CVE, len(packages))
		for _, pkg := range packages {
			out[pkg] = []*cve.CVE{}
		}
		return out, nil
	}
	eco := strings.ToLower(strings.TrimSpace(ecosystem))
	if eco == "" || len(packages) == 0 {
		return map[string][]*cve.CVE{}, nil
	}
	var rows []osvMirrorRow
	activeGenerationID := m.activeCVEGenerationID(ctx)
	useGenerationScope := activeGenerationID > 0 && m.hasOSVMirrorForGeneration(ctx, activeGenerationID)
	// Single join query then group in Go (production-friendly).
	query := `
SELECT
  v.id AS vuln_id,
  v.summary,
  v.details,
  v.severity,
  v.cvss_score,
  v.aliases,
  p.package_name,
  r.range_type,
  r.introduced,
  r.fixed,
  r.last_affected
FROM osv_packages p
JOIN osv_vulnerabilities v ON v.id = p.vuln_id
JOIN osv_ranges r ON r.package_id = p.id
WHERE p.ecosystem = ? AND p.package_name IN ?
  AND UPPER(TRIM(r.range_type)) IN ('SEMVER', 'ECOSYSTEM')
`
	args := []interface{}{eco, packages}
	if useGenerationScope {
		query += `
  AND p.catalog_generation_id = ?
  AND v.catalog_generation_id = ?
  AND r.catalog_generation_id = ?
`
		args = append(args, activeGenerationID, activeGenerationID, activeGenerationID)
	}
	if err := m.postgresDB.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("OSV mirror bulk join query: %w", err)
	}

	out := make(map[string][]*cve.CVE)
	var nvdEnrichTargets []nvdEnrichTarget
	for _, row := range rows {
		if row.VulnID == "" || row.PackageName == "" {
			continue
		}
		rt := strings.ToUpper(strings.TrimSpace(row.RangeType))
		switch rt {
		case "SEMVER":
			// all mirrored ecosystems
		case "ECOSYSTEM":
			// Go modules use SEMVER in OSV; ECOSYSTEM rows for go are skipped at ingest and ignored here.
			if eco == "go" || !isDistroOSVEcosystem(eco) {
				continue
			}
		default:
			continue
		}
		introduced := strings.TrimSpace(row.Introduced)
		if introduced == "0" {
			introduced = ""
		}
		fixed := strings.TrimSpace(row.Fixed)
		lastAffected := strings.TrimSpace(row.LastAffected)
		constraint := buildOSVRangeConstraint(introduced, fixed, lastAffected)

		cveObj := &cve.CVE{
			ID:           row.VulnID,
			Description:  row.Details,
			Severity:     row.Severity,
			CVSSScore:    row.CVSSScore,
			CVSSVector:   "",
			Constraint:   constraint,
			FixedVersion: fixed,
			Published:    time.Time{},
			Modified:     time.Time{},
			References:   nil,
		}

		// OSV ↔ CVE catalog: if OSV has CVE alias, enrich from the central `cves` table when present.
		if strings.TrimSpace(row.Aliases) != "" {
			var aliases []string
			if err := json.Unmarshal([]byte(row.Aliases), &aliases); err == nil {
				for _, a := range aliases {
					a = strings.TrimSpace(a)
					if strings.HasPrefix(strings.ToUpper(a), "CVE-") {
						cveObj.ID = a
						nvdEnrichTargets = append(nvdEnrichTargets, nvdEnrichTarget{cveID: a, out: cveObj})
						break
					}
				}
			}
		}

		out[row.PackageName] = append(out[row.PackageName], cveObj)
	}

	m.batchEnrichCVEsFromNVD(ctx, nvdEnrichTargets)

	// Ensure all packages are present in map
	for _, pkg := range packages {
		if _, ok := out[pkg]; !ok {
			out[pkg] = []*cve.CVE{}
		}
	}
	return out, nil
}

// batchEnrichCVEsFromNVD loads `cves` rows for many CVE IDs in one query (avoids N+1 in OSV bulk path).
func (m *Manager) batchEnrichCVEsFromNVD(ctx context.Context, targets []nvdEnrichTarget) {
	if len(targets) == 0 || m.postgresDB == nil {
		return
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, t := range targets {
		id := strings.TrimSpace(t.cveID)
		if id == "" || t.out == nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	var dbRows []models.CVE
	if err := m.postgresDB.WithContext(ctx).Where("cve_id IN ? AND deleted_at IS NULL", ids).Find(&dbRows).Error; err != nil {
		return
	}
	byID := make(map[string]models.CVE, len(dbRows))
	for i := range dbRows {
		byID[dbRows[i].CVEID] = dbRows[i]
	}
	for _, t := range targets {
		id := strings.TrimSpace(t.cveID)
		if id == "" || t.out == nil {
			continue
		}
		if dbCVE, ok := byID[id]; ok {
			applyNVDRowToCVE(&dbCVE, t.out)
		}
	}
}

func (m *Manager) queryPostgres(ctx context.Context, ecosystem, name string) ([]*cve.CVE, error) {
	eco := strings.ToLower(strings.TrimSpace(ecosystem))
	pkg := strings.TrimSpace(name)
	if pkg == "" {
		return []*cve.CVE{}, nil
	}

	out, err := m.queryPostgresPackageVulnsForPackage(ctx, eco, pkg)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 && isDistroPackageEcosystemForCPESupplement(eco) {
		nvdOut, err := m.queryPostgresPackageVulnsForPackage(ctx, "nvd", pkg)
		if err != nil {
			return out, nil
		}
		out = mergeCVEByIDUnique(out, nvdOut)
	}
	return out, nil
}

func (m *Manager) queryPostgresPackageVulnsForPackage(ctx context.Context, ecosystem, packageName string) ([]*cve.CVE, error) {
	eco := strings.ToLower(strings.TrimSpace(ecosystem))
	pkg := strings.TrimSpace(packageName)
	if m.postgresDB == nil || pkg == "" {
		return []*cve.CVE{}, nil
	}
	var rows []models.PackageVulnerability
	if err := m.postgresDB.WithContext(ctx).
		Where("ecosystem = ? AND package_name = ? AND deleted_at IS NULL", eco, pkg).
		Preload("CVE", "deleted_at IS NULL").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*cve.CVE, 0, len(rows))
	for _, pv := range rows {
		if c := packageVulnerabilityToCVE(pv); c != nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// queryPostgresPackageVulnsBulk loads package_vulnerabilities for many packages and ecosystems (e.g. nvd supplement for distro OSV).
func (m *Manager) queryPostgresPackageVulnsBulk(ctx context.Context, ecosystems []string, packages []string) (map[string][]*cve.CVE, error) {
	out := make(map[string][]*cve.CVE)
	if m.postgresDB == nil || len(packages) == 0 || len(ecosystems) == 0 {
		return out, nil
	}
	query := m.postgresDB.WithContext(ctx).
		Where("ecosystem IN ? AND package_name IN ? AND deleted_at IS NULL", ecosystems, packages)
	if activeGenerationID := m.activeCVEGenerationID(ctx); activeGenerationID > 0 && m.hasPackageVulnsForGeneration(ctx, activeGenerationID) {
		query = query.Where("catalog_generation_id = ?", activeGenerationID)
	}
	var rows []models.PackageVulnerability
	if err := query.
		Preload("CVE", "deleted_at IS NULL").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, pv := range rows {
		if c := packageVulnerabilityToCVE(pv); c != nil {
			out[pv.PackageName] = append(out[pv.PackageName], c)
		}
	}
	return out, nil
}

func (m *Manager) activeCVEGenerationID(ctx context.Context) uint {
	if m.postgresDB == nil || !m.postgresDB.Migrator().HasTable("catalog_generations") {
		return 0
	}
	var generation models.CatalogGeneration
	if err := m.postgresDB.WithContext(ctx).
		Where("catalog_type = ? AND status = ?", "cve", "active").
		Order("activated_at DESC, id DESC").
		First(&generation).Error; err != nil {
		return 0
	}
	return generation.ID
}

func (m *Manager) hasPackageVulnsForGeneration(ctx context.Context, generationID uint) bool {
	if generationID == 0 || m.postgresDB == nil || !m.postgresDB.Migrator().HasTable("package_vulnerabilities") || !m.postgresDB.Migrator().HasColumn(&models.PackageVulnerability{}, "catalog_generation_id") {
		return false
	}
	var count int64
	if err := m.postgresDB.WithContext(ctx).
		Model(&models.PackageVulnerability{}).
		Where("catalog_generation_id = ? AND deleted_at IS NULL", generationID).
		Limit(1).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func (m *Manager) hasOSVMirrorForGeneration(ctx context.Context, generationID uint) bool {
	if generationID == 0 || m.postgresDB == nil || !m.postgresDB.Migrator().HasTable("osv_packages") || !m.postgresDB.Migrator().HasColumn(&models.OSVPackage{}, "catalog_generation_id") {
		return false
	}
	var count int64
	if err := m.postgresDB.WithContext(ctx).
		Model(&models.OSVPackage{}).
		Where("catalog_generation_id = ?", generationID).
		Limit(1).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func packageVulnerabilityToCVE(pv models.PackageVulnerability) *cve.CVE {
	if pv.CVEID == "" {
		return nil
	}
	constraint := buildConstraintFromPV(pv)
	if constraint == "" && pv.AffectedRange != "" {
		constraint = pv.AffectedRange
	}
	fixed := pv.FixedVersion
	if fixed == "" {
		fixed = pv.VersionEndExcluding
	}
	var published time.Time
	var modified time.Time
	if pv.CVE.PublishedDate != nil {
		published = *pv.CVE.PublishedDate
	}
	if pv.CVE.LastModifiedDate != nil {
		modified = *pv.CVE.LastModifiedDate
	}
	return &cve.CVE{
		ID:           pv.CVEID,
		Description:  pv.CVE.Description,
		Severity:     pv.CVE.Severity,
		CVSSScore:    pv.CVE.CVSSScore,
		CVSSVector:   pv.CVE.CVSSVector,
		Constraint:   constraint,
		FixedVersion: fixed,
		Published:    published,
		Modified:     modified,
		References:   nil,
	}
}

func mergeCVEByIDUnique(primary, extra []*cve.CVE) []*cve.CVE {
	seen := make(map[string]struct{}, len(primary)+len(extra))
	out := make([]*cve.CVE, 0, len(primary)+len(extra))
	for _, list := range [][]*cve.CVE{primary, extra} {
		for _, c := range list {
			if c == nil || c.ID == "" {
				continue
			}
			if _, ok := seen[c.ID]; ok {
				continue
			}
			seen[c.ID] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}

func buildConstraintFromPV(pv models.PackageVulnerability) string {
	parts := make([]string, 0, 4)

	// Start bounds
	if pv.VersionStartIncluding != "" && pv.VersionStartIncluding != "0" {
		parts = append(parts, ">="+pv.VersionStartIncluding)
	}
	if pv.VersionStartExcluding != "" && pv.VersionStartExcluding != "0" {
		parts = append(parts, ">"+pv.VersionStartExcluding)
	}

	// End bounds
	if pv.VersionEndIncluding != "" {
		parts = append(parts, "<="+pv.VersionEndIncluding)
	}
	if pv.VersionEndExcluding != "" {
		parts = append(parts, "<"+pv.VersionEndExcluding)
	}

	return strings.Join(parts, ", ")
}

// buildOSVRangeConstraint builds a matcher constraint string from OSV range fields (introduced/fixed/last_affected).
func buildOSVRangeConstraint(introduced, fixed, lastAffected string) string {
	parts := make([]string, 0, 2)
	if introduced != "" && introduced != "0" {
		parts = append(parts, ">="+introduced)
	}
	if fixed != "" {
		parts = append(parts, "<"+fixed)
	} else if lastAffected != "" {
		parts = append(parts, "<="+lastAffected)
	}
	return strings.Join(parts, ", ")
}

// EnrichCVEFromNVD merges CVE details from the `cves` table for a given OSV vulnerability ID when aliases reference CVE-* IDs.
// It uses OSVVulnerability.Aliases (JSON) to find a CVE-* alias, then loads severity/CVSS/metadata into cveOut.
func (m *Manager) EnrichCVEFromNVD(ctx context.Context, osvID string, cveOut *cve.CVE) {
	if cveOut == nil || m.postgresDB == nil || strings.TrimSpace(osvID) == "" {
		return
	}
	if !m.hasOSVMirrorTables() {
		return
	}

	var osv models.OSVVulnerability
	if err := m.postgresDB.WithContext(ctx).Where("id = ?", osvID).First(&osv).Error; err != nil {
		return
	}
	if strings.TrimSpace(osv.Aliases) == "" {
		return
	}

	var aliases []string
	if err := json.Unmarshal([]byte(osv.Aliases), &aliases); err != nil {
		return
	}
	for _, a := range aliases {
		a = strings.TrimSpace(a)
		if strings.HasPrefix(strings.ToUpper(a), "CVE-") {
			m.enrichCVEFromNVDByCVEID(ctx, a, cveOut)
			return
		}
	}
}

// enrichCVEFromNVDByCVEID merges details from cves table into cveOut when present.
func (m *Manager) enrichCVEFromNVDByCVEID(ctx context.Context, cveID string, cveOut *cve.CVE) {
	if cveOut == nil || m.postgresDB == nil || strings.TrimSpace(cveID) == "" {
		return
	}
	var dbCVE models.CVE
	if err := m.postgresDB.WithContext(ctx).
		Where("cve_id = ? AND deleted_at IS NULL", cveID).
		First(&dbCVE).Error; err != nil {
		return
	}
	applyNVDRowToCVE(&dbCVE, cveOut)
}

func applyNVDRowToCVE(dbCVE *models.CVE, cveOut *cve.CVE) {
	if dbCVE == nil || cveOut == nil {
		return
	}
	cveOut.Severity = dbCVE.Severity
	cveOut.CVSSScore = dbCVE.CVSSScore
	cveOut.CVSSVector = dbCVE.CVSSVector
	if dbCVE.PublishedDate != nil {
		cveOut.Published = *dbCVE.PublishedDate
	}
	if dbCVE.LastModifiedDate != nil {
		cveOut.Modified = *dbCVE.LastModifiedDate
	}
	if strings.TrimSpace(dbCVE.References) != "" {
		var refs []string
		if err := json.Unmarshal([]byte(dbCVE.References), &refs); err == nil {
			cveOut.References = refs
		}
	}
}

// EnsureCVEExists upserts a CVE into the cves table so that CVEMatch can reference it
// when the row is not yet present. Idempotent: if CVEID
// already exists (e.g. from OSV), the record is left unchanged.
func (m *Manager) EnsureCVEExists(ctx context.Context, cveData *cve.CVE) error {
	if m.postgresDB == nil || cveData == nil || cveData.ID == "" {
		return nil
	}
	refsJSON := ""
	if len(cveData.References) > 0 {
		b, _ := json.Marshal(cveData.References)
		refsJSON = string(b)
	}
	row := models.CVE{
		CVEID:            cveData.ID,
		CVSSScore:        cveData.CVSSScore,
		CVSSVector:       cveData.CVSSVector,
		Severity:         strings.ToUpper(cveData.Severity),
		Description:      cveData.Description,
		Source:           "nvd",
		References:       refsJSON,
		PublishedDate:    &cveData.Published,
		LastModifiedDate: &cveData.Modified,
	}
	return m.postgresDB.WithContext(ctx).Where("cve_id = ?", cveData.ID).FirstOrCreate(&row).Error
}

// getMirrorVersion returns the mirror_state version for name (e.g. "osv") for cache key.
// Returns "" if table/row missing so cache key falls back to "*" (no version).
func (m *Manager) getMirrorVersion(ctx context.Context, name string) string {
	if v := frozenMirrorVersionFromContext(ctx, name); v != "" {
		return v
	}
	if m.postgresDB == nil || strings.TrimSpace(name) == "" {
		return ""
	}
	if !m.postgresDB.Migrator().HasTable("mirror_state") {
		return ""
	}
	var row models.MirrorState
	if err := m.postgresDB.WithContext(ctx).Where("name = ?", name).First(&row).Error; err != nil {
		return ""
	}
	return fmt.Sprintf("%d", row.Version)
}

// GetMirrorVersion returns mirror_state.version for a mirror name (e.g. "osv").
// It returns "" when mirror_state is missing, so callers can fall back to a default.
func (m *Manager) GetMirrorVersion(ctx context.Context, name string) string {
	return m.getMirrorVersion(ctx, name)
}

// IncrementMirrorVersion bumps mirror_state.version for name so cache keys that include it miss (invalidate on sync).
func (m *Manager) IncrementMirrorVersion(ctx context.Context, name string) error {
	if m.postgresDB == nil || strings.TrimSpace(name) == "" {
		return fmt.Errorf("db or name required")
	}
	if !m.postgresDB.Migrator().HasTable("mirror_state") {
		return fmt.Errorf("mirror_state table missing")
	}
	// Upsert: create with version=1 if not exists, else increment
	var row models.MirrorState
	err := m.postgresDB.WithContext(ctx).Where("name = ?", name).First(&row).Error
	if err != nil {
		if err := m.postgresDB.WithContext(ctx).Create(&models.MirrorState{Name: name, Version: 1}).Error; err != nil {
			return err
		}
	} else {
		row.Version++
		if err := m.postgresDB.WithContext(ctx).Save(&row).Error; err != nil {
			return err
		}
	}
	if m.cache != nil {
		m.cache.Clear()
	}
	return nil
}

// UpdateDatabase runs OSV mirror sync (if configured), then bumps mirror_state version so cache invalidates.
// Call after external OSV sync or when FORTUNA_OSV_SOURCE_DIR is set (loads JSON from that dir).
func (m *Manager) UpdateDatabase(ctx context.Context) error {
	if m.postgresDB == nil {
		return fmt.Errorf("postgres required for UpdateDatabase")
	}
	// Optional: sync OSV from directory (env FORTUNA_OSV_SOURCE_DIR)
	if dir := strings.TrimSpace(os.Getenv("FORTUNA_OSV_SOURCE_DIR")); dir != "" {
		n, err := osv.LoadFromDir(ctx, m.postgresDB, dir)
		if err != nil {
			m.logger.Printf("⚠️  OSV sync from %q: %v", dir, err)
			// Continue to bump version so any partial load still invalidates cache
		} else {
			m.logger.Printf("✅ OSV sync: ingested %d documents from %q", n, dir)
		}
	}
	// Sync alias table: seed is idempotent in migration; no-op here or re-apply if we add file-based alias load later.
	// Bump mirror version so matcher cache (key includes version) misses and uses fresh data.
	if err := m.IncrementMirrorVersion(ctx, "osv"); err != nil {
		return fmt.Errorf("increment mirror version: %w", err)
	}
	m.logger.Printf("✅ UpdateDatabase: mirror_state osv version incremented")
	return nil
}

// cveCacheEntry holds cached CVEs and expiry time (P1-2: fix cache TTL bug)
type cveCacheEntry struct {
	cves      []*cve.CVE
	expiresAt time.Time
}

// CVECache provides in-memory caching for CVE queries with TTL (P1-2: Get() checks expiry)
type CVECache struct {
	cache      map[string]cveCacheEntry
	ttl        time.Duration
	maxEntries int // 0 = unlimited (not recommended for long-lived core)
}

const defaultCVECacheMaxEntries = 10000

// NewCVECache creates a new CVE cache (TTL 30m; max entries from FORTUNA_CVE_CACHE_MAX_ENTRIES or default).
func NewCVECache() *CVECache {
	max := defaultCVECacheMaxEntries
	if s := strings.TrimSpace(os.Getenv("FORTUNA_CVE_CACHE_MAX_ENTRIES")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			max = n
		}
	}
	return &CVECache{
		cache:      make(map[string]cveCacheEntry),
		ttl:        30 * time.Minute,
		maxEntries: max,
	}
}

// Get retrieves from cache; returns false if missing or expired (entry removed)
func (c *CVECache) Get(key string) ([]*cve.CVE, bool) {
	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.cache, key)
		return nil, false
	}
	return entry.cves, true
}

// Set stores in cache with TTL from now
func (c *CVECache) Set(key string, cves []*cve.CVE) {
	if c == nil {
		return
	}
	if _, exists := c.cache[key]; !exists && c.maxEntries > 0 && len(c.cache) >= c.maxEntries {
		c.evictOne()
	}
	c.cache[key] = cveCacheEntry{
		cves:      cves,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *CVECache) evictOne() {
	if c == nil || len(c.cache) == 0 {
		return
	}
	var victim string
	var victimExp time.Time
	first := true
	for k, v := range c.cache {
		if first || v.expiresAt.Before(victimExp) || (v.expiresAt.Equal(victimExp) && k < victim) {
			first = false
			victim = k
			victimExp = v.expiresAt
		}
	}
	if victim != "" {
		delete(c.cache, victim)
	}
}

// Clear removes all entries (e.g. after mirror sync so stale keys are not retained until TTL).
func (c *CVECache) Clear() {
	if c == nil {
		return
	}
	c.cache = make(map[string]cveCacheEntry)
}
