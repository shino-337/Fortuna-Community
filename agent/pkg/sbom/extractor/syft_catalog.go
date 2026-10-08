package extractor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Syft is the primary cataloger: it reads a docker-archive of the image the Agent already
// fetched (from the node's containerd through the image-export helper, or the registry), so
// it needs no network, no runtime socket and no registry credentials. The Fortuna parsers
// remain as the fallback when Syft is missing or fails.

const (
	// maxSyftOutputBytes bounds the JSON we read back; a hostile image could otherwise make
	// Syft emit an arbitrarily large document.
	maxSyftOutputBytes = 256 << 20
	defaultMaxPackages = 10000
)

// SyftCatalog is what the Agent keeps from a Syft run.
type SyftCatalog struct {
	Packages  []Package
	OS        OSInfo
	GoVersion string // oldest Go toolchain among the image's Go binaries (stdlib CVEs)
}

// syftCommandEnv is the whole environment Syft runs with. The Agent's own environment
// (Core endpoint, credential paths, tokens) is not passed through, and every option that
// could reach the network is off.
func syftCommandEnv() []string {
	tmp := os.TempDir()
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=" + tmp,
		"TMPDIR=" + tmp,
		"XDG_CACHE_HOME=" + filepath.Join(tmp, ".cache"),
		"SYFT_CHECK_FOR_APP_UPDATE=false",
		"SYFT_GOLANG_SEARCH_REMOTE_LICENSES=false",
		"SYFT_GOLANG_SEARCH_LOCAL_MOD_CACHE_LICENSES=false",
		"SYFT_JAVA_USE_NETWORK=false",
		"SYFT_JAVA_RESOLVE_TRANSITIVE_DEPENDENCIES=false",
		"SYFT_JAVASCRIPT_SEARCH_REMOTE_LICENSES=false",
		"SYFT_PYTHON_SEARCH_REMOTE_LICENSES=false",
		"SYFT_PARALLELISM=2",
	}
	return env
}

// CatalogArchive runs Syft on a docker-archive file and converts the result.
func (s *SyftAdapter) CatalogArchive(ctx context.Context, archivePath string) (*SyftCatalog, error) {
	if s == nil {
		return nil, errors.New("syft adapter not configured")
	}
	if _, err := exec.LookPath(s.syftBin); err != nil {
		return nil, fmt.Errorf("syft binary not found (bin=%q)", s.syftBin)
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	out, err := os.CreateTemp("", "fortuna-syft-*.json")
	if err != nil {
		return nil, fmt.Errorf("syft output file: %w", err)
	}
	outPath := out.Name()
	_ = out.Close()
	defer func() { _ = os.Remove(outPath) }()

	// File catalogers (digests and metadata of every file) are not used and would make
	// the document many times larger.
	cmd := exec.CommandContext(ctx, s.syftBin, "scan", "docker-archive:"+archivePath,
		"-o", "syft-json="+outPath, "--quiet", "--select-catalogers", "-file")
	cmd.Env = syftCommandEnv()
	var stderr strings.Builder
	cmd.Stderr = &limitedWriter{w: &stderr, n: 4096}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("syft timed out after %s", s.timeout)
		}
		return nil, fmt.Errorf("syft failed: %w (stderr=%q)", err, strings.TrimSpace(stderr.String()))
	}

	info, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("syft output: %w", err)
	}
	if info.Size() > maxSyftOutputBytes {
		return nil, fmt.Errorf("syft output too large (%d bytes)", info.Size())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("syft output: %w", err)
	}
	return parseSyftCatalog(data, s.maxPkgs)
}

type limitedWriter struct {
	w interface{ WriteString(string) (int, error) }
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n > 0 {
		chunk := p
		if len(chunk) > l.n {
			chunk = chunk[:l.n]
		}
		_, _ = l.w.WriteString(string(chunk))
		l.n -= len(chunk)
	}
	return len(p), nil
}

type syftDocument struct {
	Artifacts []syftArtifact `json:"artifacts"`
	Distro    struct {
		ID        string `json:"id"`
		VersionID string `json:"versionID"`
	} `json:"distro"`
}

