// Package feed downloads vulnerability advisories from upstream feeds into a local directory
// that the CVE loader reads.
package feed

import (
	"archive/zip"
	"bufio"
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
	"sync"
	"time"
)

// DefaultOSVBaseURL is the public OSV export bucket; each ecosystem has <base>/<Ecosystem>/all.zip,
// <base>/<Ecosystem>/modified_id.csv ("<modified>,<ID>" per advisory) and <base>/<Ecosystem>/<ID>.json.
const DefaultOSVBaseURL = "https://osv-vulnerabilities.storage.googleapis.com"

// DefaultOSVEcosystems are the OSV ecosystems Fortuna SBOMs produce packages for.
var DefaultOSVEcosystems = []string{
	"Debian", "Ubuntu", "Alpine", "Rocky Linux", "AlmaLinux", "Red Hat", "Wolfi", "Chainguard",
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
	maxIndexBytes        = 256 << 20 // modified_id.csv; Chainguard's is ~55 MiB
	// maxIncrementalFetches: past this many changed advisories one archive download is cheaper.
	maxIncrementalFetches = 5000
	incrementalWorkers    = 8
	// cursorOverlap re-reads advisories modified shortly before the last cursor, in case the
	// export published them late; unchanged files are not rewritten.
	cursorOverlap = time.Hour
)

