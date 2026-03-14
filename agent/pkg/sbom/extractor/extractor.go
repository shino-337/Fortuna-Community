package extractor

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/namespaces"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/fortuna/agent/pkg/sbom/signatures"
)

// Extractor extracts SBOM from container images using custom parsers
type Extractor struct {
	parsers map[string]Parser
	logger  *log.Logger
	cache   *DiskCache // optional on-disk cache (Finding #8.5 / B2)
}

// NewExtractor creates a new custom SBOM extractor.
// If SBOM_CACHE_DIR is set, enables on-disk cache for SBOM by digest + signature version.
func NewExtractor() *Extractor {
	logger := log.New(log.Writer(), "[SBOMExtractor] ", log.LstdFlags)
	cacheDir := strings.TrimSpace(os.Getenv("SBOM_CACHE_DIR"))
	if cacheDir == "" {
		cacheDir = "/var/lib/fortuna/sbom-cache"
	}
	if cacheDir == "0" || cacheDir == "disabled" || cacheDir == "off" {
		cacheDir = ""
	}
	return &Extractor{
		parsers: map[string]Parser{
			"dpkg":      NewDpkgParser(),
			"apk":       NewApkParser(),
			"rpm":       NewRpmParser(),
			"npm":       NewNpmParser(),
			"pip":       NewPipParser(),
			"gomod":     NewGoModParser(),
			"distroless": NewDistrolessParser(), // Finding #8.2 / C1: walk /bin, /usr/bin, /usr/lib
		},
		logger: logger,
		cache:  NewDiskCache(cacheDir, logger),
	}
}

// ExtractSBOM extracts SBOM from a container image
func (e *Extractor) ExtractSBOM(
	ctx context.Context,
	imageRef string,
) (*RawSBOM, error) {
	start := time.Now()
	e.logger.Printf("Extracting SBOM from image: %s", imageRef)

	// 1. Get image (local-first approach)
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %w", err)
	}

	img, err := e.getImage(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	// Extract image digest for caching
	var imageDigest string
	if digestRef, ok := ref.(name.Digest); ok {
		imageDigest = digestRef.DigestStr()
		e.logger.Printf("✅ Extracted digest from reference: %s", imageDigest)
	} else {
		// Get digest from image manifest
		e.logger.Printf("🔍 Attempting to get digest from image manifest...")
		if digest, err := img.Digest(); err == nil {
			imageDigest = digest.String()
			e.logger.Printf("✅ Extracted digest from image manifest: %s", imageDigest)
		} else {
			e.logger.Printf("⚠️  Failed to get digest from image: %v", err)
			// Use image reference as fallback (but log warning)
			imageDigest = ref.Name()
			e.logger.Printf("⚠️  Using image reference as digest fallback: %s", imageDigest)
		}
	}

	// 1b. Cache lookup (Finding #8.5 / B2): skip extract if we have a valid cached SBOM
	sigVersion := signatures.Version()
	if e.cache != nil {
		if cached, err := e.cache.Get(imageDigest, sigVersion); err == nil && cached != nil {
			cached.ImageName = imageRef
			cached.SignatureVersion = sigVersion
			return cached, nil
		}
	}

	// 2. OCI config for labels (ref name, distroless hint – Finding #8.1)
	var imageConfig *v1.ConfigFile
	if cfg, err := img.ConfigFile(); err == nil {
		imageConfig = cfg
	}

	// 3. Get image layers
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	// 4. Extract filesystem
	fs, err := e.buildFilesystem(ctx, layers)
	if err != nil {
		return nil, fmt.Errorf("failed to build filesystem: %w", err)
	}

	// 5. Detect OS (filesystem + optional OCI labels)
	osInfo := e.detectOS(fs, imageConfig)
	e.logger.Printf("Detected OS: %s %s", osInfo.Name, osInfo.Version)

	// 5. Run OS-specific parsers (skip parsers that won't work for this OS)
	allPackages := make([]Package, 0)
	parsersToRun := e.selectParsersForOS(osInfo.Name)

	for _, parserName := range parsersToRun {
		parser := e.parsers[parserName]
		packages, err := parser.Parse(fs)
		if err != nil {
			e.logger.Printf("   Parser %s: not applicable (OS: %s)", parserName, osInfo.Name)
			continue
		}
		if len(packages) > 0 {
			e.logger.Printf("✅ Parser %s found %d packages", parserName, len(packages))
			allPackages = append(allPackages, packages...)
		} else if parserName == "npm" || parserName == "pip" || parserName == "gomod" || parserName == "distroless" {
			e.logger.Printf("   Parser %s: 0 packages (no matching files in image)", parserName)
		}
	}

	// 6. Deduplicate
	deduped := e.deduplicate(allPackages)

	// 7. SBOM-level source/confidence: if any package is from distroless-heuristic, mark SBOM accordingly
	sbomSource := "parsers"
	confidence := "high"
	for _, pkg := range deduped {
		if pkg.Source == "distroless-heuristic" {
			sbomSource = "distroless-heuristic"
			if pkg.Confidence == "medium" {
				confidence = "medium"
			} else if confidence != "medium" {
				confidence = "low"
			}
			break
		}
	}

	// 8. Distroless/system fallback (Finding #8): when no package manager found, emit one synthetic component
	if len(deduped) == 0 {
		synthetic := e.syntheticPackageFromImage(imageRef, imageConfig)
		deduped = append(deduped, synthetic)
		sbomSource = "distroless-heuristic"
		confidence = synthetic.Confidence
		e.logger.Printf("   No packages from parsers; added synthetic component for distroless/system image: %s (PURL=%s)", synthetic.Name, synthetic.PURL)
	}

	sbom := &RawSBOM{
		ImageName:        imageRef,
		ImageDigest:      imageDigest,
		OS:               osInfo,
		Packages:         deduped,
		ExtractedAt:      time.Now(),
		SBOMSource:       sbomSource,
		Confidence:       confidence,
		SignatureVersion: sigVersion,
	}

	if e.cache != nil {
		_ = e.cache.Set(imageDigest, sigVersion, sbom)
	}

	e.logger.Printf("✅ Extracted %d unique packages in %v", len(deduped), time.Since(start))
	return sbom, nil
}

