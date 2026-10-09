package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Manager reads the versioned vulnerability catalog (vuln_advisories, vuln_affected) for the
// SBOM matcher.
type Manager struct {
	postgresDB *gorm.DB
	source     string // "postgres" (OSV in DB)
	cache      *CVECache
	logger     *log.Logger
}

// NewPostgresManager creates a manager that queries the versioned catalog written by the
// vulnerability loader.
func NewPostgresManager(db *gorm.DB) *Manager {
	return &Manager{
		postgresDB: db,
		source:     "postgres",
		cache:      NewCVECache(),
		logger:     log.New(log.Writer(), "[CVEDatabaseManager] ", log.LstdFlags),
	}
}

// GetGoStdlibVulns returns the vulnerabilities of the Go standard library (package "stdlib").
func (m *Manager) GetGoStdlibVulns(ctx context.Context) ([]*cve.CVE, error) {
	pkgMap, err := m.GetVulnerabilitiesForPackages(ctx, "go", []string{"stdlib"})
	if err != nil {
		return nil, err
	}
	return pkgMap["stdlib"], nil
}

// GetVulnerabilitiesForPackages returns, per package name, the vulnerabilities of the active
// catalog generation that affect it. Without a loaded catalog every package gets none.
func (m *Manager) GetVulnerabilitiesForPackages(
	ctx context.Context,
	ecosystem string,
	packages []string, // Package names only
) (map[string][]*cve.CVE, error) {
	if m.source != "postgres" || m.postgresDB == nil {
		return nil, fmt.Errorf("postgres required for bulk CVE query (SBOM flow)")
	}

	eco := strings.ToLower(strings.TrimSpace(ecosystem))
	if gen := m.versionedCatalogGeneration(ctx); gen > 0 {
		return m.getFromVersionedCatalog(ctx, gen, ecosystem, eco, packages)
	}
	m.logger.Printf("⚠️  No vulnerability catalog generation is loaded; %d %s packages are matched against nothing", len(packages), eco)
	result := make(map[string][]*cve.CVE, len(packages))
	for _, pkg := range packages {
		result[pkg] = []*cve.CVE{}
	}
	return result, nil
}

// ResolveGoModuleAliasCandidates returns all names to try for a catalog lookup: exact match canonical + prefix-match canonicals.
// E.g. github.com/coreos/etcd/client/v3 with alias github.com/coreos/etcd -> go.etcd.io/etcd yields [go.etcd.io/etcd, go.etcd.io/etcd/client/v3]
// so that the catalog (which may list only go.etcd.io/etcd) is queried correctly.
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
	// Open interval (no fix yet) affected from the first version.
	if len(parts) == 0 {
		return ">=0"
	}
	return strings.Join(parts, ", ")
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
