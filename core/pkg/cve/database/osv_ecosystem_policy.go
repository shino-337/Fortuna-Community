// OSV mirror ecosystem policy: which feeds use bulk osv_* queries and supplemental CVE merges.
// Distro ECOSYSTEM ranges are ingested by pkg/mirror/osv (non-go); generic PURL matching uses
// comparatorEcosystemForBulkBatch in the matcher so constraints use the correct comparator.
package database

import "strings"

// isDistroOSVEcosystem is true for Linux distro feeds in OSV that primarily use ECOSYSTEM
// range events (apk/dpkg/rpm-style versions), as opposed to registry semver (npm, go modules, …).
func isDistroOSVEcosystem(eco string) bool {
	switch strings.ToLower(strings.TrimSpace(eco)) {
	case "alpine", "debian", "ubuntu",
		"redhat", "centos", "fedora", "oraclelinux", "amazon",
		"photon", "opensuse", "sles", "rocky", "alma",
		"archlinux", "wolfi", "chainguard":
		return true
	default:
		return false
	}
}

// useOSVMirrorForEcosystem enables bulk reads from osv_* mirror tables for these ecosystems
// when the tables exist (aligned with OSV loader normalization).
func useOSVMirrorForEcosystem(eco string) bool {
	switch strings.ToLower(strings.TrimSpace(eco)) {
	case "go",
		"alpine", "debian", "ubuntu",
		"npm", "pypi", "cargo", "maven", "nuget", "rubygems",
		"redhat", "centos", "fedora", "oraclelinux", "amazon",
		"photon", "opensuse", "sles", "rocky", "alma",
		"archlinux", "wolfi", "chainguard":
		return true
	default:
		return false
	}
}

// isDistroPackageEcosystemForCPESupplement is true for distro package feeds where we may merge
// supplemental CPE-scoped rows from package_vulnerabilities (ecosystem bucket used for CPE data)
// when the OSV mirror path returns no CVEs for that package. Do not include generic "linux"
// — RPM rows may still be keyed as "linux" in legacy data.
func isDistroPackageEcosystemForCPESupplement(eco string) bool {
	switch strings.ToLower(strings.TrimSpace(eco)) {
	case "alpine", "apk", "debian", "deb", "ubuntu", "rpm",
		"redhat", "centos", "fedora", "oraclelinux", "amazon",
		"photon", "opensuse", "sles", "rocky", "alma",
		"archlinux", "wolfi", "chainguard":
		return true
	default:
		return false
	}
}