// ResolveDigest resolves an immutable image digest (sha256:...) without extracting layers.
// This is used for cache-first SBOM behavior.
func (e *Extractor) ResolveDigest(ctx context.Context, imageRef string) (string, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// If the reference already includes a digest, trust it
	if digestRef, ok := ref.(name.Digest); ok {
		return digestRef.DigestStr(), nil
	}

	img, err := e.getImage(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to get image for digest resolution: %w", err)
	}

	d, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("failed to resolve digest: %w", err)
	}
	return d.String(), nil
}

// getImage retrieves an image using local-first approach
// 1. Try containerd first (Kubernetes default runtime; fast, no rate limits)
// 2. Fall back to remote registry if not found or content missing locally
//
// Containerd can fail with "content digest ... not found" when the image
// metadata exists but layer blobs are missing on this node (e.g. agent runs
// on a different node, or content was GC'd). Set SBOM_PREFER_REGISTRY=1 to
// skip containerd and always use the registry.
func (e *Extractor) getImage(ctx context.Context, ref name.Reference) (v1.Image, error) {
	useRegistry := os.Getenv("SBOM_PREFER_REGISTRY") == "1" || os.Getenv("SBOM_PREFER_REGISTRY") == "true"

	if !useRegistry {
		if img, err := e.getImageFromContainerd(ctx, ref); err == nil {
			return img, nil
		} else {
			e.logger.Printf("⚠️  Containerd fetch failed (image/layer may be missing on this node): %v", err)
			e.logger.Printf("🔍 Falling back to remote registry: %s", ref.Name())
		}
	} else {
		e.logger.Printf("Using remote registry (SBOM_PREFER_REGISTRY): %s", ref.Name())
	}

	// Fall back to remote registry
	img, err := remote.Image(ref, remote.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from remote registry: %w", err)
	}
	e.logger.Printf("Fetched image from remote registry: %s", ref.Name())
	return img, nil
}

