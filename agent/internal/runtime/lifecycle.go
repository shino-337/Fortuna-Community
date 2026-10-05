package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
	"github.com/fortuna/api/collection"
)

type ProducerLifecycleReporter struct {
	coreURL          string
	sessionID        string
	sessionStartedAt time.Time
	producers        []collection.RuntimeProducerDeclaration
	httpClient       *http.Client
	mu               sync.Mutex
	stopping         bool
	stopReported     bool
}

func NewProducerLifecycleReporter(coreURL, sessionID string, sessionStartedAt time.Time, producers []collection.RuntimeProducerDeclaration) *ProducerLifecycleReporter {
	copyProducers := append([]collection.RuntimeProducerDeclaration(nil), producers...)
	collection.SortRuntimeProducerDeclarations(copyProducers)
	return &ProducerLifecycleReporter{
		coreURL:          strings.TrimRight(coreURL, "/"),
		sessionID:        strings.TrimSpace(sessionID),
		sessionStartedAt: sessionStartedAt.UTC(),
		producers:        copyProducers,
		httpClient:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *ProducerLifecycleReporter) Report(agentState string) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Serialize the entire POST, not only the state flag. Otherwise a running
	// heartbeat that passed the check could be delayed and arrive after stopping
	// with a later ReportedAt, reopening the lifecycle.
	if r.stopping && agentState == collection.RuntimeAgentRunning {
		return nil
	}
	if agentState == collection.RuntimeAgentStopping {
		if r.stopReported {
			return nil
		}
		r.stopping = true
	}
	producers := append([]collection.RuntimeProducerDeclaration(nil), r.producers...)
	if agentState == collection.RuntimeAgentStopping {
		for i := range producers {
			producers[i].Enabled = false
			producers[i].Authoritative = false
		}
	}
	manifest := collection.RuntimeProducerManifest{
		Version:          collection.RuntimeProducerManifestVersion,
		SessionID:        r.sessionID,
		SessionStartedAt: r.sessionStartedAt,
		ReportedAt:       time.Now().UTC(),
		AgentState:       agentState,
		Producers:        producers,
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, r.coreURL+"/api/v2/runtime/producers", bytes.NewReader(body))
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
		return fmt.Errorf("runtime producer manifest POST failed: %s", resp.Status)
	}
	if agentState == collection.RuntimeAgentStopping {
		r.stopReported = true
	}
	return nil
}

func (r *ProducerLifecycleReporter) Stop() error {
	return r.Report(collection.RuntimeAgentStopping)
}

// Start renews the producer lifecycle lease. Graceful shutdown explicitly closes
// the lease; crash/network loss expires it server-side.
func (r *ProducerLifecycleReporter) Start(ctx context.Context) {
	if r == nil {
		return
	}
	ticker := time.NewTicker(collection.RuntimeProducerHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- r.Stop() }()
			select {
			case <-shutdownCtx.Done():
			case <-done:
			}
			return
		case <-ticker.C:
			_ = r.Report(collection.RuntimeAgentRunning)
		}
	}
}
