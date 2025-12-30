package messaging

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

// Publisher publishes messages to NATS streams
type Publisher struct {
	js nats.JetStreamContext
}

// NewPublisher creates a new publisher
func NewPublisher(client *NATSClient) *Publisher {
	js := client.JetStream()
	return &Publisher{js: js}
}

// PublishInventory publishes an inventory item to the appropriate stream
// Uses fortuna.raw.* subject pattern as per Architecture Review Issue #2
func (p *Publisher) PublishInventory(itemType string, data interface{}) error {
	subject := fmt.Sprintf("fortuna.raw.%s", itemType)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal inventory item: %w", err)
	}

	if _, err := p.js.Publish(subject, jsonData); err != nil {
		return fmt.Errorf("failed to publish inventory item: %w", err)
	}

	log.Printf("[Publisher] Published %s to %s", itemType, subject)
	return nil
}

// PublishEvent publishes a runtime event
func (p *Publisher) PublishEvent(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if _, err := p.js.Publish("fortuna.events.runtime", jsonData); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("[Publisher] Published event to fortuna.events.runtime")
	return nil
}

// PublishInsight publishes an insight
func (p *Publisher) PublishInsight(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal insight: %w", err)
	}

	if _, err := p.js.Publish("fortuna.insights.created", jsonData); err != nil {
		return fmt.Errorf("failed to publish insight: %w", err)
	}

	log.Printf("[Publisher] Published insight to fortuna.insights.created")
	return nil
}

// PublishSBOMCreated publishes an SBOM created event (event-driven SBOM→CVE pipeline)
func (p *Publisher) PublishSBOMCreated(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal sbom event: %w", err)
	}

	if _, err := p.js.Publish("fortuna.sbom.created", jsonData); err != nil {
		return fmt.Errorf("failed to publish sbom.created: %w", err)
	}

	log.Printf("[Publisher] Published event to fortuna.sbom.created")
	return nil
}
