package extractor

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/namespaces"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	sbomversion "github.com/fortuna/agent/pkg/sbom/version"
	"github.com/fortuna/agent/pkg/sbom/signatures"
)

// Extractor extracts SBOM from container images using custom parsers
type Extractor struct {
	parsers map[string]Parser
	logger  *log.Logger
	cache   *DiskCache // optional on-disk cache (Finding #8.5 / B2)

	// Syft fallback (slow, comprehensive) for distroless/minimal images where package-manager metadata is missing.
	syftEnabled             bool
	syftAdapter             *SyftAdapter
	syftMinPackageThreshold int
	syftCache               *SyftResultCache
	syftMaxRetries          int
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

	// Syft fallback flags.
	// Default: enabled.
	syftEnabled := true
	if v := strings.TrimSpace(os.Getenv("SBOM_USE_SYFT_FALLBACK")); v != "" {
		switch strings.ToLower(v) {
		case "0", "false", "off", "disabled":
			syftEnabled = false
		}
	}

	syftMinPkgs := 20
	if v := strings.TrimSpace(os.Getenv("SBOM_SYFT_MIN_PACKAGE_THRESHOLD")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			syftMinPkgs = n
		}
	}

	// Timeout accepts either Go duration (e.g. "300s") or raw seconds (e.g. "300").
	syftTimeout := 5 * time.Minute
	if v := strings.TrimSpace(os.Getenv("SBOM_SYFT_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			syftTimeout = d
		} else if n, err := strconv.Atoi(v); err == nil && n > 0 {
			syftTimeout = time.Duration(n) * time.Second
		}
	}

	syftMaxPkgs := 1000
	if v := strings.TrimSpace(os.Getenv("SBOM_SYFT_MAX_PACKAGES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			syftMaxPkgs = n
		}
	}

	// Syft retry (transient failures).
	syftMaxRetries := 2
	if v := strings.TrimSpace(os.Getenv("SBOM_SYFT_MAX_RETRIES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			syftMaxRetries = n
		}
	}

	// Syft in-memory cache (per-agent process; avoids rerunning Syft for same digest repeatedly).
	syftCacheTTL := 1 * time.Hour
	if v := strings.TrimSpace(os.Getenv("SBOM_SYFT_CACHE_TTL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			syftCacheTTL = d
		} else if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			syftCacheTTL = time.Duration(n) * time.Second
		}
	}
	syftCacheMax := 256
	if v := strings.TrimSpace(os.Getenv("SBOM_SYFT_CACHE_MAX_ITEMS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			syftCacheMax = n
		}
	}
	var syftCache *SyftResultCache
	if syftEnabled && syftCacheTTL > 0 && syftCacheMax > 0 {
		syftCache = NewSyftResultCache(syftCacheTTL, syftCacheMax)
	}

	var syftAdapter *SyftAdapter
	if syftEnabled {
		syftAdapter = NewSyftAdapter(logger, syftTimeout, syftMaxPkgs)
		// If enabled but binary missing, warn once at startup.
		if !syftAdapter.HasSyftBinary() {
			logger.Printf("⚠️  WARNING: SBOM_USE_SYFT_FALLBACK is enabled but syft binary not found (bin=%q). Falling back to Fortuna-only.", syftAdapter.syftBin)
		}
	}

	return &Extractor{
		parsers: map[string]Parser{
			"dpkg":       NewDpkgParser(),
			"apk":        NewApkParser(),
			"rpm":        NewRpmParser(),
			"npm":        NewNpmParser(),
			"pip":        NewPipParser(),
			"gomod":      NewGoModParser(),
			"gobinary":   NewGoBinaryParser(),
			"maven":      NewMavenParser(),
			"cargo":      NewCargoParser(),
			"ruby":       NewRubyGemsParser(),
			"nuget":      NewNuGetParser(),
			"distroless": NewDistrolessParser(), // Finding #8.2 / C1: walk /bin, /usr/bin, /usr/lib
		},
		logger: logger,
		cache:  NewDiskCache(cacheDir, logger),

		syftEnabled:             syftEnabled,
		syftAdapter:             syftAdapter,
		syftMinPackageThreshold: syftMinPkgs,
		syftCache:               syftCache,
		syftMaxRetries:          syftMaxRetries,
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
			// Do not use cache when cached has 0 packages (e.g. distroless before synthetic fallback existed)
			// so we re-extract and get at least synthetic component (Finding #8).
			if len(cached.Packages) > 0 {
				cached.ImageName = imageRef
				cached.SignatureVersion = sigVersion
				return cached, nil
			}
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
	defer func() { _ = fs.Close() }()

	// 5. Detect OS (filesystem + optional OCI labels)
	osInfo := e.detectOS(fs, imageConfig)
	e.logger.Printf("Detected OS: %s %s", osInfo.Name, osInfo.Version)

	// 5. Run OS-specific parsers (skip parsers that won't work for this OS)
	allPackages := make([]Package, 0)
	parsersToRun := e.selectParsersForOS(osInfo.Name)
	hasOSPackages := false

	for _, parserName := range parsersToRun {
		// When dpkg/apk/rpm already returned packages, skip distroless to avoid adding junk from /usr/lib
		if parserName == "distroless" && hasOSPackages {
			e.logger.Printf("   Parser distroless: skipped (OS package manager already returned packages)")
			continue
		}
		parser := e.parsers[parserName]
		packages, err := parser.Parse(fs)
		if err != nil {
			e.logger.Printf("   Parser %s: not applicable (OS: %s)", parserName, osInfo.Name)
			continue
		}
		if len(packages) > 0 {
			e.logger.Printf("✅ Parser %s found %d packages", parserName, len(packages))
			allPackages = append(allPackages, packages...)
			if parserName == "dpkg" || parserName == "apk" || parserName == "rpm" {
				hasOSPackages = true
			}
		} else if parserName == "npm" || parserName == "pip" || parserName == "gomod" || parserName == "maven" || parserName == "cargo" || parserName == "ruby" || parserName == "nuget" || parserName == "distroless" {
			e.logger.Printf("   Parser %s: 0 packages (no matching files in image)", parserName)
		}
	}

	// 5b. Apply signature hints (versionFromTag/digestMap) to distroless/system binaries to reduce @unknown (Finding #8.3 B1/B2)
	imageTag := ref.Identifier()
	sigData := signatures.LoadDistroless()
	deduped := e.deduplicate(allPackages)
	// Debug: log version resolution inputs for control-plane (when we have signature entries)
	if sigData != nil && imageConfig != nil {
		for _, k := range []string{"org.opencontainers.image.version", "io.k8s.display-version", "org.opencontainers.image.ref.name"} {
			if v, ok := imageConfig.Config.Labels[k]; ok && v != "" {
				e.logger.Printf("   [control-plane version] OCI label %s=%q", k, v)
			}
		}
	}
	e.logger.Printf("   [control-plane version] imageTag=%q imageDigest=%s (digestMap/tag/labels will be applied to known binaries)", imageTag, imageDigest)
	deduped = applySignatureHints(e.logger, deduped, imageTag, imageDigest, imageConfig, sigData)

	// 5c. Set PURL pkg:deb/<distro>/name@version for dpkg/apk so Core queries OSV debian/ubuntu/alpine
	deduped = setOSPackagePURLs(deduped, osInfo)

	// 5c.1 Expand Debian transitive dependencies from dpkg metadata (Depends/Pre-Depends).
	// This improves CVE coverage for images where direct package lists miss linked runtime deps.
	deduped = e.expandDebianTransitivePackages(deduped, fs, osInfo)
	deduped = setOSPackagePURLs(deduped, osInfo)

	// 5d. Syft fallback (Slow path): distroless/minimal images where package-manager metadata is missing.
	fortunaPkgCount := len(deduped)
	if e.syftAdapter != nil && e.shouldInvokeSyft(fortunaPkgCount, osInfo.Name) {
		cacheKey := imageDigest
		if cacheKey == "" {
			cacheKey = imageRef
		}

		if cached := e.syftCache.Get(cacheKey); len(cached) > 0 {
			e.logger.Printf("✅ [FALLBACK] Syft cache hit (%d packages); merging...", len(cached))
			merged := e.mergeSyftPackages(deduped, cached)
			deduped = e.deduplicate(merged)
			deduped = setOSPackagePURLs(deduped, osInfo)
			goto afterSyftFallback
		}

		e.logger.Printf("   [FALLBACK] Fortuna found %d packages; invoking Syft...", fortunaPkgCount)
		syftPkgs, err := e.invokeSyftWithRetry(ctx, imageRef)
		if err != nil {
			e.logger.Printf("   [FALLBACK] Syft discovery failed (continuing with Fortuna results): %v", err)
		} else if len(syftPkgs) > 0 {
			e.logger.Printf("✅ [FALLBACK] Syft found %d packages; merging...", len(syftPkgs))
			e.syftCache.Set(cacheKey, syftPkgs)
			merged := e.mergeSyftPackages(deduped, syftPkgs)
			deduped = e.deduplicate(merged)
			// Ensure OS package PURLs for Core OSV queries.
			deduped = setOSPackagePURLs(deduped, osInfo)
		} else {
			e.logger.Printf("   [FALLBACK] Syft found 0 packages")
		}
	}
afterSyftFallback:

	// 6b. Enrich versions from distroless binaries when Syft/native parsers yield "@unknown"
	deduped = e.enrichBinaryVersions(deduped, fs)

	// 6. Deduplicate


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

	goToolchain := ""
	if gp, ok := e.parsers["gobinary"].(*GoBinaryParser); ok {
		goToolchain = gp.ToolchainVersion(fs)
	}
	if goToolchain == "" {
		goToolchain = goVersionFromImageConfig(imageConfig)
	}
	goToolchain = normalizeGoToolchainVersionForSBOM(goToolchain)
	if goToolchain != "" {
		e.logger.Printf("   Go toolchain version (stdlib matching): %s", goToolchain)
	}

	e.logSBOMFilesystemMetrics(fs)

	sbom := &RawSBOM{
		ImageName:        imageRef,
		ImageDigest:      imageDigest,
		OS:               osInfo,
		Packages:         deduped,
		ExtractedAt:      time.Now(),
		SBOMSource:       sbomSource,
		Confidence:       confidence,
		SignatureVersion: sigVersion,
		GoVersion:        goToolchain,
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
// Lỗi "content digest sha256:xxx: not found": metadata image có trong containerd (ImageService.Get
// thành công) nhưng một hoặc nhiều layer blob thiếu trong ContentStore trên node này (vd. image
// chưa pull đủ, content bị GC, hoặc pod chạy node khác nên node này chỉ có reference). Agent sẽ
// tự fallback sang registry và vẫn extract SBOM bình thường. Để bỏ qua containerd hoàn toàn:
// SBOM_PREFER_REGISTRY=1.
func (e *Extractor) getImage(ctx context.Context, ref name.Reference) (v1.Image, error) {
	useRegistry := os.Getenv("SBOM_PREFER_REGISTRY") == "1" || os.Getenv("SBOM_PREFER_REGISTRY") == "true"

	if !useRegistry {
		if img, err := e.getImageFromContainerd(ctx, ref); err == nil {
			return img, nil
		} else {
			e.logger.Printf("[INFO] Local containerd miss (layer/content not on this node): %v", err)
			e.logger.Printf("[INFO] Falling back to registry: %s", ref.Name())
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

// logSBOMFilesystemMetrics logs A5 Phase 2c counters (indexed vs materialize, lazy ReadFile volume).
// Disable with SBOM_FS_METRICS=0|off|false.
func (e *Extractor) logSBOMFilesystemMetrics(fs *Filesystem) {
	if fs == nil {
		return
	}
	s := strings.TrimSpace(os.Getenv("SBOM_FS_METRICS"))
	if s == "0" || strings.EqualFold(s, "off") || strings.EqualFold(s, "false") {
		return
	}
	m := fs.MetricsSnapshot()
	if m.Mode == fsModeIndexed {
		e.logger.Printf("[SBOM FS] mode=indexed paths=%d materialized=%d (%s) lazy_reads=%d (%s)",
			m.IndexedPaths, m.MaterializedFiles, formatBytesIEC(m.MaterializedBytes),
			m.LazyReadOps, formatBytesIEC(int64(m.LazyReadBytes)))
		return
	}
	e.logger.Printf("[SBOM FS] mode=materialize files=%d stored=%s",
		m.MaterializedFiles, formatBytesIEC(m.MaterializedBytes))
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

		if err := fs.ExtractTar(ctx, i, uncompressed); err != nil {
			_ = uncompressed.Close()
			e.logger.Printf("⚠️  Layer %d ExtractTar failed: %v", i, err)
			continue
		}
		_ = uncompressed.Close()
	}

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

	// Language package managers + Go binary analyzer (run for all OS types)
	languageParsers := []string{"npm", "pip", "gomod", "gobinary", "maven", "cargo", "ruby", "nuget"}

	// If OS is unknown/distroless or no OS parsers matched, try all parsers + distroless (C1)
	if len(osParsers) == 0 || osLower == "unknown" || osLower == "distroless" {
		e.logger.Printf("   OS '%s', trying all parsers including distroless and gobinary", osName)
		return []string{"dpkg", "apk", "rpm", "npm", "pip", "gomod", "gobinary", "maven", "cargo", "ruby", "nuget", "distroless"}
	}

	// Combine OS parsers + language parsers; for debian/ubuntu also add distroless so
	// distroless/base images (e.g. gcr.io/distroless/static-debian12) get binary packages or at least synthetic
	allParsers := append(osParsers, languageParsers...)
	if strings.Contains(osLower, "debian") || strings.Contains(osLower, "ubuntu") {
		allParsers = append(allParsers, "distroless")
	}
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

func (e *Extractor) shouldInvokeSyft(fortunaPkgCount int, osName string) bool {
	if e == nil || e.syftAdapter == nil || !e.syftEnabled {
		return false
	}
	osLower := strings.ToLower(strings.TrimSpace(osName))

	// Distroless/minimal images often yield 0 Fortuna packages (missing dpkg/apk/rpm metadata),
	// so treat 0 as a strong signal to run Syft.
	if fortunaPkgCount == 0 {
		return osLower == "unknown" || osLower == "generic" || strings.Contains(osLower, "distroless")
	}

	// If we are in distroless mode but Fortuna extracted only a handful of packages,
	// Syft can still fill in the missing OS-level package inventory.
	if strings.Contains(osLower, "distroless") && fortunaPkgCount < e.syftMinPackageThreshold {
		return true
	}

	return false
}

func (e *Extractor) mergeSyftPackages(fortuna []Package, syft []Package) []Package {
	// Prefer Fortuna packages when duplicate by PURL; otherwise include Syft discoveries.
	seen := make(map[string]bool, len(fortuna)+len(syft))
	merged := make([]Package, 0, len(fortuna)+len(syft))

	packageKey := func(p Package) string {
		if p.PURL != "" {
			return "purl:" + strings.ToLower(strings.TrimSpace(p.PURL))
		}
		return p.Type + "|" + p.Name + "|" + p.Version
	}

	for _, p := range fortuna {
		key := packageKey(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, p)
	}

	for _, p := range syft {
		key := packageKey(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, p)
	}

	return merged
}

func (e *Extractor) expandDebianTransitivePackages(pkgs []Package, fs *Filesystem, osInfo OSInfo) []Package {
	if fs == nil {
		return pkgs
	}
	osLower := strings.ToLower(strings.TrimSpace(osInfo.Name))
	if !(strings.Contains(osLower, "debian") || strings.Contains(osLower, "ubuntu") || strings.Contains(osLower, "distroless")) {
		return pkgs
	}

	infos := collectDebianPackageInfos(fs)
	if len(infos) == 0 {
		return pkgs
	}

	existing := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		if p.Type == "deb" {
			existing[p.Name] = true
		}
	}

	visited := make(map[string]bool)
	toAdd := make([]Package, 0)
	for _, p := range pkgs {
		if p.Type != "deb" || strings.TrimSpace(p.Name) == "" {
			continue
		}
		e.resolveDebianDepsRecursive(p.Name, infos, existing, visited, 0, &toAdd)
	}

	if len(toAdd) == 0 {
		return pkgs
	}
	return append(pkgs, toAdd...)
}

func (e *Extractor) resolveDebianDepsRecursive(
	name string,
	infos map[string]DebianPackageInfo,
	existing map[string]bool,
	visited map[string]bool,
	depth int,
	out *[]Package,
) {
	const maxDepth = 10
	if depth > maxDepth || name == "" {
		return
	}
	vKey := fmt.Sprintf("%s|%d", name, depth)
	if visited[vKey] {
		return
	}
	visited[vKey] = true

	info, ok := infos[name]
	if !ok {
		return
	}
	deps := append([]DebianDependency{}, info.PreDepends...)
	deps = append(deps, info.Depends...)
	for _, depName := range pickDependencyCandidates(deps, infos) {
		if depName == "" || depName == name {
			continue
		}
		if !existing[depName] {
			if depInfo, ok := infos[depName]; ok {
				existing[depName] = true
				*out = append(*out, Package{
					Name:          depInfo.Name,
					Version:       depInfo.Version,
					Type:          "deb",
					Source:        "dpkg-transitive",
					Confidence:    "medium",
					SourcePackage: depInfo.Name,
				})
			}
		}
		e.resolveDebianDepsRecursive(depName, infos, existing, visited, depth+1, out)
	}
}

func pickDependencyCandidates(deps []DebianDependency, infos map[string]DebianPackageInfo) []string {
	out := make([]string, 0)
	for i := 0; i < len(deps); {
		// Group alternatives by OR.
		group := []DebianDependency{deps[i]}
		j := i + 1
		for j < len(deps) && deps[j].Or {
			group = append(group, deps[j])
			j++
		}
		chosen := ""
		// Prefer first option present in this image.
		for _, d := range group {
			if _, ok := infos[d.Package]; ok {
				chosen = d.Package
				break
			}
		}
		// Fallback to first option.
		if chosen == "" && len(group) > 0 {
			chosen = group[0].Package
		}
		if chosen != "" {
			out = append(out, chosen)
		}
		i = j
	}
	return out
}

func collectDebianPackageInfos(fs *Filesystem) map[string]DebianPackageInfo {
	out := make(map[string]DebianPackageInfo)
	merge := func(items []DebianPackageInfo) {
		for _, it := range items {
			if strings.TrimSpace(it.Name) == "" {
				continue
			}
			out[it.Name] = it
		}
	}

	if content, err := fs.ReadFile(dpkgStatusFile); err == nil {
		merge(parseDpkgStatusDetails(string(content)))
	}
	for _, path := range fs.PathsUnder(dpkgStatusDir) {
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".md5sums") || base == "" || base == "." {
			continue
		}
		content, err := fs.ReadFile(path)
		if err != nil {
			continue
		}
		merge(parseDpkgStatusDetails(string(content)))
	}
	return out
}

// enrichBinaryVersions attempts to fill missing versions for binary-derived SBOM components
// (primarily distroless/control-plane) by extracting version metadata from the binary itself.
//
// It is best-effort: if the binary content can't be read (e.g., A5 file-too-large), versions remain "unknown".
func (e *Extractor) enrichBinaryVersions(pkgs []Package, fs *Filesystem) []Package {
	if fs == nil {
		return pkgs
	}
	ve := sbomversion.NewExtractor(e.logger)

	for i := range pkgs {
		p := &pkgs[i]
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}

		// Only enrich when we currently have no usable version.
		purlHasUnknown := strings.Contains(strings.TrimSpace(p.PURL), "@unknown")
		if strings.TrimSpace(p.Version) != "unknown" && !purlHasUnknown {
			continue
		}

		candidates := candidateBinaryPaths(name)

		var content []byte
		found := false
		for _, path := range candidates {
			if !fs.FileExists(path) {
				continue
			}
			b, err := fs.ReadFile(path)
			if err != nil || len(b) == 0 {
				continue
			}
			content = b
			found = true
			break
		}
		if !found {
			continue
		}

		rawVer, _, conf := ve.ExtractFromBinary(content, name)
		if rawVer == "" {
			continue
		}
		normVer := sbomversion.NormalizeVersionForPURL(p.PURL, p.Type, rawVer)

		p.Version = normVer
		if strings.TrimSpace(p.PURL) != "" && strings.Contains(p.PURL, "@") {
			p.PURL = sbomversion.UpdatePURLVersion(p.PURL, normVer)
		}

		// Don't change Source; only update confidence. This keeps SBOM-level provenance stable.
		if conf != "" {
			p.Confidence = sbomversion.FormatConfidence(p.Confidence, conf)
		}
	}

	// Normalize already-known versions (e.g., Go "v1.28.0" => "1.28.0") to improve OSV exact matching.
	for i := range pkgs {
		p := &pkgs[i]
		if strings.TrimSpace(p.Version) == "" || strings.TrimSpace(p.Version) == "unknown" {
			continue
		}
		if strings.TrimSpace(p.PURL) == "" || !strings.Contains(p.PURL, "@") {
			continue
		}
		norm := sbomversion.NormalizeVersionForPURL(p.PURL, p.Type, p.Version)
		if norm != "" && norm != p.Version {
			p.Version = norm
			p.PURL = sbomversion.UpdatePURLVersion(p.PURL, norm)
		}
	}

	return pkgs
}

func candidateBinaryPaths(name string) []string {
	// Most distroless/control-plane binaries live under /bin, /usr/bin, /usr/local/bin.
	// Add a couple of extra common locations as best-effort.
	return []string{
		"/bin/" + name,
		"/usr/bin/" + name,
		"/usr/local/bin/" + name,
		"/sbin/" + name,
		"/usr/sbin/" + name,
		"/lib/" + name,
		"/usr/lib/" + name,
	}
}

func (e *Extractor) invokeSyftWithRetry(ctx context.Context, imageRef string) ([]Package, error) {
	if e == nil || e.syftAdapter == nil {
		return nil, nil
	}
	max := e.syftMaxRetries
	if max < 0 {
		max = 0
	}
	var lastErr error
	for attempt := 0; attempt <= max; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pkgs, err := e.syftAdapter.DiscoverPackages(ctx, imageRef)
		if err == nil {
			return pkgs, nil
		}
		lastErr = err
		if !isTransientSyftError(err) || attempt == max {
			break
		}
		// Exponential backoff: 1s, 2s, 4s...
		backoff := time.Duration(1<<attempt) * time.Second
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isTransientSyftError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "timeout"),
		strings.Contains(s, "tls handshake timeout"),
		strings.Contains(s, "connection reset"),
		strings.Contains(s, "connection refused"),
		strings.Contains(s, "no such host"),
		strings.Contains(s, "temporary failure"),
		strings.Contains(s, "i/o timeout"),
		strings.Contains(s, "eof"),
		strings.Contains(s, "502"),
		strings.Contains(s, "503"),
		strings.Contains(s, "504"):
		return true
	default:
		return false
	}
}

