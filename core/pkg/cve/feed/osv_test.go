package feed

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestOSVFetcherSyncsAdvisories(t *testing.T) {
	var mu sync.Mutex
	archives := map[string][]byte{}
	etags := map[string]string{}
	requests := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests[r.URL.Path]++
		data, ok := archives[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("If-None-Match") == etags[r.URL.Path] {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etags[r.URL.Path])
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	set := func(path, etag string, files map[string]string) {
		mu.Lock()
		defer mu.Unlock()
		archives[path] = zipOf(t, files)
		etags[path] = etag
	}
	set("/Debian/all.zip", `"v1"`, map[string]string{
		"DEBIAN-CVE-2025-1.json": `{"id":"DEBIAN-CVE-2025-1"}`,
		"DEBIAN-CVE-2025-2.json": `{"id":"DEBIAN-CVE-2025-2"}`,
		"../escape.json":         `{"id":"escape"}`,
		"dir/nested.json":        `{"id":"nested"}`,
		"MISMATCH.json":          `{"id":"OTHER"}`,
	})
	set("/Rocky Linux/all.zip", `"r1"`, map[string]string{"RLSA-2025:1.json": `{"id":"RLSA-2025:1"}`})

	dir := t.TempDir()
	f := &OSVFetcher{BaseURL: srv.URL, Dir: dir}
	ctx := context.Background()

	res, err := f.Fetch(ctx, []string{"Debian", "Rocky Linux", "Missing"})
	if err != nil {
		t.Fatal(err)
	}
	if !Changed(res) || len(Failed(res)) != 1 || Failed(res)[0] != "Missing" {
		t.Fatalf("results = %+v", res)
	}
	entries, _ := os.ReadDir(f.AdvisoryDir())
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{"DEBIAN-CVE-2025-1.json", "DEBIAN-CVE-2025-2.json", "RLSA-2025:1.json"}
	if len(names) != len(want) {
		t.Fatalf("advisories = %v, want %v", names, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.json")); err == nil {
		t.Fatal("path traversal entry was written")
	}

	// Unchanged ETag: no download body, nothing changes.
	res, _ = f.Fetch(ctx, []string{"Debian"})
	if Changed(res) || !res[0].NotModified {
		t.Fatalf("second fetch = %+v", res)
	}

	// New export: one advisory changed, one removed; the unchanged file keeps its mtime.
	st1, _ := os.Stat(filepath.Join(f.AdvisoryDir(), "DEBIAN-CVE-2025-1.json"))
	set("/Debian/all.zip", `"v2"`, map[string]string{
		"DEBIAN-CVE-2025-1.json": `{"id":"DEBIAN-CVE-2025-1"}`,
		"DEBIAN-CVE-2025-3.json": `{"id":"DEBIAN-CVE-2025-3"}`,
	})
	res, _ = f.Fetch(ctx, []string{"Debian"})
	if res[0].Written != 1 || res[0].Removed != 1 {
		t.Fatalf("third fetch = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(f.AdvisoryDir(), "DEBIAN-CVE-2025-2.json")); !os.IsNotExist(err) {
		t.Fatal("removed advisory still on disk")
	}
	st2, _ := os.Stat(filepath.Join(f.AdvisoryDir(), "DEBIAN-CVE-2025-1.json"))
	if !st1.ModTime().Equal(st2.ModTime()) {
		t.Fatal("unchanged advisory was rewritten")
	}
}

// After the first archive download, runs read modified_id.csv and fetch only what changed.
func TestOSVFetcherIncremental(t *testing.T) {
	var mu sync.Mutex
	files := map[string]string{} // path -> body
	etags := map[string]string{}
	requests := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests[r.URL.Path]++
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if etag := etags[r.URL.Path]; etag != "" {
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", etag)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	adv := func(id, modified, extra string) string {
		return `{"id":"` + id + `","modified":"` + modified + `"` + extra + `}`
	}
	put := func(path, body, etag string) {
		mu.Lock()
		defer mu.Unlock()
		files[path] = body
		etags[path] = etag
	}
	count := func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return requests[path]
	}

	a1 := adv("ALSA-1", "2026-10-01T00:00:00Z", "")
	a2 := adv("ALSA-2", "2026-10-02T00:00:00Z", "")
	mu.Lock()
	files["/AlmaLinux/all.zip"] = string(zipOf(t, map[string]string{"ALSA-1.json": a1, "ALSA-2.json": a2}))
	mu.Unlock()

	dir := t.TempDir()
	f := &OSVFetcher{BaseURL: srv.URL, Dir: dir}
	ctx := context.Background()
	if res, err := f.Fetch(ctx, []string{"AlmaLinux"}); err != nil || res[0].Written != 2 {
		t.Fatalf("first fetch = %+v, %v", res, err)
	}

	// ALSA-2 changed, ALSA-3 is new, ALSA-1 was removed from the export.
	a2b := adv("ALSA-2", "2026-10-05T00:00:00Z", `,"summary":"updated"`)
	a3 := adv("ALSA-3", "2026-10-05T01:00:00Z", "")
	put("/AlmaLinux/ALSA-2.json", a2b, "")
	put("/AlmaLinux/ALSA-3.json", a3, "")
	put("/AlmaLinux/modified_id.csv", "2026-10-05T01:00:00Z,ALSA-3\n2026-10-05T00:00:00Z,ALSA-2\nnot a line\n2026-10-04T00:00:00Z,../escape\n", `"i1"`)
	res, err := f.Fetch(ctx, []string{"AlmaLinux"})
	if err != nil || res[0].Err != nil || res[0].Written != 2 || res[0].Removed != 1 || res[0].Advisories != 2 {
		t.Fatalf("incremental fetch = %+v, %v", res, err)
	}
	if count("/AlmaLinux/all.zip") != 1 {
		t.Fatalf("archive downloaded %d times, want once", count("/AlmaLinux/all.zip"))
	}
	if got, _ := os.ReadFile(filepath.Join(f.AdvisoryDir(), "ALSA-2.json")); string(got) != a2b {
		t.Fatalf("ALSA-2 = %s", got)
	}
	if _, err := os.Stat(filepath.Join(f.AdvisoryDir(), "ALSA-1.json")); !os.IsNotExist(err) {
		t.Fatal("removed advisory still on disk")
	}

	// Unchanged index: nothing is downloaded.
	res, _ = f.Fetch(ctx, []string{"AlmaLinux"})
	if !res[0].NotModified || count("/AlmaLinux/ALSA-3.json") != 1 {
		t.Fatalf("unchanged index fetch = %+v", res)
	}

	// A failed advisory download keeps the cursor, so the next run retries it.
	put("/AlmaLinux/modified_id.csv", "2026-10-06T00:00:00Z,ALSA-4\n2026-10-05T01:00:00Z,ALSA-3\n2026-10-05T00:00:00Z,ALSA-2\n", `"i2"`)
	mu.Lock()
	files["/AlmaLinux/ALSA-4.json"] = "" // invalid JSON: skipped, not an error
	mu.Unlock()
	srvFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/AlmaLinux/ALSA-4.json" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		srv.Config.Handler.ServeHTTP(w, r)
	}))
	defer srvFail.Close()
	f.BaseURL = srvFail.URL
	if res, _ := f.Fetch(ctx, []string{"AlmaLinux"}); res[0].Err == nil {
		t.Fatalf("expected failure, got %+v", res)
	}
	f.BaseURL = srv.URL
	put("/AlmaLinux/ALSA-4.json", adv("ALSA-4", "2026-10-06T00:00:00Z", ""), "")
	if res, _ := f.Fetch(ctx, []string{"AlmaLinux"}); res[0].Err != nil || res[0].Written != 1 {
		t.Fatalf("retry = %+v", res)
	}
}
