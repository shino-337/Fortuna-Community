package extractor

import (
	"strings"

)

// GoModParser parses Go packages from go.sum or go.mod
type GoModParser struct{}

// NewGoModParser creates a new go.mod parser
func NewGoModParser() *GoModParser {
	return &GoModParser{}
}

// Parse parses go packages
func (p *GoModParser) Parse(fs *Filesystem) ([]Package, error) {
	packages := make([]Package, 0)

	// Strategy 1: Parse go.sum
	if sumContent, err := fs.ReadFile("/go.sum"); err == nil {
		pkgs, _ := p.parseGoSum(sumContent)
		packages = append(packages, pkgs...)
	}

	// Strategy 2: Parse go.mod
	if modContent, err := fs.ReadFile("/go.mod"); err == nil {
		pkgs, _ := p.parseGoMod(modContent)
		packages = append(packages, pkgs...)
	}

	return packages, nil
}

// parseGoSum parses go.sum file
// Format: module path version hash
// Example: github.com/gin-gonic/gin v1.7.0 h1:hash...
func (p *GoModParser) parseGoSum(content []byte) ([]Package, error) {
	packages := make([]Package, 0)
	seen := make(map[string]bool)

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		version := strings.TrimSuffix(parts[1], "/go.mod")

		// Deduplicate
		key := name + "@" + version
		if seen[key] {
			continue
		}
		seen[key] = true

		packages = append(packages, Package{
			Name:    name,
			Version: version,
			Type:    "go",
		})
	}

	return packages, nil
}

// parseGoMod parses go.mod file
// Format: require module path version
func (p *GoModParser) parseGoMod(content []byte) ([]Package, error) {
	packages := make([]Package, 0)

	lines := strings.Split(string(content), "\n")
	inRequire := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if strings.HasPrefix(line, ")") {
			inRequire = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			// Single line require
			parts := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(parts) >= 2 {
				packages = append(packages, Package{
					Name:    parts[0],
					Version: parts[1],
					Type:    "go",
				})
			}
		} else if inRequire {
			// Multi-line require block
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				packages = append(packages, Package{
					Name:    parts[0],
					Version: parts[1],
					Type:    "go",
				})
			}
		}
	}

	return packages, nil
}

