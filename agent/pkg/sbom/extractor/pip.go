package extractor

import (
	"strings"

)

// PipParser parses Python packages from requirements.txt or *.dist-info/METADATA
type PipParser struct{}

// NewPipParser creates a new pip parser
func NewPipParser() *PipParser {
	return &PipParser{}
}

// Parse parses pip packages
func (p *PipParser) Parse(fs *Filesystem) ([]Package, error) {
	packages := make([]Package, 0)

	// Strategy 1: Parse requirements.txt
	if reqContent, err := fs.ReadFile("/requirements.txt"); err == nil {
		pkgs, _ := p.parseRequirements(reqContent)
		packages = append(packages, pkgs...)
	}

	// Strategy 2: Find all *.dist-info/METADATA
	metadataFiles := fs.Glob("/**/*.dist-info/METADATA")
	for _, path := range metadataFiles {
		content, err := fs.ReadFile(path)
		if err != nil {
			continue
		}

		pkg, err := p.parseMetadata(content)
		if err != nil {
			continue
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// parseRequirements parses requirements.txt
func (p *PipParser) parseRequirements(content []byte) ([]Package, error) {
	packages := make([]Package, 0)

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse format: package==version or package>=version, etc.
		parts := strings.FieldsFunc(line, func(r rune) bool {
			return r == '=' || r == '>' || r == '<' || r == '!'
		})

		if len(parts) >= 1 {
			name := strings.TrimSpace(parts[0])
			version := ""
			if len(parts) >= 2 {
				version = strings.TrimSpace(parts[1])
			}

			if name != "" {
				packages = append(packages, Package{
					Name:    name,
					Version: version,
					Type:    "pypi",
				})
			}
		}
	}

	return packages, nil
}

// parseMetadata parses METADATA file (RFC 822-like format)
func (p *PipParser) parseMetadata(content []byte) (Package, error) {
	pkg := Package{Type: "pypi"}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Name: ") {
			pkg.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name: "))
		} else if strings.HasPrefix(line, "Version: ") {
			pkg.Version = strings.TrimSpace(strings.TrimPrefix(line, "Version: "))
		}
	}

	return pkg, nil
}

