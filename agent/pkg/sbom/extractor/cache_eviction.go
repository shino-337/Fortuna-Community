package extractor

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Cache limits (CACHE-1). Set via environment; 0 / empty = disabled for that limit.
//
//	SBOM_CACHE_MAX_AGE — max age of a cache file (e.g. 168h, 720h). Older entries are removed on Get and eligible for eviction.
//	SBOM_CACHE_MAX_TOTAL_BYTES — max total size of all *.json under the cache dir; after Set, oldest files removed first.
//	SBOM_CACHE_MAX_FILES — max number of *.json files; after Set, oldest removed first.
func cacheLimitsFromEnv() (maxAge time.Duration, maxTotalBytes int64, maxFiles int) {
	if s := strings.TrimSpace(os.Getenv("SBOM_CACHE_MAX_AGE")); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			maxAge = d
		}
	}
	if s := strings.TrimSpace(os.Getenv("SBOM_CACHE_MAX_TOTAL_BYTES")); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			maxTotalBytes = v
		}
	}
	if s := strings.TrimSpace(os.Getenv("SBOM_CACHE_MAX_FILES")); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			maxFiles = v
		}
	}
	return maxAge, maxTotalBytes, maxFiles
}

type cacheEntryInfo struct {
	path    string
	size    int64
	modTime time.Time
}

func listCacheJSONFiles(dir string) ([]cacheEntryInfo, int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}
	var out []cacheEntryInfo
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(dir, name)
		fi, err := e.Info()
		if err != nil {
			fi, err = os.Stat(path)
			if err != nil {
				continue
			}
		}
		sz := fi.Size()
		total += sz
		out = append(out, cacheEntryInfo{path: path, size: sz, modTime: fi.ModTime()})
	}
	return out, total, nil
}

// evictCacheDir removes oldest *.json files until within maxTotalBytes and maxFiles (if set).
func evictCacheDir(dir string, log logger, maxTotalBytes int64, maxFiles int) {
	if dir == "" || (maxTotalBytes <= 0 && maxFiles <= 0) {
		return
	}
	files, totalSize, err := listCacheJSONFiles(dir)
	if err != nil {
		if log != nil {
			log.Printf("⚠️  SBOM cache eviction: list dir: %v", err)
		}
		return
	}
	if len(files) == 0 {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	remaining := files
	for len(remaining) > 0 {
		overBytes := maxTotalBytes > 0 && totalSize > maxTotalBytes
		overCount := maxFiles > 0 && len(remaining) > maxFiles
		if !overBytes && !overCount {
			break
		}
		f := remaining[0]
		if err := os.Remove(f.path); err != nil {
			if log != nil {
				log.Printf("⚠️  SBOM cache eviction: remove %s: %v", f.path, err)
			}
			remaining = remaining[1:]
			continue
		}
		totalSize -= f.size
		remaining = remaining[1:]
		if log != nil {
			log.Printf("🗑️  SBOM cache evicted (oldest): %s", filepath.Base(f.path))
		}
	}
}