func (e *Extractor) getImageFromContainerd(ctx context.Context, ref name.Reference) (v1.Image, error) {
	socket := getEnv("CONTAINERD_SOCKET", "/run/containerd/containerd.sock")
	if _, err := os.Stat(socket); err != nil {
		return nil, fmt.Errorf("containerd socket not available: %w", err)
	}

	namespace := getEnv("CONTAINERD_NAMESPACE", "k8s.io")
	client, err := containerd.New(socket)
	if err != nil {
		return nil, fmt.Errorf("containerd client error: %w", err)
	}
	defer client.Close()

	cctx := namespaces.WithNamespace(ctx, namespace)
	candidates := containerdImageNames(ref.Name())

	var selected string
	for _, name := range candidates {
		if _, err := client.ImageService().Get(cctx, name); err == nil {
			selected = name
			break
		}
	}
	if selected == "" {
		return nil, fmt.Errorf("image not found in containerd: %s", ref.Name())
	}

	tmp, err := os.CreateTemp("", "fortuna-image-*.tar")
	if err != nil {
		return nil, fmt.Errorf("temp file error: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	if err := archive.Export(cctx, client.ContentStore(), tmp, archive.WithImage(client.ImageService(), selected)); err != nil {
		return nil, fmt.Errorf("containerd export error: %w", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("containerd export seek error: %w", err)
	}

	img, err := tarball.ImageFromPath(tmp.Name(), nil)
	if err != nil {
		return nil, fmt.Errorf("tarball parse error: %w", err)
	}

	// Materialize image into memory so we can close/remove the temp file.
	// Otherwise layer.Uncompressed() would read from the deleted file and fail.
	digest, _ := img.Digest()
	configFile, _ := img.ConfigFile()
	rawConfig, _ := img.RawConfigFile()
	manifest, _ := img.Manifest()
	rawManifest, _ := img.RawManifest()
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("image layers: %w", err)
	}
	memLayers := make([]*memLayer, 0, len(layers))
	for _, layer := range layers {
		diffID, _ := layer.DiffID()
		layerDigest, _ := layer.Digest()
		rc, err := layer.Uncompressed()
		if err != nil {
			e.logger.Printf("⚠️  Layer Uncompressed failed (materialize): %v", err)
			continue
		}
		blob, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			e.logger.Printf("⚠️  Layer read failed: %v", err)
			continue
		}
		memLayers = append(memLayers, &memLayer{diffID: diffID, digest: layerDigest, blob: blob})
	}

	e.logger.Printf("✅ Found image in containerd: %s (materialized %d layers)", selected, len(memLayers))
	return &materializedImage{
		configFile:  configFile,
		rawConfig:   rawConfig,
		digest:      digest,
		manifest:    manifest,
		rawManifest: rawManifest,
		layers:      memLayers,
	}, nil
}

