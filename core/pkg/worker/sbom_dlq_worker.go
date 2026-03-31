package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/nats-io/nats.go"
)

// SBOMDLQWorker consumes fortuna.sbom.created.dlq (events that failed primary JetStream publish
// after retries in handler_sbom.go). It provides operational visibility (structured log + metric)
// without auto-replay (replay should be explicit to avoid storms).
type SBOMDLQWorker struct {
	logger *log.Logger
	js     nats.JetStreamContext
}

// NewSBOMDLQWorker creates a DLQ consumer for SBOM_CREATED.
func NewSBOMDLQWorker(js nats.JetStreamContext) *SBOMDLQWorker {
	return &SBOMDLQWorker{logger: log.Default(), js: js}
}

// Subject is the JetStream subject for SBOM_CREATED dead letters.
func (w *SBOMDLQWorker) Subject() string {
	return "fortuna.sbom.created.dlq"
}

// Process replays SBOM_CREATED from DLQ with bounded retries.
func (w *SBOMDLQWorker) Process(ctx context.Context, msg *nats.Msg) error {
	var ev sbom.SBOMCreatedEvent
	if err := json.Unmarshal(msg.Data, &ev); err != nil {
		w.logger.Printf("[SBOM_DLQ] WARN: unmarshal failed: %v (bytes=%d)", err, len(msg.Data))
		metrics.SBOMCreatedDLQConsumedTotal.Inc()
		return nil
	}

	attempt := 1
	if md, err := msg.Metadata(); err == nil && md != nil {
		attempt = int(md.NumDelivered)
	}
	maxAttempts := sbomDLQReplayMaxAttempts()
	if w.js == nil {
		w.logger.Printf("[SBOM_DLQ] WARN: JetStream unavailable, cannot replay type=%q sbom_id=%d pod_uid=%q",
			ev.Type, ev.SBOMID, ev.PodUID)
		metrics.SBOMCreatedDLQConsumedTotal.Inc()
		return nil
	}
	if _, err := w.js.Publish("fortuna.sbom.created", msg.Data); err != nil {
		if attempt < maxAttempts {
			return &RetryableError{Err: fmt.Errorf("sbom dlq replay publish failed attempt=%d/%d sbom_id=%d: %w", attempt, maxAttempts, ev.SBOMID, err)}
		}
		w.logger.Printf("[SBOM_DLQ] drop after max attempts=%d type=%q sbom_id=%d pod_uid=%q err=%v",
			maxAttempts, ev.Type, ev.SBOMID, ev.PodUID, err)
		metrics.SBOMCreatedDLQConsumedTotal.Inc()
		return nil
	}
	w.logger.Printf("[SBOM_DLQ] replayed type=%q sbom_id=%d pod_uid=%q attempt=%d/%d",
		ev.Type, ev.SBOMID, ev.PodUID, attempt, maxAttempts)
	metrics.SBOMCreatedDLQConsumedTotal.Inc()
	return nil
}

func sbomDLQReplayMaxAttempts() int {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_SBOM_DLQ_REPLAY_MAX_ATTEMPTS"))
	if raw == "" {
		return 5
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 5
	}
	return n
}
