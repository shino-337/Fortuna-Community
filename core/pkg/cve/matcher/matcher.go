package matcher

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Matcher matches CVEs against SBOM components
type Matcher struct {
	dbManager   *database.Manager
	comparator  *VersionComparator
	db          *gorm.DB
	logger      *log.Logger
}

// NewMatcher creates a new CVE matcher
func NewMatcher(
	dbManager *database.Manager,
	db *gorm.DB,
) *Matcher {
	return &Matcher{
		dbManager:  dbManager,
		comparator: NewVersionComparator(),
		db:         db,
		logger:     log.New(log.Writer(), "[CVEMatcher] ", log.LstdFlags),
	}
}

// MatchSBOM matches CVEs against an SBOM. If componentsOverride is non-nil, use it (P1-5 snapshot);
// otherwise load components from DB. This avoids race when components are soft-deleted after event publish.
func (m *Matcher) MatchSBOM(
	ctx context.Context,
	sbom *models.SBOM,
	componentsOverride []*models.SBOMComponent,
) ([]*models.CVEMatch, error) {
	m.logger.Printf("Matching CVEs for SBOM ID %d (%d packages)", sbom.ID, sbom.PackageCount)

	var components []models.SBOMComponent
	if componentsOverride != nil {
		for _, c := range componentsOverride {
			components = append(components, *c)
		}
		m.logger.Printf("Using %d components from snapshot (P1-5)", len(components))
	} else {
		if err := m.db.WithContext(ctx).
			Where("sbom_id = ? AND deleted_at IS NULL", sbom.ID).
			Find(&components).Error; err != nil {
			return nil, fmt.Errorf("failed to load SBOM components: %w", err)
		}
	}

	m.logger.Printf("Found %d components to match", len(components))

	matches := make([]*models.CVEMatch, 0)

	// OPTIMIZATION: Group components by ecosystem and query CVEs in bulk.
	// Additionally, for Kubernetes control-plane components (kube-apiserver, coredns, etcd, …),
	// use component → module prefix mapping (P2-2) to query OSV Go instead of NVD keyword heuristic.
	ecosystemPackages := make(map[string][]string)
	componentsByName := make(map[string]*models.SBOMComponent)
	purlsByName := make(map[string]*PURL)

	// Optional: load K8s component → module prefix map once per MatchSBOM invocation.
	k8sMap, k8sMapErr := cve.LoadK8sComponentMap()
	if k8sMapErr != nil {
		m.logger.Printf("⚠️  Failed to load K8s component map: %v (will continue without mapping)", k8sMapErr)
		k8sMap = nil
	}

	for i := range components {
		component := &components[i]

		// Skip junk distroless heuristic components early
		if isDistrolessHeuristicJunk(component) {
			continue
		}

		// 1. Parse PURL
		purl, err := ParsePURL(component.PURL)
		if err != nil {
			m.logger.Printf("⚠️  Failed to parse PURL %s: %v", component.PURL, err)
			continue
		}

		// Normalize ecosystem for DB queries (use OS to map generic→distro for OSV match)
		queryEcosystem := normalizeQueryEcosystemWithOS(purl, sbom.OSName)

		componentKey := component.ComponentName

		// If K8s component mapping is available, try to map control-plane component → Go module prefix.
		if k8sMap != nil {
			normName := strings.ToLower(strings.TrimSpace(component.ComponentName))
			if mapping, ok := k8sMap[normName]; ok && mapping.ModulePrefix != "" {
				// Build a synthetic PURL-like view for the Go module.
				moduleName := mapping.ModulePrefix
				version := strings.TrimSpace(component.ComponentVersion)
				if mapping.Ecosystem == "go" && version != "" && !strings.HasPrefix(version, "v") {
					version = "v" + version
				}

				// We reuse the existing bulk query path by treating modulePrefix as package name
				// in the "go" ecosystem (OSV Go mirror).
				queryEcosystem = "go"
				purl.Name = moduleName
				purl.Ecosystem = "go"
				// Note: we keep component.ComponentName/version as-is for persisted matches.
				componentKey = moduleName
			}
		}

		// Group by ecosystem using the (possibly remapped) name.
		ecosystemPackages[queryEcosystem] = append(ecosystemPackages[queryEcosystem], componentKey)
		componentsByName[componentKey] = component
		purlsByName[componentKey] = purl
	}

	// 2. Bulk query CVEs for all packages per ecosystem
	for ecosystem, packageNames := range ecosystemPackages {
		m.logger.Printf("Bulk querying CVEs for %d packages in ecosystem %s", len(packageNames), ecosystem)

		packageCVEs, err := m.dbManager.GetVulnerabilitiesForPackages(ctx, ecosystem, packageNames)
		if err != nil {
			m.logger.Printf("⚠️  Failed to bulk query CVEs for ecosystem %s: %v", ecosystem, err)
			continue
		}

		totalCVEs := 0
		for _, cves := range packageCVEs {
			totalCVEs += len(cves)
		}
		m.logger.Printf("Found %d total CVEs for %d packages in ecosystem %s", totalCVEs, len(packageNames), ecosystem)

		// 3. Process each package's CVEs
		for pkgName, cves := range packageCVEs {
			component := componentsByName[pkgName]
			purl := purlsByName[pkgName]

			if component == nil || purl == nil {
				continue
			}

			// Check version constraints for each CVE
			for _, cveData := range cves {
				vulnerable, err := m.comparator.IsVulnerable(
					component.ComponentVersion,
					cveData.Constraint,
					purl.Ecosystem,
				)
				if err != nil {
					m.logger.Printf("⚠️  Version comparison failed for %s: %v", component.ComponentName, err)
					continue
				}

				if !vulnerable {
					continue // Not vulnerable
				}

				// 4. Create match
				match := &models.CVEMatch{
					SBOMID:         sbom.ID,
					PodUID:         sbom.PodUID,
					ContainerName:  sbom.ContainerName,
					CVEID:          cveData.ID,
					PackageName:    component.ComponentName,
					PackageVersion: component.ComponentVersion,
					PURL:           component.PURL,
					Severity:       strings.ToUpper(cveData.Severity),
					CVSS:           float32(cveData.CVSSScore), // Convert to float32
					FixedVersion:   cveData.FixedVersion,
					MatchedBy:      "fortuna-core-cve-matcher",
					MatchedAt:      component.CreatedAt,
				}

				matches = append(matches, match)
			}
		}
	}

	// 2b. NVD fallback for heuristic SBOMs when postgres/OSV returned 0 (Finding #8.4 / DISTROLESS_SBOM_SPEC)
	if useNVDFallbackForHeuristic(sbom) {
		matchedNames := make(map[string]bool)
		for _, match := range matches {
			matchedNames[match.PackageName] = true
		}
		for i := range components {
			component := &components[i]
			if matchedNames[component.ComponentName] {
				continue
			}
			purl := purlsByName[component.ComponentName]
			if purl == nil {
				continue
			}
			queryEco := normalizeQueryEcosystemWithOS(purl, sbom.OSName)
			nvdName := normalizeComponentNameForNVD(component.ComponentName) // so whitelist + NVD keyword match (e.g. registry.k8s.io/coredns → coredns)
			tryNVD := isNVDFallbackWhitelisted(component.ComponentName)
			cves, err := m.dbManager.GetVulnerabilitiesForPackageWithOptions(ctx, queryEco, nvdName, component.ComponentVersion, &database.QueryOptions{TryNVDFallback: tryNVD})
			if err != nil || len(cves) == 0 {
				continue
			}
			for _, cveData := range cves {
				var vulnerable bool
				if cveData.Constraint == "" {
					// NVD does not provide version ranges: treat as potentially affected so we persist and show on dashboard
					vulnerable = true
				} else {
					var errV error
					vulnerable, errV = m.comparator.IsVulnerable(component.ComponentVersion, cveData.Constraint, purl.Ecosystem)
					if errV != nil || !vulnerable {
						continue
					}
				}
				// Persist NVD CVE into cves table so dashboard Preload("CVE") and detail view work
				if cveData.Constraint == "" {
					if err := m.dbManager.EnsureCVEExists(ctx, cveData); err != nil {
						m.logger.Printf("⚠️  Failed to persist NVD CVE %s: %v", cveData.ID, err)
					}
				}
				matchedBy := "fortuna-core-cve-matcher"
				if cveData.Constraint == "" {
					matchedBy = "nvd-fallback"
				}
				matches = append(matches, &models.CVEMatch{
					SBOMID:         sbom.ID,
					PodUID:         sbom.PodUID,
					ContainerName:  sbom.ContainerName,
					CVEID:          cveData.ID,
					PackageName:    component.ComponentName,
					PackageVersion: component.ComponentVersion,
					PURL:           component.PURL,
					Severity:       strings.ToUpper(cveData.Severity),
					CVSS:           float32(cveData.CVSSScore),
					FixedVersion:   cveData.FixedVersion,
					MatchedBy:      matchedBy,
					MatchedAt:      component.CreatedAt,
				})
			}
		}
	}

	m.logger.Printf("✅ Found %d CVE matches for SBOM ID %d", len(matches), sbom.ID)
	return matches, nil
}