// applySignatureHints updates packages using signature DB (versionFromTag, digestMap) to reduce unknown versions.
// If logger is non-nil, logs version resolution for each known binary (control-plane debug).
func applySignatureHints(logger *log.Logger, pkgs []Package, imageTag string, imageDigest string, imageConfig *v1.ConfigFile, sig *signatures.DistrolessJSON) []Package {
	if sig == nil || len(sig.Binaries) == 0 {
		return pkgs
	}
	for i := range pkgs {
		meta, ok := sig.Binaries[pkgs[i].Name]
		if !ok {
			continue
		}
		version := pkgs[i].Version
		source := "none"
		// Digest override wins
		if meta.DigestMap != nil {
			if v, ok := meta.DigestMap[imageDigest]; ok && v != "" {
				version = v
				source = "digestMap"
			}
		}
		// Tag-based version if allowed and digest did not override
		if version == "unknown" && meta.VersionFromTag && imageTag != "" && !strings.HasPrefix(imageTag, "sha256:") {
			version = imageTag
			source = "tag"
		}
		// Label-based version if still unknown
		if version == "unknown" && len(meta.LabelKeys) > 0 && imageConfig != nil {
			for _, k := range meta.LabelKeys {
				if v, ok := imageConfig.Config.Labels[k]; ok && strings.TrimSpace(v) != "" {
					version = strings.TrimSpace(v)
					source = "label:" + k
					break
				}
			}
		}
		// Fallback: org.opencontainers.image.ref.name often contains "repo:tag" (e.g. registry.k8s.io/kube-apiserver:v1.29.15)
		if version == "unknown" && imageConfig != nil && imageConfig.Config.Labels != nil {
			if v, ok := imageConfig.Config.Labels[labelRefName]; ok && strings.TrimSpace(v) != "" {
				if idx := strings.LastIndex(v, ":"); idx >= 0 && idx < len(v)-1 {
					version = strings.TrimSpace(v[idx+1:])
					source = "label:ref.name"
				}
			}
		}
		if logger != nil {
			logger.Printf("   [control-plane version] binary=%s version_before=unknown version_after=%q source=%s (digestMap_empty=%v tag_is_sha=%v)",
				pkgs[i].Name, version, source, len(meta.DigestMap) == 0, strings.HasPrefix(imageTag, "sha256:"))
		}
		if version != "" && version != pkgs[i].Version {
			pkgs[i].Version = version
			pkgs[i].PURL = fmt.Sprintf("pkg:generic/%s@%s", pkgs[i].Name, version)
			// Version from tag/label/digest → raise confidence to medium to enable NVD fallback
			if pkgs[i].Confidence == "low" {
				pkgs[i].Confidence = "medium"
			}
		}
		if meta.PURL != "" && version != "" && strings.Contains(meta.PURL, "@") {
			// Replace suffix after @ with chosen version
			if idx := strings.LastIndex(meta.PURL, "@"); idx != -1 {
				pkgs[i].PURL = meta.PURL[:idx+1] + version
			}
		}
		if meta.Confidence != "" && (version == "" || version == "unknown") {
			pkgs[i].Confidence = meta.Confidence
		}
	}
	return pkgs
}

