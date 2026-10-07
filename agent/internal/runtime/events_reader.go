package runtime

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/fortuna/agent/internal/redact"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
	"github.com/fortuna/api/collection"
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
	SourceRecordID  string                 `json:"source_record_id,omitempty"`
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
	fileInfo   os.FileInfo
	lineBuf    []byte
	// ingestion quality counters
	invalidLines  uint64
	sentBatches   uint64
	sentEvents    uint64
	failedBatches uint64
	failedEvents  uint64
	v2Success     uint64
	coverage      *CoverageReporter
}

func NewReader(path string, poll time.Duration, coreURL string, sessionIDs ...string) *Reader {
	if poll <= 0 {
		poll = 5 * time.Second
	}
	return &Reader{
		path:       path,
		poll:       poll,
		coreURL:    strings.TrimRight(coreURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     log.New(log.Writer(), "[RuntimeEvents] ", log.LstdFlags),
		coverage:   NewCoverageReporter(coreURL, "runtime-file", collection.RuntimeSourceFile, sessionIDs...),
	}
}

func (r *Reader) SetCoverageCadence(cadence time.Duration) {
	if r != nil && r.coverage != nil {
		r.coverage.SetCadence(cadence)
	}
}

func (r *Reader) Start(ctx context.Context) {
	// Match Falco behavior: perform one immediate observation instead of waiting
	// an entire poll interval before discovering source errors or queued events.
	r.readAndSend()
	ticker := time.NewTicker(r.poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := r.coverage.Flush(); err != nil {
				r.logger.Printf("Runtime coverage final flush failed: %v", err)
			}
			return
		case <-ticker.C:
			r.readAndSend()
		}
	}
}

func (r *Reader) readAndSend() {
	stats := CoverageStats{}
	reason := ""
	defer func() {
		if err := r.coverage.Observe(time.Now().UTC(), stats, reason); err != nil {
			r.logger.Printf("Runtime coverage report failed: %v", err)
		}
	}()

	f, err := os.Open(r.path)
	if err != nil {
		stats.Errors++
		reason = "runtime event file unavailable"
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		stats.Errors++
		reason = "runtime event stat failed"
		return
	}
	fileSize := st.Size()
	rotated := r.fileInfo != nil && !os.SameFile(r.fileInfo, st)
	if rotated || r.offset > fileSize {
		r.offset = 0
		r.lineBuf = nil
		stats.Errors++
		reason = mergeCoverageReason(reason, "runtime event file rotated or truncated")
	}
	r.fileInfo = st

	startOffset := r.offset
	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		r.logger.Printf("Failed to seek runtime events file: %v", err)
		stats.Errors++
		reason = mergeCoverageReason(reason, "runtime event seek failed")
		return
	}

	chunk, err := io.ReadAll(f)
	if err != nil {
		r.logger.Printf("Runtime events read error: %v", err)
		stats.Errors++
		reason = mergeCoverageReason(reason, "runtime event read failed")
		return
	}

	// The durable source file is the retry buffer for incomplete records. Offset
	// advances only past newline-terminated records, never past a trailing
	// partial prefix. lineBuf is diagnostic state only; reconstruction always
	// re-reads the partial bytes from disk. This prevents a process restart from
	// silently skipping a prefix that existed only in memory.
	data := chunk
	r.lineBuf = nil
	events := make([]Event, 0, 10)
	consumed := int64(0)
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			r.lineBuf = append([]byte(nil), data...)
			break
		}
		rawLine := data[:idx]
		line := bytes.TrimSpace(rawLine)
		recordOffset := startOffset + consumed
		step := int64(idx + 1)
		consumed += step
		data = data[idx+1:]
		if len(line) == 0 {
			continue
		}
		var evt Event
		if err := json.Unmarshal(line, &evt); err != nil {
			atomic.AddUint64(&r.invalidLines, 1)
			stats.Invalid++
			reason = mergeCoverageReason(reason, "invalid runtime event JSON")
			r.logger.Printf("Invalid runtime event JSON: %v", err)
			continue
		}
		// Source identity is derived from the physical JSONL record, not the
		// second-granularity semantic EventID carried inside the record.
		evt.SourceRecordID = sourceFileRecordID(r.path, st, recordOffset, rawLine, 0)
		events = append(events, evt)
	}

	nextOffset := startOffset + consumed
	if len(r.lineBuf) != 0 {
		stats.Errors++
		reason = mergeCoverageReason(reason, "partial runtime event record pending")
	}

	if len(events) == 0 {
		// Completed malformed/blank records are consumed, while an incomplete
		// trailing record remains unread at nextOffset and is re-read intact.
		r.offset = nextOffset
		return
	}

	stats.Emitted += uint64(len(events))
	if err := r.send(events); err != nil {
		stats.Errors++
		reason = mergeCoverageReason(reason, "runtime event delivery failed")
		atomic.AddUint64(&r.failedBatches, 1)
		atomic.AddUint64(&r.failedEvents, uint64(len(events)))
		r.logger.Printf("Failed to send runtime events: %v", err)
		r.logIngestionStats("send_failed")
		// Re-read this whole slice from durable source on retry.
		r.offset = startOffset
		return
	}
	r.offset = nextOffset
	stats.Delivered += uint64(len(events))
	atomic.AddUint64(&r.sentBatches, 1)
	atomic.AddUint64(&r.sentEvents, uint64(len(events)))
	r.logIngestionStats("send_ok")
}