func containerdImageNames(refName string) []string {
	names := []string{refName}
	if strings.HasPrefix(refName, "index.docker.io/") {
		names = append(names, strings.Replace(refName, "index.docker.io", "docker.io", 1))
	}
	if strings.HasPrefix(refName, "docker.io/") {
		names = append(names, strings.Replace(refName, "docker.io", "index.docker.io", 1))
	}
	return names
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// buildFilesystem builds a virtual filesystem from image layers (base → top; overlay semantics).
func (e *Extractor) buildFilesystem(ctx context.Context, layers []v1.Layer) (*Filesystem, error) {
	fs := NewFilesystem()

	for i, layer := range layers {
		uncompressed, err := layer.Uncompressed()
		if err != nil {
			e.logger.Printf("⚠️  Layer %d Uncompressed failed: %v", i, err)
			continue
		}

		if err := fs.ExtractTar(ctx, uncompressed); err != nil {
			_ = uncompressed.Close()
			e.logger.Printf("⚠️  Layer %d ExtractTar failed: %v", i, err)
			continue
		}
		_ = uncompressed.Close()
	}

	// Debug: count paths that matter for npm/SBOM
	n := 0
	for path := range fs.files {
		if strings.Contains(path, "package.json") {
			n++
		}
	}
	e.logger.Printf("   Virtual FS: %d files total, %d paths containing package.json", len(fs.files), n)

	return fs, nil
}

// OCI label keys (Finding #8.1)
const (
	labelRefName = "org.opencontainers.image.ref.name"
)

// detectOS detects the operating system from filesystem and optionally OCI image config labels (Finding #8.1).
// Distroless is identified by: (1) /etc/os-release with PRETTY_NAME or NAME containing "Distroless", or
// (2) OCI image config labels with key/value containing "distroless".
func (e *Extractor) detectOS(fs *Filesystem, imageConfig *v1.ConfigFile) OSInfo {
	// 1. /etc/os-release (most Linux distros; Google distroless sets PRETTY_NAME="Distroless")
	if content, err := fs.ReadFile("/etc/os-release"); err == nil {
		osInfo := parseOSRelease(string(content))
		if isDistrolessFromOSRelease(string(content)) {
			osInfo.Name = "distroless"
			e.logger.Printf("   /etc/os-release suggests distroless (PRETTY_NAME or NAME)")
		}
		return osInfo
	}
	if content, err := fs.ReadFile("/etc/debian_version"); err == nil {
		return OSInfo{Name: "debian", Version: strings.TrimSpace(string(content))}
	}
	if content, err := fs.ReadFile("/etc/alpine-release"); err == nil {
		return OSInfo{Name: "alpine", Version: strings.TrimSpace(string(content))}
	}

	// 2. No os-release: use OCI labels for distroless hint and version
	if imageConfig != nil && imageConfig.Config.Labels != nil {
		labels := imageConfig.Config.Labels
		version := labels[labelRefName]
		if version == "" {
			version = "unknown"
		}
		for k, v := range labels {
			if strings.Contains(strings.ToLower(k), "distroless") || strings.Contains(strings.ToLower(v), "distroless") {
				e.logger.Printf("   OCI label suggests distroless: %s=%s", k, v)
				return OSInfo{Name: "distroless", Version: version}
			}
		}
		return OSInfo{Name: "unknown", Version: version}
	}
	return OSInfo{Name: "unknown", Version: "unknown"}
}

// isDistrolessFromOSRelease returns true if os-release content indicates distroless (e.g. PRETTY_NAME="Distroless").
func isDistrolessFromOSRelease(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "pretty_name=\"distroless\"") ||
		strings.Contains(lower, "name=\"distroless\"") ||
		strings.Contains(lower, "pretty_name=distroless") ||
		strings.Contains(lower, "name=distroless")
}

// selectParsersForOS selects parsers based on detected OS
// This avoids running dpkg on Alpine, apk on Debian, etc.
func (e *Extractor) selectParsersForOS(osName string) []string {
	// Normalize OS name
	osLower := strings.ToLower(osName)

	// OS-specific parsers
	osParsers := make([]string, 0)

	// Debian/Ubuntu
	if strings.Contains(osLower, "debian") || strings.Contains(osLower, "ubuntu") {
		osParsers = append(osParsers, "dpkg")
	}

	// Alpine
	if strings.Contains(osLower, "alpine") {
		osParsers = append(osParsers, "apk")
	}

	// RHEL/CentOS/Fedora
	if strings.Contains(osLower, "rhel") || strings.Contains(osLower, "centos") ||
		strings.Contains(osLower, "fedora") || strings.Contains(osLower, "rocky") ||
		strings.Contains(osLower, "alma") {
		osParsers = append(osParsers, "rpm")
	}

	// Language package managers (run for all OS types)
	languageParsers := []string{"npm", "pip", "gomod"}

	// If OS is unknown/distroless or no OS parsers matched, try all parsers + distroless (C1)
	if len(osParsers) == 0 || osLower == "unknown" || osLower == "distroless" {
		e.logger.Printf("   OS '%s', trying all parsers including distroless", osName)
		return []string{"dpkg", "apk", "rpm", "npm", "pip", "gomod", "distroless"}
	}

	// Combine OS parsers + language parsers
	allParsers := append(osParsers, languageParsers...)
	e.logger.Printf("   Selected parsers for OS '%s': %v", osName, allParsers)
	return allParsers
}

