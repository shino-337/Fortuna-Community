package poddetail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEventsCollectorIsolatesOwnershipRejection(t *testing.T) {
	acceptStale := false
	received := map[string]int{}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request struct {
			Events []K8sEventPayload `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, event := range request.Events {
			if event.EventUID == "stale" && !acceptStale {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":"pod_ownership_mismatch"}`))
				return
			}
		}
		for _, event := range request.Events {
			received[event.EventUID]++
		}
	}))
	defer server.Close()
	collector := NewEventsCollector(nil, server.URL, "cluster-a")
	collector.queue = []K8sEventPayload{{EventUID: "good-1"}, {EventUID: "stale"}, {EventUID: "good-2"}}
	collector.flush(context.Background())
	if received["good-1"] != 1 || received["good-2"] != 1 || received["stale"] != 0 {
		t.Fatalf("unexpected accepted events: %#v", received)
	}
	if len(collector.queue) != 0 || len(collector.quarantine) != 1 || collector.quarantine[0].EventUID != "stale" {
		t.Fatalf("ownership rejection must be quarantined without blocking valid events: queue=%v quarantine=%v", collector.queue, collector.quarantine)
	}
	beforeRetry := requests
	collector.flush(context.Background())
	if requests != beforeRetry {
		t.Fatalf("quarantined event was retried before the retry interval: requests=%d before=%d", requests, beforeRetry)
	}
	acceptStale = true
	collector.lastQuarantineRetry = time.Now().Add(-eventsQuarantineRetry)
	collector.flush(context.Background())
	if received["stale"] != 1 || len(collector.quarantine) != 0 {
		t.Fatalf("quarantined event was not retried after recovery: received=%#v quarantine=%v", received, collector.quarantine)
	}
}

func TestEventsCollectorDoesNotSplitOtherForbiddenErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"agent_cluster_mismatch"}`))
	}))
	defer server.Close()
	collector := NewEventsCollector(nil, server.URL, "cluster-a")
	collector.queue = []K8sEventPayload{{EventUID: "one"}, {EventUID: "two"}}
	collector.flush(context.Background())
	if requests != 1 || len(collector.queue) != 2 || len(collector.quarantine) != 0 {
		t.Fatalf("non-ownership forbidden response must leave batch intact: requests=%d queue=%v quarantine=%v", requests, collector.queue, collector.quarantine)
	}
}

func TestEventsCollectorDeduplicatesResyncedQuarantine(t *testing.T) {
	collector := NewEventsCollector(nil, "", "cluster-a")
	event := &corev1.Event{ObjectMeta: metav1.ObjectMeta{UID: "event-uid", Namespace: "ns", Name: "event"}}
	collector.enqueue(event)
	collector.enqueue(event)
	if len(collector.queue) != 1 {
		t.Fatalf("informer resync duplicated queued event: %v", collector.queue)
	}
	collector.quarantine = collector.queue
	collector.queue = nil
	collector.enqueue(event)
	if len(collector.queue) != 0 || len(collector.quarantine) != 1 {
		t.Fatalf("informer resync duplicated quarantined event: queue=%v quarantine=%v", collector.queue, collector.quarantine)
	}
}
