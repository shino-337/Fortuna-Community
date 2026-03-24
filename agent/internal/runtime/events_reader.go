package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
}

type Reader struct {
	path       string
	poll       time.Duration
	coreURL    string
	httpClient *http.Client
	logger     *log.Logger
	offset     int64
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

	if _, err := f.Seek(r.offset, io.SeekStart); err != nil {
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
			r.logger.Printf("Invalid runtime event JSON: %v", err)
			continue
		}
		events = append(events, evt)
	}

	if err := scanner.Err(); err != nil {
		r.logger.Printf("Runtime events read error: %v", err)
	}

	pos, _ := f.Seek(0, io.SeekCurrent)
	r.offset = pos

	if len(events) == 0 {
		return
	}

	if err := r.send(events); err != nil {
		r.logger.Printf("Failed to send runtime events: %v", err)
	}
}

func (r *Reader) send(events []Event) error {
	body, _ := json.Marshal(events)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/runtime/events", r.coreURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("runtime events POST failed: %s", resp.Status)
	}
	return nil
}
