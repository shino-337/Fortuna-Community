package poddetail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	eventsFlushInterval = 30 * time.Second
	eventsBatchMax      = 200
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
	client      kubernetes.Interface
	coreBaseURL string
	clusterID   string
	httpClient  *http.Client
	mu          sync.Mutex
	queue       []K8sEventPayload
}

// NewEventsCollector creates an EventsCollector.
func NewEventsCollector(client kubernetes.Interface, coreBaseURL, clusterID string) *EventsCollector {
	return &EventsCollector{
		client:      client,
		coreBaseURL: coreBaseURL,
		clusterID:   clusterID,
		httpClient:   &http.Client{Timeout: 20 * time.Second},
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
	e.queue = append(e.queue, payload)
	if len(e.queue) > eventsBatchMax*2 {
		e.queue = e.queue[len(e.queue)-eventsBatchMax : len(e.queue)]
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
	batch := e.queue
	e.queue = make([]K8sEventPayload, 0, eventsBatchMax*2)
	e.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	// Send in chunks to avoid huge payload
	for i := 0; i < len(batch); i += eventsBatchMax {
		end := i + eventsBatchMax
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[i:end]
		if err := e.post(ctx, chunk); err != nil {
			log.Printf("[PodDetail] events POST failed: %v (re-queuing %d events)", err, len(chunk))
			e.mu.Lock()
			e.queue = append(chunk, e.queue...)
			if len(e.queue) > eventsBatchMax*2 {
				e.queue = e.queue[:eventsBatchMax*2]
			}
			e.mu.Unlock()
			return
		}
	}
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
		return fmt.Errorf("POST %s: %d", url, resp.StatusCode)
	}
	return nil
}
