package poddetail

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/fortuna/agent/internal/corehttp"
)

const (
	eventsFlushInterval   = 30 * time.Second
	eventsBatchMax        = 200
	eventsQuarantineMax   = 400
	eventsQuarantineRetry = 5 * time.Minute
)

// K8sEventPayload matches Core K8sEvent ingest JSON.
type K8sEventPayload struct {
	EventUID       string     `json:"eventUid"`
	Namespace      string     `json:"namespace"`
	EventName      string     `json:"eventName"`
	InvolvedKind   string     `json:"involvedKind"`
	InvolvedUID    string     `json:"involvedUid"`
	InvolvedName   string     `json:"involvedName"`
	Reason         string     `json:"reason"`
	Message        string     `json:"message"`
	EventType      string     `json:"eventType"`
	Count          int        `json:"count"`
	FirstTimestamp *time.Time `json:"firstTimestamp,omitempty"`
	LastTimestamp  *time.Time `json:"lastTimestamp,omitempty"`
}

// EventsCollector watches K8s Events and POSTs batches to Core.
type EventsCollector struct {
	client              kubernetes.Interface
	coreBaseURL         string
	clusterID           string
	httpClient          *http.Client
	mu                  sync.Mutex
	queue               []K8sEventPayload
	quarantine          []K8sEventPayload
	lastQuarantineRetry time.Time
	nextDeliveryAttempt time.Time
}

// NewEventsCollector creates an EventsCollector.
func NewEventsCollector(client kubernetes.Interface, coreBaseURL, clusterID string) *EventsCollector {
	return &EventsCollector{
		client:      client,
		coreBaseURL: coreBaseURL,
		clusterID:   clusterID,
		httpClient:  &http.Client{Timeout: 20 * time.Second},
		queue:       make([]K8sEventPayload, 0, eventsBatchMax*2),
	}
}

// Start runs the informer and periodic flush until ctx is done.
func (e *EventsCollector) Start(ctx context.Context) {
	if e.coreBaseURL == "" {
		log.Printf("[PodDetail] Events collector: CORE_HTTP_ENDPOINT empty, skipping")
		return
	}
	factory := informers.NewSharedInformerFactoryWithOptions(e.client, 30*time.Second, informers.WithNamespace(metav1.NamespaceAll))
	informer := factory.Core().V1().Events().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.enqueue,
		UpdateFunc: func(_, newObj interface{}) { e.enqueue(newObj) },
	})
	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return
	}
	log.Printf("[PodDetail] Events collector started (flush every %v)", eventsFlushInterval)
	ticker := time.NewTicker(eventsFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			e.flush(ctx)
			return
		case <-ticker.C:
			e.flush(ctx)
		}
	}
}

func (e *EventsCollector) enqueue(obj interface{}) {
	ev, ok := obj.(*corev1.Event)
	if !ok {
		return
	}
	payload := eventToPayload(ev)
	e.mu.Lock()
	if payload.EventUID != "" {
		for i := range e.queue {
			if e.queue[i].EventUID == payload.EventUID {
				e.queue[i] = payload
				e.mu.Unlock()
				return
			}
		}
		for i := range e.quarantine {
			if e.quarantine[i].EventUID == payload.EventUID {
				e.quarantine[i] = payload
				e.mu.Unlock()
				return
			}
		}
	}
	e.queue = append(e.queue, payload)
	if len(e.queue) > eventsQuarantineMax {
		log.Printf("[PodDetail] events queue overflow: dropping %d oldest events", len(e.queue)-eventsQuarantineMax)
		e.queue = e.queue[len(e.queue)-eventsQuarantineMax:]
	}
	e.mu.Unlock()
}

func eventToPayload(ev *corev1.Event) K8sEventPayload {
	p := K8sEventPayload{
		EventUID:     string(ev.UID),
		Namespace:    ev.Namespace,
		EventName:    ev.Name,
		InvolvedKind: ev.InvolvedObject.Kind,
		InvolvedUID:  string(ev.InvolvedObject.UID),
		InvolvedName: ev.InvolvedObject.Name,
		Reason:       ev.Reason,
		Message:      ev.Message,
		EventType:    string(ev.Type),
		Count:        int(ev.Count),
	}
	if !ev.FirstTimestamp.IsZero() {
		t := ev.FirstTimestamp.Time
		p.FirstTimestamp = &t
	}
	if !ev.LastTimestamp.IsZero() {
		t := ev.LastTimestamp.Time
		p.LastTimestamp = &t
	}
	return p
}

