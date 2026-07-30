// Package epss fetches FIRST.org EPSS scores for CVE IDs (optional enrichment, RISK-1).
package epss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fortuna/core/pkg/metrics"
)

const defaultBaseURL = "https://api.first.org/data/v1/epss"

// Enabled returns true when FORTUNA_EPSS_ENABLED is 1/true.
func Enabled() bool {
	s := strings.TrimSpace(strings.ToLower(os.Getenv("FORTUNA_EPSS_ENABLED")))
	return s == "1" || s == "true" || s == "yes"
}

type cacheEntry struct {
	epss        float64
	percentile  float64
	expiresUnix int64
}

// Client is a small FIRST EPSS HTTP client with in-process TTL cache.
type Client struct {
	baseURL string
	http    *http.Client
	mu      sync.Mutex
	cache   map[string]cacheEntry
	ttl     time.Duration
}

// NewClient builds a client from env (FORTUNA_EPSS_BASE_URL optional).
func NewClient() *Client {
	base := strings.TrimSpace(os.Getenv("FORTUNA_EPSS_BASE_URL"))
	if base == "" {
		base = defaultBaseURL
	}
	ttl := 24 * time.Hour
	if s := strings.TrimSpace(os.Getenv("FORTUNA_EPSS_CACHE_TTL")); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			ttl = d
		}
	}
	return &Client{
		baseURL: strings.TrimRight(base, "/"),
		http: &http.Client{
			Timeout: 8 * time.Second,
		},
		cache: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

type apiResponse struct {
	Data []struct {
		CVE        string `json:"cve"`
		EPSS       string `json:"epss"`
		Percentile string `json:"percentile"`
	} `json:"data"`
}

// Lookup returns EPSS score in [0,1] and percentile in [0,1]. ok is false on miss/error.
func (c *Client) Lookup(ctx context.Context, cveID string) (epss float64, percentile float64, ok bool) {
	cveID = strings.TrimSpace(strings.ToUpper(cveID))
	if cveID == "" || c == nil {
		return 0, 0, false
	}

	now := time.Now().Unix()
	c.mu.Lock()
	if e, hit := c.cache[cveID]; hit && e.expiresUnix > now {
		c.mu.Unlock()
		metrics.EPSSCacheHitsTotal.Inc()
		return e.epss, e.percentile, true
	}
	c.mu.Unlock()

	u, err := url.Parse(c.baseURL)
	if err != nil {
		metrics.EPSSLookupErrorsTotal.Inc()
		return 0, 0, false
	}
	q := u.Query()
	q.Set("cve", cveID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		metrics.EPSSLookupErrorsTotal.Inc()
		return 0, 0, false
	}
	req.Header.Set("User-Agent", "fortuna-core/epss")

	metrics.EPSSLookupsTotal.Inc()
	resp, err := c.http.Do(req)
	if err != nil {
		metrics.EPSSLookupErrorsTotal.Inc()
		return 0, 0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		metrics.EPSSLookupErrorsTotal.Inc()
		return 0, 0, false
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		metrics.EPSSLookupErrorsTotal.Inc()
		return 0, 0, false
	}
	if len(body.Data) == 0 {
		return 0, 0, false
	}
	row := body.Data[0]
	ep, err1 := strconv.ParseFloat(strings.TrimSpace(row.EPSS), 64)
	pc, err2 := strconv.ParseFloat(strings.TrimSpace(row.Percentile), 64)
	if err1 != nil || err2 != nil {
		metrics.EPSSLookupErrorsTotal.Inc()
		return 0, 0, false
	}

	exp := now + int64(c.ttl.Seconds())
	c.mu.Lock()
	c.cache[cveID] = cacheEntry{epss: ep, percentile: pc, expiresUnix: exp}
	c.mu.Unlock()
	return ep, pc, true
}

// EvidenceJSON builds Insight.evidence JSON for successful lookup.
func EvidenceJSON(epssScore, percentile float64) string {
	b, err := json.Marshal(map[string]interface{}{
		"epss":            epssScore,
		"epss_percentile": percentile,
		"epss_source":     "first.org",
	})
	if err != nil {
		return ""
	}
	return string(b)
}

var defaultClient = sync.OnceValue(func() *Client { return NewClient() })

// LookupDefault uses the process-wide client.
func LookupDefault(ctx context.Context, cveID string) (epss float64, percentile float64, ok bool) {
	return defaultClient().Lookup(ctx, cveID)
}

// EpssResult holds one successful EPSS lookup.
type EpssResult struct {
	EPSS       float64
	Percentile float64
}

// LookupManyDefault runs parallel Lookups on the process-wide client.
// Concurrency: FORTUNA_EPSS_CONCURRENCY (default 8); use LookupMany for explicit concurrency in tests.
func LookupManyDefault(ctx context.Context, cveIDs []string) map[string]EpssResult {
	return defaultClient().LookupMany(ctx, cveIDs, 0)
}

// LookupMany runs parallel Lookups for unique CVE IDs. If concurrency <= 0, uses FORTUNA_EPSS_CONCURRENCY (default 8).
func (c *Client) LookupMany(ctx context.Context, cveIDs []string, concurrency int) map[string]EpssResult {
	seen := make(map[string]struct{})
	uniq := make([]string, 0)
	for _, id := range cveIDs {
		id = strings.TrimSpace(strings.ToUpper(id))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return nil
	}
	lim := concurrency
	if lim <= 0 {
		lim = 8
		if s := strings.TrimSpace(os.Getenv("FORTUNA_EPSS_CONCURRENCY")); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				lim = n
			}
		}
	}
	if c == nil {
		return nil
	}
	out := make(map[string]EpssResult)
	var mu sync.Mutex
	sem := make(chan struct{}, lim)
	var wg sync.WaitGroup
	for _, id := range uniq {
		id := id
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			e, p, ok := c.Lookup(ctx, id)
			if !ok {
				return
			}
			mu.Lock()
			out[id] = EpssResult{EPSS: e, Percentile: p}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

