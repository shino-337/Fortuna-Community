// Package enrich loads per-CVE exploitation data into the vulnerabilities table: the CISA
// Known Exploited Vulnerabilities catalog and FIRST EPSS scores. Both are downloaded whole
// (about 2 MB and 2 MB gzipped) by the vulnerability database update job, so the matcher reads
// them from the database instead of calling out per finding.
package enrich

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	// DefaultKEVURL is the CISA KEV catalog.
	DefaultKEVURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"
	// DefaultEPSSURL is the daily EPSS score file for every CVE.
	DefaultEPSSURL = "https://epss.empiricalsecurity.com/epss_scores-current.csv.gz"

	feedKEV  = "kev"
	feedEPSS = "epss"

	// maxFeedBytes bounds a download; both feeds are a few MB.
	maxFeedBytes = 256 << 20
)

// KEVEntry is one CVE of the KEV catalog.
type KEVEntry struct {
	CVEID      string
	DateAdded  time.Time
	DueDate    *time.Time
	Ransomware bool
}

// EPSSScore is one CVE's EPSS probability and percentile on ScoreDate.
type EPSSScore struct {
	CVEID      string
	Score      float64
	Percentile float64
}

// ParseKEV reads the CISA KEV JSON catalog.
func ParseKEV(r io.Reader) ([]KEVEntry, error) {
	var doc struct {
		Vulnerabilities []struct {
			CVEID                      string `json:"cveID"`
			DateAdded                  string `json:"dateAdded"`
			DueDate                    string `json:"dueDate"`
			KnownRansomwareCampaignUse string `json:"knownRansomwareCampaignUse"`
		} `json:"vulnerabilities"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse KEV catalog: %w", err)
	}
	out := make([]KEVEntry, 0, len(doc.Vulnerabilities))
	seen := map[string]bool{}
	for _, v := range doc.Vulnerabilities {
		id := strings.ToUpper(strings.TrimSpace(v.CVEID))
		added, err := time.Parse("2006-01-02", strings.TrimSpace(v.DateAdded))
		if !strings.HasPrefix(id, "CVE-") || err != nil || seen[id] {
			continue
		}
		seen[id] = true
		e := KEVEntry{CVEID: id, DateAdded: added, Ransomware: strings.EqualFold(strings.TrimSpace(v.KnownRansomwareCampaignUse), "known")}
		if due, err := time.Parse("2006-01-02", strings.TrimSpace(v.DueDate)); err == nil {
			e.DueDate = &due
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, errors.New("KEV catalog lists no CVE")
	}
	return out, nil
}

// ParseEPSS reads the EPSS score file (gzipped or plain CSV): an optional
// "#model_version:…,score_date:2026-10-08T00:00:00Z" line, a "cve,epss,percentile" header and
// one row per CVE. It returns the scores and their date.
func ParseEPSS(r io.Reader) ([]EPSSScore, time.Time, error) {
	br := bufio.NewReader(r)
	if magic, err := br.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("open EPSS gzip: %w", err)
		}
		defer gz.Close()
		br = bufio.NewReader(gz)
	}
	var scoreDate time.Time
	if first, err := br.Peek(1); err == nil && first[0] == '#' {
		line, _ := br.ReadString('\n')
		for _, kv := range strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "#")), ",") {
			if k, v, ok := strings.Cut(kv, ":"); ok && strings.TrimSpace(k) == "score_date" {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(v)); err == nil {
					scoreDate = t.UTC().Truncate(24 * time.Hour)
				}
			}
		}
	}
	cr := csv.NewReader(br)
	cr.ReuseRecord = true
	header, err := cr.Read()
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("read EPSS header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	ci, okC := col["cve"]
	ei, okE := col["epss"]
	pi, okP := col["percentile"]
	if !okC || !okE || !okP {
		return nil, time.Time{}, fmt.Errorf("EPSS header %q lacks cve, epss or percentile", strings.Join(header, ","))
	}
	var out []EPSSScore
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("read EPSS row: %w", err)
		}
		id := strings.ToUpper(strings.TrimSpace(rec[ci]))
		score, err1 := strconv.ParseFloat(strings.TrimSpace(rec[ei]), 64)
		pct, err2 := strconv.ParseFloat(strings.TrimSpace(rec[pi]), 64)
		if !strings.HasPrefix(id, "CVE-") || err1 != nil || err2 != nil || score < 0 || score > 1 || pct < 0 || pct > 1 {
			continue
		}
		out = append(out, EPSSScore{CVEID: id, Score: score, Percentile: pct})
	}
	if len(out) == 0 {
		return nil, time.Time{}, errors.New("EPSS file has no scores")
	}
	if scoreDate.IsZero() {
		scoreDate = time.Now().UTC().Truncate(24 * time.Hour)
	}
	return out, scoreDate, nil
}

// StoreKEV makes the KEV columns of vulnerabilities match the catalog: listed CVEs get their
// dates and ransomware flag, CVEs no longer listed lose them. It returns how many are listed.
func StoreKEV(ctx context.Context, db *gorm.DB, entries []KEVEntry) (int, error) {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMP TABLE fortuna_kev (vuln_id VARCHAR(255) PRIMARY KEY, added DATE NOT NULL, due DATE, ransomware BOOLEAN NOT NULL) ON COMMIT DROP`).Error; err != nil {
			return err
		}
		if err := insertChunks(tx, "INSERT INTO fortuna_kev VALUES ", "(?,?,?,?)", 4, len(entries), func(i int) []interface{} {
			e := entries[i]
			return []interface{}{e.CVEID, e.DateAdded, e.DueDate, e.Ransomware}
		}); err != nil {
			return err
		}
		for _, stmt := range []string{
			`UPDATE vulnerabilities SET kev_added_at = NULL, kev_due_date = NULL, kev_ransomware = NULL, kev_updated_at = now()
			 WHERE kev_added_at IS NOT NULL AND vuln_id NOT IN (SELECT vuln_id FROM fortuna_kev)`,
			`INSERT INTO vulnerabilities (vuln_id, kev_added_at, kev_due_date, kev_ransomware, kev_updated_at)
			 SELECT vuln_id, added, due, ransomware, now() FROM fortuna_kev
			 ON CONFLICT (vuln_id) DO UPDATE SET kev_added_at = EXCLUDED.kev_added_at, kev_due_date = EXCLUDED.kev_due_date,
			   kev_ransomware = EXCLUDED.kev_ransomware, kev_updated_at = EXCLUDED.kev_updated_at`,
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("store KEV: %w", err)
	}
	return len(entries), nil
}

