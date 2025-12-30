package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/cve/database/nvd"
	"github.com/fortuna/core/pkg/cve/database/trivy"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Manager manages CVE database access (Trivy DB + NVD API)
// NOTE: Trivy DB is used as a DATA SOURCE only (read-only BoltDB file).
// We do NOT use Trivy tool/server - this is zero-dependency design.
type Manager struct {
	postgresDB *gorm.DB
	source     string // postgres | trivy | nvd
	trivyDB *trivy.Reader
	nvdAPI  *nvd.Client
	cache   *CVECache
	logger  *log.Logger
}

// NewManager creates a new CVE database manager
func NewManager(trivyDBPath string) (*Manager, error) {
	source := os.Getenv("FORTUNA_CVE_SOURCE")
	if source == "" {
		source = "postgres" // default to postgres (OSV JSON loaded into DB)
	}

	// Initialize Trivy DB reader (just data, not code!)
	trivyReader, err := trivy.NewReader(trivyDBPath)
	if err != nil {
		log.Printf("⚠️  Failed to open Trivy DB: %v (will use NVD API only)", err)
		trivyReader = nil
	}

	// Initialize NVD API client (fallback)
	nvdClient := nvd.NewClient()

	// Initialize cache
	cache := NewCVECache()

	return &Manager{
		source:  source,
		trivyDB: trivyReader,
		nvdAPI:  nvdClient,
		cache:   cache,
		logger:  log.New(log.Writer(), "[CVEDatabaseManager] ", log.LstdFlags),
	}, nil
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

// GetVulnerabilitiesForPackage gets CVEs for a package
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

	// If configured, query PostgreSQL as primary source
	if m.source == "postgres" && m.postgresDB != nil {
		cves, err := m.queryPostgres(ctx, ecosystem, name)
		if err != nil {
			m.logger.Printf("⚠️  PostgreSQL CVE query failed: %v", err)
			return nil, err
		}

		// Cache result
		m.cache.Set(cacheKey, cves)
		m.logger.Printf("✅ Found %d candidate CVEs (postgres) for %s:%s@%s", len(cves), ecosystem, name, version)
		return cves, nil
	}

	// 2. Query Trivy DB (primary source)
	var cves []*cve.CVE
	var err error

	if m.trivyDB != nil {
		cves, err = m.trivyDB.Query(ctx, ecosystem, name, version)
		if err != nil {
			m.logger.Printf("⚠️  Trivy DB query failed: %v", err)
		}
	}

	// 3. Fallback to NVD API if Trivy DB has no data
	if len(cves) == 0 {
		m.logger.Printf("🔄 Trivy DB returned no results, trying NVD API...")
		cves, err = m.nvdAPI.Query(ctx, ecosystem, name, version)
		if err != nil {
			m.logger.Printf("⚠️  NVD API query failed: %v", err)
			return nil, err
		}
	}

	// 4. Cache result
	m.cache.Set(cacheKey, cves)

	m.logger.Printf("✅ Found %d CVEs for %s:%s@%s", len(cves), ecosystem, name, version)
	return cves, nil
}

// GetVulnerabilitiesForPackages gets CVEs for multiple packages in bulk (OPTIMIZATION)
// Returns a map of package name -> CVEs
func (m *Manager) GetVulnerabilitiesForPackages(
	ctx context.Context,
	ecosystem string,
	packages []string, // Package names only
) (map[string][]*cve.CVE, error) {
	if m.source != "postgres" || m.postgresDB == nil {
		// Fallback to individual queries for non-postgres sources
		result := make(map[string][]*cve.CVE)
		for _, pkg := range packages {
			cves, err := m.GetVulnerabilitiesForPackage(ctx, ecosystem, pkg, "")
			if err != nil {
				m.logger.Printf("⚠️  Failed to query CVEs for %s: %v", pkg, err)
				continue
			}
			result[pkg] = cves
		}
		return result, nil
	}

	// Check cache first
	result := make(map[string][]*cve.CVE)
	uncachedPackages := make([]string, 0, len(packages))

	for _, pkg := range packages {
		cacheKey := fmt.Sprintf("%s:%s:*", ecosystem, pkg)
		if cached, ok := m.cache.Get(cacheKey); ok {
			result[pkg] = cached
		} else {
			uncachedPackages = append(uncachedPackages, pkg)
		}
	}

	if len(uncachedPackages) == 0 {
		m.logger.Printf("✅ Bulk cache hit for all %d packages", len(packages))
		return result, nil
	}

	// Bulk query PostgreSQL for uncached packages
	eco := strings.ToLower(strings.TrimSpace(ecosystem))

	var rows []models.PackageVulnerability
	if err := m.postgresDB.WithContext(ctx).
		Where("ecosystem = ? AND package_name IN ? AND deleted_at IS NULL", eco, uncachedPackages).
		Preload("CVE", "deleted_at IS NULL").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("bulk query postgres: %w", err)
	}

	// Group CVEs by package name
	packageCVEs := make(map[string][]*cve.CVE)
	for _, pv := range rows {
		if pv.CVEID == "" || pv.PackageName == "" {
			continue
		}

		// Build CVE object
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

		cveObj := &cve.CVE{
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

		packageCVEs[pv.PackageName] = append(packageCVEs[pv.PackageName], cveObj)
	}

	// Cache and add to result
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

func (m *Manager) queryPostgres(ctx context.Context, ecosystem, name string) ([]*cve.CVE, error) {
	eco := strings.ToLower(strings.TrimSpace(ecosystem))
	pkg := strings.TrimSpace(name)
	if pkg == "" {
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
		if pv.CVEID == "" {
			continue
		}

		// Build constraint string for matcher to evaluate
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

		out = append(out, &cve.CVE{
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
		})
	}

	return out, nil
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

// UpdateDatabase updates the CVE database
func (m *Manager) UpdateDatabase(ctx context.Context) error {
	// This would be called by a CronJob to update Trivy DB
	// For now, just log
	m.logger.Printf("Database update requested (not implemented yet)")
	return nil
}

// CVECache provides in-memory caching for CVE queries
type CVECache struct {
	cache map[string][]*cve.CVE
	ttl   time.Duration
}

// NewCVECache creates a new CVE cache
func NewCVECache() *CVECache {
	return &CVECache{
		cache: make(map[string][]*cve.CVE),
		ttl:   1 * time.Hour, // 1 hour TTL
	}
}

// Get retrieves from cache
func (c *CVECache) Get(key string) ([]*cve.CVE, bool) {
	// For simplicity, no TTL checking (can be enhanced)
	val, ok := c.cache[key]
	return val, ok
}

// Set stores in cache
func (c *CVECache) Set(key string, cves []*cve.CVE) {
	c.cache[key] = cves
}

