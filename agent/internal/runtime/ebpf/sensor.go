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
}

func NewSensor(mode, coreURL, nodeName string, flushInterval time.Duration, bufferSize int, simulate bool) *Sensor {
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	if bufferSize <= 0 {
		bufferSize = 200
	}
	return &Sensor{
		mode:          normalizeEBPFMode(mode),
		coreURL:       strings.TrimRight(coreURL, "/"),
		nodeName:      nodeName,
		podUID:        strings.TrimSpace(os.Getenv("POD_UID")),
		flushInterval: flushInterval,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		eventCh:       make(chan runtime.Event, bufferSize),
		simulate:      simulate,
	}
}

func (s *Sensor) Start(ctx context.Context) {
	var opts ciliumebpf.CollectionOptions
	_ = opts
	pf := RunPreflight()
	if !pf.Ready {
		log.Printf("[eBPF] preflight not ready (%s); fail-open: disabling eBPF sensor", pf.Reason)
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
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.send(batch); err != nil {
			log.Printf("[eBPF] send batch failed (%d): %v", len(batch), err)
		}
		batch = batch[:0]
	}
	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case evt := <-s.eventCh:
			batch = append(batch, evt)
			if len(batch) >= 50 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Sensor) send(events []runtime.Event) error {
	if len(events) == 0 {
		return nil
	}
	if s.coreURL == "" {
		return fmt.Errorf("coreURL is empty")
	}
	body, _ := json.Marshal(events)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/runtime/events", s.coreURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("runtime events POST failed: %s", resp.Status)
	}
	return nil
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