// StoreEPSS writes the scores of one EPSS day. Scores of CVEs absent from the file are kept
// with their older date.
func StoreEPSS(ctx context.Context, db *gorm.DB, scores []EPSSScore, scoreDate time.Time) (int, error) {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMP TABLE fortuna_epss (vuln_id VARCHAR(255) PRIMARY KEY, score REAL NOT NULL, pct REAL NOT NULL) ON COMMIT DROP`).Error; err != nil {
			return err
		}
		if err := insertChunks(tx, "INSERT INTO fortuna_epss VALUES ", "(?,?,?)", 3, len(scores), func(i int) []interface{} {
			s := scores[i]
			return []interface{}{s.CVEID, s.Score, s.Percentile}
		}); err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO vulnerabilities (vuln_id, epss_score, epss_percentile, epss_date, epss_updated_at)
			SELECT vuln_id, score, pct, ?, now() FROM fortuna_epss
			ON CONFLICT (vuln_id) DO UPDATE SET epss_score = EXCLUDED.epss_score, epss_percentile = EXCLUDED.epss_percentile,
			  epss_date = EXCLUDED.epss_date, epss_updated_at = EXCLUDED.epss_updated_at`, scoreDate).Error
	})
	if err != nil {
		return 0, fmt.Errorf("store EPSS: %w", err)
	}
	return len(scores), nil
}

// insertChunks inserts n rows of cols values each, keeping every statement under PostgreSQL's
// 65535 bind parameters. Duplicate keys in one feed keep the first row.
func insertChunks(tx *gorm.DB, head, rowTemplate string, cols, n int, row func(int) []interface{}) error {
	per := 60000 / cols
	for start := 0; start < n; start += per {
		end := start + per
		if end > n {
			end = n
		}
		var sb strings.Builder
		sb.WriteString(head)
		args := make([]interface{}, 0, (end-start)*cols)
		for i := start; i < end; i++ {
			if i > start {
				sb.WriteByte(',')
			}
			sb.WriteString(rowTemplate)
			args = append(args, row(i)...)
		}
		sb.WriteString(" ON CONFLICT DO NOTHING")
		if err := tx.Exec(sb.String(), args...).Error; err != nil {
			return err
		}
	}
	return nil
}

// Result reports one feed of a Run.
type Result struct {
	Feed    string
	Rows    int
	Skipped bool // unchanged since the last run (same ETag)
	Err     error
}

// Run downloads and stores the KEV catalog and the EPSS scores. An empty URL skips that feed.
// A feed whose ETag is unchanged is not stored again. Each feed's outcome is recorded in
// vuln_feed_state; one feed failing does not stop the other.
func Run(ctx context.Context, db *gorm.DB, client *http.Client, kevURL, epssURL string) []Result {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	var results []Result
	if kevURL != "" {
		results = append(results, runFeed(ctx, db, client, feedKEV, kevURL, func(r io.Reader) (int, error) {
			entries, err := ParseKEV(r)
			if err != nil {
				return 0, err
			}
			return StoreKEV(ctx, db, entries)
		}))
	}
	if epssURL != "" {
		results = append(results, runFeed(ctx, db, client, feedEPSS, epssURL, func(r io.Reader) (int, error) {
			scores, day, err := ParseEPSS(r)
			if err != nil {
				return 0, err
			}
			return StoreEPSS(ctx, db, scores, day)
		}))
	}
	return results
}

func runFeed(ctx context.Context, db *gorm.DB, client *http.Client, feed, url string, store func(io.Reader) (int, error)) Result {
	res := Result{Feed: feed}
	var prevETag string
	db.WithContext(ctx).Raw(`SELECT etag FROM vuln_feed_state WHERE feed = ? AND scope = ''`, feed).Scan(&prevETag)

	etag, err := func() (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "fortuna-vulndb-update")
		if prevETag != "" {
			req.Header.Set("If-None-Match", prevETag)
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotModified {
			res.Skipped = true
			return prevETag, nil
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
		}
		n, err := store(io.LimitReader(resp.Body, maxFeedBytes))
		res.Rows = n
		return resp.Header.Get("ETag"), err
	}()
	res.Err = err
	if err != nil {
		db.WithContext(ctx).Exec(`INSERT INTO vuln_feed_state (feed, scope, last_error) VALUES (?, '', ?)
			ON CONFLICT (feed, scope) DO UPDATE SET last_error = EXCLUDED.last_error`, feed, err.Error())
		return res
	}
	db.WithContext(ctx).Exec(`INSERT INTO vuln_feed_state (feed, scope, etag, last_success_at, last_error) VALUES (?, '', ?, now(), '')
		ON CONFLICT (feed, scope) DO UPDATE SET etag = EXCLUDED.etag, last_success_at = EXCLUDED.last_success_at, last_error = ''`, feed, etag)
	return res
}