type syftArtifact struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Type     string `json:"type"`
	FoundBy  string `json:"foundBy"`
	PURL     string `json:"purl"`
	Licenses []struct {
		Value          string `json:"value"`
		SPDXExpression string `json:"spdxExpression"`
	} `json:"licenses"`
	CPEs []struct {
		CPE string `json:"cpe"`
	} `json:"cpes"`
}

func parseSyftCatalog(data []byte, maxPkgs int) (*SyftCatalog, error) {
	var doc syftDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("syft json: %w", err)
	}
	if maxPkgs <= 0 {
		maxPkgs = defaultMaxPackages
	}

	cat := &SyftCatalog{OS: OSInfo{Name: "unknown", Version: "unknown"}}
	if id := strings.ToLower(strings.TrimSpace(doc.Distro.ID)); id != "" {
		cat.OS = OSInfo{Name: id, Version: strings.TrimSpace(doc.Distro.VersionID), Distro: id}
		if cat.OS.Version == "" {
			cat.OS.Version = "unknown"
		}
	}

	var goVersions []string
	seen := make(map[string]bool, len(doc.Artifacts))
	for _, a := range doc.Artifacts {
		pkg, ok := packageFromSyft(a)
		if !ok {
			continue
		}
		// Syft reports each Go binary's toolchain as a "stdlib" module; Core matches the
		// standard library through SBOM.GoVersion instead.
		if pkg.Type == "go" && pkg.Name == "stdlib" {
			if v := strings.TrimSpace(pkg.Version); v != "" {
				goVersions = append(goVersions, v)
			}
			continue
		}
		key := pkg.PURL
		if key == "" {
			key = pkg.Type + "|" + pkg.Name + "@" + pkg.Version
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		cat.Packages = append(cat.Packages, pkg)
		if len(cat.Packages) >= maxPkgs {
			break
		}
	}
	cat.GoVersion = oldestGoVersion(goVersions)
	return cat, nil
}

// packageFromSyft converts one Syft artifact. Name is the name vulnerability feeds use for
// the ecosystem (Maven "group:artifact", npm with its scope); PURL is Syft's, which follows
// the PURL spec (binary package name, full version, arch, distro and upstream qualifiers).
func packageFromSyft(a syftArtifact) (Package, bool) {
	name := strings.TrimSpace(a.Name)
	if name == "" {
		return Package{}, false
	}
	version := strings.TrimSpace(a.Version)
	if version == "" {
		version = "unknown"
	}
	purl := strings.TrimSpace(a.PURL)
	if purl == "" {
		// Nothing to match it against (e.g. a Windows PE resource in a pip wheel).
		return Package{}, false
	}
	pkg := Package{
		Name:       name,
		Version:    version,
		Type:       fortunaTypeForSyft(a.Type, purl),
		PURL:       purl,
		Source:     provenanceForSyftCataloger(a.FoundBy),
		Confidence: "high",
	}
	if a.Type == "binary" {
		// Version read from bytes in an unmanaged binary (binary classifier).
		pkg.Confidence = "medium"
	}

	q := purlQualifiers(purl)
	pkg.Arch = q["arch"]
	if up := q["upstream"]; up != "" && (pkg.Type == "deb" || pkg.Type == "apk") {
		upName, upVersion, _ := strings.Cut(up, "@")
		pkg.SourcePackage = upName
		pkg.SourceVersion = upVersion
	}
	if pkg.Type == "maven" {
		if group := purlNamespace(purl); group != "" {
			pkg.Name = group + ":" + name
		}
	}

	for _, l := range a.Licenses {
		v := strings.TrimSpace(l.SPDXExpression)
		if v == "" {
			v = strings.TrimSpace(l.Value)
		}
		if v != "" && !strings.HasPrefix(v, "sha256:") {
			pkg.Licenses = append(pkg.Licenses, v)
		}
	}
	for _, c := range a.CPEs {
		if v := strings.TrimSpace(c.CPE); v != "" {
			pkg.CPEs = append(pkg.CPEs, v)
		}
	}
	return pkg, true
}

