package ebpf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ciliumebpf "github.com/cilium/ebpf"
	"github.com/fortuna/agent/internal/corehttp"
	"github.com/fortuna/agent/internal/runtime"
	"github.com/fortuna/api/collection"
)

// Sensor is phase-1 scaffold for R9.
// Current behavior is fail-open preflight only; runtime pipeline keeps working even if eBPF is unavailable.
type Sensor struct {
	coreURL       string
	nodeName      string
	podUID        string
	flushInterval time.Duration
	httpClient    *http.Client
	eventCh       chan runtime.Event
	simulate      bool

	mode          string
	links         []io.Closer
	emittedEvents uint64
	droppedEvents uint64

	coverageMu        sync.Mutex
	coverage          *runtime.CoverageReporter
	coverageEmitted   uint64
	coverageDelivered uint64
	coverageDropped   uint64
	coverageInvalid   uint64
	coverageErrors    uint64
	deliveryPending   uint32
}

func ProducerIDForMode(mode string) string {
	return "ebpf-" + normalizeEBPFMode(mode)
}

func NewSensor(mode, coreURL, nodeName string, flushInterval time.Duration, bufferSize int, simulate bool, sessionIDs ...string) *Sensor {
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	if bufferSize <= 0 {
		bufferSize = 200
	}
	normalizedMode := normalizeEBPFMode(mode)
	return &Sensor{
		mode:          normalizedMode,
		coreURL:       strings.TrimRight(coreURL, "/"),
		nodeName:      nodeName,
		podUID:        strings.TrimSpace(os.Getenv("POD_UID")),
		flushInterval: flushInterval,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		eventCh:       make(chan runtime.Event, bufferSize),
		simulate:      simulate,
		coverage:      runtime.NewCoverageReporter(coreURL, "ebpf-"+normalizedMode, collection.RuntimeSourceEBPF, sessionIDs...),
	}
}

func (s *Sensor) SetCoverageCadence(cadence time.Duration) {
	if s != nil && s.coverage != nil {
		s.coverage.SetCadence(cadence)
	}
}

func (s *Sensor) Start(ctx context.Context) {
	var opts ciliumebpf.CollectionOptions
	_ = opts
	pf := RunPreflight()
	if !pf.Ready {
		log.Printf("[eBPF] preflight not ready (%s); fail-open: disabling eBPF sensor", pf.Reason)
		if err := s.coverage.Observe(time.Now().UTC(), runtime.CoverageStats{Errors: 1}, "ebpf preflight unavailable: "+pf.Reason); err != nil {
			log.Printf("[eBPF] failed to report preflight coverage failure: %v", err)
		}
		return
	}
	s.mode = normalizeEBPFMode(s.mode)
	log.Printf("[eBPF] sensor stream enabled (mode=%s, preflight passed, flush=%s simulate=%v)", s.mode, s.flushInterval, s.simulate)
	s.attachSelectedTracepoints()
	defer s.closeLinks()

	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		s.flushLoop(ctx)
	}()
	if s.simulate {
		go s.simulateLoop(ctx)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// Wait for flushLoop to account the final retained batch before closing
			// the last coverage window.
			select {
			case <-flushDone:
			case <-time.After(12 * time.Second):
				atomic.AddUint64(&s.coverageErrors, 1)
				atomic.StoreUint32(&s.deliveryPending, 1)
			}
			s.reportCoverage()
			if err := s.coverage.Flush(); err != nil {
				log.Printf("[eBPF] coverage final flush failed: %v", err)
			}
			log.Printf("[eBPF] stopping sensor (mode=%s emitted=%d dropped=%d)", s.mode, atomic.LoadUint64(&s.emittedEvents), atomic.LoadUint64(&s.droppedEvents))
			return
		case <-ticker.C:
			s.reportCoverage()
			log.Printf("[eBPF] heartbeat mode=%s emitted=%d dropped=%d", s.mode, atomic.LoadUint64(&s.emittedEvents), atomic.LoadUint64(&s.droppedEvents))
		}
	}
}

func (s *Sensor) enqueueEvent(evt runtime.Event) {
	select {
	case s.eventCh <- evt:
		atomic.AddUint64(&s.emittedEvents, 1)
	default:
		atomic.AddUint64(&s.droppedEvents, 1)
		atomic.AddUint64(&s.coverageDropped, 1)
	}
}

