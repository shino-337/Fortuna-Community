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
			currentPkg.Version = strings.TrimPrefix(line, "Version: ")
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

