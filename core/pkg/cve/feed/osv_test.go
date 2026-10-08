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
	set("/Rocky Linux/all.zip", `"r1"`, map[string]string{"RLSA-2025-1.json": `{"id":"RLSA-2025-1"}`})

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
	want := []string{"DEBIAN-CVE-2025-1.json", "DEBIAN-CVE-2025-2.json", "RLSA-2025-1.json"}
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
