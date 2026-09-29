package poddetail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
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

func TestEventsCollectorRateLimitStopsFlushAndPreservesQuarantine(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	collector := NewEventsCollector(nil, server.URL, "cluster-a")
	for i := 0; i < 300; i++ {
		collector.queue = append(collector.queue, K8sEventPayload{EventUID: fmt.Sprint(i)})
	}
	collector.quarantine = []K8sEventPayload{{EventUID: "stale"}}
	collector.flush(t.Context())
	if requests != 1 || len(collector.queue) != 300 || len(collector.quarantine) != 1 || time.Until(collector.nextDeliveryAttempt) < 110*time.Second {
		t.Fatalf("429 did not stop flush/preserve queues: requests=%d queue=%d quarantine=%v retry=%s", requests, len(collector.queue), collector.quarantine, collector.nextDeliveryAttempt)
	}
	collector.flush(t.Context())
	if requests != 1 {
		t.Fatalf("backoff ignored: %d requests", requests)
	}
}

func TestEventsCollectorQuarantineRetryHasSharedRequestBudget(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"pod_ownership_mismatch"}`))
	}))
	defer server.Close()
	collector := NewEventsCollector(nil, server.URL, "cluster-a")
	for i := 0; i < eventsQuarantineMax; i++ {
		collector.quarantine = append(collector.quarantine, K8sEventPayload{EventUID: fmt.Sprint(i)})
	}
	collector.flush(t.Context())
	if requests > corehttp.DeliveryRequestLimit || len(collector.queue) != 0 || len(collector.quarantine) != eventsQuarantineMax {
		t.Fatalf("quarantine retry burst/loss: requests=%d hot=%d quarantine=%d", requests, len(collector.queue), len(collector.quarantine))
	}
}

func TestEventsCollectorQuarantineBudgetDoesNotStarveRecoveredEvents(t *testing.T) {
	requests, recovered := 0, 0
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
			if event.EventUID != "recovered" {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":"pod_ownership_mismatch"}`))
				return
			}
		}
		recovered += len(request.Events)
	}))
	defer server.Close()
	collector := NewEventsCollector(nil, server.URL, "cluster-a")
	for i := 0; i < 32; i++ {
		collector.quarantine = append(collector.quarantine, K8sEventPayload{EventUID: fmt.Sprint(i)})
	}
	collector.quarantine = append(collector.quarantine, K8sEventPayload{EventUID: "recovered"})
	for attempt := 0; attempt < 33 && recovered == 0; attempt++ {
		collector.lastQuarantineRetry = time.Now().Add(-eventsQuarantineRetry)
		before := requests
		collector.flush(t.Context())
		if requests-before > corehttp.DeliveryRequestLimit || len(collector.queue) != 0 || len(collector.quarantine)+recovered != 33 {
			t.Fatalf("unbounded retry or lost evidence: requests=%d hot=%d quarantine=%d recovered=%d", requests-before, len(collector.queue), len(collector.quarantine), recovered)
		}
	}
	if recovered != 1 || len(collector.quarantine) != 32 {
		t.Fatalf("permanently rejected prefix starved recovered event: recovered=%d quarantine=%d", recovered, len(collector.quarantine))
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

func TestEventsCollectorResyncDuringDeliveryPreservesQueueClassAndNewestVersion(t *testing.T) {
	for _, status := range []int{403, 429} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var collector *EventsCollector
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				collector.enqueue(&corev1.Event{ObjectMeta: metav1.ObjectMeta{UID: "in-flight"}, Count: 2})
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"code":"pod_ownership_mismatch"}`))
			}))
			defer server.Close()
			collector = NewEventsCollector(nil, server.URL, "cluster-a")
			collector.queue = []K8sEventPayload{{EventUID: "in-flight", Count: 1}}
			collector.flush(t.Context())
			if status == 403 {
				if len(collector.queue) != 0 || len(collector.quarantine) != 1 || collector.quarantine[0].Count != 2 {
					t.Fatalf("resync bypassed quarantine/lost newest version: queue=%v quarantine=%v", collector.queue, collector.quarantine)
				}
			} else if len(collector.queue) != 1 || collector.queue[0].Count != 2 || len(collector.quarantine) != 0 {
				t.Fatalf("resync duplicated retry/lost newest version: queue=%v quarantine=%v", collector.queue, collector.quarantine)
			}
			collector.flush(t.Context())
			if requests != 1 {
				t.Fatalf("resync bypassed retry cadence: %d", requests)
			}
		})
	}
}
