package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fortuna/core/pkg/metrics"
	"github.com/nats-io/nats.go"
)

// DLQConfig configures dead letter queue behavior
type DLQConfig struct {
	Enabled         bool
	StreamName      string
	Subject         string
	Retention       time.Duration
	AlertThreshold  int // Alert if message count exceeds this
}

// DefaultDLQConfig returns default DLQ configuration
func DefaultDLQConfig() DLQConfig {
	return DLQConfig{
		Enabled:        true,
		StreamName:     "fortuna-dlq",
		Subject:        "fortuna.dlq.>",
		Retention:      7 * 24 * time.Hour, // 7 days
		AlertThreshold: 100,
	}
}

// DLQManager manages dead letter queue operations
type DLQManager struct {
	js     nats.JetStreamContext
	config DLQConfig

	alertMu       sync.Mutex
	lastAlertAt   time.Time
	alertCooldown time.Duration
}

// NewDLQManager creates a new DLQ manager
func NewDLQManager(js nats.JetStreamContext, config DLQConfig) (*DLQManager, error) {
	manager := &DLQManager{
		js:     js,
		config: config,
		// Cooldown to avoid repeated alerts/log spam while DLQ remains above threshold.
		alertCooldown: 5 * time.Minute,
	}

	if !config.Enabled {
		return manager, nil
	}

	// Setup DLQ stream
	if err := manager.setupDLQStream(); err != nil {
		return nil, fmt.Errorf("failed to setup DLQ stream: %w", err)
	}

	return manager, nil
}

// setupDLQStream creates or updates the DLQ stream
func (m *DLQManager) setupDLQStream() error {
	streamConfig := &nats.StreamConfig{
		Name:      m.config.StreamName,
		Subjects:  []string{m.config.Subject},
		Retention: nats.LimitsPolicy,
		MaxAge:    m.config.Retention,
		Storage:   nats.FileStorage,
		Replicas:  1,
	}

	// Try to add stream (will update if exists)
	_, err := m.js.AddStream(streamConfig)
	if err != nil {
		// Try to update if stream exists
		_, updateErr := m.js.UpdateStream(streamConfig)
		if updateErr != nil {
			return fmt.Errorf("failed to add/update DLQ stream: %w (update error: %v)", err, updateErr)
		}
		log.Printf("[DLQ] Updated existing DLQ stream: %s", m.config.StreamName)
	} else {
		log.Printf("[DLQ] Created DLQ stream: %s", m.config.StreamName)
	}

	return nil
}

// DLQMessage represents a message sent to DLQ
type DLQMessage struct {
	OriginalSubject string          `json:"original_subject"`
	OriginalData    json.RawMessage `json:"original_data"`
	Error           string          `json:"error"`
	ErrorType       string          `json:"error_type"`
	Attempts        int             `json:"attempts"`
	Timestamp       time.Time       `json:"timestamp"`
	WorkerName      string          `json:"worker_name"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// SendToDLQ sends a failed message to the dead letter queue
func (m *DLQManager) SendToDLQ(ctx context.Context, originalMsg *nats.Msg, err error, attempts int, workerName string, metadata map[string]interface{}) error {
	if !m.config.Enabled {
		log.Printf("[DLQ] DLQ disabled, not sending message to DLQ")
		return nil
	}

	errorType := "unknown"
	if ClassifyError(err) == ErrorTypeRetryable {
		errorType = "retryable"
	} else if ClassifyError(err) == ErrorTypeNonRetryable {
		errorType = "non-retryable"
	} else if ClassifyError(err) == ErrorTypeFatal {
		errorType = "fatal"
	}

	dlqMsg := DLQMessage{
		OriginalSubject: originalMsg.Subject,
		OriginalData:    originalMsg.Data,
		Error:           err.Error(),
		ErrorType:       errorType,
		Attempts:        attempts,
		Timestamp:       time.Now(),
		WorkerName:      workerName,
		Metadata:        metadata,
	}

	dlqData, err := json.Marshal(dlqMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	// Publish to DLQ
	subject := fmt.Sprintf("fortuna.dlq.%s", workerName)
	if _, err := m.js.Publish(subject, dlqData); err != nil {
		return fmt.Errorf("failed to publish to DLQ: %w", err)
	}

	log.Printf("[DLQ] Sent message to DLQ: subject=%s, worker=%s, attempts=%d, error=%v",
		originalMsg.Subject, workerName, attempts, err)

	// Check if we should alert
	if err := m.checkAlertThreshold(); err != nil {
		log.Printf("[DLQ] Warning: Failed to check alert threshold: %v", err)
	}

	return nil
}

// checkAlertThreshold checks if DLQ message count exceeds threshold
func (m *DLQManager) checkAlertThreshold() error {
	streamInfo, err := m.js.StreamInfo(m.config.StreamName)
	if err != nil {
		return fmt.Errorf("failed to get stream info: %w", err)
	}

	msgs := streamInfo.State.Msgs
	threshold := uint64(m.config.AlertThreshold)
	if msgs <= threshold {
		return nil
	}

	// Avoid spamming while count stays above threshold.
	m.alertMu.Lock()
	defer m.alertMu.Unlock()

	if !m.lastAlertAt.IsZero() && time.Since(m.lastAlertAt) < m.alertCooldown {
		return nil
	}
	m.lastAlertAt = time.Now()

	log.Printf("[DLQ] ALERT: DLQ message count (%d) exceeds threshold (%d) (stream=%s)",
		msgs, m.config.AlertThreshold, m.config.StreamName)
	metrics.DLQThresholdExceededTotal.WithLabelValues(m.config.StreamName).Inc()

	return nil
}

// GetDLQStats returns statistics about the DLQ
func (m *DLQManager) GetDLQStats() (map[string]interface{}, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("DLQ is disabled")
	}

	streamInfo, err := m.js.StreamInfo(m.config.StreamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	stats := map[string]interface{}{
		"stream_name":    m.config.StreamName,
		"message_count":  streamInfo.State.Msgs,
		"byte_count":     streamInfo.State.Bytes,
		"consumer_count": streamInfo.State.Consumers,
		"first_seq":      streamInfo.State.FirstSeq,
		"last_seq":       streamInfo.State.LastSeq,
	}

	return stats, nil
}



