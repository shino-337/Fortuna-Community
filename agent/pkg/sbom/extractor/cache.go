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
type DiskCache struct {
	dir    string
	logger logger
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
	return &DiskCache{dir: dir, logger: log}
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
	}
}

func (r rawSBOMJSON) toRawSBOM() *RawSBOM {
	t, _ := time.Parse(time.RFC3339, r.ExtractedAt)
	return &RawSBOM{
		ImageName:   r.ImageName,
		ImageDigest: r.ImageDigest,
		OS:          r.OS,
		Packages:    r.Packages,
		ExtractedAt: t,
		SBOMSource:  r.SBOMSource,
		Confidence:  r.Confidence,
	}
}
