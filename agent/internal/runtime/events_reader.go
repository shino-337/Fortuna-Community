package runtime

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
)

type Event struct {
	EventType      string                 `json:"event_type"`
	MitreTechnique string                 `json:"mitre_technique"`
	Signal         string                 `json:"signal"`
	Severity       string                 `json:"severity"`
	Pod            map[string]interface{} `json:"pod"`
	Runtime        string                 `json:"runtime"`
	Syscall        string                 `json:"syscall"`
	Target         string                 `json:"target"`
	Capability     string                 `json:"capability,omitempty"`
	Capabilities   []string               `json:"capabilities"`
	Timestamp      int64                  `json:"timestamp"`

	// Canonical contract fields (optional): used by core POST /api/v2/runtime/events.
	EventID         string                 `json:"event_id,omitempty"`
	ObservedAt      string                 `json:"observed_at,omitempty"`      // RFC3339
	IngestedAt      string                 `json:"ingested_at,omitempty"`      // RFC3339
	ResolutionState string                 `json:"resolution_state,omitempty"` // resolved|partial|unresolved
	SourceKind      string                 `json:"source_kind,omitempty"`
	SourceSensorID  string                 `json:"source_sensor_id,omitempty"`
	SourceRule      string                 `json:"source_rule,omitempty"`
	PayloadJSON     map[string]interface{} `json:"payload_json,omitempty"`
	PayloadHash     string                 `json:"payload_hash,omitempty"`
	Confidence      float64                `json:"confidence,omitempty"`
}

type Reader struct {
	path       string
	poll       time.Duration
	coreURL    string
	httpClient *http.Client
	logger     *log.Logger
	offset     int64
	// ingestion quality counters
	invalidLines  uint64
	sentBatches   uint64
	sentEvents    uint64
	failedBatches uint64
	failedEvents  uint64
	v2Success     uint64
	v1Fallback    uint64
}

func NewReader(path string, poll time.Duration, coreURL string) *Reader {
	return &Reader{
		path:       path,
		poll:       poll,
		coreURL:    strings.TrimRight(coreURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     log.New(log.Writer(), "[RuntimeEvents] ", log.LstdFlags),
	}
}

func (r *Reader) Start(ctx context.Context) {
	ticker := time.NewTicker(r.poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.readAndSend()
		}
	}
}

func (r *Reader) readAndSend() {
	f, err := os.Open(r.path)
	if err != nil {
		return
	}
	defer f.Close()

	startOffset := r.offset
	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		r.logger.Printf("Failed to seek runtime events file: %v", err)
		return
	}

	scanner := bufio.NewScanner(f)
	events := make([]Event, 0, 10)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var evt Event
		if err := json.Unmarshal(line, &evt); err != nil {
			atomic.AddUint64(&r.invalidLines, 1)
			r.logger.Printf("Invalid runtime event JSON: %v", err)
			continue
		}
		events = append(events, evt)
	}

	if err := scanner.Err(); err != nil {
		r.logger.Printf("Runtime events read error: %v", err)
		// Keep the previous offset so a transient read error cannot discard data.
		return
	}

	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		r.logger.Printf("Failed to determine runtime events offset: %v", err)
		return
	}

	if len(events) == 0 {
		// Invalid/empty records should not be retried forever when there is no
		// deliverable event in this slice.
		r.offset = pos
		return
	}

	if err := r.send(events); err != nil {
		atomic.AddUint64(&r.failedBatches, 1)
		atomic.AddUint64(&r.failedEvents, uint64(len(events)))
		r.logger.Printf("Failed to send runtime events: %v", err)
		r.logIngestionStats("send_failed")
		// Do not advance. The same file slice is retried on the next poll. This is
		// required when Core temporarily rejects ingest while inventory/identity
		// state is converging.
		r.offset = startOffset
		return
	}
	r.offset = pos
	atomic.AddUint64(&r.sentBatches, 1)
	atomic.AddUint64(&r.sentEvents, uint64(len(events)))
	r.logIngestionStats("send_ok")
}

func (r *Reader) send(events []Event) error {
	// Enrich events with canonical v2 fields if missing.
	now := time.Now().UTC()
	for i := range events {
		ev := &events[i]

		podUID := ""
		if ev.Pod != nil {
			if v, ok := ev.Pod["uid"].(string); ok {
				podUID = v
			}
		}
		podUID = strings.TrimSpace(podUID)

		ts := ev.Timestamp
		if ts <= 0 {
			ts = now.Unix()
		}

		observedAt := time.Unix(ts, 0).UTC()
		if strings.TrimSpace(ev.ObservedAt) == "" {
			ev.ObservedAt = observedAt.Format(time.RFC3339)
		}
		if strings.TrimSpace(ev.IngestedAt) == "" {
			ev.IngestedAt = now.Format(time.RFC3339)
		}
		if strings.TrimSpace(ev.ResolutionState) == "" {
			ev.ResolutionState = "unresolved"
		} else {
			ev.ResolutionState = normalizeResolutionState(ev.ResolutionState)
		}
		if strings.TrimSpace(ev.SourceKind) == "" {
			if strings.TrimSpace(ev.Runtime) != "" {
				ev.SourceKind = strings.TrimSpace(ev.Runtime)
			} else {
				ev.SourceKind = "agent"
			}
		}

		if strings.TrimSpace(ev.EventID) == "" {
			h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d", podUID, ev.Syscall, ev.Target, ts)))
			ev.EventID = hex.EncodeToString(h[:])
		}

		if ev.PayloadJSON == nil {
			ev.PayloadJSON = map[string]interface{}{
				"syscall":    ev.Syscall,
				"target":     ev.Target,
				"signal":     ev.Signal,
				"event_type": ev.EventType,
			}
		}
		if strings.TrimSpace(ev.PayloadHash) == "" {
			b, _ := json.Marshal(ev.PayloadJSON)
			h := sha256.Sum256(b)
			ev.PayloadHash = hex.EncodeToString(h[:])
		}
	}

	body, _ := json.Marshal(events)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v2/runtime/events", r.coreURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	corehttp.ApplyOptionalAuthorization(req)

	resp, err := r.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			atomic.AddUint64(&r.v2Success, 1)
			return nil
		}
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("runtime events v2 POST failed: %s", resp.Status)
		}
	}

	// Fallback to v1
	req2, err2 := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/runtime/events", r.coreURL), bytes.NewReader(body))
	if err2 != nil {
		return err2
	}
	req2.Header.Set("Content-Type", "application/json")
	corehttp.ApplyOptionalAuthorization(req2)
	resp2, err3 := r.httpClient.Do(req2)
	if err3 != nil {
		return err3
	}
	defer resp2.Body.Close()

	if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		return fmt.Errorf("runtime events POST failed: %s", resp2.Status)
	}
	atomic.AddUint64(&r.v1Fallback, 1)
	return nil
}

func (r *Reader) logIngestionStats(status string) {
	r.logger.Printf(
		"[IngestQuality] status=%s sent_batches=%d sent_events=%d failed_batches=%d failed_events=%d invalid_lines=%d v2_success=%d v1_fallback=%d",
		status,
		atomic.LoadUint64(&r.sentBatches),
		atomic.LoadUint64(&r.sentEvents),
		atomic.LoadUint64(&r.failedBatches),
		atomic.LoadUint64(&r.failedEvents),
		atomic.LoadUint64(&r.invalidLines),
		atomic.LoadUint64(&r.v2Success),
		atomic.LoadUint64(&r.v1Fallback),
	)
}

func normalizeResolutionState(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "resolved":
		return "resolved"
	case "partial":
		return "partial"
	default:
		return "unresolved"
	}
}