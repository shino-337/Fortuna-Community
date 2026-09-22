package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fortuna/agent/internal/corehttp"
)

// EventsV2Ack is the Core whole-batch acknowledgement. Accepted counts base
// runtime events that committed or were exact idempotent replays; Processed is
// intentionally a separate REP/classification count.
type EventsV2Ack struct {
	Accepted  int `json:"accepted"`
	Processed int `json:"processed"`
	Replayed  int `json:"replayed"`
}

// PostEventsV2 is the single runtime-event transport used by file, Falco and
// eBPF producers. HTTP 2xx without an exact accepted count is not delivery.
func PostEventsV2(ctx context.Context, client *http.Client, coreURL string, events []Event) (EventsV2Ack, error) {
	var ack EventsV2Ack
	if len(events) == 0 {
		return ack, nil
	}
	if client == nil {
		return ack, fmt.Errorf("runtime events http client is nil")
	}
	coreURL = strings.TrimRight(strings.TrimSpace(coreURL), "/")
	if coreURL == "" {
		return ack, fmt.Errorf("runtime events core URL is empty")
	}
	PrepareEventsV2(events)
	body, err := json.Marshal(events)
	if err != nil {
		return ack, fmt.Errorf("marshal runtime events: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, coreURL+"/api/v2/runtime/events", bytes.NewReader(body))
	if err != nil {
		return ack, err
	}
	req.Header.Set("Content-Type", "application/json")
	corehttp.ApplyOptionalAuthorization(req)
	resp, err := client.Do(req)
	if err != nil {
		return ack, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return ack, fmt.Errorf("runtime events v2 POST failed: %s", resp.Status)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	if err := dec.Decode(&ack); err != nil {
		return ack, fmt.Errorf("runtime events v2 ACK invalid: %w", err)
	}
	if ack.Accepted != len(events) {
		return ack, fmt.Errorf("runtime events v2 partial ACK: accepted=%d sent=%d", ack.Accepted, len(events))
	}
	return ack, nil
}
