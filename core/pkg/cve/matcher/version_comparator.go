package matcher

import (
	"fmt"
	"log"
	"strconv"
	"strings"

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

	switch ecosystem {
	case "deb", "debian", "ubuntu":
		return vc.compareDebianVersion(installedVersion, constraint)
	case "rpm", "redhat", "centos":
		return vc.compareRPMVersion(installedVersion, constraint)
	case "apk", "alpine":
		return vc.compareAlpineVersion(installedVersion, constraint)
	case "npm", "pypi", "go":
		return vc.compareSemver(installedVersion, constraint)
	default:
		// Fallback to semantic versioning
		return vc.compareSemver(installedVersion, constraint)
	}
}

// compareDebianVersion compares Debian package versions
// Format: [epoch:]upstream_version[-debian_revision]
// Example: 1:1.1.1d-0+deb10u7
func (vc *VersionComparator) compareDebianVersion(
	installed string,
	constraint string,
) (bool, error) {
	// Support multi-part constraints like: ">= 1.0, < 2.0"
	parts := strings.Split(constraint, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse constraint operator
		op, targetVersion := vc.parseConstraint(part)
		if targetVersion == "" {
			// No target version -> treat as not vulnerable for this constraint
			return false, nil
		}

		// Compare versions using Debian's dpkg --compare-versions logic
		result := vc.dpkgCompareVersions(installed, targetVersion)

		// Apply operator; all parts must match
		ok := false
		switch op {
		case "<":
			ok = result < 0
		case "<=":
			ok = result <= 0
		case ">":
			ok = result > 0
		case ">=":
			ok = result >= 0
		case "==":
			ok = result == 0
		default:
			return false, fmt.Errorf("unknown operator: %s", op)
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}

// dpkgCompareVersions compares two Debian versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func (vc *VersionComparator) dpkgCompareVersions(v1, v2 string) int {
	// Split into epoch:version-revision
	epoch1, ver1, rev1 := vc.splitDebianVersion(v1)
	epoch2, ver2, rev2 := vc.splitDebianVersion(v2)

	// Compare epoch
	if epoch1 != epoch2 {
		if epoch1 < epoch2 {
			return -1
		}
		return 1
	}

	// Compare upstream version
	cmp := vc.compareDebianVersionPart(ver1, ver2)
	if cmp != 0 {
		return cmp
	}

	// Compare revision
	return vc.compareDebianVersionPart(rev1, rev2)
}

// splitDebianVersion splits Debian version into epoch, version, revision
// Format: [epoch:]upstream_version[-debian_revision]
func (vc *VersionComparator) splitDebianVersion(v string) (int, string, string) {
	epoch := 0
	version := v
	revision := ""

	// Extract epoch
	if idx := strings.Index(v, ":"); idx != -1 {
		if e, err := strconv.Atoi(v[:idx]); err == nil {
			epoch = e
		}
		version = v[idx+1:]
	}

	// Extract revision
	if idx := strings.LastIndex(version, "-"); idx != -1 {
		revision = version[idx+1:]
		version = version[:idx]
	}

	return epoch, version, revision
}

// compareDebianVersionPart compares Debian version parts
// Simplified implementation - for production, use full dpkg algorithm
func (vc *VersionComparator) compareDebianVersionPart(v1, v2 string) int {
	// Debian version comparison rules:
	// - Letters < numbers
	// - Compare character by character
	// - Non-alphanumeric < alphanumeric

	// For now, use string comparison (simplified)
	// TODO: Implement full Debian version comparison algorithm
	if v1 < v2 {
		return -1
	} else if v1 > v2 {
		return 1
	}
	return 0
}

// compareRPMVersion compares RPM package versions
// Format: [epoch:]version-release
// Example: 1:1.1.1k-5.el8
func (vc *VersionComparator) compareRPMVersion(
	installed string,
	constraint string,
) (bool, error) {
	// Similar to Debian but simpler
	// For now, use semantic versioning as fallback
	// TODO: Implement full RPM version comparison
	return vc.compareSemver(installed, constraint)
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


