package runtime

import (
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
	lineBuf    []byte
	fileInfo   os.FileInfo
	coverage   *CoverageReporter
	// ingestion quality counters
	invalidLines  uint64
	sentBatches   uint64
	sentEvents    uint64
	failedBatches uint64
	failedEvents  uint64
	v2Success     uint64
}

func NewReader(path string, poll time.Duration, coreURL string) *Reader {
	if poll <= 0 {
		poll = 5 * time.Second
	}
	r := &Reader{
		path:       path,
		poll:       poll,
		coreURL:    strings.TrimRight(coreURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     log.New(log.Writer(), "[RuntimeEvents] ", log.LstdFlags),
	}
	r.coverage = NewCoverageReporter(r.coreURL, "runtime-file", "file", r.httpClient)
	return r
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
	obs := CoverageObservation{}
	defer func() {
		obs.End = time.Now().UTC()
		if r.coverage != nil {
			if err := r.coverage.Report(context.Background(), obs); err != nil {
				r.logger.Printf("Runtime coverage report failed: %v", err)
			}
		}
	}()

	f, err := os.Open(r.path)
	if err != nil {
		obs.Failed = true
		obs.Reason = "source_open_failed"
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		obs.Failed = true
		obs.Reason = "source_stat_failed"
		return
	}
	if r.fileInfo != nil && !os.SameFile(r.fileInfo, st) {
		// Rotation can strand unread bytes in the old inode. Reset the cursor for
		// the new file, but break coverage continuity because loss is possible.
		r.offset = 0
		r.lineBuf = nil
		obs.Failed = true
		obs.Reason = "source_rotated"
	} else if r.offset > st.Size() {
		r.offset = 0
		r.lineBuf = nil
		obs.Failed = true
		obs.Reason = "source_truncated"
	}
	r.fileInfo = st

	startOffset := r.offset
	startBuf := append([]byte(nil), r.lineBuf...)
	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		obs.Failed = true
		obs.Reason = joinCoverageReason(obs.Reason, "source_seek_failed")
		return
	}
	chunk, err := io.ReadAll(f)
	if err != nil {
		obs.Failed = true
		obs.Reason = joinCoverageReason(obs.Reason, "source_read_failed")
		return
	}

	data := append(append([]byte(nil), startBuf...), chunk...)
	r.lineBuf = nil
	events := make([]Event, 0, 10)
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			r.lineBuf = append([]byte(nil), data...)
			break
		}
		line := bytes.TrimSpace(data[:idx])
		data = data[idx+1:]
		if len(line) == 0 {
			continue
		}
		var evt Event
		if err := json.Unmarshal(line, &evt); err != nil {
			atomic.AddUint64(&r.invalidLines, 1)
			obs.Invalid++
			r.logger.Printf("Invalid runtime event JSON: %v", err)
			continue
		}
		events = append(events, evt)
	}

	nextOffset := startOffset + int64(len(chunk))
	if len(r.lineBuf) != 0 {
		// A half-written record may later become a real event whose timestamp
		// falls inside this interval. Do not claim clean silence across it.
		obs.Failed = true
		obs.Reason = joinCoverageReason(obs.Reason, "partial_record_pending")
	}

	if len(events) == 0 {
		r.offset = nextOffset
		return
	}

	obs.Emitted = uint64(len(events))
	if err := r.send(events); err != nil {
		atomic.AddUint64(&r.failedBatches, 1)
		atomic.AddUint64(&r.failedEvents, uint64(len(events)))
		obs.Failed = true
		obs.Reason = joinCoverageReason(obs.Reason, "delivery_failed_retained")
		r.logger.Printf("Failed to send runtime events: %v", err)
		r.logIngestionStats("send_failed")
		// Restore both cursor components: the exact same byte slice is retried.
		r.offset = startOffset
		r.lineBuf = startBuf
		return
	}
	obs.Delivered = uint64(len(events))
	r.offset = nextOffset
	atomic.AddUint64(&r.sentBatches, 1)
	atomic.AddUint64(&r.sentEvents, uint64(len(events)))
	r.logIngestionStats("send_ok")
}

// PrepareEventsV2 fills canonical metadata for every runtime producer while preserving sensor values.
func PrepareEventsV2(events []Event) {
	for i := range events {
		ev := &events[i]

		podUID := ""
		if ev.Pod != nil {
			if v, ok := ev.Pod["uid"].(string); ok {
				podUID = v
			}
		}
		podUID = strings.TrimSpace(podUID)

		if ev.Timestamp > 0 && strings.TrimSpace(ev.ObservedAt) == "" {
			ev.ObservedAt = time.Unix(ev.Timestamp, 0).UTC().Format(time.RFC3339)
		}
		// ingested_at is intentionally left empty when the producer did not
		// supply one. Core stamps the durable receipt time. Injecting time.Now()
		// here would make the same retained event change across retries.
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
		if ev.Confidence <= 0 {
			ev.Confidence = 0.5
		}
		if ev.Confidence > 1 {
			ev.Confidence = 1
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
		if strings.TrimSpace(ev.EventID) == "" {
			stable := fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s|%s|%s|%s|%s|%s",
				podUID, ev.Syscall, ev.Target, ev.Timestamp, ev.Runtime, ev.SourceKind,
				ev.SourceSensorID, ev.SourceRule, ev.Signal, ev.EventType, ev.Capability, ev.PayloadHash)
			h := sha256.Sum256([]byte(stable))
			ev.EventID = hex.EncodeToString(h[:])
		}
	}
}

func (r *Reader) send(events []Event) error {
	if _, err := PostEventsV2(context.Background(), r.httpClient, r.coreURL, events); err != nil {
		return err
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
