package extractor

import (
	"regexp"
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

	// Strategy 1: Parse requirements.txt from common roots.
	// Note: don't use Glob("/**/...") because Filesystem.Glob doesn't support recursive "**".
	commonPyRoots := []string{"", "/app", "/usr/src/app", "/workspace", "/opt", "/opt/app"}
	for _, root := range commonPyRoots {
		path := root + "/requirements.txt"
		if root == "" {
			path = "/requirements.txt"
		}
		if reqContent, err := fs.ReadFile(path); err == nil {
			pkgs, _ := p.parseRequirements(reqContent)
			for _, pkg := range pkgs {
				addPkg(pkg)
			}
			// one lockfile is enough for coverage
			break
		}
	}

	// Strategy 2: Discover dist-info metadata anywhere in the image.
	// Use suffix+filter instead of recursive glob.
	metadataFiles := fs.FindPathsBySuffix("METADATA")
	for _, path := range metadataFiles {
		// Expect ".../<something>.dist-info/METADATA".
		if !strings.Contains(path, ".dist-info/") || !strings.HasSuffix(path, "/METADATA") {
			continue
		}
		content, err := fs.ReadFile(path)
		if err != nil {
			continue
		}

		pkg, err := p.parseMetadata(content)
		if err != nil {
			continue
		}
		addPkg(pkg)
	}

	return packages, nil
}

// parseRequirements parses requirements.txt
func (p *PipParser) parseRequirements(content []byte) ([]Package, error) {
	packages := make([]Package, 0)

	lines := strings.Split(string(content), "\n")
	// Best-effort PEP 440 requirement spec parsing.
	// - exact pins (== / === / =) => store pinned version
	// - non-exact specs (~=, >=, <=, !=, <, >, etc) => store "unknown" (range can't be represented as a single installed version)
	reqRe := regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)(\[[^\]]+\])?\s*(==|===|=|~=|!=|<=|>=|<|>)?\s*([A-Za-z0-9][A-Za-z0-9.+_-]*)?\s*$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Drop inline comments.
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}

		// Skip recursive includes and unsupported syntax.
		if strings.HasPrefix(line, "-r ") || strings.HasPrefix(line, "--requirement ") {
			continue
		}
		if strings.HasPrefix(line, "-e ") || strings.HasPrefix(line, "--editable ") {
			continue
		}

		// Environment markers: only parse the left side.
		if idx := strings.Index(line, ";"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}

		m := reqRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		name := strings.TrimSpace(m[1])
		op := strings.TrimSpace(m[3])
		versionToken := strings.TrimSpace(m[4])

		if name == "" {
			continue
		}

		version := "unknown"
		switch op {
		case "==", "===", "=":
			if versionToken != "" {
				version = versionToken
			}
		default:
			// Non-exact specs => "unknown" by design.
			version = "unknown"
		}

		packages = append(packages, Package{
			Name:    name,
			Version: version,
			Type:    "pypi",
		})
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

