package messaging

import (
	"fmt"
	"log"
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
		nats.Name("ksam-core"),
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

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to get JetStream context: %w", err)
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
			name:     "ksam-raw",
			subjects: []string{"ksam.raw.pods", "ksam.raw.serviceaccounts", "ksam.raw.roles", "ksam.raw.rolebindings"},
		},
		{
			name:     "ksam-events",
			subjects: []string{"ksam.events.runtime"},
		},
		{
			name:     "ksam-insights",
			subjects: []string{"ksam.insights.created"},
		},
		{
			name:     "ksam-normalized",
			subjects: []string{"ksam.normalized.>"},
		},
	}

	for _, stream := range streams {
		cfg := &nats.StreamConfig{
			Name:      stream.name,
			Subjects:  stream.subjects,
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour, // 7 days
			Storage:   nats.FileStorage,
			Replicas:  3,
		}

		_, err := c.js.AddStream(cfg)
		if err != nil && err != nats.ErrStreamNameAlreadyInUse {
			return fmt.Errorf("failed to create stream %s: %w", stream.name, err)
		}
		log.Printf("[NATS] Stream %s ready", stream.name)
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
	sub, err := c.js.Subscribe(subject, handler, nats.Durable("ksam-worker"))
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
