package extractor

import (
	"path/filepath"
	"strings"
)

// DpkgParser parses Debian/Ubuntu packages from /var/lib/dpkg/status and
// /var/lib/dpkg/status.d/* (Google Distroless images store per-package status
// files there instead of a single monolithic file — same approach as Trivy/Syft).
type DpkgParser struct{}

func NewDpkgParser() *DpkgParser {
	return &DpkgParser{}
}

const (
	dpkgStatusFile = "/var/lib/dpkg/status"
	dpkgStatusDir  = "/var/lib/dpkg/status.d/"
)

// DebianDependency is a parsed dependency entry from dpkg metadata.
// If Or=true, this entry is an alternative to the previous dependency in the same group.
type DebianDependency struct {
	Package string
	Or      bool
}

// DebianPackageInfo holds dependency metadata used for transitive expansion.
type DebianPackageInfo struct {
	Name       string
	Version    string
	Depends    []DebianDependency
	PreDepends []DebianDependency
}

func (p *DpkgParser) Parse(fs *Filesystem) ([]Package, error) {
	packages := make([]Package, 0)
	seen := make(map[string]bool)

	// 1. Standard monolithic status file (traditional Debian/Ubuntu images).
	if content, err := fs.ReadFile(dpkgStatusFile); err == nil {
		pkgs := parseDpkgStatus(string(content))
		for _, pkg := range pkgs {
			key := pkg.Name + "\x00" + pkg.Version
			if !seen[key] {
				seen[key] = true
				packages = append(packages, pkg)
			}
		}
	}

	// 2. Distroless status.d/ directory: each non-md5sums file is a single
	//    dpkg status entry (Package: ..., Version: ..., etc.)
	//    Ref: var/lib/dpkg/status.d/ in gcr.io/distroless/* and Kubernetes
	//    control-plane images (kube-apiserver, kube-proxy, etcd, etc.)
	for _, path := range fs.PathsUnder(dpkgStatusDir) {
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".md5sums") || base == "" || base == "." {
			continue
		}
		content, err := fs.ReadFile(path)
		if err != nil {
			continue
		}
		pkgs := parseDpkgStatus(string(content))
		for _, pkg := range pkgs {
			key := pkg.Name + "\x00" + pkg.Version
			if !seen[key] {
				seen[key] = true
				packages = append(packages, pkg)
			}
		}
	}

	if len(packages) == 0 {
		return nil, nil
	}
	return packages, nil
}

// parseDpkgStatus parses dpkg status format (works for both monolithic file
// and individual status.d/* entries).
func parseDpkgStatus(content string) []Package {
	packages := make([]Package, 0)
	lines := strings.Split(content, "\n")
	var currentPkg Package
	var inPackage bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			if inPackage && currentPkg.Name != "" {
				packages = append(packages, currentPkg)
				currentPkg = Package{}
				inPackage = false
			}
			continue
		}

		if strings.HasPrefix(line, "Package: ") {
			currentPkg.Name = strings.TrimPrefix(line, "Package: ")
			currentPkg.Type = "deb"
			inPackage = true
		} else if strings.HasPrefix(line, "Version: ") {
			rawVersion := strings.TrimSpace(strings.TrimPrefix(line, "Version: "))
			currentPkg.Epoch, currentPkg.Version = splitDebianEpochVersion(rawVersion)
		} else if strings.HasPrefix(line, "Source: ") {
			source := strings.TrimSpace(strings.TrimPrefix(line, "Source: "))
			// Format can be either:
			//   "openssl"
			//   "openssl (3.0.8-1)"
			if idx := strings.Index(source, "("); idx > 0 {
				currentPkg.SourcePackage = strings.TrimSpace(source[:idx])
				sv := strings.TrimSpace(strings.TrimSuffix(source[idx+1:], ")"))
				currentPkg.SourceVersion = sv
			} else {
				currentPkg.SourcePackage = source
			}
		} else if strings.HasPrefix(line, "Architecture: ") {
			currentPkg.Arch = strings.TrimPrefix(line, "Architecture: ")
		} else if strings.HasPrefix(line, "Status: ") {
			status := strings.TrimPrefix(line, "Status: ")
			if !strings.Contains(status, "installed") {
				inPackage = false
			}
		}
	}

	if inPackage && currentPkg.Name != "" {
		packages = append(packages, currentPkg)
	}

	return packages
}

// parseDpkgStatusDetails parses only fields needed for transitive dependency expansion.
func parseDpkgStatusDetails(content string) []DebianPackageInfo {
	out := make([]DebianPackageInfo, 0)
	lines := strings.Split(content, "\n")
	var cur DebianPackageInfo
	var inPackage bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if inPackage && cur.Name != "" {
				out = append(out, cur)
			}
			cur = DebianPackageInfo{}
			inPackage = false
			continue
		}

		switch {
		case strings.HasPrefix(line, "Package: "):
			cur.Name = strings.TrimSpace(strings.TrimPrefix(line, "Package: "))
			inPackage = true
		case strings.HasPrefix(line, "Version: "):
			rawVersion := strings.TrimSpace(strings.TrimPrefix(line, "Version: "))
			_, cur.Version = splitDebianEpochVersion(rawVersion)
		case strings.HasPrefix(line, "Depends: "):
			cur.Depends = parseDebianDependencies(strings.TrimSpace(strings.TrimPrefix(line, "Depends: ")))
		case strings.HasPrefix(line, "Pre-Depends: "):
			cur.PreDepends = parseDebianDependencies(strings.TrimSpace(strings.TrimPrefix(line, "Pre-Depends: ")))
		case strings.HasPrefix(line, "Status: "):
			status := strings.TrimSpace(strings.TrimPrefix(line, "Status: "))
			if !strings.Contains(status, "installed") {
				inPackage = false
			}
		}
	}

	if inPackage && cur.Name != "" {
		out = append(out, cur)
	}
	return out
}

func parseDebianDependencies(depStr string) []DebianDependency {
	if strings.TrimSpace(depStr) == "" {
		return nil
	}
	out := make([]DebianDependency, 0)
	andGroups := strings.Split(depStr, ",")
	for _, group := range andGroups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		alts := strings.Split(group, "|")
		for i, alt := range alts {
			name := normalizeDependencyPackageName(alt)
			if name == "" {
				continue
			}
			out = append(out, DebianDependency{
				Package: name,
				Or:      i > 0,
			})
		}
	}
	return out
}

func normalizeDependencyPackageName(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Remove architecture/profile qualifiers and version constraints.
	if idx := strings.Index(s, "("); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "["); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "<"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	// Debian qualifier, e.g. "python3:any"
	if idx := strings.Index(s, ":"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func splitDebianEpochVersion(raw string) (string, string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", ""
	}
	idx := strings.Index(v, ":")
	if idx <= 0 {
		return "", v
	}
	epoch := strings.TrimSpace(v[:idx])
	version := strings.TrimSpace(v[idx+1:])
	if epoch == "" || version == "" {
		return "", v
	}
	return epoch, version
}
