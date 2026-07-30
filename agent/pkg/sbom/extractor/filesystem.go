package extractor

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

// SBOM_FS_MODE controls RAM vs. completeness for layer extraction (A5 Phase 2).
//   - materialize (default): full content in memory (legacy behavior).
//   - indexed: path index for all regular files; only selective paths keep []byte content.
const (
	envFSMode            = "SBOM_FS_MODE"
	envFSSpoolDir        = "SBOM_FS_SPOOL_DIR"
	fsModeMaterialize    = "materialize"
	fsModeIndexed        = "indexed"
)

// lazyLayerRef points to file payload bytes inside a spooled uncompressed layer stream (A5 Phase 2b).
type lazyLayerRef struct {
	layerIdx int
	offset   int64
	size     int64
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// Filesystem represents a virtual filesystem extracted from image layers
type Filesystem struct {
	files         map[string][]byte
	logger        *log.Logger
	totalBytes    int64
	maxTotalBytes int64
	maxFileBytes  int64
	// skipPathPrefixes: optional memory optimization (A5+); do not store files under these absolute path prefixes.
	skipPathPrefixes []string

	// mode: materialize | indexed (see envFSMode).
	mode string
	// pathSizes: set only in indexed mode — visible paths after overlay (path → tar size).
	// Discovery APIs union pathSizes with files; ReadFile uses files or lazyRefs (spool).
	pathSizes map[string]int64
	// lazyRefs: indexed mode — non-materialized payloads in layer spool files (disk-backed).
	lazyRefs map[string]lazyLayerRef
	// layerSpools: one temp file per layer index holding the full uncompressed tar stream (tee).
	layerSpools []*os.File

	// Phase 2c: lazy ReadFile stats (disk spool → memory).
	lazyReadOps   atomic.Uint64
	lazyReadBytes atomic.Uint64
}

// FSMetrics is a point-in-time snapshot for logging / observability (A5 Phase 2c).
type FSMetrics struct {
	Mode              string
	IndexedPaths      int
	MaterializedFiles int
	MaterializedBytes int64
	LazyReadOps       uint64
	LazyReadBytes     uint64
}

// MetricsSnapshot returns current counter values (safe to call anytime).
func (fs *Filesystem) MetricsSnapshot() FSMetrics {
	if fs == nil {
		return FSMetrics{}
	}
	m := FSMetrics{
		Mode:              fs.Mode(),
		MaterializedFiles: len(fs.files),
		MaterializedBytes: fs.totalBytes,
		LazyReadOps:       fs.lazyReadOps.Load(),
		LazyReadBytes:     fs.lazyReadBytes.Load(),
	}
	if fs.pathSizes != nil {
		m.IndexedPaths = len(fs.pathSizes)
	}
	return m
}

// formatBytesIEC renders n as KiB/MiB/GiB (base 1024) for log lines.
func formatBytesIEC(n int64) string {
	if n < 0 {
		n = 0
	}
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	x := float64(n)
	switch {
	case n < 1024*1024:
		return fmt.Sprintf("%.2f KiB", x/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.2f MiB", x/(1024*1024))
	default:
		return fmt.Sprintf("%.2f GiB", x/(1024*1024*1024))
	}
}

// NewFilesystem creates a new filesystem
func NewFilesystem() *Filesystem {
	maxTotal := int64(1 << 30) // 1GiB default safety cap
	maxFile := int64(64 << 20) // 64MiB per-file default safety cap

	if s := strings.TrimSpace(os.Getenv("SBOM_FS_MAX_TOTAL_BYTES")); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			maxTotal = v
		}
	}
	if s := strings.TrimSpace(os.Getenv("SBOM_FS_MAX_FILE_BYTES")); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			maxFile = v
		}
	}

	mode := fsModeMaterialize
	if s := strings.TrimSpace(os.Getenv(envFSMode)); s != "" && !strings.EqualFold(s, fsModeMaterialize) {
		if strings.EqualFold(s, fsModeIndexed) {
			mode = fsModeIndexed
		}
	}

	fs := &Filesystem{
		files:         make(map[string][]byte),
		logger:        log.New(log.Writer(), "[Filesystem] ", log.LstdFlags),
		maxTotalBytes: maxTotal,
		maxFileBytes:  maxFile,
		mode:          mode,
	}
	if mode == fsModeIndexed {
		fs.pathSizes = make(map[string]int64)
		fs.lazyRefs = make(map[string]lazyLayerRef)
	}
	if p := strings.TrimSpace(os.Getenv("SBOM_FS_SKIP_PATH_PREFIXES")); p != "" && !strings.EqualFold(p, "off") {
		for _, part := range strings.Split(p, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if !strings.HasPrefix(part, "/") {
				part = "/" + part
			}
			fs.skipPathPrefixes = append(fs.skipPathPrefixes, filepath.Clean(part))
		}
	}
	return fs
}