// advisoryFileRE accepts flat OSV advisory file names; Red Hat, AlmaLinux and Rocky IDs contain
// a colon (RHSA-2026:1234).
var advisoryFileRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,200}\.json$`)

// OSVFetcher mirrors OSV ecosystem exports into Dir/advisories, one <ID>.json per advisory.
// The first run downloads each ecosystem's all.zip; later runs read its modified_id.csv and
// download only the advisories modified since, falling back to the archive when too many
// changed. Unchanged advisories are not rewritten, so the loader's mtime/size tracking only
// reprocesses what changed; advisories that disappear from an ecosystem's export are removed.
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
	// Cursor is the newest advisory "modified" time seen; IndexETag the last modified_id.csv ETag.
	Cursor    time.Time `json:"cursor,omitempty"`
	IndexETag string    `json:"indexETag,omitempty"`
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

func (f *OSVFetcher) baseURL() string {
	if base := strings.TrimRight(f.BaseURL, "/"); base != "" {
		return base
	}
	return DefaultOSVBaseURL
}

func (f *OSVFetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return &http.Client{Timeout: 30 * time.Minute}
}

func (f *OSVFetcher) maxFileBytes() int64 {
	if f.MaxFileBytes > 0 {
		return f.MaxFileBytes
	}
	return defaultMaxFileBytes
}

func (f *OSVFetcher) fetchEcosystem(ctx context.Context, eco string, state *feedState) FetchResult {
	prev := state.Ecosystems[eco]
	if prev != nil && !prev.Cursor.IsZero() {
		if res, ok := f.fetchIncremental(ctx, eco, prev, state); ok {
			return res
		}
	}
	return f.fetchArchive(ctx, eco, prev, state)
}

// fetchArchive downloads <eco>/all.zip and syncs every advisory in it.
func (f *OSVFetcher) fetchArchive(ctx context.Context, eco string, prev *ecosystemState, state *feedState) FetchResult {
	res := FetchResult{Ecosystem: eco}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL()+"/"+url.PathEscape(eco)+"/all.zip", nil)
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
	resp, err := f.client().Do(req)
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

	ids, written, cursor, err := f.extract(tmp, n)
	if err != nil {
		res.Err = err
		return res
	}
	res.Advisories = len(ids)
	res.Written = written
	if err := checkShrink(prev, len(ids)); err != nil {
		res.Err = err
		return res
	}
	if res.Removed, err = f.removeUnexported(state, eco, prev, ids); err != nil {
		res.Err = err
		return res
	}
	state.Ecosystems[eco] = &ecosystemState{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedAt:    time.Now().UTC(),
		IDs:          ids,
		Cursor:       cursor,
	}
	return res
}

// fetchIncremental reads <eco>/modified_id.csv and downloads the advisories modified since the
// last cursor. ok=false asks for an archive download instead (too many changes, or no usable
// index); a failure is returned with ok=true so the previous files are kept.
func (f *OSVFetcher) fetchIncremental(ctx context.Context, eco string, prev *ecosystemState, state *feedState) (FetchResult, bool) {
	res := FetchResult{Ecosystem: eco}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL()+"/"+url.PathEscape(eco)+"/modified_id.csv", nil)
	if err != nil {
		res.Err = err
		return res, true
	}
	if prev.IndexETag != "" {
		req.Header.Set("If-None-Match", prev.IndexETag)
	}
	resp, err := f.client().Do(req)
	if err != nil {
		res.Err = fmt.Errorf("index: %w", err)
		return res, true
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		res.NotModified = true
		res.Advisories = len(prev.IDs)
		return res, true
	}
	if resp.StatusCode != http.StatusOK {
		f.logf("OSV %s: index HTTP %d, downloading the archive", eco, resp.StatusCode)
		return res, false
	}
	entries, err := parseModifiedIndex(io.LimitReader(resp.Body, maxIndexBytes+1))
	if err != nil {
		res.Err = fmt.Errorf("index: %w", err)
		return res, true
	}

	ids := make([]string, 0, len(entries))
	var changed []string
	cursor := prev.Cursor
	since := prev.Cursor.Add(-cursorOverlap)
	for _, e := range entries {
		ids = append(ids, e.id)
		if e.modified.After(since) {
			changed = append(changed, e.id)
		}
		if e.modified.After(cursor) {
			cursor = e.modified
		}
	}
	sort.Strings(ids)
	if err := checkShrink(prev, len(ids)); err != nil {
		res.Err = err
		return res, true
	}
	if len(changed) > maxIncrementalFetches {
		f.logf("OSV %s: %d advisories changed, downloading the archive", eco, len(changed))
		return res, false
	}
	written, err := f.fetchAdvisories(ctx, eco, changed)
	if err != nil {
		res.Err = err
		return res, true
	}
	res.Advisories = len(ids)
	res.Written = written
	if res.Removed, err = f.removeUnexported(state, eco, prev, ids); err != nil {
		res.Err = err
		return res, true
	}
	next := *prev
	next.FetchedAt = time.Now().UTC()
	next.IDs = ids
	next.Cursor = cursor
	next.IndexETag = resp.Header.Get("ETag")
	state.Ecosystems[eco] = &next
	return res, true
}

type indexEntry struct {
	modified time.Time
	id       string
}

// parseModifiedIndex reads "<RFC 3339 modified>,<ID>" lines; lines with an unusable ID are skipped.
func parseModifiedIndex(r io.Reader) ([]indexEntry, error) {
	var out []indexEntry
	var read int64
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 64<<10)
	for sc.Scan() {
		line := sc.Text()
		read += int64(len(line)) + 1
		if read > maxIndexBytes {
			return nil, fmt.Errorf("larger than %d bytes", maxIndexBytes)
		}
		ts, id, ok := strings.Cut(strings.TrimSpace(line), ",")
		if !ok || !advisoryFileRE.MatchString(id+".json") {
			continue
		}
		modified, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			continue
		}
		out = append(out, indexEntry{modified: modified, id: id})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// fetchAdvisories downloads <eco>/<ID>.json for each id. An advisory that is gone (404) is
// skipped; any other failure fails the ecosystem so its cursor does not advance.
func (f *OSVFetcher) fetchAdvisories(ctx context.Context, eco string, ids []string) (int, error) {
	var (
		mu       sync.Mutex
		written  int
		firstErr error
		wg       sync.WaitGroup
	)
	work := make(chan string)
	for i := 0; i < incrementalWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				changed, err := f.fetchAdvisory(ctx, eco, id)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", id, err)
				}
				if changed {
					written++
				}
				mu.Unlock()
			}
		}()
	}
	for _, id := range ids {
		mu.Lock()
		failed := firstErr != nil
		mu.Unlock()
		if failed || ctx.Err() != nil {
			break
		}
		work <- id
	}
	close(work)
	wg.Wait()
	if firstErr == nil {
		firstErr = ctx.Err()
	}
	return written, firstErr
}

func (f *OSVFetcher) fetchAdvisory(ctx context.Context, eco, id string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL()+"/"+url.PathEscape(eco)+"/"+url.PathEscape(id)+".json", nil)
	if err != nil {
		return false, err
	}
	resp, err := f.client().Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		f.logf("OSV %s: %s listed but not found, skipping", eco, id)
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	max := f.maxFileBytes()
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return false, err
	}
	if int64(len(data)) > max {
		f.logf("OSV %s: skipping %s (larger than %d bytes)", eco, id, max)
		return false, nil
	}
	if docID, _, err := advisoryHeader(data); err != nil || docID != id {
		f.logf("OSV %s: skipping %s (id does not match)", eco, id)
		return false, nil
	}
	return writeIfChanged(filepath.Join(f.AdvisoryDir(), id+".json"), data)
}

// advisoryHeader returns an advisory's id and modified time (zero when missing or malformed).
func advisoryHeader(data []byte) (string, time.Time, error) {
	var doc struct {
		ID       string `json:"id"`
		Modified string `json:"modified"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", time.Time{}, err
	}
	modified, _ := time.Parse(time.RFC3339Nano, doc.Modified)
	return doc.ID, modified, nil
}

