package worker

import (
	"context"
	"encoding/json"
	"log"

	"github.com/fortuna/core/pkg/metrics"
	"github.com/nats-io/nats.go"
)

// SBOMDLQWorker consumes fortuna.sbom.created.dlq (events that failed primary JetStream publish
// after retries in handler_sbom.go). It provides operational visibility (structured log + metric)
// without auto-replay (replay should be explicit to avoid storms).
type SBOMDLQWorker struct {
	logger *log.Logger
}

// NewSBOMDLQWorker creates a DLQ consumer for SBOM_CREATED.
func NewSBOMDLQWorker() *SBOMDLQWorker {
	return &SBOMDLQWorker{logger: log.Default()}
}

// Subject is the JetStream subject for SBOM_CREATED dead letters.
func (w *SBOMDLQWorker) Subject() string {
	return "fortuna.sbom.created.dlq"
}

// Process parses minimal fields, increments metrics, and logs. Always returns nil so the
// message can be acked (at-least-once visibility; duplicate delivery may re-log).
func (w *SBOMDLQWorker) Process(ctx context.Context, msg *nats.Msg) error {
	_ = ctx
	var payload struct {
		Type   string `json:"type"`
		SBOMID uint64 `json:"sbom_id"`
		PodUID string `json:"pod_uid"`
	}
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		w.logger.Printf("[SBOM_DLQ] WARN: unmarshal failed: %v (bytes=%d)", err, len(msg.Data))
	} else {
		w.logger.Printf("[SBOM_DLQ] dead-letter type=%q sbom_id=%d pod_uid=%q bytes=%d",
			payload.Type, payload.SBOMID, payload.PodUID, len(msg.Data))
	}
	metrics.SBOMCreatedDLQConsumedTotal.Inc()
	return nil
}