// deduplicate removes duplicate packages
func (e *Extractor) deduplicate(packages []Package) []Package {
	seen := make(map[string]bool)
	deduped := make([]Package, 0)

	for _, pkg := range packages {
		key := fmt.Sprintf("%s:%s:%s", pkg.Type, pkg.Name, pkg.Version)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, pkg)
		}
	}

	return deduped
}

// syntheticPackageFromImage returns one synthetic package for distroless/system images (0 packages).
// Uses OCI config label org.opencontainers.image.ref.name for version when available (Finding #8.1).
// Sets PURL pkg:generic/name@version and Source/Confidence for Core CVE matching (Finding #8.2, 8.4).
func (e *Extractor) syntheticPackageFromImage(imageRef string, imageConfig *v1.ConfigFile) Package {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		p := Package{Name: "unknown-image", Version: "unknown", Type: "generic", Source: "distroless-heuristic", Confidence: "low"}
		p.PURL = "pkg:generic/unknown-image@unknown"
		return p
	}
	fullName := ref.Context().Name()
	parts := strings.Split(fullName, "/")
	namePart := "unknown-image"
	if len(parts) > 0 {
		namePart = parts[len(parts)-1]
	}
	version := ref.Identifier()
	if version == "" {
		version = "unknown"
	}
	// Prefer OCI label when present (e.g. tag from build)
	confidence := "low"
	if imageConfig != nil && imageConfig.Config.Labels != nil {
		if v := imageConfig.Config.Labels[labelRefName]; v != "" {
			if idx := strings.LastIndex(v, ":"); idx >= 0 && idx < len(v)-1 {
				version = v[idx+1:]
			} else {
				version = v
			}
			confidence = "medium" // version from OCI label is more reliable
		}
	}
	// PURL per Finding #8.2: pkg:generic/name@version for NVD/join
	purl := fmt.Sprintf("pkg:generic/%s@%s", namePart, version)
	return Package{
		Name:       namePart,
		Version:    version,
		Type:       "generic",
		PURL:       purl,
		Source:     "distroless-heuristic",
		Confidence: confidence,
	}
}

// parseOSRelease parses /etc/os-release (ID=, VERSION_ID=). Distroless override is done in detectOS via isDistrolessFromOSRelease.
func parseOSRelease(content string) OSInfo {
	osInfo := OSInfo{Name: "unknown", Version: "unknown"}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			osInfo.Name = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			osInfo.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}
	return osInfo
}

// Package represents a package found in the image
type Package struct {
	Name       string
	Version    string
	Type       string // deb, apk, rpm, npm, pypi, go, generic
	Arch       string
	PURL       string // Canonical Package URL e.g. pkg:generic/coredns@1.11.0 (Finding #8.2)
	Source     string // "parsers" | "distroless-heuristic" | "label-metadata" (Finding #8.4)
	Confidence string // "low" | "medium" | "high"
}

// RawSBOM represents the raw extracted SBOM
type RawSBOM struct {
	ImageName   string
	ImageDigest string // Image digest for caching
	OS          OSInfo
	Packages    []Package
	ExtractedAt time.Time
	// SBOM-level provenance (Finding #8.4) – set when synthetic/heuristic is used
	SBOMSource  string // "parsers" | "distroless-heuristic" | "label-metadata"
	Confidence  string // "low" | "medium" | "high"
	// SignatureVersion (B3): version of signature DB for cache invalidation
	SignatureVersion string
}

// OSInfo represents OS information
type OSInfo struct {
	Name    string
	Version string
}

// Parser interface for package parsers
type Parser interface {
	Parse(fs *Filesystem) ([]Package, error)
}