func (e *EventsCollector) flush(ctx context.Context) {
	e.mu.Lock()
	now := time.Now()
	if now.Before(e.nextDeliveryAttempt) {
		e.mu.Unlock()
		return
	}
	batch := e.queue
	e.queue = make([]K8sEventPayload, 0, eventsBatchMax*2)
	var quarantine []K8sEventPayload
	if len(e.quarantine) > 0 && now.Sub(e.lastQuarantineRetry) >= eventsQuarantineRetry {
		quarantine = e.quarantine
		e.quarantine = nil
		e.lastQuarantineRetry = now
	}
	e.mu.Unlock()
	var retry, denied, deferredQuarantine []K8sEventPayload
	budget := &corehttp.DeliveryBudget{Remaining: corehttp.DeliveryRequestLimit, Now: now}
	// Isolate ownership-rejected events without letting one stale Pod block
	// unrelated events. Other failures remain retryable as an intact batch.
	for i := 0; i < len(batch); i += eventsBatchMax {
		end := i + eventsBatchMax
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[i:end]
		result := corehttp.DeliverIsolated(ctx, chunk, budget, "pod_ownership_mismatch", e.post)
		retry = append(retry, result.Retry...)
		denied = append(denied, result.Rejected...)
	}
	// Retry quarantine as a bounded batch, using only the remaining fresh-event
	// budget. A transient failure must not promote quarantine into the hot queue.
	for i := 0; i < len(quarantine); i += eventsBatchMax {
		end := min(i+eventsBatchMax, len(quarantine))
		result := corehttp.DeliverIsolated(ctx, quarantine[i:end], budget, "pod_ownership_mismatch", e.post)
		denied = append(denied, result.Rejected...)
		deferredQuarantine = append(deferredQuarantine, result.Retry...)
	}
	// Give deferred evidence the next turn before retrying records already
	// rejected in this flush, so a permanently stale prefix cannot starve it.
	denied = append(deferredQuarantine, denied...)
	e.mu.Lock()
	e.nextDeliveryAttempt = budget.RetryAt
	if len(quarantine) > 0 || (len(e.quarantine) == 0 && len(denied) > 0) {
		e.lastQuarantineRetry = time.Now()
	}
	for _, event := range denied {
		if event.EventUID != "" {
			duplicate := false
			for i := range e.quarantine {
				if e.quarantine[i].EventUID == event.EventUID {
					e.quarantine[i] = event
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
		}
		e.quarantine = append(e.quarantine, event)
	}
	// Informer updates can arrive while HTTP delivery has detached both queues.
	// Merge their newest version back into the right queue, never hot-requeue an
	// event just classified as quarantined or duplicate a transient retry.
	quarantined := make(map[string]int, len(e.quarantine))
	for i, event := range e.quarantine {
		if event.EventUID != "" {
			quarantined[event.EventUID] = i
		}
	}
	queue := make([]K8sEventPayload, 0, len(retry)+len(e.queue))
	positions := make(map[string]int)
	for _, events := range [][]K8sEventPayload{retry, e.queue} {
		for _, event := range events {
			if event.EventUID != "" {
				if i, ok := quarantined[event.EventUID]; ok {
					e.quarantine[i] = event
					continue
				}
				if i, ok := positions[event.EventUID]; ok {
					queue[i] = event
					continue
				}
				positions[event.EventUID] = len(queue)
			}
			queue = append(queue, event)
		}
	}
	e.queue = queue
	if len(e.queue) > eventsQuarantineMax {
		log.Printf("[PodDetail] events retry queue overflow: dropping %d oldest events", len(e.queue)-eventsQuarantineMax)
		e.queue = e.queue[len(e.queue)-eventsQuarantineMax:]
	}
	if len(e.quarantine) > eventsQuarantineMax {
		log.Printf("[PodDetail] events ownership quarantine overflow: dropping %d oldest events", len(e.quarantine)-eventsQuarantineMax)
		e.quarantine = e.quarantine[len(e.quarantine)-eventsQuarantineMax:]
	}
	e.mu.Unlock()
}

func (e *EventsCollector) post(ctx context.Context, events []K8sEventPayload) error {
	body := map[string]interface{}{
		"clusterId": e.clusterID,
		"events":    events,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := e.coreBaseURL + "/api/v1/agent/pod-events"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	corehttp.ApplyOptionalAuthorization(req)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := corehttp.DecodePostError(resp, time.Now())
		log.Printf("[PodDetail] events POST failed: %v", err)
		return err
	}
	return nil
}
