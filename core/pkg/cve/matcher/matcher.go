package matcher

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/models"
	"github.com/hashicorp/go-version"
	"gorm.io/gorm"
)

var goStrictSemver = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
var goPseudoVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+-(0\.)?\d{14}-[0-9a-f]{7,}$`)
var goLooseSemver = regexp.MustCompile(`^v?\d+\.\d+$`)

// Matcher matches CVEs against SBOM components
type Matcher struct {
	dbManager   *database.Manager
	comparator  *VersionComparator
	db          *gorm.DB
	logger      *log.Logger
	// cache for OSV mirror lookups: module -> vulnerabilities
	osvCache map[string][]models.OSVVulnerability
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
		osvCache:   make(map[string][]models.OSVVulnerability),
	}
}

// MatchSBOM matches CVEs against an SBOM. If componentsOverride is non-nil, use it (P1-5 snapshot);
// otherwise load components from DB. This avoids race when components are soft-deleted after event publish.
func (m *Matcher) MatchSBOM(
	ctx context.Context,
	sbom *models.SBOM,
	componentsOverride []*models.SBOMComponent,
) ([]*models.CVEMatch, error) {
	// SBOM lifecycle: only match finalized SBOMs to avoid races with mutable components.
	status := strings.ToLower(strings.TrimSpace(sbom.Status))
	if status != "" && status != "finalized" {
		m.logger.Printf("Skipping CVE matching for SBOM ID %d: status=%q (only finalized SBOMs are matched)", sbom.ID, sbom.Status)
		return nil, nil
	}

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

	components = m.resolveComponentsForMatching(ctx, sbom, components)
	m.logger.Printf("Found %d components to match (after resolution)", len(components))

	matches := make([]*models.CVEMatch, 0)

	// OPTIMIZATION: Group components by ecosystem and query CVEs in bulk.
	// For Go: use prefix list + alias resolution (go_module_alias) so renames (e.g. coreos/etcd → go.etcd.io/etcd) still match.
	ecosystemPackages := make(map[string][]string)
	componentsByName := make(map[string]*models.SBOMComponent)
	purlsByName := make(map[string]*PURL)
	// Go-only: resolved module name -> list of (component, purl) to run version check for
	goResolvedToPairs := make(map[string][]struct {
		component *models.SBOMComponent
		purl      *PURL
	})
	goResolvedNamesSet := make(map[string]struct{})

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

		// For Go: use prefix list + alias resolution so OSV mirror lookup matches renames (e.g. github.com/coreos/etcd → go.etcd.io/etcd).
		if queryEcosystem == "go" {
			modulePath := strings.TrimSpace(purl.Name)
			prefixes := normalizeGoModulePrefixes(modulePath)
			for _, p := range prefixes {
				candidates := m.dbManager.ResolveGoModuleAliasCandidates(ctx, p)
				if len(candidates) == 0 {
					candidates = []string{p}
				}
				for _, resolved := range candidates {
					goResolvedNamesSet[resolved] = struct{}{}
					goResolvedToPairs[resolved] = append(goResolvedToPairs[resolved], struct {
						component *models.SBOMComponent
						purl      *PURL
					}{component, purl})
				}
			}
			continue
		}

		// Group by ecosystem using the (possibly remapped) name.
		ecosystemPackages[queryEcosystem] = append(ecosystemPackages[queryEcosystem], componentKey)
		componentsByName[componentKey] = component
		purlsByName[componentKey] = purl
	}

	// 2a. Go: bulk query by resolved names (prefix + alias), then run version check per (component, purl)
	if len(goResolvedNamesSet) > 0 {
		allResolved := make([]string, 0, len(goResolvedNamesSet))
		for n := range goResolvedNamesSet {
			allResolved = append(allResolved, n)
		}
		m.logger.Printf("Bulk querying Go CVEs for %d resolved module names (prefix+alias)", len(allResolved))
		packageCVEs, err := m.dbManager.GetVulnerabilitiesForPackages(ctx, "go", allResolved)
		if err != nil {
			m.logger.Printf("⚠️  Failed to bulk query Go CVEs: %v", err)
		} else {
			seenMatch := make(map[string]map[string]bool) // componentDedupKey -> CVEID -> true
			for resolvedName, cves := range packageCVEs {
				pairs := goResolvedToPairs[resolvedName]
				for _, pair := range pairs {
					comp := pair.component
					purl := pair.purl
					dedupKey := comp.PURL + "|" + comp.ComponentVersion + "|" + comp.ComponentName
					if seenMatch[dedupKey] == nil {
						seenMatch[dedupKey] = make(map[string]bool)
					}
					for _, cveData := range cves {
						if seenMatch[dedupKey][cveData.ID] {
							continue
						}
						vulnerable, err := m.comparator.IsVulnerable(comp.ComponentVersion, cveData.Constraint, purl.Ecosystem)
						if err != nil {
							continue
						}
						if !vulnerable {
							continue
						}
						seenMatch[dedupKey][cveData.ID] = true
						matches = append(matches, &models.CVEMatch{
							SBOMID:         sbom.ID,
							PodUID:         sbom.PodUID,
							ContainerName:  sbom.ContainerName,
							CVEID:          cveData.ID,
							PackageName:    comp.ComponentName,
							PackageVersion: comp.ComponentVersion,
							PURL:           comp.PURL,
							Severity:       strings.ToUpper(cveData.Severity),
							CVSS:           float32(cveData.CVSSScore),
							FixedVersion:   cveData.FixedVersion,
							MatchedBy:      "fortuna-core-cve-matcher",
							MatchedAt:      comp.CreatedAt,
						})
					}
				}
			}
		}
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

	// Go stdlib matcher (P2-x): match vulnerabilities based on sbom.GoVersion and OSV mirror stdlib entries.
	m.matchGoStdlib(ctx, sbom, &matches)

	// Deterministic output contract: sorted by package identity, then version, then CVE ID.
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].PackageName != matches[j].PackageName {
			return matches[i].PackageName < matches[j].PackageName
		}
		if matches[i].PackageVersion != matches[j].PackageVersion {
			return matches[i].PackageVersion < matches[j].PackageVersion
		}
		if matches[i].CVEID != matches[j].CVEID {
			return matches[i].CVEID < matches[j].CVEID
		}
		return matches[i].PURL < matches[j].PURL
	})

	return matches, nil
}

// resolveComponentsForMatching applies noise-reduction and multi-source conflict resolution.
// It is deterministic and does not mutate DB state.
func (m *Matcher) resolveComponentsForMatching(
	ctx context.Context,
	sbom *models.SBOM,
	components []models.SBOMComponent,
) []models.SBOMComponent {
	type candidate struct {
		c       models.SBOMComponent
		p       *PURL
		eco     string
		key     string
		pri     int
	}

	versionClass := func(eco string, v string) int {
		eco = strings.ToLower(strings.TrimSpace(eco))
		v = strings.TrimSpace(v)
		if eco == "go" {
			// STRICT: vX.Y.Z or pseudo-version; LOOSE: vX.Y; INVALID otherwise.
			if goStrictSemver.MatchString(v) || goPseudoVersion.MatchString(v) {
				return 2
			}
			if goLooseSemver.MatchString(v) {
				return 1
			}
			return 0
		}
		return 1
	}

	trustRank := func(tl string) int {
		switch strings.ToLower(strings.TrimSpace(tl)) {
		case "high":
			return 2
		case "medium":
			return 1
		case "low":
			return 0
		default:
			return 2
		}
	}

	semverForSort := func(eco string, v string) *version.Version {
		eco = strings.ToLower(strings.TrimSpace(eco))
		if eco != "go" && eco != "npm" && eco != "pypi" && eco != "generic" {
			return nil
		}
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "v") {
			v = strings.TrimPrefix(v, "v")
		}
		ver, err := version.NewVersion(v)
		if err != nil {
			return nil
		}
		return ver
	}

	priority := func(src string) int {
		s := strings.ToLower(strings.TrimSpace(src))
		switch s {
		case "gobinary":
			return 100
		case "gomod":
			return 80
		case "os", "dpkg", "apk", "rpm":
			return 60
		case "distroless-heuristic", "label-metadata", "heuristic":
			return 20
		case "gobinary-main":
			return 0 // non-matchable
		default:
			// Unknown sources: keep but low priority
			return 10
		}
	}

	cands := make([]candidate, 0, len(components))
	var highCount, mediumCount, lowCount int
	hasNonLow := false
	for i := range components {
		c := components[i]

		// Hard skip: main Go binary is inventory noise, not matchable (spec).
		if strings.EqualFold(strings.TrimSpace(c.Source), "gobinary-main") {
			continue
		}

		p, err := ParsePURL(c.PURL)
		if err != nil || p == nil {
			// Keep behavior: unparseable PURL not matchable
			metrics.MatcherComponentsShadowedTotal.WithLabelValues("invalid").Inc()
			continue
		}
		tl := strings.ToLower(strings.TrimSpace(c.TrustLevel))
		if tl == "" {
			tl = "high"
		}
		if tl == "high" || tl == "medium" {
			hasNonLow = true
		}
		switch tl {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		default:
			lowCount++
		}

		eco := normalizeQueryEcosystemWithOS(p, sbom.OSName)
		// Canonical identity key:
		// - go: full module path
		// - distro ecosystems: include namespace if present
		// - generic: name
		nameKey := strings.TrimSpace(p.Name)
		if eco != "go" {
			ns := strings.TrimSpace(p.Namespace)
			arch := ""
			if p.Qualifiers != nil {
				arch = strings.TrimSpace(p.Qualifiers["arch"])
			}
			if ns != "" && arch != "" {
				nameKey = ns + ":" + arch + "/" + nameKey
			} else if ns != "" {
				nameKey = ns + "/" + nameKey
			} else if arch != "" {
				nameKey = ":" + arch + "/" + nameKey
			}
		}
		key := eco + ":" + strings.ToLower(nameKey)

		cands = append(cands, candidate{
			c:   c,
			p:   p,
			eco: eco,
			key: key,
			pri: priority(c.Source),
		})
	}

	metrics.MatcherComponentsTotal.Add(float64(len(cands)))
	metrics.MatcherInvocationsTotal.Inc()
	metrics.MatcherCandidatesTotal.Add(float64(len(cands)))
	metrics.MatcherCandidatesLowTrustTotal.Add(float64(lowCount))
	if len(cands) > 0 {
		metrics.MatcherLowTrustRatio.Set(float64(lowCount) / float64(len(cands)))
	}

	// Graceful degradation: if we have any HIGH/MED components, drop LOW trust ones.
	// If everything is LOW (e.g. distroless-only heuristic), allow matching in fallback mode.
	if hasNonLow {
		filtered := cands[:0]
		var skippedLow int
		for _, cand := range cands {
			tl := strings.ToLower(strings.TrimSpace(cand.c.TrustLevel))
			if tl == "" {
				tl = "high"
			}
			if tl == "low" {
				skippedLow++
				continue
			}
			filtered = append(filtered, cand)
		}
		cands = filtered
		if skippedLow > 0 {
			metrics.MatcherComponentsSkippedLowTrustTotal.Add(float64(skippedLow))
		}
		m.logger.Printf("[MatcherTrust] sbom_id=%d high=%d medium=%d low=%d mode=normal", sbom.ID, highCount, mediumCount, lowCount)
	} else {
		metrics.MatcherComponentsFallbackModeTotal.Inc()
		metrics.MatcherFallbackInvocationsTotal.Inc()
		metrics.MatcherFallbackRatio.Set(1.0)
		m.logger.Printf("[MatcherTrust] sbom_id=%d high=%d medium=%d low=%d mode=fallback", sbom.ID, highCount, mediumCount, lowCount)

		// Fallback explosion guard: if all components are LOW trust and there are too many,
		// limit the candidate set deterministically to reduce blast radius.
		const fallbackLimit = 50
		if len(cands) > fallbackLimit {
			sort.Slice(cands, func(i, j int) bool {
				vi := versionClass(cands[i].eco, cands[i].p.Version)
				vj := versionClass(cands[j].eco, cands[j].p.Version)
				if vi != vj {
					return vi > vj
				}
				if cands[i].key != cands[j].key {
					return cands[i].key < cands[j].key
				}
				return cands[i].c.PURL < cands[j].c.PURL
			})
			dropped := len(cands) - fallbackLimit
			cands = cands[:fallbackLimit]
			metrics.MatcherComponentsFallbackLimitedTotal.Add(float64(dropped))
		}
	}
	if hasNonLow {
		metrics.MatcherFallbackRatio.Set(0.0)
	}

	// Group by canonical key, pick a single winner per key.
	groups := make(map[string]candidate)
	for _, cand := range cands {
		cur, ok := groups[cand.key]
		if !ok {
			groups[cand.key] = cand
			continue
		}
		// Deterministic winner selection:
		// priority DESC, trust DESC, version DESC (semantic where possible), purl ASC.
		if cand.pri != cur.pri {
			if cand.pri > cur.pri {
				metrics.MatcherComponentsShadowedTotal.WithLabelValues("priority").Inc()
				groups[cand.key] = cand
			} else {
				metrics.MatcherComponentsShadowedTotal.WithLabelValues("priority").Inc()
			}
			continue
		}
		tr1, tr2 := trustRank(cand.c.TrustLevel), trustRank(cur.c.TrustLevel)
		if tr1 != tr2 {
			if tr1 > tr2 {
				metrics.MatcherComponentsShadowedTotal.WithLabelValues("conflict").Inc()
				groups[cand.key] = cand
			} else {
				metrics.MatcherComponentsShadowedTotal.WithLabelValues("conflict").Inc()
			}
			continue
		}
		v1, v2 := semverForSort(cand.eco, cand.p.Version), semverForSort(cur.eco, cur.p.Version)
		if v1 != nil && v2 != nil {
			if v1.GreaterThan(v2) {
				metrics.MatcherComponentsShadowedTotal.WithLabelValues("conflict").Inc()
				groups[cand.key] = cand
				continue
			}
			if v2.GreaterThan(v1) {
				metrics.MatcherComponentsShadowedTotal.WithLabelValues("conflict").Inc()
				continue
			}
		} else if v1 == nil && v2 == nil {
			// Explicit tie-break when semver parsing fails: version string ASC.
			if cand.p.Version != cur.p.Version {
				if cand.p.Version < cur.p.Version {
					metrics.MatcherComponentsShadowedTotal.WithLabelValues("conflict").Inc()
					groups[cand.key] = cand
				} else {
					metrics.MatcherComponentsShadowedTotal.WithLabelValues("conflict").Inc()
				}
				continue
			}
		}
		// Final tie-breaker: stable by PURL string (ASC)
		if cand.c.PURL < cur.c.PURL {
			metrics.MatcherComponentsShadowedTotal.WithLabelValues("conflict").Inc()
			groups[cand.key] = cand
		} else {
			metrics.MatcherComponentsShadowedTotal.WithLabelValues("conflict").Inc()
		}
	}

	// Deterministic output order + duplicate collapse (eco,name,version).
	out := make([]models.SBOMComponent, 0, len(groups))
	for _, cand := range groups {
		out = append(out, cand.c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PURL != out[j].PURL {
			return out[i].PURL < out[j].PURL
		}
		return out[i].ComponentName < out[j].ComponentName
	})
	seen := make(map[string]bool)
	deduped := out[:0]
	for _, c := range out {
		p, err := ParsePURL(c.PURL)
		if err != nil || p == nil {
			continue
		}
		eco := normalizeQueryEcosystemWithOS(p, sbom.OSName)
		ns := strings.ToLower(strings.TrimSpace(p.Namespace))
		arch := ""
		if p.Qualifiers != nil {
			arch = strings.ToLower(strings.TrimSpace(p.Qualifiers["arch"]))
		}
		k := eco + ":" + ns + ":" + arch + ":" + strings.ToLower(strings.TrimSpace(p.Name)) + "@" + strings.TrimSpace(p.Version)
		if seen[k] {
			metrics.MatcherComponentsShadowedTotal.WithLabelValues("duplicate").Inc()
			continue
		}
		seen[k] = true
		deduped = append(deduped, c)
	}
	out = deduped
	return out
}

// normalizeGoModulePrefixes returns candidate Go module prefixes for a given module path.
// Example: "k8s.io/kubernetes/cmd/kube-apiserver" ->
// ["k8s.io/kubernetes/cmd/kube-apiserver", "k8s.io/kubernetes/cmd", "k8s.io/kubernetes"].
// It stops when there are fewer than 3 segments (github.com/org/repo) to avoid matching non-modules (e.g. "k8s.io").
func normalizeGoModulePrefixes(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return []string{path}
	}

	// Minimum module depth varies by host:
	// - github.com/org/repo (3 segments) is the common minimum for VCS hosts
	// - k8s.io/kubernetes (2 segments) is a real Go module used by OSV
	minParts := 2
	host := strings.ToLower(parts[0])
	switch host {
	case "github.com", "gitlab.com", "bitbucket.org":
		minParts = 3
	}

	out := make([]string, 0, len(parts))
	for i := len(parts); i >= minParts; i-- {
		prefix := strings.Join(parts[:i], "/")
		out = append(out, prefix)

		// Preserve major version modules like /v4: stop after emitting ".../v4".
		if i >= minParts && isGoMajorVersionSegment(parts[i-1]) {
			break
		}
	}
	return out
}

func isGoMajorVersionSegment(seg string) bool {
	seg = strings.TrimSpace(seg)
	if len(seg) < 2 || seg[0] != 'v' {
		return false
	}
	for i := 1; i < len(seg); i++ {
		if seg[i] < '0' || seg[i] > '9' {
			return false
		}
	}
	return true
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

// registryCanonicalName maps known registry-style names to canonical product/component name for whitelist/NVD.
// e.g. "registry.k8s.io/coredns" → "coredns" (per SBOM_Flow_distroless_Remediation §4).
var registryCanonicalName = map[string]string{
	"registry.k8s.io/coredns": "coredns",
	"registry.k8s.io/etcd":    "etcd",
}

// normalizeComponentNameForNVD normalizes component name for whitelist check and NVD keyword search.
// e.g. "registry.k8s.io/coredns/coredns" → "coredns", "coredns/coredns" → "coredns" (per SBOM_Flow_distroless_Remediation §4).
// It also normalizes some known control-plane aliases (e.g. "kubernetes-apiserver" → "kube-apiserver")
// so that CPE product matching can rely on canonical names.
func normalizeComponentNameForNVD(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return n
	}
	if idx := strings.LastIndex(n, "/"); idx >= 0 && idx < len(n)-1 {
		n = strings.TrimSpace(n[idx+1:])
	}
	// Normalize known control-plane aliases to canonical component names
	switch strings.ToLower(n) {
	case "kubernetes-apiserver", "k8s-apiserver", "apiserver":
		return "kube-apiserver"
	case "kubernetes-controller-manager", "k8s-controller-manager":
		return "kube-controller-manager"
	case "kubernetes-scheduler", "k8s-scheduler":
		return "kube-scheduler"
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

