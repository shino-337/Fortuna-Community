// Package kev loads CISA Known Exploited Vulnerabilities catalog (optional RISK-1+ enrichment).
package kev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fortuna/core/pkg/metrics"
)

const defaultFeedURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

// Enabled returns true when FORTUNA_KEV_ENABLED is 1/true.
func Enabled() bool {
	s := strings.TrimSpace(strings.ToLower(os.Getenv("FORTUNA_KEV_ENABLED")))
	return s == "1" || s == "true" || s == "yes"
}

type feedDoc struct {
	Vulnerabilities []struct {
		CVEID string `json:"cveID"`
	} `json:"vulnerabilities"`
}

// Catalog holds a CVE ID set refreshed from the CISA JSON feed.
type Catalog struct {
	mu     sync.RWMutex
	set    map[string]struct{}
	url    string
	client *http.Client
	ttl    time.Duration
}

// NewCatalog from env (FORTUNA_KEV_URL, FORTUNA_KEV_REFRESH default 6h).
func NewCatalog() *Catalog {
	u := strings.TrimSpace(os.Getenv("FORTUNA_KEV_URL"))
	if u == "" {
		u = defaultFeedURL
	}
	ttl := 6 * time.Hour
	if s := strings.TrimSpace(os.Getenv("FORTUNA_KEV_REFRESH")); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			ttl = d
		}
	}
	return &Catalog{
		set: make(map[string]struct{}),
		url: u,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		ttl: ttl,
	}
}

var defaultCatalogOnce sync.Once
var defaultCatalogInst *Catalog

// DefaultCatalog returns the process-wide KEV catalog (singleton).
func DefaultCatalog() *Catalog {
	defaultCatalogOnce.Do(func() {
		defaultCatalogInst = NewCatalog()
	})
	return defaultCatalogInst
}

// Contains reports whether cveID is listed in the KEV catalog (after refresh).
func Contains(cveID string) bool {
	if !Enabled() {
		return false
	}
	cveID = strings.TrimSpace(strings.ToUpper(cveID))
	if cveID == "" {
		return false
	}
	c := DefaultCatalog()
	c.mu.RLock()
	n := len(c.set)
	_, ok := c.set[cveID]
	c.mu.RUnlock()
	if ok {
		return true
	}
	// Lazy load once if empty (background refresh may not have run yet)
	if n == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), c.client.Timeout)
		_ = c.Refresh(ctx)
		cancel()
		c.mu.RLock()
		_, ok = c.set[cveID]
		c.mu.RUnlock()
	}
	return ok
}

// Refresh downloads and replaces the in-memory set.
func (c *Catalog) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		metrics.KEVRefreshErrorsTotal.Inc()
		return err
	}
	req.Header.Set("User-Agent", "fortuna-core/kev")
	resp, err := c.client.Do(req)
	if err != nil {
		metrics.KEVRefreshErrorsTotal.Inc()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		metrics.KEVRefreshErrorsTotal.Inc()
		return fmt.Errorf("kev feed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		metrics.KEVRefreshErrorsTotal.Inc()
		return err
	}
	var doc feedDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		metrics.KEVRefreshErrorsTotal.Inc()
		return err
	}
	next := make(map[string]struct{}, len(doc.Vulnerabilities))
	for _, v := range doc.Vulnerabilities {
		id := strings.TrimSpace(strings.ToUpper(v.CVEID))
		if id != "" {
			next[id] = struct{}{}
		}
	}
	c.mu.Lock()
	c.set = next
	c.mu.Unlock()
	metrics.KEVCatalogSize.Set(float64(len(next)))
	metrics.KEVRefreshTotal.Inc()
	return nil
}

// StartBackgroundRefresh runs Refresh on interval until ctx done (call from main when KEV enabled).
func StartBackgroundRefresh(ctx context.Context, c *Catalog) {
	if c == nil {
		c = DefaultCatalog()
	}
	t := time.NewTicker(c.ttl)
	defer t.Stop()
	_ = c.Refresh(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = c.Refresh(context.Background())
		}
	}
}