func (fs *Filesystem) spoolDir() string {
	d := strings.TrimSpace(os.Getenv(envFSSpoolDir))
	if d == "" {
		return os.TempDir()
	}
	return d
}

func (fs *Filesystem) setLayerSpool(layerIdx int, f *os.File) error {
	for len(fs.layerSpools) <= layerIdx {
		fs.layerSpools = append(fs.layerSpools, nil)
	}
	if old := fs.layerSpools[layerIdx]; old != nil {
		_ = old.Close()
		_ = os.Remove(old.Name())
	}
	fs.layerSpools[layerIdx] = f
	return nil
}

// Close removes spooled layer temp files (indexed mode). Safe to call multiple times.
func (fs *Filesystem) Close() error {
	if fs == nil || len(fs.layerSpools) == 0 {
		return nil
	}
	var errs []error
	for _, f := range fs.layerSpools {
		if f == nil {
			continue
		}
		name := f.Name()
		if err := f.Close(); err != nil {
			errs = append(errs, err)
		}
		_ = os.Remove(name)
	}
	fs.layerSpools = nil
	return errors.Join(errs...)
}

// Mode returns the resolved filesystem mode (materialize or indexed).
func (fs *Filesystem) Mode() string {
	if fs == nil || fs.mode == "" {
		return fsModeMaterialize
	}
	return fs.mode
}

// IndexedPathCount returns the number of overlay-visible paths in indexed mode (0 in materialize mode).
func (fs *Filesystem) IndexedPathCount() int {
	if fs == nil || fs.pathSizes == nil {
		return 0
	}
	return len(fs.pathSizes)
}

// shouldMaterializeIndexed returns true if path should keep []byte content in indexed mode.
func shouldMaterializeIndexed(path string) bool {
	// OS / package managers
	if strings.HasSuffix(path, "/lib/apk/db/installed") {
		return true
	}
	if strings.HasSuffix(path, "/var/lib/dpkg/status") {
		return true
	}
	// Distroless: individual package status files under status.d/ (Trivy parity).
	if strings.Contains(path, "/var/lib/dpkg/status.d/") && !strings.HasSuffix(path, ".md5sums") {
		return true
	}
	if strings.HasSuffix(path, "rpm-packages.list") {
		return true
	}
	rpmdbExact := []string{
		"/var/lib/rpm/rpmdb.sqlite",
		"/usr/lib/sysimage/rpm/rpmdb.sqlite",
		"/usr/lib/sysimage/rpm/rpmdb.sqlite3",
		"/usr/lib/sysimage/rpm/Packages.db",
		"/var/lib/rpm/Packages",
		"/usr/lib/sysimage/rpm/Packages",
		"/var/lib/rpmmanifest/container-manifest-2",
		"/var/lib/rpmmanifest/container-manifest-1",
	}
	for _, p := range rpmdbExact {
		if path == p {
			return true
		}
	}
	if path == "/etc/os-release" || path == "/etc/debian_version" || path == "/etc/alpine-release" {
		return true
	}

	// Language / ecosystem lockfiles & manifests
	suffixes := []string{
		"requirements.txt",
		"go.sum",
		"go.mod",
		"package-lock.json",
		"package.json",
		"pom.xml",
		"Cargo.lock",
		"Gemfile.lock",
		"packages.lock.json",
		"project.assets.json",
	}
	for _, suf := range suffixes {
		if strings.HasSuffix(path, suf) {
			return true
		}
	}

	// pip: only typical METADATA locations
	if strings.HasSuffix(path, "METADATA") &&
		(strings.Contains(path, ".dist-info/") || strings.Contains(path, ".egg-info/")) {
		return true
	}

	return false
}

func (fs *Filesystem) deleteFromWhiteoutMaps(p string) {
	delete(fs.files, p)
	if fs.pathSizes != nil {
		delete(fs.pathSizes, p)
	}
	if fs.lazyRefs != nil {
		delete(fs.lazyRefs, p)
	}
}

