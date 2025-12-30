package messaging

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSClient manages NATS JetStream connection
type NATSClient struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	servers string
}

// NewNATSClient creates a new NATS client
func NewNATSClient(servers string) (*NATSClient, error) {
	opts := []nats.Option{
		nats.Name("fortuna-core"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(10),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Printf("[NATS] Disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[NATS] Reconnected to %s", nc.ConnectedUrl())
		}),
	}

	conn, err := nats.Connect(servers, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Wait for JetStream to be available (with retries)
	var js nats.JetStreamContext
	maxRetries := 10
	retryDelay := 2 * time.Second
	for i := 0; i < maxRetries; i++ {
		js, err = conn.JetStream()
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			log.Printf("[NATS] JetStream not available yet (attempt %d/%d), retrying in %v...", i+1, maxRetries, retryDelay)
			time.Sleep(retryDelay)
		}
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to get JetStream context after %d retries: %w", maxRetries, err)
	}

	client := &NATSClient{
		conn:    conn,
		js:      js,
		servers: servers,
	}

	// Setup streams
	if err := client.SetupStreams(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to setup streams: %w", err)
	}

	log.Printf("[NATS] Connected to %s", servers)
	return client, nil
}

// SetupStreams creates required NATS JetStream streams
func (c *NATSClient) SetupStreams() error {
	streams := []struct {
		name     string
		subjects []string
	}{
		{
			name:     "fortuna-raw",
			subjects: []string{"fortuna.raw.pods", "fortuna.raw.serviceaccounts", "fortuna.raw.roles", "fortuna.raw.rolebindings"},
		},
		{
			name:     "fortuna-events",
			subjects: []string{"fortuna.events.runtime", "fortuna.sbom.>", "fortuna.cve.>"},
		},
		{
			name:     "fortuna-insights",
			subjects: []string{"fortuna.insights.created"},
		},
		{
			name:     "fortuna-normalized",
			subjects: []string{"fortuna.normalized.>"},
		},
	}

	for _, stream := range streams {
		// OPTIMIZED: Configure retention policy based on stream type
		// Use WorkQueuePolicy for better message cleanup (delete after all consumers ack)
		// Increase retention time to prevent message loss during high load
		maxAge := 7 * 24 * time.Hour // Default: 7 days for events/insights
		retention := nats.LimitsPolicy

		if stream.name == "fortuna-raw" || stream.name == "fortuna-normalized" {
			// For pod-related streams: Use 24h retention with WorkQueuePolicy
			// This prevents message loss while still cleaning up processed messages
			maxAge = 24 * time.Hour
			retention = nats.WorkQueuePolicy // Delete after ALL consumers ack
		} else if stream.name == "fortuna-events" {
			// For SBOM/CVE events: Longer retention for retry safety
			maxAge = 48 * time.Hour
			retention = nats.WorkQueuePolicy
		}

		cfg := &nats.StreamConfig{
			Name:      stream.name,
			Subjects:  stream.subjects,
			Retention: retention,
			MaxAge:    maxAge,
			Storage:   nats.FileStorage,
			Replicas:  1, // Use 1 replica for now to avoid quorum issues during startup
			// Add limits to prevent unbounded growth
			MaxMsgs:     1000000,              // Max 1M messages per stream
			MaxBytes:    10 * 1024 * 1024 * 1024, // Max 10GB per stream
			Discard:     nats.DiscardOld,       // Discard oldest when limits reached
		}

		// Retry stream creation with exponential backoff
		maxRetries := 5
		retryDelay := 2 * time.Second
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			// Try to add stream, if exists, update it
			_, err := c.js.AddStream(cfg)
			if err == nil {
				log.Printf("[NATS] Stream %s ready (retention: %v)", stream.name, maxAge)
				lastErr = nil
				break
			} else if err == nats.ErrStreamNameAlreadyInUse {
				// Stream already exists - check if we need to update it
				// Note: Retention policy cannot be changed after stream creation
				// So we just log and continue if stream exists
				info, infoErr := c.js.StreamInfo(stream.name)
				if infoErr == nil {
					log.Printf("[NATS] Stream %s already exists (retention: %v), skipping update", stream.name, info.Config.Retention)
					lastErr = nil
					break
				} else {
					// If we can't get stream info, try to update (but it may fail)
					_, updateErr := c.js.UpdateStream(cfg)
					if updateErr != nil {
						// If update fails due to retention policy change, just log and continue
						if strings.Contains(updateErr.Error(), "retention policy") {
							log.Printf("[NATS] Stream %s exists with different retention policy, using existing configuration", stream.name)
							lastErr = nil
							break
						}
						log.Printf("[NATS] Warning: Failed to update stream %s: %v", stream.name, updateErr)
						lastErr = updateErr
					} else {
						log.Printf("[NATS] Updated stream %s retention to %v", stream.name, maxAge)
						lastErr = nil
						break
					}
				}
			} else {
				lastErr = err
				if i < maxRetries-1 {
					log.Printf("[NATS] Failed to create stream %s (attempt %d/%d): %v, retrying in %v...", stream.name, i+1, maxRetries, err, retryDelay)
					time.Sleep(retryDelay)
					retryDelay *= 2 // Exponential backoff
				}
			}
		}
		if lastErr != nil {
			return fmt.Errorf("failed to create stream %s after %d retries: %w", stream.name, maxRetries, lastErr)
		}
	}

	return nil
}

// Publish publishes a message to a subject
func (c *NATSClient) Publish(subject string, data []byte) error {
	_, err := c.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}
	return nil
}

// Subscribe creates a subscription to a subject
func (c *NATSClient) Subscribe(subject string, handler func(*nats.Msg)) (*nats.Subscription, error) {
	sub, err := c.js.Subscribe(subject, handler, nats.Durable("fortuna-worker"))
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}
	return sub, nil
}

// Close closes the NATS connection
func (c *NATSClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// JetStream returns the JetStream context
func (c *NATSClient) JetStream() nats.JetStreamContext {
	return c.js
}
