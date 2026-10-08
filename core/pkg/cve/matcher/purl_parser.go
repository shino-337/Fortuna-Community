package matcher

import (
	"fmt"
	"net/url"
	"strings"
)

// PURL represents a parsed Package URL
type PURL struct {
	Type      string // pkg
	Ecosystem string // deb, npm, pypi, etc.
	Namespace string // debian, alpine, etc.
	Name      string // openssl
	Version   string // 1.1.1d
	// Qualifiers are parsed from the query segment after the version (e.g. ?arch=amd64).
	Qualifiers map[string]string
}

// ParsePURL parses a Package URL string following the purl spec:
// pkg:<type>/<namespace>/<name>@<version>?<qualifiers>#<subpath>
// Examples:
//   - pkg:deb/debian/libssl3@3.0.15-1~deb12u1?arch=amd64&distro=debian-12&upstream=openssl
//   - pkg:npm/%40babel/core@7.24.0
//   - pkg:pypi/django@3.2.5
//
// Segments and qualifier values are percent-decoded, and the version is taken from the
// last '@' of the path so qualifiers such as upstream=openssl@3.0.15 do not break parsing.
func ParsePURL(purlString string) (*PURL, error) {
	if !strings.HasPrefix(purlString, "pkg:") {
		return nil, fmt.Errorf("invalid PURL: missing pkg: prefix")
	}

	rest := strings.TrimPrefix(purlString, "pkg:")
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest = rest[:i]
	}

	var qualifiers map[string]string
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		qualifiers = parsePURLQualifiers(rest[i+1:])
		rest = rest[:i]
	}

	at := strings.LastIndexByte(rest, '@')
	if at < 0 {
		return nil, fmt.Errorf("invalid PURL: missing version")
	}
	pathPart := rest[:at]
	version, err := url.PathUnescape(rest[at+1:])
	if err != nil {
		return nil, fmt.Errorf("invalid PURL: bad version encoding: %w", err)
	}
	if version == "" {
		return nil, fmt.Errorf("invalid PURL: missing version")
	}

	pathComponents := strings.Split(strings.Trim(pathPart, "/"), "/")
	if len(pathComponents) < 2 {
		return nil, fmt.Errorf("invalid PURL: invalid path")
	}
	for i, seg := range pathComponents {
		dec, err := url.PathUnescape(seg)
		if err != nil {
			return nil, fmt.Errorf("invalid PURL: bad path encoding: %w", err)
		}
		pathComponents[i] = dec
	}

	purl := &PURL{
		Type:       "pkg",
		Version:    version,
		Qualifiers: qualifiers,
	}

	eco := strings.ToLower(pathComponents[0])
	switch {
	case len(pathComponents) == 2:
		// pkg:npm/lodash@4.17.21
		// OR pkg:PACKAGE_TYPE_APK/alpine-baselayout@3.4.3-r2 (no namespace)
		purl.Ecosystem = pathComponents[0]
		purl.Name = pathComponents[1]
	case eco == "go" || eco == "golang":
		// pkg:go/github.com/coreos/etcd/client/v3@v3.3.0 — full module path as name for prefix + alias resolution
		purl.Ecosystem = "go"
		purl.Name = strings.Join(pathComponents[1:], "/")
	case len(pathComponents) == 3:
		// pkg:deb/debian/openssl@1.1.1d
		// OR pkg:apk/alpine/package@version
		purl.Ecosystem = pathComponents[0]
		purl.Namespace = pathComponents[1]
		purl.Name = pathComponents[2]
	default:
		// The spec allows multi-segment namespaces (e.g. pkg:golang-style paths in other types).
		purl.Ecosystem = pathComponents[0]
		purl.Namespace = strings.Join(pathComponents[1:len(pathComponents)-1], "/")
		purl.Name = pathComponents[len(pathComponents)-1]
	}

	return purl, nil
}

func parsePURLQualifiers(q string) map[string]string {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	qualifiers := make(map[string]string)
	for _, kv := range strings.Split(q, "&") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		k, v, ok := strings.Cut(kv, "=")
		k = strings.ToLower(strings.TrimSpace(k))
		if !ok || k == "" {
			continue
		}
		if dec, err := url.PathUnescape(v); err == nil {
			v = dec
		}
		qualifiers[k] = strings.TrimSpace(v)
	}
	return qualifiers
}

// distroSourcePackage returns the source/origin package name and version a distro
// binary package was built from, taken from the purl `upstream` qualifier
// (e.g. upstream=openssl or upstream=openssl@3.0.15-1~deb12u1). Debian, Ubuntu and
// Alpine advisories are keyed by source package, so this is the name to query.
func distroSourcePackage(p *PURL) (name, version string) {
	if p == nil || p.Qualifiers == nil {
		return "", ""
	}
	up := strings.TrimSpace(p.Qualifiers["upstream"])
	if up == "" {
		return "", ""
	}
	name, version, _ = strings.Cut(up, "@")
	// Syft emits upstream as "name@version" and Debian source versions may be given in
	// parentheses by some tools ("name (version)").
	if i := strings.IndexByte(name, ' '); i >= 0 {
		v := strings.Trim(strings.TrimSpace(name[i+1:]), "()")
		name = name[:i]
		if version == "" {
			version = v
		}
	}
	return strings.TrimSpace(name), strings.TrimSpace(version)
}