// useNVDFallbackForHeuristic returns true when SBOM is from distroless/heuristic and
// confidence is not high, so we should try NVD API when postgres/OSV has no match (Finding #8.4).
func useNVDFallbackForHeuristic(sbom *models.SBOM) bool {
	src := strings.ToLower(strings.TrimSpace(sbom.SbomSource))
	conf := strings.ToLower(strings.TrimSpace(sbom.Confidence))
	if src != "distroless-heuristic" && src != "label-metadata" {
		return false
	}
	return conf != "high"
}

// isDistrolessHeuristicJunk returns true for components that should be skipped entirely:
// version=unknown, source=distroless-heuristic, and name not in control-plane/runtime whitelist.
func isDistrolessHeuristicJunk(c *models.SBOMComponent) bool {
	if c == nil {
		return true
	}
	if strings.TrimSpace(c.ComponentVersion) != "unknown" {
		return false
	}
	if strings.TrimSpace(c.Source) != "distroless-heuristic" {
		return false
	}
	return !isNVDFallbackWhitelisted(c.ComponentName)
}

// registryCanonicalName maps known registry-style names (no slash) to canonical name for whitelist/NVD.
// e.g. "registry.k8s.io/coredns" → "coredns" (per SBOM_Flow_distroless_Remediation §4).
var registryCanonicalName = map[string]string{
	"registry.k8s.io/coredns": "coredns",
}

