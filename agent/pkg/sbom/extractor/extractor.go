package extractor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

	// 5. Run all parsers
	allPackages := make([]Package, 0)

	for name, parser := range e.parsers {
		packages, err := parser.Parse(fs)
		if err != nil {
			e.logger.Printf("⚠️  Parser %s failed: %v", name, err)
			continue
		}

		if len(packages) > 0 {
			e.logger.Printf("✅ Parser %s found %d packages", name, len(packages))
			allPackages = append(allPackages, packages...)
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
	// Try local daemon first (containerd/Docker)
	// Convert reference to tag for daemon
	e.logger.Printf("🔍 Attempting to load image from local daemon: %s", ref.Name())
	
	// Convert reference to tag if it's not already a tag
	var tagRef name.Tag
	if tag, ok := ref.(name.Tag); ok {
		tagRef = tag
	} else {
		// Try to parse as tag
		if parsedTag, err := name.NewTag(ref.Name()); err == nil {
			tagRef = parsedTag
		} else {
			// If we can't convert to tag, skip daemon and go straight to remote
			e.logger.Printf("⚠️  Cannot convert reference to tag for daemon, using remote: %v", err)
			img, err := remote.Image(ref, remote.WithContext(ctx))
			if err != nil {
				return nil, fmt.Errorf("failed to fetch from remote registry: %w", err)
			}
			e.logger.Printf("✅ Fetched image from remote registry")
			return img, nil
		}
	}
	
	img, err := daemon.Image(tagRef, daemon.WithContext(ctx))
	if err == nil {
		e.logger.Printf("✅ Found image in local daemon (no remote fetch needed)")
		return img, nil
	}

	// Log daemon error but continue to remote fallback
	e.logger.Printf("⚠️  Image not in local daemon (%v), falling back to remote registry", err)

	// Fall back to remote registry
	e.logger.Printf("🔍 Fetching image from remote registry: %s", ref.Name())
	img, err = remote.Image(ref, remote.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from remote registry: %w", err)
	}

	e.logger.Printf("✅ Fetched image from remote registry")
	return img, nil
}

// buildFilesystem builds a virtual filesystem from image layers
func (e *Extractor) buildFilesystem(ctx context.Context, layers []v1.Layer) (*Filesystem, error) {
	fs := NewFilesystem()

	for _, layer := range layers {
		uncompressed, err := layer.Uncompressed()
		if err != nil {
			continue
		}

		// Extract tar archive
		if err := fs.ExtractTar(ctx, uncompressed); err != nil {
			e.logger.Printf("⚠️  Failed to extract layer: %v", err)
			continue
		}
	}

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

