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
	// For cluster mode (3 replicas), wait for quorum (2/3 nodes)
	var js nats.JetStreamContext
	maxRetries := 15 // Increased retries for cluster quorum
	retryDelay := 3 * time.Second
	for i := 0; i < maxRetries; i++ {
		js, err = conn.JetStream()
		if err == nil {
			// Verify cluster is ready (for 3 replicas, need quorum = 2)
			// This is handled by NATS server automatically
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
			subjects: []string{"fortuna.insights.created", "fortuna.insights.updated"},
		},
		{
			name:     "fortuna-normalized",
			subjects: []string{"fortuna.normalized.>"},
		},
		{
			name:     "fortuna-siem",
			subjects: []string{"fortuna.siem.events"},
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

		// Calculate stream limits based on retention and storage capacity
		// For 3 replicas: Each stream replicated 3x, total storage = 30GB (3 x 10GB)
		// Stream storage: 4 streams * 1GB = 4GB, leaving 26GB buffer across replicas
		// Per-replica: 4GB streams + 8.67GB buffer = ~12.67GB per replica (within 10GB limit per PVC)
		// Note: With 3 replicas, each stream is replicated, so actual storage per replica is lower
		maxMsgs := int64(100000)  // Max 100K messages per stream
		maxBytes := int64(1 * 1024 * 1024 * 1024) // Max 1GB per stream
		
		// Adjust limits based on stream importance and retention
		if stream.name == "fortuna-events" {
			// SBOM/CVE events are critical - allow more messages
			maxMsgs = 200000  // 200K messages for events
			maxBytes = 2 * 1024 * 1024 * 1024 // 2GB for events stream
		} else if stream.name == "fortuna-insights" {
			// Insights are important but less frequent
			maxMsgs = 50000   // 50K messages for insights
			maxBytes = 512 * 1024 * 1024 // 512MB for insights
		}

		cfg := &nats.StreamConfig{
			Name:      stream.name,
			Subjects:  stream.subjects,
			Retention: retention,
			MaxAge:    maxAge,
			Storage:   nats.FileStorage,
			Replicas:  3, // Use 3 replicas for HA (quorum = 2)
			MaxMsgs:   maxMsgs,
			MaxBytes:  maxBytes,
			Discard:   nats.DiscardOld, // Discard oldest when limits reached
		}

		// Retry stream creation with exponential backoff
		// For cluster mode, may need more retries to ensure quorum
		maxRetries := 10 // Increased for cluster quorum
		retryDelay := 3 * time.Second
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			// Try to add stream, if exists, update it
			// With 3 replicas, stream creation requires quorum (2/3 nodes)
			_, err := c.js.AddStream(cfg)
			if err == nil {
				log.Printf("[NATS] Stream %s ready (replicas: %d, retention: %v, maxBytes: %dMB, maxMsgs: %d)", 
					stream.name, cfg.Replicas, maxAge, cfg.MaxBytes/(1024*1024), cfg.MaxMsgs)
				lastErr = nil
				break
			} else if err == nats.ErrStreamNameAlreadyInUse {
				// Stream already exists - try to update to add new subjects (e.g. fortuna.insights.updated)
				_, updateErr := c.js.UpdateStream(cfg)
				if updateErr != nil {
					if strings.Contains(updateErr.Error(), "retention policy") {
						log.Printf("[NATS] Stream %s exists with different retention policy, using existing configuration", stream.name)
					} else {
						log.Printf("[NATS] Stream %s already exists; update (new subjects): %v", stream.name, updateErr)
					}
				} else {
					log.Printf("[NATS] Updated stream %s with subjects %v", stream.name, cfg.Subjects)
				}
				lastErr = nil
				break
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

// Conn returns the core NATS connection (for non-JetStream pub/sub, e.g. fortuna.insights.updated → Risk Center WS).
func (c *NATSClient) Conn() *nats.Conn {
	return c.conn
}
