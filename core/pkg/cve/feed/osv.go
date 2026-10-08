// Package feed downloads vulnerability advisories from upstream feeds into a local directory
// that the CVE loader reads.
package feed

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DefaultOSVBaseURL is the public OSV export bucket; each ecosystem has <base>/<Ecosystem>/all.zip.
const DefaultOSVBaseURL = "https://osv-vulnerabilities.storage.googleapis.com"

// DefaultOSVEcosystems are the OSV ecosystems Fortuna SBOMs produce packages for.
var DefaultOSVEcosystems = []string{
	"Debian", "Ubuntu", "Alpine", "Rocky Linux", "AlmaLinux", "Wolfi", "Chainguard",
	"Go", "npm", "PyPI", "Maven", "RubyGems", "crates.io", "NuGet", "Packagist",
}

const (
	defaultMaxZipBytes  = 2 << 30  // 2 GiB per ecosystem archive
	defaultMaxFileBytes = 16 << 20 // 16 MiB per advisory
	stateFileName       = ".osv-feed-state.json"
	// minIDsForShrinkCheck: exports at least this large must not lose more than half their
	// advisories between two runs.
	minIDsForShrinkCheck = 100
	advisoryDirName      = "advisories"
)

var advisoryFileRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,200}\.json$`)

// OSVFetcher mirrors OSV ecosystem exports into Dir/advisories, one <ID>.json per advisory.
// Unchanged advisories are not rewritten, so the loader's mtime/size tracking only reprocesses
// what changed; advisories that disappear from an ecosystem's export are removed.
type OSVFetcher struct {
	BaseURL      string
	Dir          string
	Client       *http.Client
	MaxZipBytes  int64
	MaxFileBytes int64
	Logger       *log.Logger
}

// FetchResult reports what one ecosystem fetch changed.
type FetchResult struct {
	Ecosystem   string
	NotModified bool
	Advisories  int
	Written     int
	Removed     int
	Err         error
}

type ecosystemState struct {
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"lastModified,omitempty"`
	FetchedAt    time.Time `json:"fetchedAt"`
	IDs          []string  `json:"ids"`
}

type feedState struct {
	Ecosystems map[string]*ecosystemState `json:"ecosystems"`
}

// AdvisoryDir is where advisories are written; pass it to the loader as its source directory.
func (f *OSVFetcher) AdvisoryDir() string { return filepath.Join(f.Dir, advisoryDirName) }

// Fetch downloads each ecosystem whose export changed since the last run (by ETag) and syncs
// its advisories. A failed ecosystem is reported in its result and leaves its files untouched;
// the others still update.
func (f *OSVFetcher) Fetch(ctx context.Context, ecosystems []string) ([]FetchResult, error) {
	if strings.TrimSpace(f.Dir) == "" {
		return nil, errors.New("feed: Dir required")
	}
	if err := os.MkdirAll(f.AdvisoryDir(), 0o755); err != nil {
		return nil, fmt.Errorf("feed: create advisory dir: %w", err)
	}
	state, err := f.loadState()
	if err != nil {
		return nil, err
	}
	results := make([]FetchResult, 0, len(ecosystems))
	for _, eco := range ecosystems {
		eco = strings.TrimSpace(eco)
		if eco == "" {
			continue
		}
		res := f.fetchEcosystem(ctx, eco, state)
		if res.Err != nil {
			f.logf("OSV %s: %v", eco, res.Err)
		} else if res.NotModified {
			f.logf("OSV %s: not modified", eco)
		} else {
			f.logf("OSV %s: %d advisories, %d written, %d removed", eco, res.Advisories, res.Written, res.Removed)
		}
		results = append(results, res)
		if err := f.saveState(state); err != nil {
			return results, err
		}
	}
	return results, nil
}

// Changed reports whether any fetch wrote or removed an advisory.
func Changed(results []FetchResult) bool {
	for _, r := range results {
		if r.Written > 0 || r.Removed > 0 {
			return true
		}
	}
	return false
}

// Failed returns the ecosystems whose fetch failed.
func Failed(results []FetchResult) []string {
	var out []string
	for _, r := range results {
		if r.Err != nil {
			out = append(out, r.Ecosystem)
		}
	}
	return out
}