func (fs *Filesystem) deleteOpaqueUnderDir(dir string) {
	prefix := dir
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	for p := range fs.files {
		if strings.HasPrefix(p, prefix) || p == dir {
			delete(fs.files, p)
		}
	}
	if fs.pathSizes != nil {
		for p := range fs.pathSizes {
			if strings.HasPrefix(p, prefix) || p == dir {
				delete(fs.pathSizes, p)
			}
		}
	}
	if fs.lazyRefs != nil {
		for p := range fs.lazyRefs {
			if strings.HasPrefix(p, prefix) || p == dir {
				delete(fs.lazyRefs, p)
			}
		}
	}
}

// ExtractTar extracts a tar archive into the filesystem (OCI overlay semantics).
// Handles whiteout (.wh.filename, .wh..wh..opq) so layer N can remove files from layer N-1.
// layerIdx is the layer ordinal (0 = base); required for indexed-mode spooling and lazy offsets.
func (fs *Filesystem) ExtractTar(ctx context.Context, layerIdx int, r io.Reader) error {
	var tr *tar.Reader
	var cr *countingReader

	if fs.pathSizes != nil {
		layerFile, err := os.CreateTemp(fs.spoolDir(), fmt.Sprintf("fortuna-layer-%03d-*.layer", layerIdx))
		if err != nil {
			return err
		}
		fs.setLayerSpool(layerIdx, layerFile)
		cr = &countingReader{r: r}
		tr = tar.NewReader(io.TeeReader(cr, layerFile))
	} else {
		tr = tar.NewReader(r)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Normalize path once (used for whiteout and for storing)
		path := filepath.Clean(header.Name)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		dir := filepath.Dir(path)
		base := filepath.Base(path)

		// OCI whiteout: .wh.<name> means remove <name> from lower layers
		if strings.HasPrefix(base, ".wh.") {
			if base == ".wh..wh..opq" {
				// Opaque dir: remove all files under dir from lower layers
				fs.deleteOpaqueUnderDir(dir)
			} else {
				// Remove single file/dir: target = dir + name without .wh.
				target := filepath.Join(dir, strings.TrimPrefix(base, ".wh."))
				if !strings.HasPrefix(target, "/") {
					target = "/" + target
				}
				fs.deleteFromWhiteoutMaps(target)
			}
			// Consume body so next header is valid
			if header.Size > 0 {
				_, _ = io.CopyN(io.Discard, tr, header.Size)
			}
			continue
		}

		// Only store regular files (symlinks/dirs skipped for SBOM file-level scan)
		if header.Typeflag != tar.TypeReg {
			if header.Size > 0 {
				_, _ = io.CopyN(io.Discard, tr, header.Size)
			}
			continue
		}

		// A5 guard: skip oversized file CONTENT to reduce OOM risk on large images.
		// Still register the path so discovery APIs (PathsUnder, Glob) can find it.
		// This allows path-only parsers (distroless) to work even for large binaries.
		if fs.maxFileBytes > 0 && header.Size > fs.maxFileBytes {
			if fs.pathSizes != nil {
				var contentOff int64
				if cr != nil {
					contentOff = cr.n
				}
				fs.pathSizes[path] = header.Size
				fs.lazyRefs[path] = lazyLayerRef{layerIdx: layerIdx, offset: contentOff, size: header.Size}
			} else {
				// Materialize mode: register path with nil content for discovery.
				if _, exists := fs.files[path]; !exists {
					fs.files[path] = nil
				}
			}
			if header.Size > 0 {
				_, _ = io.CopyN(io.Discard, tr, header.Size)
			}
			continue
		}

		if fs.shouldSkipPathForA5(path) {
			if header.Size > 0 {
				_, _ = io.CopyN(io.Discard, tr, header.Size)
			}
			continue
		}

		if fs.pathSizes != nil {
			var contentOff int64
			if cr != nil {
				contentOff = cr.n
			}
			fs.pathSizes[path] = header.Size

			wantMat := shouldMaterializeIndexed(path)
			if wantMat && (fs.maxTotalBytes == 0 || fs.totalBytes+header.Size <= fs.maxTotalBytes) {
				content := make([]byte, header.Size)
				if _, err := io.ReadFull(tr, content); err != nil {
					continue
				}
				fs.files[path] = content
				fs.totalBytes += header.Size
				delete(fs.lazyRefs, path)
				continue
			}
			// Not materialized (or over total cap): payload remains in layer spool for ReadFile.
			fs.lazyRefs[path] = lazyLayerRef{layerIdx: layerIdx, offset: contentOff, size: header.Size}
			if header.Size > 0 {
				_, _ = io.CopyN(io.Discard, tr, header.Size)
			}
			continue
		}

		// Materialize mode (legacy): full content, subject to total-bytes cap before read.
		if fs.maxTotalBytes > 0 && fs.totalBytes+header.Size > fs.maxTotalBytes {
			if header.Size > 0 {
				_, _ = io.CopyN(io.Discard, tr, header.Size)
			}
			continue
		}

		content := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, content); err != nil {
			continue
		}

		fs.files[path] = content
		fs.totalBytes += header.Size
	}

	return nil
}

