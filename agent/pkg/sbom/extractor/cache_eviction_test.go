package extractor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEvictCacheDir_OldestFirst(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	paths := []struct {
		name string
		tm   time.Time
		sz   int
	}{
		{"a.json", base, 100},
		{"b.json", base.AddDate(0, 0, 1), 100},
		{"c.json", base.AddDate(0, 0, 2), 100},
	}
	for _, p := range paths {
		fp := filepath.Join(dir, p.name)
		buf := make([]byte, p.sz)
		if err := os.WriteFile(fp, buf, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(fp, p.tm, p.tm); err != nil {
			t.Fatal(err)
		}
	}

	// Max 2 files -> evict oldest (a.json)
	evictCacheDir(dir, nil, 0, 2)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 files, got %d", len(entries))
	}
	if _, err := os.Stat(filepath.Join(dir, "a.json")); err == nil {
		t.Fatal("expected a.json (oldest) evicted")
	}
}

func TestEvictCacheDir_MaxBytes(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, name := range []string{"old.json", "new.json"} {
		fp := filepath.Join(dir, name)
		_ = os.WriteFile(fp, []byte("12345"), 0644)
		tm := base
		if i == 1 {
			tm = base.AddDate(0, 0, 10)
		}
		_ = os.Chtimes(fp, tm, tm)
	}
	// total ~10 bytes, max 8 -> evict oldest
	evictCacheDir(dir, nil, 8, 0)
	if _, err := os.Stat(filepath.Join(dir, "old.json")); err == nil {
		t.Fatal("expected old.json evicted")
	}
}

func TestCacheLimitsFromEnv(t *testing.T) {
	t.Setenv("SBOM_CACHE_MAX_AGE", "48h")
	t.Setenv("SBOM_CACHE_MAX_TOTAL_BYTES", "1048576")
	t.Setenv("SBOM_CACHE_MAX_FILES", "100")
	a, b, n := cacheLimitsFromEnv()
	if a != 48*time.Hour || b != 1048576 || n != 100 {
		t.Fatalf("got age=%v bytes=%d files=%d", a, b, n)
	}
}