func (f *OSVFetcher) fetchEcosystem(ctx context.Context, eco string, state *feedState) FetchResult {
	res := FetchResult{Ecosystem: eco}
	prev := state.Ecosystems[eco]

	base := strings.TrimRight(f.BaseURL, "/")
	if base == "" {
		base = DefaultOSVBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+url.PathEscape(eco)+"/all.zip", nil)
	if err != nil {
		res.Err = err
		return res
	}
	if prev != nil {
		if prev.ETag != "" {
			req.Header.Set("If-None-Match", prev.ETag)
		}
		if prev.LastModified != "" {
			req.Header.Set("If-Modified-Since", prev.LastModified)
		}
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		res.Err = fmt.Errorf("download: %w", err)
		return res
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified && prev != nil {
		res.NotModified = true
		res.Advisories = len(prev.IDs)
		return res
	}
	if resp.StatusCode != http.StatusOK {
		res.Err = fmt.Errorf("download: HTTP %d", resp.StatusCode)
		return res
	}

	tmp, err := os.CreateTemp(f.Dir, ".osv-*.zip")
	if err != nil {
		res.Err = err
		return res
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	maxZip := f.MaxZipBytes
	if maxZip <= 0 {
		maxZip = defaultMaxZipBytes
	}
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, maxZip+1))
	if err != nil {
		res.Err = fmt.Errorf("download: %w", err)
		return res
	}
	if n > maxZip {
		res.Err = fmt.Errorf("archive larger than %d bytes", maxZip)
		return res
	}

	ids, written, err := f.extract(tmp, n)
	if err != nil {
		res.Err = err
		return res
	}
	res.Advisories = len(ids)
	res.Written = written
	// A truncated or broken export must not delete half the catalog.
	if prev != nil && len(prev.IDs) >= minIDsForShrinkCheck && len(ids) < len(prev.IDs)/2 {
		res.Err = fmt.Errorf("export has %d advisories, previous had %d; keeping the previous ones", len(ids), len(prev.IDs))
		return res
	}

	// Remove advisories this ecosystem no longer exports, unless another ecosystem still does.
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
	}
	if prev != nil {
		for _, id := range prev.IDs {
			if keep[id] || exportedByOther(state, eco, id) {
				continue
			}
			if err := os.Remove(filepath.Join(f.AdvisoryDir(), id+".json")); err == nil {
				res.Removed++
			} else if !errors.Is(err, os.ErrNotExist) {
				res.Err = fmt.Errorf("remove %s: %w", id, err)
				return res
			}
		}
	}
	state.Ecosystems[eco] = &ecosystemState{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedAt:    time.Now().UTC(),
		IDs:          ids,
	}
	return res
}

func exportedByOther(state *feedState, eco, id string) bool {
	for other, st := range state.Ecosystems {
		if other == eco || st == nil {
			continue
		}
		for _, x := range st.IDs {
			if x == id {
				return true
			}
		}
	}
	return false
}

// extract writes each advisory of the archive to AdvisoryDir/<ID>.json. Entries that are not a
// flat <ID>.json, exceed the size limit, or whose "id" does not match the file name are skipped.
func (f *OSVFetcher) extract(r io.ReaderAt, size int64) (ids []string, written int, err error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, 0, fmt.Errorf("open archive: %w", err)
	}
	maxFile := f.MaxFileBytes
	if maxFile <= 0 {
		maxFile = defaultMaxFileBytes
	}
	seen := make(map[string]bool, len(zr.File))
	for _, zf := range zr.File {
		name := zf.Name
		if zf.FileInfo().IsDir() || name != filepath.Base(name) || !advisoryFileRE.MatchString(name) {
			continue
		}
		if zf.UncompressedSize64 > uint64(maxFile) {
			f.logf("OSV: skipping %s (%d bytes)", name, zf.UncompressedSize64)
			continue
		}
		data, err := readZipFile(zf, maxFile)
		if err != nil {
			return nil, 0, fmt.Errorf("read %s: %w", name, err)
		}
		id := strings.TrimSuffix(name, ".json")
		var doc struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data, &doc); err != nil || doc.ID != id {
			f.logf("OSV: skipping %s (id does not match file name)", name)
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		changed, err := writeIfChanged(filepath.Join(f.AdvisoryDir(), name), data)
		if err != nil {
			return nil, 0, err
		}
		if changed {
			written++
		}
	}
	sort.Strings(ids)
	return ids, written, nil
}

func readZipFile(zf *zip.File, max int64) ([]byte, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("larger than %d bytes", max)
	}
	return data, nil
}

// writeIfChanged replaces path atomically when its content differs, keeping the old mtime
// (and so the loader's tracking) for unchanged advisories.
func writeIfChanged(path string, data []byte) (bool, error) {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return false, nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".adv-*")
	if err != nil {
		return false, err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return false, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return false, err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		os.Remove(tmp.Name())
		return false, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return false, err
	}
	return true, nil
}

func (f *OSVFetcher) loadState() (*feedState, error) {
	st := &feedState{Ecosystems: map[string]*ecosystemState{}}
	data, err := os.ReadFile(filepath.Join(f.Dir, stateFileName))
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, fmt.Errorf("feed: read state: %w", err)
	}
	if err := json.Unmarshal(data, st); err != nil {
		// A corrupt state file only costs a full download.
		f.logf("OSV: ignoring unreadable state file: %v", err)
		return &feedState{Ecosystems: map[string]*ecosystemState{}}, nil
	}
	if st.Ecosystems == nil {
		st.Ecosystems = map[string]*ecosystemState{}
	}
	return st, nil
}

func (f *OSVFetcher) saveState(st *feedState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if _, err := writeIfChanged(filepath.Join(f.Dir, stateFileName), data); err != nil {
		return fmt.Errorf("feed: write state: %w", err)
	}
	return nil
}

func (f *OSVFetcher) logf(format string, args ...interface{}) {
	if f.Logger != nil {
		f.Logger.Printf(format, args...)
	}
}
