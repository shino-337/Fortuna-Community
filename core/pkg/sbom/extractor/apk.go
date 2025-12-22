package extractor

import (
	"strings"

)

// ApkParser parses Alpine packages from /lib/apk/db/installed
type ApkParser struct{}

// NewApkParser creates a new apk parser
func NewApkParser() *ApkParser {
	return &ApkParser{}
}

// Parse parses apk installed database
func (p *ApkParser) Parse(fs *Filesystem) ([]Package, error) {
	// Read apk installed file
	content, err := fs.ReadFile("/lib/apk/db/installed")
	if err != nil {
		return nil, err // File not found is OK (not an Alpine image)
	}

	packages := make([]Package, 0)

	// Parse apk format
	// Format:
	// P:openssl
	// V:1.1.1g-r0
	// A:x86_64
	// L:GPL-2.0-or-later
	// ...

	lines := strings.Split(string(content), "\n")
		var currentPkg Package

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			// End of package entry
			if currentPkg.Name != "" {
				packages = append(packages, currentPkg)
				currentPkg = Package{}
			}
			continue
		}

		if strings.HasPrefix(line, "P:") {
			currentPkg.Name = strings.TrimPrefix(line, "P:")
			currentPkg.Type = "apk"
		} else if strings.HasPrefix(line, "V:") {
			currentPkg.Version = strings.TrimPrefix(line, "V:")
		} else if strings.HasPrefix(line, "A:") {
			currentPkg.Arch = strings.TrimPrefix(line, "A:")
		}
	}

	// Add last package if exists
	if currentPkg.Name != "" {
		packages = append(packages, currentPkg)
	}

	return packages, nil
}