// setOSPackagePURLs sets PURL pkg:deb/<distro>/name@version (or apk) for dpkg/apk packages
// when PURL is empty, so Core matches OSV debian/ubuntu/alpine data (e.g. openssl 3.0.18-1~deb12u2).
func setOSPackagePURLs(pkgs []Package, osInfo OSInfo) []Package {
	distro := strings.ToLower(strings.TrimSpace(osInfo.Name))
	for i := range pkgs {
		p := &pkgs[i]
		if p.PURL != "" {
			continue
		}
		switch p.Type {
		case "deb":
			d := distro
			if d == "" {
				d = "debian"
			}
			nameForPURL := p.Name
			if sp := strings.TrimSpace(p.SourcePackage); sp != "" {
				nameForPURL = sp
			}
			p.PURL = fmt.Sprintf("pkg:deb/%s/%s@%s", d, nameForPURL, p.Version)
		case "apk":
			d := distro
			if d == "" {
				d = "alpine"
			}
			p.PURL = fmt.Sprintf("pkg:apk/%s/%s@%s", d, p.Name, p.Version)
		}
	}
	return pkgs
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
	// When version came from ref tag (e.g. distroless-hello:e2e → e2e), use medium so NVD fallback is enabled
	if confidence == "low" && version != "unknown" {
		confidence = "medium"
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
	SourcePackage string // for OS packages (e.g., deb Source: openssl for binary libssl3)
	SourceVersion string // parsed from Source field when available
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
	// GoVersion: toolchain used to build Go binaries (buildinfo / GOLANG_VERSION). Sent to Core for stdlib CVE matching.
	GoVersion string
}

// goVersionFromImageConfig reads GOLANG_VERSION / GO_VERSION from OCI config (common on official golang images).
func goVersionFromImageConfig(imageConfig *v1.ConfigFile) string {
	if imageConfig == nil {
		return ""
	}
	for _, env := range imageConfig.Config.Env {
		env = strings.TrimSpace(env)
		if strings.HasPrefix(env, "GOLANG_VERSION=") {
			return strings.TrimSpace(strings.TrimPrefix(env, "GOLANG_VERSION="))
		}
		if strings.HasPrefix(env, "GO_VERSION=") {
			return strings.TrimSpace(strings.TrimPrefix(env, "GO_VERSION="))
		}
	}
	return ""
}

// normalizeGoToolchainVersionForSBOM aligns with Core stdlib matcher (expects go1.x.y or 1.x.y after trim).
func normalizeGoToolchainVersionForSBOM(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "go") {
		return s
	}
	if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
		return "go" + s
	}
	return s
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