func (s *Sensor) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	batch := make([]runtime.Event, 0, 50)

	flush := func() bool {
		if len(batch) == 0 {
			return true
		}
		if err := s.send(batch); err != nil {
			log.Printf("[eBPF] send batch failed (%d), retaining for retry: %v", len(batch), err)
			return false
		}
		batch = batch[:0]
		return true
	}
	finalize := func() {
		if flush() || len(batch) == 0 {
			return
		}
		// A process shutdown is the only point where an in-memory retained batch can
		// no longer be retried. Account for it explicitly instead of silently clearing.
		atomic.AddUint64(&s.droppedEvents, uint64(len(batch)))
		atomic.AddUint64(&s.coverageDropped, uint64(len(batch)))
		log.Printf("[eBPF] dropping %d retained events after final shutdown send failure", len(batch))
	}

	retryPending := false
	for {
		if retryPending {
			// Keep memory bounded while a failed batch is pending. New events remain in
			// the bounded channel; enqueueEvent's existing dropped counter records overflow.
			select {
			case <-ctx.Done():
				finalize()
				return
			case <-ticker.C:
				retryPending = !flush()
			}
			continue
		}

		select {
		case <-ctx.Done():
			finalize()
			return
		case evt := <-s.eventCh:
			batch = append(batch, evt)
			if len(batch) >= 50 {
				retryPending = !flush()
			}
		case <-ticker.C:
			retryPending = !flush()
		}
	}
}

func (s *Sensor) send(events []runtime.Event) error {
	if len(events) == 0 {
		return nil
	}
	// Keep emitted/delivered/error accounting in one coverage window. Without
	// this lock a heartbeat could split an in-flight HTTP attempt across windows.
	s.coverageMu.Lock()
	defer s.coverageMu.Unlock()
	atomic.AddUint64(&s.coverageEmitted, uint64(len(events)))
	fail := func(err error) error {
		atomic.AddUint64(&s.coverageErrors, 1)
		atomic.StoreUint32(&s.deliveryPending, 1)
		return err
	}
	if s.coreURL == "" {
		return fail(fmt.Errorf("coreURL is empty"))
	}
	runtime.PrepareEventsV2(events)
	body, _ := json.Marshal(events)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v2/runtime/events", s.coreURL), bytes.NewReader(body))
	if err != nil {
		return fail(err)
	}
	req.Header.Set("Content-Type", "application/json")
	corehttp.ApplyOptionalAuthorization(req)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fail(fmt.Errorf("runtime events POST failed: %s", resp.Status))
	}
	atomic.AddUint64(&s.coverageDelivered, uint64(len(events)))
	atomic.StoreUint32(&s.deliveryPending, 0)
	return nil
}

func (s *Sensor) reportCoverage() {
	s.coverageMu.Lock()
	defer s.coverageMu.Unlock()
	stats := runtime.CoverageStats{
		Emitted:   atomic.SwapUint64(&s.coverageEmitted, 0),
		Delivered: atomic.SwapUint64(&s.coverageDelivered, 0),
		Dropped:   atomic.SwapUint64(&s.coverageDropped, 0),
		Invalid:   atomic.SwapUint64(&s.coverageInvalid, 0),
		Errors:    atomic.SwapUint64(&s.coverageErrors, 0),
	}
	// The built-in sensor currently attaches no-op tracepoints and does not
	// observe real exec/connect syscall records. It may verify pipeline plumbing,
	// but it must never establish clean runtime coverage. When the real collector
	// lands, removing this fail-closed marker requires dedicated observation/loss
	// regressions.
	stats.Errors++
	reason := "ebpf sensor is experimental/no-op; authoritative coverage unavailable"
	if atomic.LoadUint32(&s.deliveryPending) != 0 {
		stats.Errors++
		reason += "; delivery backlog pending"
	}
	if err := s.coverage.Observe(time.Now().UTC(), stats, reason); err != nil {
		log.Printf("[eBPF] coverage report failed: %v", err)
	}
}

func (s *Sensor) simulateLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().Unix()
			if s.mode == "exec" || s.mode == "all" {
				s.enqueueEvent(runtime.Event{
					EventType:      "runtime.exec",
					MitreTechnique: "T1059",
					Signal:         "EBPF_EXEC_EVENT",
					Severity:       "medium",
					Syscall:        "execve",
					Target:         "/bin/sh",
					Runtime:        "ebpf",
					Timestamp:      now,
					Capability:     "EBPF_EXEC_TRACE",
					Pod:            map[string]interface{}{"uid": s.podUID, "node": s.nodeName},
				})
			}
			if s.mode == "connect" || s.mode == "all" {
				s.enqueueEvent(runtime.Event{
					EventType:      "runtime.connect",
					MitreTechnique: "T1046",
					Signal:         "EBPF_CONNECT_EVENT",
					Severity:       "medium",
					Syscall:        "connect",
					Target:         "1.1.1.1:443",
					Runtime:        "ebpf",
					Timestamp:      now,
					Capability:     "EBPF_CONNECT_TRACE",
					Pod:            map[string]interface{}{"uid": s.podUID, "node": s.nodeName},
				})
			}
		}
	}
}

func normalizeEBPFMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "exec", "connect", "all":
		return m
	default:
		return "exec"
	}
}