func sourceFileRecordID(path string, st os.FileInfo, offset int64, raw []byte, ordinal int) string {
	fileIdentity := "unknown"
	if st != nil {
		if stat, ok := st.Sys().(*syscall.Stat_t); ok {
			fileIdentity = fmt.Sprintf("%d:%d", uint64(stat.Dev), uint64(stat.Ino))
		} else {
			fileIdentity = st.Name()
		}
	}
	rawHash := sha256.Sum256(raw)
	sum := sha256.Sum256([]byte(fmt.Sprintf(
		"runtime-source-record-v1|%s|%s|%d|%d|%x",
		path, fileIdentity, offset, ordinal, rawHash,
	)))
	return hex.EncodeToString(sum[:])
}

func newEphemeralSourceRecordID() string {
	sum := sha256.Sum256([]byte("runtime-ephemeral-record-v1|" + cryptorand.Text()))
	return hex.EncodeToString(sum[:])
}

// PrepareEventsV2 fills canonical metadata for every runtime producer while preserving sensor values.
func PrepareEventsV2(events []Event) {
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
		// Non-file producers (for example eBPF) receive a per-observation identity
		// once, before serialization. The Event object is retained across HTTP retry,
		// so this remains stable without collapsing legitimate same-second events.
		if strings.TrimSpace(ev.SourceRecordID) == "" {
			ev.SourceRecordID = newEphemeralSourceRecordID()
		}

		// Every producer (file, Falco, eBPF) passes through here: hide credentials in the
		// target and in the payload's text before the batch leaves the node.
		ev.Target = redact.CommandLine(ev.Target)
		if redactPayloadStrings(ev.PayloadJSON) {
			ev.PayloadHash = ""
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
}

// redactPayloadStrings hides credentials in every string of a payload, in place, and reports
// whether anything changed.
func redactPayloadStrings(v interface{}) bool {
	changed := false
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			if s, ok := val.(string); ok {
				if clean := redact.Text(s); clean != s {
					t[k] = clean
					changed = true
				}
				continue
			}
			if redactPayloadStrings(val) {
				changed = true
			}
		}
	case []interface{}:
		for i, val := range t {
			if s, ok := val.(string); ok {
				if clean := redact.Text(s); clean != s {
					t[i] = clean
					changed = true
				}
				continue
			}
			if redactPayloadStrings(val) {
				changed = true
			}
		}
	}
	return changed
}

func (r *Reader) send(events []Event) error {
	PrepareEventsV2(events)
	body, _ := json.Marshal(events)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v2/runtime/events", r.coreURL), bytes.NewReader(body))
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
		return fmt.Errorf("runtime events v2 POST failed: %s", resp.Status)
	}
	atomic.AddUint64(&r.v2Success, 1)
	return nil
}

func (r *Reader) logIngestionStats(status string) {
	r.logger.Printf(
		"[IngestQuality] status=%s sent_batches=%d sent_events=%d failed_batches=%d failed_events=%d invalid_lines=%d v2_success=%d",
		status,
		atomic.LoadUint64(&r.sentBatches),
		atomic.LoadUint64(&r.sentEvents),
		atomic.LoadUint64(&r.failedBatches),
		atomic.LoadUint64(&r.failedEvents),
		atomic.LoadUint64(&r.invalidLines),
		atomic.LoadUint64(&r.v2Success),
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
