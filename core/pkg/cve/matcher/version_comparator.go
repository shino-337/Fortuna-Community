package matcher

import (
	"fmt"
	"log"
	"strings"

	debversion "github.com/knqyf263/go-deb-version"
	rpmversion "github.com/knqyf263/go-rpm-version"
	"github.com/hashicorp/go-version"
)

// VersionComparator compares versions for different ecosystems
type VersionComparator struct {
	logger *log.Logger
}

// NewVersionComparator creates a new version comparator
func NewVersionComparator() *VersionComparator {
	return &VersionComparator{
		logger: log.New(log.Writer(), "[VersionComparator] ", log.LstdFlags),
	}
}

// IsVulnerable checks if installed version matches the constraint (vulnerable)
func (vc *VersionComparator) IsVulnerable(
	installedVersion string,
	constraint string,
	ecosystem string,
) (bool, error) {
	if constraint == "" {
		return false, nil // No constraint = not vulnerable
	}

	// Normalize ecosystem name (handle PACKAGE_TYPE_* formats)
	normalizedEco := strings.ToLower(strings.TrimSpace(ecosystem))
	if strings.HasPrefix(normalizedEco, "package_type_") {
		// Extract ecosystem from PACKAGE_TYPE_DEB -> deb, PACKAGE_TYPE_APK -> apk, etc.
		normalizedEco = strings.TrimPrefix(normalizedEco, "package_type_")
	}

	switch normalizedEco {
	case "deb", "debian", "ubuntu":
		return vc.compareDebianVersion(installedVersion, constraint)
	case "rpm", "redhat", "centos":
		return vc.compareRPMVersion(installedVersion, constraint)
	case "apk", "alpine":
		return vc.compareAlpineVersion(installedVersion, constraint)
	case "npm", "pypi", "go":
		return vc.compareSemver(installedVersion, constraint)
	case "generic":
		// Distroless/control-plane (kube-apiserver, coredns, etc.): version from tag/label/digestMap, use semver (e.g. v1.29.0)
		return vc.compareSemver(installedVersion, constraint)
	default:
		// D5: For unknown ecosystems, avoid skipping by returning a best-effort match.
		// We try semver constraint evaluation first; if parsing fails, fall back to a conservative "not vulnerable".
		// This keeps the matcher deterministic and prevents unnecessary CVE drop.
		ok, err := vc.compareSemver(installedVersion, constraint)
		if err == nil {
			return ok, nil
		}

		// Extra conservative fallback: if constraint looks like an exact "==X", do string equality on cleaned versions.
		c := strings.TrimSpace(constraint)
		if strings.HasPrefix(c, "==") {
			target := strings.TrimSpace(c[2:])
			installedClean := vc.cleanVersion(installedVersion)
			targetClean := vc.cleanVersion(target)
			if installedClean == targetClean {
				return true, nil
			}
			return false, nil
		}

		return false, nil
	}
}

// compareDebianVersion compares Debian package versions using go-deb-version library
// Format: [epoch:]upstream_version[-debian_revision]
// Example: 1:1.1.1d-0+deb10u7, 2.12.7+dfsg+really2.9.14-2.1+deb13u2
// Uses github.com/knqyf263/go-deb-version per ADR-001 (no self-implementation)
func (vc *VersionComparator) compareDebianVersion(
	installed string,
	constraint string,
) (bool, error) {
	// Parse installed version using go-deb-version
	v1, err := debversion.NewVersion(installed)
	if err != nil {
		return false, fmt.Errorf("failed to parse Debian version %s: %w", installed, err)
	}

	// Support multi-part constraints like: ">= 1.0, < 2.0"
	parts := strings.Split(constraint, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse constraint operator
		op, targetVersionStr := vc.parseConstraint(part)
		if targetVersionStr == "" {
			// No target version -> treat as not vulnerable for this constraint
			return false, nil
		}

		// Parse target version using go-deb-version
		v2, err := debversion.NewVersion(targetVersionStr)
		if err != nil {
			vc.logger.Printf("⚠️  Failed to parse constraint version %s: %v", targetVersionStr, err)
			// If constraint version can't be parsed, skip this constraint part
			continue
		}

		// Compare versions using go-deb-version
		// Compare returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
		cmp := v1.Compare(v2)

		// Apply operator; all parts must match
		ok := false
		switch op {
		case "<":
			ok = cmp < 0
		case "<=":
			ok = cmp <= 0
		case ">":
			ok = cmp > 0
		case ">=":
			ok = cmp >= 0
		case "==":
			ok = cmp == 0
		default:
			return false, fmt.Errorf("unknown operator: %s", op)
		}

		if !ok {
			// This constraint part doesn't match
			return false, nil
		}
	}

	// All constraint parts matched
	return true, nil
}

