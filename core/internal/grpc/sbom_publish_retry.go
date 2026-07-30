package grpc

import (
	"context"
	"time"
)

const (
	sbomCreatedSubject    = "fortuna.sbom.created"
	sbomCreatedDLQSubject = "fortuna.sbom.created.dlq"
	sbomPublishMaxAttempts = 5
	sbomPublishBaseDelay   = 250 * time.Millisecond
	sbomPublishMaxSleep    = 3 * time.Second
)

// sbomPublishFunc publishes to a JetStream subject (typically natsClient.Publish).
type sbomPublishFunc func(subject string, data []byte) error

// publishSBOMCreatedWithRetry publishes eventJSON to the primary subject with exponential
// backoff (C1). If all attempts fail or context is cancelled, best-effort publish to DLQ.
// Returns primaryErr (last error from primary, or ctx.Err()) when DLQ was attempted or would be;
// dlqErr is non-nil only if DLQ publish failed after primary failure.
func publishSBOMCreatedWithRetry(
	ctx context.Context,
	publish sbomPublishFunc,
	eventJSON []byte,
	sleep func(time.Duration),
) (primaryErr error, dlqErr error) {
	if sleep == nil {
		sleep = time.Sleep
	}
	var lastErr error
	for attempt := 1; attempt <= sbomPublishMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			lastErr = err
			break
		}
		if err := publish(sbomCreatedSubject, eventJSON); err == nil {
			return nil, nil
		} else {
			lastErr = err
			if attempt < sbomPublishMaxAttempts {
				d := sbomPublishBaseDelay * time.Duration(1<<(attempt-1))
				if d > sbomPublishMaxSleep {
					d = sbomPublishMaxSleep
				}
				sleep(d)
			}
		}
	}
	if lastErr != nil {
		dlqErr = publish(sbomCreatedDLQSubject, eventJSON)
		return lastErr, dlqErr
	}
	return nil, nil
}
