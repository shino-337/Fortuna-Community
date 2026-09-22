package ebpf

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	ciliumebpf "github.com/cilium/ebpf"
	"github.com/fortuna/agent/internal/runtime"
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
	coverage      *runtime.CoverageReporter
}

func NewSensor(mode, coreURL, nodeName string, flushInterval time.Duration, bufferSize int, simulate bool) *Sensor {
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	if bufferSize <= 0 {
		bufferSize = 200
	}
	s := &Sensor{
		mode:          normalizeEBPFMode(mode),
		coreURL:       strings.TrimRight(coreURL, "/"),
		nodeName:      nodeName,
		podUID:        strings.TrimSpace(os.Getenv("POD_UID")),
		flushInterval: flushInterval,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		eventCh:       make(chan runtime.Event, bufferSize),
		simulate:      simulate,
	}
	s.coverage = runtime.NewCoverageReporter(s.coreURL, "ebpf-"+s.mode, "ebpf", s.httpClient)
	return s
}

func (s *Sensor) Start(ctx context.Context) {
	var opts ciliumebpf.CollectionOptions
	_ = opts
	pf := RunPreflight()
	if !pf.Ready {
		log.Printf("[eBPF] preflight not ready (%s); fail-open: disabling eBPF sensor", pf.Reason)
		if s.coverage != nil {
			ctxReport, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = s.coverage.Report(ctxReport, runtime.CoverageObservation{
				End: time.Now().UTC(), Failed: true, Reason: "preflight_not_ready:" + pf.Reason,
			})
			cancel()
		}
		return
	}
	s.mode = normalizeEBPFMode(s.mode)
	log.Printf("[eBPF] sensor stream enabled (mode=%s, preflight passed, flush=%s simulate=%v)", s.mode, s.flushInterval, s.simulate)
	s.attachSelectedTracepoints()
	defer s.closeLinks()

	go s.flushLoop(ctx)
	if s.simulate {
		go s.simulateLoop(ctx)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[eBPF] stopping sensor (mode=%s emitted=%d dropped=%d)", s.mode, atomic.LoadUint64(&s.emittedEvents), atomic.LoadUint64(&s.droppedEvents))
			return
		case <-ticker.C:
			// Phase-1: placeholder counters for observability before full attach pipeline.
			// These counters are intentionally explicit so rollout can verify health signals.
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
	}
}

func (s *Sensor) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	batch := make([]runtime.Event, 0, 50)

	var windowEmitted, windowDelivered uint64
	windowFailed := false
	windowReason := ""
	lastDropped := atomic.LoadUint64(&s.droppedEvents)

	recordAttempt := func(attempted, delivered uint64, err error) {
		windowEmitted += attempted
		windowDelivered += delivered
		if err != nil {
			windowFailed = true
			windowReason = "delivery_failed_retained"
		}
	}
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		n := uint64(len(batch))
		if err := s.send(batch); err != nil {
			recordAttempt(n, 0, err)
			log.Printf("[eBPF] send batch failed (%d), retaining for retry: %v", len(batch), err)
			return err
		}
		recordAttempt(n, n, nil)
		batch = batch[:0]
		return nil
	}
	drainAvailable := func() error {
		for {
			select {
			case evt := <-s.eventCh:
				batch = append(batch, evt)
				if len(batch) >= 50 {
					if err := flush(); err != nil {
						return err
					}
				}
			default:
				return nil
			}
		}
	}
	report := func(reportCtx context.Context, forceFailed bool, reason string) {
		currentDropped := atomic.LoadUint64(&s.droppedEvents)
		droppedDelta := currentDropped - lastDropped
		lastDropped = currentDropped
		obs := runtime.CoverageObservation{
			End: time.Now().UTC(), Emitted: windowEmitted, Delivered: windowDelivered,
			Dropped: droppedDelta, Failed: windowFailed || forceFailed,
			Reason: windowReason,
		}
		if reason != "" {
			if obs.Reason == "" {
				obs.Reason = reason
			} else if !strings.Contains(obs.Reason, reason) {
				obs.Reason += ";" + reason
			}
		}
		if s.coverage != nil {
			if err := s.coverage.Report(reportCtx, obs); err != nil {
				log.Printf("[eBPF] coverage report failed: %v", err)
			}
		}
		windowEmitted, windowDelivered = 0, 0
		windowFailed = false
		windowReason = ""
	}

	retryPending := false
	for {
		if retryPending {
			select {
			case <-ctx.Done():
				// A retained batch that still cannot be delivered at shutdown is
				// permanent loss. Account it before the final coverage receipt.
				if err := flush(); err != nil && len(batch) != 0 {
					atomic.AddUint64(&s.droppedEvents, uint64(len(batch)))
					log.Printf("[eBPF] dropping %d retained events after final shutdown send failure", len(batch))
					batch = batch[:0]
				}
				reportCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				report(reportCtx, true, "shutdown")
				cancel()
				return
			case <-ticker.C:
				retryPending = flush() != nil
				report(context.Background(), retryPending, "")
			}
			continue
		}

		select {
		case <-ctx.Done():
			_ = drainAvailable()
			finalErr := flush()
			if finalErr != nil && len(batch) != 0 {
				atomic.AddUint64(&s.droppedEvents, uint64(len(batch)))
				log.Printf("[eBPF] dropping %d retained events after final shutdown send failure", len(batch))
				batch = batch[:0]
			}
			reportCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			report(reportCtx, finalErr != nil, "shutdown")
			cancel()
			return
		case evt := <-s.eventCh:
			batch = append(batch, evt)
			if len(batch) >= 50 {
				retryPending = flush() != nil
			}
		case <-ticker.C:
			if err := drainAvailable(); err != nil {
				retryPending = true
			} else {
				retryPending = flush() != nil
			}
			report(context.Background(), retryPending, "")
		}
	}
}

func (s *Sensor) send(events []runtime.Event) error {
	if len(events) == 0 {
		return nil
	}
	_, err := runtime.PostEventsV2(context.Background(), s.httpClient, s.coreURL, events)
	return err
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
