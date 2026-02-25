package extractor

import (
	"encoding/json"
	"strings"

)

// NpmParser parses Node.js packages from package-lock.json or package.json
type NpmParser struct{}

// NewNpmParser creates a new npm parser
func NewNpmParser() *NpmParser {
	return &NpmParser{}
}

// commonNodeRoots are typical WORKDIRs in Node images (npm parser checks these)
var commonNodeRoots = []string{"", "/app", "/usr/src/app", "/home/node/app", "/opt/app"}

// Parse parses npm packages from package-lock.json and node_modules/*/package.json
func (p *NpmParser) Parse(fs *Filesystem) ([]Package, error) {
	packages := make([]Package, 0)
	seen := make(map[string]bool)

	// Strategy 1: Parse package-lock.json from common roots
	for _, root := range commonNodeRoots {
		path := root + "/package-lock.json"
		if root == "" {
			path = "/package-lock.json"
		}
		if lockContent, err := fs.ReadFile(path); err == nil {
			pkgs, _ := p.parsePackageLock(lockContent)
			for _, pkg := range pkgs {
				key := pkg.Name + "@" + pkg.Version
				if !seen[key] {
					seen[key] = true
					packages = append(packages, pkg)
				}
			}
			break // one lock file is enough
		}
	}

	// Strategy 2: Find all package.json in node_modules under common roots
	for _, root := range commonNodeRoots {
		var pattern string
		if root == "" {
			pattern = "/node_modules/*/package.json"
		} else {
			if !strings.HasPrefix(root, "/") {
				root = "/" + root
			}
			pattern = root + "/node_modules/*/package.json"
		}
		moduleDirs := fs.Glob(pattern)
		for _, path := range moduleDirs {
			content, err := fs.ReadFile(path)
			if err != nil {
				continue
			}
			pkg, err := p.parsePackageJson(content)
			if err != nil || pkg.Name == "" {
				continue
			}
			key := pkg.Name + "@" + pkg.Version
			if !seen[key] {
				seen[key] = true
				packages = append(packages, pkg)
			}
		}
	}

	return packages, nil
}

// parsePackageLock parses package-lock.json
func (p *NpmParser) parsePackageLock(content []byte) ([]Package, error) {
	var lockFile struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}

	if err := json.Unmarshal(content, &lockFile); err != nil {
		return nil, err
	}

	packages := make([]Package, 0)
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

		packages = append(packages, Package{
			Name:    name,
			Version: pkg.Version,
			Type:    "npm",
		})
	}

	return packages, nil
}

// parsePackageJson parses package.json
func (p *NpmParser) parsePackageJson(content []byte) (Package, error) {
	var pkgJson struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	if err := json.Unmarshal(content, &pkgJson); err != nil {
		return Package{}, err
	}

	return Package{
		Name:    pkgJson.Name,
		Version: pkgJson.Version,
		Type:    "npm",
	}, nil
}

