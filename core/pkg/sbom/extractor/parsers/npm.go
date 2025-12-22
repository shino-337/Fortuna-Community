package parsers

import (
	"encoding/json"
	"strings"

	"github.com/ksam/core/pkg/sbom/extractor"
)

// NpmParser parses Node.js packages from package-lock.json or package.json
type NpmParser struct{}

// NewNpmParser creates a new npm parser
func NewNpmParser() *NpmParser {
	return &NpmParser{}
}

// Parse parses npm packages
func (p *NpmParser) Parse(fs *extractor.Filesystem) ([]extractor.Package, error) {
	packages := make([]extractor.Package, 0)

	// Strategy 1: Parse package-lock.json
	if lockContent, err := fs.ReadFile("/package-lock.json"); err == nil {
		pkgs, _ := p.parsePackageLock(lockContent)
		packages = append(packages, pkgs...)
	}

	// Strategy 2: Find all package.json in node_modules
	moduleDirs := fs.Glob("/node_modules/*/package.json")
	for _, path := range moduleDirs {
		content, err := fs.ReadFile(path)
		if err != nil {
			continue
		}

		pkg, err := p.parsePackageJson(content)
		if err != nil {
			continue
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// parsePackageLock parses package-lock.json
func (p *NpmParser) parsePackageLock(content []byte) ([]extractor.Package, error) {
	var lockFile struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}

	if err := json.Unmarshal(content, &lockFile); err != nil {
		return nil, err
	}

	packages := make([]extractor.Package, 0)
	for path, pkg := range lockFile.Packages {
		if pkg.Version == "" {
			continue
		}

		// Extract package name from path
		// node_modules/package-name -> package-name
		parts := strings.Split(path, "/")
		name := parts[len(parts)-1]
		if name == "" && len(parts) > 1 {
			name = parts[len(parts)-2]
		}

		packages = append(packages, extractor.Package{
			Name:    name,
			Version: pkg.Version,
			Type:    "npm",
		})
	}

	return packages, nil
}

// parsePackageJson parses package.json
func (p *NpmParser) parsePackageJson(content []byte) (extractor.Package, error) {
	var pkgJson struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	if err := json.Unmarshal(content, &pkgJson); err != nil {
		return extractor.Package{}, err
	}

	return extractor.Package{
		Name:    pkgJson.Name,
		Version: pkgJson.Version,
		Type:    "npm",
	}, nil
}