func (fs *Filesystem) shouldSkipPathForA5(path string) bool {
	if len(fs.skipPathPrefixes) == 0 {
		return false
	}
	for _, pre := range fs.skipPathPrefixes {
		if pre != "" && strings.HasPrefix(path, pre) {
			return true
		}
	}
	return false
}

// ReadFile reads a file from the filesystem
func (fs *Filesystem) ReadFile(path string) ([]byte, error) {
	// Normalize path
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if content, ok := fs.files[path]; ok {
		if content == nil {
			return nil, fmt.Errorf("file too large (content not materialized): %s", path)
		}
		return content, nil
	}

	if ref, ok := fs.lazyRefs[path]; ok {
		if fs.maxFileBytes > 0 && ref.size > fs.maxFileBytes {
			return nil, fmt.Errorf("file too large: %s (%d bytes)", path, ref.size)
		}
		if ref.layerIdx < 0 || ref.layerIdx >= len(fs.layerSpools) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		f := fs.layerSpools[ref.layerIdx]
		if f == nil {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		buf := make([]byte, ref.size)
		if _, err := f.ReadAt(buf, ref.offset); err != nil {
			return nil, err
		}
		fs.lazyReadOps.Add(1)
		fs.lazyReadBytes.Add(uint64(ref.size))
		return buf, nil
	}

	return nil, fmt.Errorf("file not found: %s", path)
}

// eachDiscoveryPath invokes fn for every overlay-visible file path (indexed: pathSizes; else files).
func (fs *Filesystem) eachDiscoveryPath(fn func(path string)) {
	if fs.pathSizes != nil {
		for p := range fs.pathSizes {
			fn(p)
		}
		return
	}
	for p := range fs.files {
		fn(p)
	}
}

// Glob finds files matching a pattern (single * per component; no **).
func (fs *Filesystem) Glob(pattern string) []string {
	matches := make([]string, 0)
	fs.eachDiscoveryPath(func(path string) {
		matched, err := filepath.Match(pattern, path)
		if err != nil {
			return
		}
		if matched {
			matches = append(matches, path)
		}
	})
	return matches
}

// FindPathsBySuffix returns all stored paths ending with suffix (e.g. "package-lock.json").
// Used by npm parser to discover lock files anywhere in the image.
func (fs *Filesystem) FindPathsBySuffix(suffix string) []string {
	out := make([]string, 0)
	fs.eachDiscoveryPath(func(path string) {
		if strings.HasSuffix(path, suffix) {
			out = append(out, path)
		}
	})
	return out
}

// FindPathsContaining returns all stored paths that contain sub and end with end (e.g. "node_modules", "package.json").
// Used by npm parser to discover node_modules/*/package.json anywhere in the image.
func (fs *Filesystem) FindPathsContaining(sub, end string) []string {
	out := make([]string, 0)
	fs.eachDiscoveryPath(func(path string) {
		if strings.Contains(path, sub) && strings.HasSuffix(path, end) {
			out = append(out, path)
		}
	})
	return out
}

// FileExists checks if a file exists
func (fs *Filesystem) FileExists(path string) bool {
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if fs.pathSizes != nil {
		_, ok := fs.pathSizes[path]
		return ok
	}
	_, exists := fs.files[path]
	return exists
}

// PathsUnder returns all stored paths that have the given prefix (e.g. "/bin/").
// Used by distroless parser to discover binaries under /bin, /usr/bin, /usr/lib.
func (fs *Filesystem) PathsUnder(prefix string) []string {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	out := make([]string, 0)
	fs.eachDiscoveryPath(func(path string) {
		if strings.HasPrefix(path, prefix) {
			out = append(out, path)
		}
	})
	return out
}
