package extractor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiskCache stores and retrieves RawSBOM by image digest + signature version (Finding #8.5 / B2).
// CACHE-1: optional TTL / max disk / max files via SBOM_CACHE_MAX_* (see cache_eviction.go).
type DiskCache struct {
	dir             string
	logger          logger
	maxAge          time.Duration
	maxTotalBytes   int64
	maxFiles        int
}

type logger interface {
	Printf(format string, v ...interface{})
}

// NewDiskCache creates a disk cache at dir (e.g. /var/lib/fortuna/sbom-cache).
// If dir is empty, returns nil (cache disabled).
func NewDiskCache(dir string, log logger) *DiskCache {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	maxAge, maxB, maxF := cacheLimitsFromEnv()
	return &DiskCache{dir: dir, logger: log, maxAge: maxAge, maxTotalBytes: maxB, maxFiles: maxF}
}

// cacheFileName returns a filesystem-safe filename for the cache entry.
func cacheFileName(digest, sigVersion string) string {
	safe := strings.ReplaceAll(digest, ":", "_")
	return fmt.Sprintf("%s.%s.json", safe, sigVersion)
}

// Get loads a cached RawSBOM for the given digest and signature version.
// Returns nil if cache is disabled, file missing, or read/parse error.
func (c *DiskCache) Get(digest, sigVersion string) (*RawSBOM, error) {
	if c == nil {
		return nil, nil
	}
	path := filepath.Join(c.dir, cacheFileName(digest, sigVersion))
	if c.maxAge > 0 {
		if fi, err := os.Stat(path); err == nil {
			if time.Since(fi.ModTime()) > c.maxAge {
				_ = os.Remove(path)
				if c.logger != nil {
					c.logger.Printf("SBOM cache miss (expired max_age): %s", digest)
				}
				return nil, nil
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		if c.logger != nil {
			c.logger.Printf("⚠️  SBOM cache read failed: %v", err)
		}
		return nil, err
	}
	var raw rawSBOMJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		if c.logger != nil {
			c.logger.Printf("⚠️  SBOM cache parse failed: %v", err)
		}
		return nil, err
	}
	sbom := raw.toRawSBOM()
	if c.logger != nil {
		c.logger.Printf("✅ SBOM cache hit for %s (sig=%s)", digest, sigVersion)
	}
	return sbom, nil
}

// Set writes RawSBOM to the cache for the given digest and signature version.
func (c *DiskCache) Set(digest, sigVersion string, sbom *RawSBOM) error {
	if c == nil {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		if c.logger != nil {
			c.logger.Printf("⚠️  SBOM cache mkdir failed: %v", err)
		}
		return err
	}
	path := filepath.Join(c.dir, cacheFileName(digest, sigVersion))
	raw := rawSBOMFrom(sbom)
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		if c.logger != nil {
			c.logger.Printf("⚠️  SBOM cache write failed: %v", err)
		}
		return err
	}
	evictCacheDir(c.dir, c.logger, c.maxTotalBytes, c.maxFiles)
	if c.logger != nil {
		c.logger.Printf("✅ SBOM cached for %s (sig=%s)", digest, sigVersion)
	}
	return nil
}

// rawSBOMJSON is the JSON-serializable shape of RawSBOM (time as RFC3339 string).
type rawSBOMJSON struct {
	ImageName   string       `json:"image_name"`
	ImageDigest string       `json:"image_digest"`
	OS          OSInfo       `json:"os"`
	Packages    []Package    `json:"packages"`
	ExtractedAt string       `json:"extracted_at"`
	SBOMSource  string       `json:"sbom_source"`
	Confidence  string       `json:"confidence"`
	// SignatureVersion is used for cache invalidation boundaries.
	SignatureVersion string `json:"signature_version"`
	// GoVersion: Go toolchain for Core stdlib CVE matching.
	GoVersion string `json:"go_version"`
}

func rawSBOMFrom(s *RawSBOM) rawSBOMJSON {
	return rawSBOMJSON{
		ImageName:   s.ImageName,
		ImageDigest: s.ImageDigest,
		OS:          s.OS,
		Packages:    s.Packages,
		ExtractedAt: s.ExtractedAt.Format(time.RFC3339),
		SBOMSource:  s.SBOMSource,
		Confidence:  s.Confidence,
		SignatureVersion: s.SignatureVersion,
		GoVersion:          s.GoVersion,
	}
}

func (r rawSBOMJSON) toRawSBOM() *RawSBOM {
	t, err := time.Parse(time.RFC3339, r.ExtractedAt)
	if err != nil {
		// Preserve determinism for cached SBOMs: parsing errors should not produce an arbitrary time.
		// Downstream relies on digest/version for identity, not ExtractedAt.
		t = time.Time{}
	}
	return &RawSBOM{
		ImageName:   r.ImageName,
		ImageDigest: r.ImageDigest,
		OS:          r.OS,
		Packages:    r.Packages,
		ExtractedAt: t,
		SBOMSource:  r.SBOMSource,
		Confidence:  r.Confidence,
		SignatureVersion: r.SignatureVersion,
		GoVersion:        r.GoVersion,
	}
}
