package worker

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/metrics"
	"github.com/nats-io/nats.go"
)

const (
	sbomEventsStreamName = "fortuna-events"
	sbomDLQSubjectName   = "fortuna.sbom.created.dlq"
)

// SBOMDLQDepthPollInterval returns poll interval from FORTUNA_SBOM_DLQ_DEPTH_POLL_INTERVAL (Go duration).
// Empty or "0" disables polling. Default 30s when unset.
func SBOMDLQDepthPollInterval() time.Duration {
	s := strings.TrimSpace(os.Getenv("FORTUNA_SBOM_DLQ_DEPTH_POLL_INTERVAL"))
	if s == "" {
		return 30 * time.Second
	}
	if s == "0" || strings.EqualFold(s, "off") || strings.EqualFold(s, "false") {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		if n, err2 := strconv.Atoi(s); err2 == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
		log.Printf("[SBOM_DLQ_DEPTH] invalid FORTUNA_SBOM_DLQ_DEPTH_POLL_INTERVAL=%q: %v (disabled)", s, err)
		return 0
	}
	return d
}

// RunSBOMDLQStreamDepthPoller updates fortuna_sbom_created_dlq_stream_messages from JetStream StreamInfo.
// Stops when ctx is cancelled. Safe to run one goroutine per Core replica.
func RunSBOMDLQStreamDepthPoller(ctx context.Context, js nats.JetStreamContext, interval time.Duration, logger *log.Logger) {
	if js == nil || interval <= 0 {
		return
	}
	if logger == nil {
		logger = log.Default()
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	poll := func() {
		info, err := js.StreamInfo(sbomEventsStreamName)
		if err != nil {
			logger.Printf("[SBOM_DLQ_DEPTH] StreamInfo(%s): %v", sbomEventsStreamName, err)
			return
		}
		var n float64
		if info.State.Subjects != nil {
			if c, ok := info.State.Subjects[sbomDLQSubjectName]; ok {
				n = float64(c)
			}
		}
		metrics.SBOMCreatedDLQStreamMessages.Set(n)
	}

	poll()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			poll()
		}
	}
}
