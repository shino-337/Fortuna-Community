package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

// InventoryItem represents a Kubernetes resource inventory item
type InventoryItem struct {
	Kind      string            `json:"kind"`
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
	RawJSON   string            `json:"raw_json"`
	Timestamp int64             `json:"timestamp"`
}

func main() {
	// Get NATS endpoint from environment or use default
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats:4222"
	}

	// Connect to NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	// Get JetStream context
	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Create streams if they don't exist
	createStreams(js)

	// Generate test messages
	messages := generateTestMessages()

	// Publish messages
	log.Printf("Publishing %d test messages...", len(messages))
	for i, msg := range messages {
		subject := fmt.Sprintf("ksam.raw.%s", getSubjectType(msg.Kind))
		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Failed to marshal message %d: %v", i, err)
			continue
		}

		_, err = js.Publish(subject, data)
		if err != nil {
			log.Printf("Failed to publish message %d to %s: %v", i, subject, err)
			continue
		}

		log.Printf("[%d/%d] Published %s: %s/%s to %s", i+1, len(messages), msg.Kind, msg.Namespace, msg.Name, subject)
		time.Sleep(100 * time.Millisecond) // Small delay between messages
	}

	log.Printf("✅ Successfully published %d test messages", len(messages))
}

func createStreams(js nats.JetStreamContext) {
	// Create ksam-raw stream
	_, err := js.AddStream(&nats.StreamConfig{
		Name:      "ksam-raw",
		Subjects:  []string{"ksam.raw.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    1 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil && err.Error() != "stream name already in use" {
		log.Printf("Warning: Failed to create ksam-raw stream: %v", err)
	} else {
		log.Printf("✅ ksam-raw stream ready")
	}

	// Create ksam-normalized stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      "ksam-normalized",
		Subjects:  []string{"ksam.normalized.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil && err.Error() != "stream name already in use" {
		log.Printf("Warning: Failed to create ksam-normalized stream: %v", err)
	} else {
		log.Printf("✅ ksam-normalized stream ready")
	}
}

func getSubjectType(kind string) string {
	switch kind {
	case "Pod":
		return "pods"
	case "ServiceAccount":
		return "serviceaccounts"
	case "Role":
		return "roles"
	case "ClusterRole":
		return "roles"
	case "RoleBinding":
		return "rolebindings"
	case "ClusterRoleBinding":
		return "rolebindings"
	default:
		return "unknown"
	}
}

func generateTestMessages() []InventoryItem {
	now := time.Now().Unix()
	messages := []InventoryItem{}

	// Test Pods
	for i := 0; i < 10; i++ {
		messages = append(messages, InventoryItem{
			Kind:      "Pod",
			UID:       fmt.Sprintf("pod-uid-%d", i),
			Name:      fmt.Sprintf("test-pod-%d", i),
			Namespace: "default",
			Labels: map[string]string{
				"app":     "test",
				"cluster":  "test-cluster",
				"version": "v1",
			},
			RawJSON:   fmt.Sprintf(`{"kind":"Pod","metadata":{"name":"test-pod-%d","namespace":"default","uid":"pod-uid-%d"}}`, i, i),
			Timestamp: now,
		})
	}

	// Test ServiceAccounts
	for i := 0; i < 5; i++ {
		messages = append(messages, InventoryItem{
			Kind:      "ServiceAccount",
			UID:       fmt.Sprintf("sa-uid-%d", i),
			Name:      fmt.Sprintf("test-sa-%d", i),
			Namespace: "default",
			Labels: map[string]string{
				"app":    "test",
				"cluster": "test-cluster",
			},
			RawJSON:   fmt.Sprintf(`{"kind":"ServiceAccount","metadata":{"name":"test-sa-%d","namespace":"default","uid":"sa-uid-%d"}}`, i, i),
			Timestamp: now,
		})
	}

	// Test Roles
	for i := 0; i < 3; i++ {
		messages = append(messages, InventoryItem{
			Kind:      "Role",
			UID:       fmt.Sprintf("role-uid-%d", i),
			Name:      fmt.Sprintf("test-role-%d", i),
			Namespace: "default",
			Labels: map[string]string{
				"cluster": "test-cluster",
			},
			RawJSON:   fmt.Sprintf(`{"kind":"Role","metadata":{"name":"test-role-%d","namespace":"default","uid":"role-uid-%d"}}`, i, i),
			Timestamp: now,
		})
	}

	// Test RoleBindings
	for i := 0; i < 3; i++ {
		messages = append(messages, InventoryItem{
			Kind:      "RoleBinding",
			UID:       fmt.Sprintf("rb-uid-%d", i),
			Name:      fmt.Sprintf("test-rb-%d", i),
			Namespace: "default",
			Labels: map[string]string{
				"cluster": "test-cluster",
			},
			RawJSON:   fmt.Sprintf(`{"kind":"RoleBinding","metadata":{"name":"test-rb-%d","namespace":"default","uid":"rb-uid-%d"}}`, i, i),
			Timestamp: now,
		})
	}

	// Test ClusterRole (for backpressure test - high volume)
	for i := 0; i < 50; i++ {
		messages = append(messages, InventoryItem{
			Kind:      "ClusterRole",
			UID:       fmt.Sprintf("cr-uid-%d", i),
			Name:      fmt.Sprintf("test-clusterrole-%d", i),
			Namespace: "",
			Labels: map[string]string{
				"cluster": "test-cluster",
			},
			RawJSON:   fmt.Sprintf(`{"kind":"ClusterRole","metadata":{"name":"test-clusterrole-%d","uid":"cr-uid-%d"}}`, i, i),
			Timestamp: now,
		})
	}

	return messages
}

