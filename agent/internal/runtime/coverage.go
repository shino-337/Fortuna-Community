package runtime

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
	"github.com/fortuna/api/collection"
)

// CoverageStats are local to one producer observation interval. Emitted counts
// delivery attempts, so a retry belongs to the later interval and cannot erase
// an earlier failed interval.
type CoverageStats struct {
	Emitted   uint64
	Delivered uint64
	Dropped   uint64
	Invalid   uint64
	Errors    uint64
}

type coverageAggregate struct {
	start  time.Time
	end    time.Time
	stats  CoverageStats
	reason string
}

// CoverageReporter provides at-least-once, exact-payload reporting for producer
// coverage. An unacknowledged report is immutable and retried with the same ID;
// newer observations accumulate separately until that report is acknowledged.
type CoverageReporter struct {
	mu         sync.Mutex
	coreURL    string
	producerID string
	sourceKind string
	httpClient *http.Client
	nextStart  time.Time
	pending    *collection.RuntimeCoverage
	backlog    *coverageAggregate
}

func NewCoverageReporter(coreURL, producerID, sourceKind string) *CoverageReporter {
	return &CoverageReporter{
		coreURL: strings.TrimRight(coreURL, "/"),
		producerID: strings.TrimSpace(producerID),
		sourceKind: strings.TrimSpace(sourceKind),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		nextStart: time.Now().UTC(),
	}
}

// Reset starts a new continuity interval without claiming the skipped time.
func (r *CoverageReporter) Reset(start time.Time) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending != nil || r.backlog != nil {
		return
	}
	r.nextStart = start.UTC()
}

// Observe closes one locally observed interval and tries to deliver all queued
// coverage in order. A coverage transport failure never changes the immutable
// pending payload.
func (r *CoverageReporter) Observe(end time.Time, stats CoverageStats, reason string) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	end = end.UTC()
	if r.nextStart.IsZero() {
		r.nextStart = end
	}
	if !end.After(r.nextStart) {
		return fmt.Errorf("runtime coverage interval did not advance")
	}
	agg := coverageAggregate{start: r.nextStart, end: end, stats: stats, reason: strings.TrimSpace(reason)}
	r.nextStart = end
	r.mergeBacklog(agg)
	return r.flushLocked()
}

func (r *CoverageReporter) Flush() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.flushLocked()
}

func (r *CoverageReporter) mergeBacklog(next coverageAggregate) {
	if r.backlog == nil {
		copy := next
		copy.reason = truncateCoverageReason(copy.reason)
		r.backlog = &copy
		return
	}
	r.backlog.end = next.end
	r.backlog.stats.Emitted += next.stats.Emitted
	r.backlog.stats.Delivered += next.stats.Delivered
	r.backlog.stats.Dropped += next.stats.Dropped
	r.backlog.stats.Invalid += next.stats.Invalid
	r.backlog.stats.Errors += next.stats.Errors
	r.backlog.reason = mergeCoverageReason(r.backlog.reason, next.reason)
}

func (r *CoverageReporter) promoteBacklog() {
	if r.pending != nil || r.backlog == nil {
		return
	}
	a := r.backlog
	r.backlog = nil
	status := "complete"
	reason := strings.TrimSpace(a.reason)
	if a.stats.Dropped != 0 || a.stats.Invalid != 0 || a.stats.Errors != 0 || a.stats.Delivered != a.stats.Emitted {
		status = "failed"
		if reason == "" {
			reason = "runtime producer loss or delivery error"
		}
	}
	r.pending = &collection.RuntimeCoverage{
		Version: collection.RuntimeCoverageVersion,
		ID: rand.Text(),
		ProducerID: r.producerID,
		SourceKind: r.sourceKind,
		Status: status,
		WindowStart: a.start,
		WindowEnd: a.end,
		Emitted: a.stats.Emitted,
		Delivered: a.stats.Delivered,
		Dropped: a.stats.Dropped,
		Invalid: a.stats.Invalid,
		Errors: a.stats.Errors,
		Reason: reason,
	}
}

func (r *CoverageReporter) flushLocked() error {
	for {
		r.promoteBacklog()
		if r.pending == nil {
			return nil
		}
		if err := r.post(*r.pending); err != nil {
			return err
		}
		r.pending = nil
	}
}

func (r *CoverageReporter) post(report collection.RuntimeCoverage) error {
	if r.coreURL == "" {
		return fmt.Errorf("runtime coverage coreURL is empty")
	}
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, r.coreURL+"/api/v2/runtime/coverage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	corehttp.ApplyOptionalAuthorization(req)
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("runtime coverage POST failed: %s", resp.Status)
	}
	return nil
}

func mergeCoverageReason(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return truncateCoverageReason(b)
	}
	if b == "" || strings.Contains(a, b) {
		return truncateCoverageReason(a)
	}
	return truncateCoverageReason(a + "; " + b)
}

func truncateCoverageReason(v string) string {
	if len(v) <= 512 {
		return v
	}
	return v[:512]
}
