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
)

// Extractor extracts SBOM from container images using custom parsers
type Extractor struct {
	parsers map[string]Parser
	logger  *log.Logger
}

// NewExtractor creates a new custom SBOM extractor
func NewExtractor() *Extractor {
	return &Extractor{
		parsers: map[string]Parser{
			"dpkg":  NewDpkgParser(),
			"apk":   NewApkParser(),
			"rpm":   NewRpmParser(),
			"npm":   NewNpmParser(),
			"pip":   NewPipParser(),
			"gomod": NewGoModParser(),
		},
		logger: log.New(log.Writer(), "[SBOMExtractor] ", log.LstdFlags),
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

	// 2. Get image layers
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	// 3. Extract filesystem
	fs, err := e.buildFilesystem(ctx, layers)
	if err != nil {
		return nil, fmt.Errorf("failed to build filesystem: %w", err)
	}

	// 4. Detect OS
	osInfo := e.detectOS(fs)
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
		} else if parserName == "npm" || parserName == "pip" || parserName == "gomod" {
			e.logger.Printf("   Parser %s: 0 packages (no matching files in image)", parserName)
		}
	}

	// 6. Deduplicate
	deduped := e.deduplicate(allPackages)

	sbom := &RawSBOM{
		ImageName:   imageRef,
		ImageDigest: imageDigest, // Include digest for caching
		OS:          osInfo,
		Packages:    deduped,
		ExtractedAt: time.Now(),
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
// 1. Try local Docker daemon first (fast, no rate limits)
// 2. Fall back to remote registry if not found locally
func (e *Extractor) getImage(ctx context.Context, ref name.Reference) (v1.Image, error) {
	// Try containerd first (Kubernetes default runtime)
	if img, err := e.getImageFromContainerd(ctx, ref); err == nil {
		return img, nil
	} else {
		if os.Getenv("SBOM_DEBUG") == "1" || os.Getenv("SBOM_DEBUG") == "true" {
			e.logger.Printf("⚠️  Containerd fetch failed (image/layer may be missing on this node): %v", err)
		}
		e.logger.Printf("Using remote registry for image: %s", ref.Name())
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

// detectOS detects the operating system from filesystem
func (e *Extractor) detectOS(fs *Filesystem) OSInfo {
	// Try to detect OS from common files
	// /etc/os-release (most Linux distros)
	if content, err := fs.ReadFile("/etc/os-release"); err == nil {
		return parseOSRelease(string(content))
	}

	// /etc/debian_version (Debian)
	if content, err := fs.ReadFile("/etc/debian_version"); err == nil {
		return OSInfo{Name: "debian", Version: strings.TrimSpace(string(content))}
	}

	// /etc/alpine-release (Alpine)
	if content, err := fs.ReadFile("/etc/alpine-release"); err == nil {
		return OSInfo{Name: "alpine", Version: strings.TrimSpace(string(content))}
	}

	// Default
	return OSInfo{Name: "unknown", Version: "unknown"}
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

	// If OS is unknown or no OS parsers matched, try all parsers
	// (safer approach for unknown distros)
	if len(osParsers) == 0 || osLower == "unknown" {
		e.logger.Printf("   Unknown OS '%s', trying all parsers", osName)
		return []string{"dpkg", "apk", "rpm", "npm", "pip", "gomod"}
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

// parseOSRelease parses /etc/os-release file
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
	Name    string
	Version string
	Type    string // deb, apk, rpm, npm, pypi, go
	Arch    string
}

// RawSBOM represents the raw extracted SBOM
type RawSBOM struct {
	ImageName   string
	ImageDigest string // Image digest for caching
	OS          OSInfo
	Packages    []Package
	ExtractedAt time.Time
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
