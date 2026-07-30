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
	seen := make(map[string]struct{})

	addPkg := func(pkg Package) {
		if pkg.Name == "" {
			return
		}
		key := pkg.Name + "@" + pkg.Version
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		packages = append(packages, pkg)
	}

	// Strategy 1: Parse go.sum from common roots first (cheap + deterministic).
	foundSum := false
	commonGoRoots := []string{"", "/app", "/src", "/workspace", "/home/node/app", "/opt", "/opt/app"}
	for _, root := range commonGoRoots {
		path := root + "/go.sum"
		if root == "" {
			path = "/go.sum"
		}
		if sumContent, err := fs.ReadFile(path); err == nil {
			pkgs, _ := p.parseGoSum(sumContent)
			for _, pkg := range pkgs {
				addPkg(pkg)
			}
			foundSum = true
			break
		}
	}

	// Strategy 2: Parse go.mod from common roots first.
	foundMod := false
	for _, root := range commonGoRoots {
		path := root + "/go.mod"
		if root == "" {
			path = "/go.mod"
		}
		if modContent, err := fs.ReadFile(path); err == nil {
			pkgs, _ := p.parseGoMod(modContent)
			for _, pkg := range pkgs {
				addPkg(pkg)
			}
			foundMod = true
			break
		}
	}

	// Fallback: discover any go.sum/go.mod by suffix (best-effort).
	if !foundSum {
		for _, path := range fs.FindPathsBySuffix("go.sum") {
			sumContent, err := fs.ReadFile(path)
			if err != nil {
				continue
			}
			pkgs, _ := p.parseGoSum(sumContent)
			for _, pkg := range pkgs {
				addPkg(pkg)
			}
			break
		}
	}
	if !foundMod {
		for _, path := range fs.FindPathsBySuffix("go.mod") {
			modContent, err := fs.ReadFile(path)
			if err != nil {
				continue
			}
			pkgs, _ := p.parseGoMod(modContent)
			for _, pkg := range pkgs {
				addPkg(pkg)
			}
			break
		}
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