// checkShrink rejects an export that lost more than half its advisories: a truncated or broken
// export must not delete half the catalog.
func checkShrink(prev *ecosystemState, n int) error {
	if prev != nil && len(prev.IDs) >= minIDsForShrinkCheck && n < len(prev.IDs)/2 {
		return fmt.Errorf("export has %d advisories, previous had %d; keeping the previous ones", n, len(prev.IDs))
	}
	return nil
}

// removeUnexported deletes advisories this ecosystem no longer exports, unless another
// ecosystem still does.
func (f *OSVFetcher) removeUnexported(state *feedState, eco string, prev *ecosystemState, ids []string) (int, error) {
	if prev == nil {
		return 0, nil
	}
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
	}
	removed := 0
	var others map[string]bool
	for _, id := range prev.IDs {
		if keep[id] {
			continue
		}
		if others == nil {
			others = exportedByOthers(state, eco)
		}
		if others[id] {
			continue
		}
		if err := os.Remove(filepath.Join(f.AdvisoryDir(), id+".json")); err == nil {
			removed++
		} else if !errors.Is(err, os.ErrNotExist) {
			return removed, fmt.Errorf("remove %s: %w", id, err)
		}
	}
	return removed, nil
}

func exportedByOthers(state *feedState, eco string) map[string]bool {
	out := map[string]bool{}
	for other, st := range state.Ecosystems {
		if other == eco || st == nil {
			continue
		}
		for _, id := range st.IDs {
			out[id] = true
		}
	}
	return out
}

// extract writes each advisory of the archive to AdvisoryDir/<ID>.json. Entries that are not a
// flat <ID>.json, exceed the size limit, or whose "id" does not match the file name are skipped.
// It also returns the newest "modified" time, the cursor for the next incremental fetch.
func (f *OSVFetcher) extract(r io.ReaderAt, size int64) (ids []string, written int, cursor time.Time, err error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, 0, cursor, fmt.Errorf("open archive: %w", err)
	}
	maxFile := f.maxFileBytes()
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
			return nil, 0, cursor, fmt.Errorf("read %s: %w", name, err)
		}
		id := strings.TrimSuffix(name, ".json")
		docID, modified, err := advisoryHeader(data)
		if err != nil || docID != id {
			f.logf("OSV: skipping %s (id does not match file name)", name)
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		if modified.After(cursor) {
			cursor = modified
		}
		changed, err := writeIfChanged(filepath.Join(f.AdvisoryDir(), name), data)
		if err != nil {
			return nil, 0, cursor, err
		}
		if changed {
			written++
		}
	}
	sort.Strings(ids)
	return ids, written, cursor, nil
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
