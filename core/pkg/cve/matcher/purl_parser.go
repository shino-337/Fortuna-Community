package matcher

import (
	"fmt"
	"strings"
)

// PURL represents a parsed Package URL
type PURL struct {
	Type      string // pkg
	Ecosystem string // deb, npm, pypi, etc.
	Namespace string // debian, alpine, etc.
	Name      string // openssl
	Version   string // 1.1.1d
}

// ParsePURL parses a Package URL string
// Format: pkg:<type>/<namespace>/<name>@<version>
// Examples:
//   - pkg:deb/debian/openssl@1.1.1d
//   - pkg:npm/lodash@4.17.21
//   - pkg:pypi/django@3.2.5
func ParsePURL(purlString string) (*PURL, error) {
	if !strings.HasPrefix(purlString, "pkg:") {
		return nil, fmt.Errorf("invalid PURL: missing pkg: prefix")
	}

	// Remove "pkg:" prefix
	rest := strings.TrimPrefix(purlString, "pkg:")

	// Split by @ to separate version
	parts := strings.Split(rest, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid PURL: missing version")
	}

	pathPart := parts[0]
	version := parts[1]

	// Split path by /
	pathComponents := strings.Split(pathPart, "/")
	if len(pathComponents) < 2 {
		return nil, fmt.Errorf("invalid PURL: invalid path")
	}

	purl := &PURL{
		Type:    "pkg",
		Version: version,
	}

	if len(pathComponents) == 2 {
		// pkg:npm/lodash@4.17.21
		// OR pkg:PACKAGE_TYPE_APK/alpine-baselayout@3.4.3-r2 (no namespace)
		purl.Ecosystem = pathComponents[0]
		purl.Name = pathComponents[1]
	} else if len(pathComponents) == 3 {
		// pkg:deb/debian/openssl@1.1.1d
		// OR pkg:apk/alpine/package@version
		purl.Ecosystem = pathComponents[0]
		purl.Namespace = pathComponents[1]
		purl.Name = pathComponents[2]
	} else if len(pathComponents) >= 2 && (strings.ToLower(pathComponents[0]) == "go" || strings.ToLower(pathComponents[0]) == "golang") {
		// pkg:go/github.com/coreos/etcd/client/v3@v3.3.0 — full module path as name for prefix + alias resolution
		purl.Ecosystem = "go"
		purl.Name = strings.Join(pathComponents[1:], "/")
	} else {
		return nil, fmt.Errorf("invalid PURL: too many path components")
	}

	return purl, nil
}