func fortunaTypeForSyft(syftType, purl string) string {
	switch strings.ToLower(strings.TrimSpace(syftType)) {
	case "deb":
		return "deb"
	case "rpm":
		return "rpm"
	case "apk":
		return "apk"
	case "npm":
		return "npm"
	case "python":
		return "pypi"
	case "go-module":
		return "go"
	case "gem":
		return "gem"
	case "rust-crate":
		return "cargo"
	case "java-archive", "jenkins-plugin", "graalvm-native-image":
		if strings.HasPrefix(purl, "pkg:maven/") {
			return "maven"
		}
		return "generic"
	case "binary":
		return "generic"
	default:
		// Ecosystems without a proto enum (nuget, composer, hex, ...): the PURL carries the
		// ecosystem and Core keeps it because no type mismatch can be detected.
		return strings.ToLower(strings.TrimSpace(syftType))
	}
}

// provenanceForSyftCataloger keeps the source names Core already ranks
// (gobinary > gomod > dpkg/apk/rpm) and marks everything else as Syft.
func provenanceForSyftCataloger(foundBy string) string {
	switch foundBy {
	case "dpkg-db-cataloger":
		return "dpkg"
	case "apk-db-cataloger":
		return "apk"
	case "rpm-db-cataloger", "rpm-archive-cataloger":
		return "rpm"
	case "go-module-binary-cataloger":
		return "gobinary"
	case "go-module-file-cataloger":
		return "gomod"
	case "binary-classifier-cataloger":
		return "binary-classifier"
	default:
		if foundBy == "" {
			return "syft"
		}
		return "syft:" + foundBy
	}
}

func purlQualifiers(purl string) map[string]string {
	out := map[string]string{}
	_, query, ok := strings.Cut(purl, "?")
	if !ok {
		return out
	}
	if i := strings.IndexByte(query, '#'); i >= 0 {
		query = query[:i]
	}
	for _, kv := range strings.Split(query, "&") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		if dv, err := url.PathUnescape(v); err == nil {
			v = dv
		}
		out[strings.ToLower(k)] = v
	}
	return out
}

// purlNamespace returns the decoded namespace of pkg:type/namespace/name@version.
func purlNamespace(purl string) string {
	body := strings.TrimPrefix(purl, "pkg:")
	if i := strings.IndexAny(body, "?#"); i >= 0 {
		body = body[:i]
	}
	if i := strings.LastIndex(body, "@"); i >= 0 {
		body = body[:i]
	}
	parts := strings.Split(body, "/")
	if len(parts) < 3 {
		return ""
	}
	ns := strings.Join(parts[1:len(parts)-1], "/")
	if d, err := url.PathUnescape(ns); err == nil {
		ns = d
	}
	return ns
}

// oldestGoVersion returns the lowest "go1.x.y" so stdlib matching covers every binary.
func oldestGoVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	sort.Slice(versions, func(i, j int) bool { return compareGoVersions(versions[i], versions[j]) < 0 })
	return normalizeGoToolchainVersionForSBOM(versions[0])
}

func compareGoVersions(a, b string) int {
	pa := strings.Split(strings.TrimPrefix(strings.TrimPrefix(a, "go"), "v"), ".")
	pb := strings.Split(strings.TrimPrefix(strings.TrimPrefix(b, "go"), "v"), ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			_, _ = fmt.Sscanf(pa[i], "%d", &x)
		}
		if i < len(pb) {
			_, _ = fmt.Sscanf(pb[i], "%d", &y)
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// writeDockerArchive writes img as a docker-archive tarball Syft can read offline.
func writeDockerArchive(ref name.Reference, img v1.Image) (string, error) {
	f, err := os.CreateTemp("", "fortuna-syft-image-*.tar")
	if err != nil {
		return "", fmt.Errorf("image archive file: %w", err)
	}
	path := f.Name()
	_ = f.Close()
	if err := tarball.WriteToFile(path, ref, img); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("write image archive: %w", err)
	}
	return path, nil
}

// extractWithSyft catalogs img with Syft. It returns an error (and the caller falls back
// to the Fortuna parsers) when Syft is unavailable, fails or times out.
func (e *Extractor) extractWithSyft(ctx context.Context, ref name.Reference, img v1.Image) (*SyftCatalog, error) {
	if e.syftAdapter == nil || !e.syftPrimary {
		return nil, errors.New("syft primary disabled")
	}
	archive, err := writeDockerArchive(ref, img)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(archive) }()
	return e.syftAdapter.CatalogArchive(ctx, archive)
}