// Removed: dpkgCompareVersions, splitDebianVersion, compareDebianVersionPart
// These functions are no longer needed as we use go-deb-version library per ADR-001

// compareRPMVersion compares RPM package versions using go-rpm-version (rpmvercmp logic).
// Format: [epoch:]version-release, e.g. 1:1.1.1k-5.el8
func (vc *VersionComparator) compareRPMVersion(
	installed string,
	constraint string,
) (bool, error) {
	v1 := rpmversion.NewVersion(installed)

	// Multi-part constraints: ">= 1.0, < 2.0"
	parts := strings.Split(constraint, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		op, targetStr := vc.parseConstraint(part)
		if targetStr == "" {
			return false, nil
		}
		v2 := rpmversion.NewVersion(targetStr)

		ok := false
		switch op {
		case "<":
			ok = v1.LessThan(v2)
		case "<=":
			ok = v1.LessThan(v2) || v1.Equal(v2)
		case ">":
			ok = v1.GreaterThan(v2)
		case ">=":
			ok = v1.GreaterThan(v2) || v1.Equal(v2)
		case "==":
			ok = v1.Equal(v2)
		default:
			return false, fmt.Errorf("unknown operator: %s", op)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// compareAlpineVersion compares Alpine package versions
// Format: version-rN
// Example: 1.1.1g-r0
func (vc *VersionComparator) compareAlpineVersion(
	installed string,
	constraint string,
) (bool, error) {
	// Strip Alpine release suffix (-rN)
	if idx := strings.Index(installed, "-r"); idx != -1 {
		installed = installed[:idx]
	}

	// Use semantic versioning for comparison
	return vc.compareSemver(installed, constraint)
}

// compareSemver compares semantic versions
func (vc *VersionComparator) compareSemver(
	installed string,
	constraint string,
) (bool, error) {
	// Use hashicorp/go-version for semantic versioning
	v1, err := version.NewVersion(installed)
	if err != nil {
		// If version parsing fails, try to clean it
		cleaned := vc.cleanVersion(installed)
		v1, err = version.NewVersion(cleaned)
		if err != nil {
			return false, fmt.Errorf("failed to parse version %s: %w", installed, err)
		}
	}

	// Parse constraint: "< 1.2.3", ">= 1.0.0, < 2.0.0"
	constraints, err := version.NewConstraint(constraint)
	if err != nil {
		return false, fmt.Errorf("failed to parse constraint %s: %w", constraint, err)
	}

	// Check if installed version satisfies constraint
	// Returns true if MATCHES constraint (i.e., vulnerable!)
	return constraints.Check(v1), nil
}

// cleanVersion cleans version string for parsing
func (vc *VersionComparator) cleanVersion(v string) string {
	// Remove common prefixes/suffixes
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	v = strings.TrimSuffix(v, "-SNAPSHOT")
	v = strings.TrimSpace(v)
	return v
}

// parseConstraint parses constraint string into operator and version
// Examples: "< 1.2.3", "<=1.2.3", ">= 1.0.0, < 2.0.0"
func (vc *VersionComparator) parseConstraint(constraint string) (string, string) {
	constraint = strings.TrimSpace(constraint)

	if strings.HasPrefix(constraint, "<=") {
		return "<=", strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, ">=") {
		return ">=", strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, "<") {
		return "<", strings.TrimSpace(constraint[1:])
	} else if strings.HasPrefix(constraint, ">") {
		return ">", strings.TrimSpace(constraint[1:])
	} else if strings.HasPrefix(constraint, "==") {
		return "==", strings.TrimSpace(constraint[2:])
	}

	// Default: exact match
	return "==", constraint
}