// normalizeComponentNameForNVD normalizes component name for whitelist check and NVD keyword search.
// e.g. "registry.k8s.io/coredns/coredns" → "coredns", "coredns/coredns" → "coredns" (per SBOM_Flow_distroless_Remediation §4).
func normalizeComponentNameForNVD(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return n
	}
	if idx := strings.LastIndex(n, "/"); idx >= 0 && idx < len(n)-1 {
		n = strings.TrimSpace(n[idx+1:])
	}
	if canonical, ok := registryCanonicalName[n]; ok {
		return canonical
	}
	return n
}

// isNVDFallbackWhitelisted returns true for control-plane and runtime names we allow
// for CVE matching and NVD fallback (openssl, glibc, kube-*, coredns, etcd, ...).
// Uses normalizeComponentNameForNVD so "coredns/coredns" and "registry.k8s.io/coredns" match.
func isNVDFallbackWhitelisted(name string) bool {
	n := normalizeComponentNameForNVD(name)
	n = strings.TrimSpace(n)
	if n == "" {
		return false
	}
	if strings.HasPrefix(n, "kube-") {
		return true
	}
	allowed := map[string]bool{
		"openssl": true, "libssl.so.3": true, "libssl.so.1.1": true,
		"glibc": true, "libc.so.6": true, "libc.so": true,
		"coredns": true, "etcd": true, "pause": true,
		"containerd-shim": true, "containerd-shim-runc-v1": true,
		"runc": true, "conntrack": true, "iptables": true,
	}
	if allowed[n] {
		return true
	}
	// Allow lib*.so* (e.g. libc.so.6 already above; other libs)
	if strings.HasPrefix(n, "lib") && (strings.Contains(n, ".so") || strings.HasSuffix(n, ".so")) {
		return true
	}
	return false
}

// normalizeQueryEcosystem maps PURL ecosystem/namespace into the ecosystem values
// stored in PostgreSQL by the OSV loader (e.g., debian/ubuntu/alpine/go).
func normalizeQueryEcosystem(p *PURL) string {
	return normalizeQueryEcosystemWithOS(p, "")
}

// normalizeQueryEcosystemWithOS is like normalizeQueryEcosystem but uses SBOM OS name
// to map generic components to the distro ecosystem when PURL is generic (e.g. from
// distroless/heuristic), so OSV Debian/Ubuntu data is matched.
func normalizeQueryEcosystemWithOS(p *PURL, sbomOSName string) string {
	if p == nil {
		return ""
	}
	eco := strings.ToLower(strings.TrimSpace(p.Ecosystem))
	ns := strings.ToLower(strings.TrimSpace(p.Namespace))
	osName := strings.ToLower(strings.TrimSpace(sbomOSName))

	switch eco {
	case "deb", "package_type_dpkg", "package_type_deb":
		// OSV loader stores ecosystem as distro (debian/ubuntu)
		// Handle both "deb", "package_type_dpkg", and "package_type_deb" formats
		if ns != "" {
			return ns
		}
		return "debian"
	case "apk", "package_type_apk":
		// OSV loader stores "alpine"
		// Handle both "apk" and "package_type_apk" formats
		if ns != "" {
			return ns
		}
		return "alpine"
	case "rpm", "package_type_rpm":
		// Use namespace if present (e.g., centos/redhat)
		if ns != "" {
			return ns
		}
		return "linux" // Most RPM vulnerabilities are in "linux" ecosystem
	case "golang", "go":
		// OSV loader normalizes to "go"
		return "go"
	case "generic":
		// Heuristic/distroless components: use SBOM OS to query OSV distro data
		if strings.Contains(osName, "debian") {
			return "debian"
		}
		if strings.Contains(osName, "ubuntu") {
			return "ubuntu"
		}
		if strings.Contains(osName, "alpine") {
			return "alpine"
		}
		return "generic"
	default:
		// For unknown ecosystems, try to use namespace or return as-is
		if ns != "" {
			return ns
		}
		return eco
	}
}

// FilterBySeverity filters matches by severity
func (m *Matcher) FilterBySeverity(
	matches []*models.CVEMatch,
	severities []string,
) []*models.CVEMatch {
	if len(severities) == 0 {
		return matches // No filter
	}

	filtered := make([]*models.CVEMatch, 0)
	severityMap := make(map[string]bool)
	for _, sev := range severities {
		severityMap[strings.ToUpper(sev)] = true
	}

	for _, match := range matches {
		if severityMap[strings.ToUpper(match.Severity)] {
			filtered = append(filtered, match)
		}
	}

	return filtered
}

