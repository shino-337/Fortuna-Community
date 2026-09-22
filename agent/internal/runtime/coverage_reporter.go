package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
	"github.com/fortuna/api/collection"
)

type CoverageObservation struct {
	End       time.Time
	Emitted   uint64
	Delivered uint64
	Dropped   uint64
	Invalid   uint64
	Failed    bool
	Reason    string
}

// CoverageReporter accumulates producer observations until Core acknowledges a
// coverage receipt. A failed coverage POST never clears the pending window.
type CoverageReporter struct {
	mu         sync.Mutex
	coreURL    string
	producerID string
	sourceKind string
	httpClient *http.Client

	windowStart time.Time
	emitted     uint64
	delivered   uint64
	dropped     uint64
	invalid     uint64
	failed      bool
	reason      string
}

func NewCoverageReporter(coreURL, producerID, sourceKind string, client *http.Client) *CoverageReporter {
	if !corehttp.ScopedAgentCredentialConfigured() {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &CoverageReporter{
		coreURL: strings.TrimRight(strings.TrimSpace(coreURL), "/"),
		producerID: strings.TrimSpace(producerID), sourceKind: strings.TrimSpace(sourceKind),
		httpClient: client, windowStart: time.Now().UTC().Truncate(time.Microsecond),
	}
}

func (r *CoverageReporter) Report(ctx context.Context, obs CoverageObservation) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	end := obs.End.UTC().Truncate(time.Microsecond)
	if end.IsZero() {
		end = time.Now().UTC().Truncate(time.Microsecond)
	}
	if r.windowStart.IsZero() {
		r.windowStart = end
	}
	if end.Before(r.windowStart) {
		obs.Failed = true
		obs.Reason = joinCoverageReason(obs.Reason, "clock_regression")
		end = r.windowStart
	}
	if obs.Delivered > obs.Emitted {
		// Keep the receipt reportable while preserving fail-closed status.
		obs.Emitted = obs.Delivered
		obs.Failed = true
		obs.Reason = joinCoverageReason(obs.Reason, "counter_inconsistent")
	}

	r.emitted += obs.Emitted
	r.delivered += obs.Delivered
	r.dropped += obs.Dropped
	r.invalid += obs.Invalid
	if obs.Failed || obs.Dropped != 0 || obs.Invalid != 0 || obs.Delivered != obs.Emitted {
		r.failed = true
	}
	r.reason = joinCoverageReason(r.reason, obs.Reason)

	status := "complete"
	if r.failed || r.dropped != 0 || r.invalid != 0 || r.delivered != r.emitted {
		status = "failed"
	}
	receipt := collection.RuntimeCoverage{
		Version: collection.RuntimeCoverageVersion,
		ProducerID: r.producerID, SourceKind: r.sourceKind, Status: status,
		WindowStart: r.windowStart, WindowEnd: end,
		Emitted: r.emitted, Delivered: r.delivered, Dropped: r.dropped, Invalid: r.invalid,
		Reason: r.reason,
	}
	receipt.ID = runtimeCoverageID(receipt)
	if err := receipt.Validate(time.Now().UTC()); err != nil {
		return fmt.Errorf("runtime coverage local validation: %w", err)
	}
	if err := r.post(ctx, receipt); err != nil {
		return err
	}

	// Reset only after Core acknowledged this exact pending window.
	r.windowStart = end
	r.emitted, r.delivered, r.dropped, r.invalid = 0, 0, 0, 0
	r.failed = false
	r.reason = ""
	return nil
}

func (r *CoverageReporter) post(ctx context.Context, receipt collection.RuntimeCoverage) error {
	if r.coreURL == "" || r.producerID == "" || r.sourceKind == "" {
		return fmt.Errorf("runtime coverage reporter is not configured")
	}
	body, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.coreURL+"/api/v2/runtime/coverage", bytes.NewReader(body))
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
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("runtime coverage POST failed: %s", resp.Status)
	}
	return nil
}

func runtimeCoverageID(c collection.RuntimeCoverage) string {
	raw := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%d|%d|%d|%d|%s",
		c.Version, c.ProducerID, c.SourceKind, c.Status,
		c.WindowStart.UTC().Format(time.RFC3339Nano), c.WindowEnd.UTC().Format(time.RFC3339Nano),
		c.Emitted, c.Delivered, c.Dropped, c.Invalid, c.Reason)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func joinCoverageReason(current, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if next == "" {
		return current
	}
	if current == "" {
		if len(next) > 512 {
			return next[:512]
		}
		return next
	}
	if strings.Contains(current, next) {
		return current
	}
	joined := current + ";" + next
	if len(joined) > 512 {
		return joined[:512]
	}
	return joined
}
