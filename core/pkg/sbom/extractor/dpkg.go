package extractor

import (
	"strings"
)

// DpkgParser parses Debian/Ubuntu packages from /var/lib/dpkg/status
type DpkgParser struct{}

// NewDpkgParser creates a new dpkg parser
func NewDpkgParser() *DpkgParser {
	return &DpkgParser{}
}

// Parse parses dpkg status file
func (p *DpkgParser) Parse(fs *Filesystem) ([]Package, error) {
	// Read dpkg status file
	content, err := fs.ReadFile("/var/lib/dpkg/status")
	if err != nil {
		return nil, err // File not found is OK (not a Debian image)
	}

	packages := make([]Package, 0)

	// Parse dpkg status format
	lines := strings.Split(string(content), "\n")
	var currentPkg Package
	var inPackage bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			// End of package entry
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
			// Only include installed packages
			status := strings.TrimPrefix(line, "Status: ")
			if !strings.Contains(status, "installed") {
				inPackage = false // Skip non-installed packages
			}
		}
	}

	// Add last package if exists
	if inPackage && currentPkg.Name != "" {
		packages = append(packages, currentPkg)
	}

	return packages, nil
}

