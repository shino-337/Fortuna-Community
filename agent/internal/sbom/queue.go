package sbom

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	maxSendRetries = 3
	sendRetryDelay = 30 * time.Second
)

// isTransientSendError returns true if the error indicates SendSBOMFinding failed due to
// Core unreachable (restart, network, DNS). Retrying later may succeed.
func isTransientSendError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "client not connected") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "unavailable") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "no such host") ||
		(strings.Contains(s, "relation \"sboms\" does not exist") || strings.Contains(s, "relation \"sbom_components\" does not exist")) ||
		strings.Contains(s, "dial tcp") && (strings.Contains(s, "i/o timeout") || strings.Contains(s, "refused"))
}

// WorkQueue manages a queue of pods to process for SBOM extraction
type WorkQueue struct {
	queue      chan *corev1.Pod
	workers    int
	processor  *Processor
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *log.Logger
	mu         sync.RWMutex
	active     map[string]bool        // Track active pods to prevent duplicates
	retryCount map[string]int         // Per-pod send retry count (transient failures)
	failedPods map[string]*corev1.Pod // Pods that exhausted retries; reconciliation re-queues them
	succeeded  map[string]bool        // Pods that completed successfully (skip during reconciliation)
}

// NewWorkQueue creates a new SBOM work queue
func NewWorkQueue(processor *Processor, workers int) *WorkQueue {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkQueue{
		queue:      make(chan *corev1.Pod, 30), // Buffer up to 30 pods (reduced from 100 to prevent memory buildup)
		workers:    workers,
		processor:  processor,
		ctx:        ctx,
		cancel:     cancel,
		logger:     log.New(log.Writer(), "[SBOMQueue] ", log.LstdFlags),
		active:     make(map[string]bool),
		retryCount: make(map[string]int),
		failedPods: make(map[string]*corev1.Pod),
		succeeded:  make(map[string]bool),
	}
}

// Queue returns the pod queue channel for use by watcher
func (q *WorkQueue) Queue() chan *corev1.Pod {
	return q.queue
}

// podKey returns the queue key for a pod (UID so recycled pods with same name get processed).
func podKey(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	return string(pod.UID)
}

// Enqueue adds a pod to the work queue
// Returns true if pod was queued, false if already queued/processing
// Key is pod UID so that recycled pods (same namespace/name, new UID) are processed.
func (q *WorkQueue) Enqueue(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	key := podKey(pod)
	label := pod.Namespace + "/" + pod.Name

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.active[key] {
		q.logger.Printf("Pod %s (uid=%s) already queued/processing, skipping", label, key)
		return false
	}

	q.active[key] = true

	select {
	case q.queue <- pod:
		q.logger.Printf("✅ Queued pod %s (uid=%s) for SBOM extraction", label, key)
		return true
	default:
		delete(q.active, key)
		q.logger.Printf("⚠️  Queue full, dropping pod %s (uid=%s)", label, key)
		return false
	}
}

// Start starts the worker pool
func (q *WorkQueue) Start() {
	q.logger.Printf("Starting SBOM work queue with %d workers", q.workers)

	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}

	q.logger.Printf("✅ SBOM work queue started")
}

// worker processes pods from the queue
func (q *WorkQueue) worker(id int) {
	defer q.wg.Done()

	q.logger.Printf("Worker %d started", id)

	for {
		select {
		case <-q.ctx.Done():
			q.logger.Printf("Worker %d stopping", id)
			return
		case pod := <-q.queue:
			if pod == nil {
				continue
			}

			key := podKey(pod)
			label := pod.Namespace + "/" + pod.Name

			q.mu.Lock()
			if !q.active[key] {
				q.active[key] = true
			}
			q.mu.Unlock()

			q.logger.Printf("[Worker %d] Processing pod %s (uid=%s)", id, label, key)

			// Process pod (this is the slow SBOM extraction - 2-3 minutes)
			// This runs asynchronously, so it doesn't block the informer
			start := time.Now()
			err := q.processor.ProcessPod(q.ctx, pod)
			if err != nil {
				q.logger.Printf("[Worker %d] ⚠️  Failed to process pod %s: %v", id, label, err)
				// On transient send failure (e.g. Core restart), re-queue so we retry after Core is back
				if isTransientSendError(err) {
					q.mu.Lock()
					n := q.retryCount[key] + 1
					q.retryCount[key] = n
					q.mu.Unlock()
					if n <= maxSendRetries {
						q.logger.Printf("[Worker %d] 🔄 Re-queuing pod %s (uid=%s) for retry %d/%d in %v (Core may have restarted)", id, label, key, n, maxSendRetries, sendRetryDelay)
						go func(p *corev1.Pod) {
							select {
							case <-q.ctx.Done():
								return
							case <-time.After(sendRetryDelay):
								select {
								case q.queue <- p:
									// re-queued
								default:
									q.mu.Lock()
									delete(q.retryCount, key)
									delete(q.active, key)
									q.mu.Unlock()
									q.logger.Printf("[Worker] ⚠️  Queue full, gave up retry for pod %s (uid=%s)", label, key)
								}
							}
						}(pod)
						continue // do not remove from active; pod will be processed again
					}
				q.mu.Lock()
				delete(q.retryCount, key)
				q.failedPods[key] = pod
				q.mu.Unlock()
				q.logger.Printf("[Worker %d] ⚠️  Gave up pod %s (uid=%s) after %d send retries; will retry on reconciliation", id, label, key, maxSendRetries)
			}
		} else {
			duration := time.Since(start)
			q.logger.Printf("[Worker %d] ✅ Completed pod %s (uid=%s) in %v", id, label, key, duration)
			q.mu.Lock()
			delete(q.retryCount, key)
			delete(q.failedPods, key)
			q.succeeded[key] = true
			q.mu.Unlock()
		}

			// Remove from active set
			q.mu.Lock()
			delete(q.active, key)
			q.mu.Unlock()
		}
	}
}

// StartReconciliation runs a periodic loop that re-queues pods whose SBOM send
// failed after exhausting retries (e.g. Core was down for extended period).
// Interval controls how often the reconciliation runs (default: 10 minutes).
func (q *WorkQueue) StartReconciliation(interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-q.ctx.Done():
				return
			case <-ticker.C:
				q.reconcileFailedPods()
			}
		}
	}()
	q.logger.Printf("✅ SBOM reconciliation started (interval=%v)", interval)
}

func (q *WorkQueue) reconcileFailedPods() {
	q.mu.Lock()
	if len(q.failedPods) == 0 {
		q.mu.Unlock()
		return
	}
	// Snapshot failed pods and clear the map; they'll be re-added if they fail again.
	toRetry := make([]*corev1.Pod, 0, len(q.failedPods))
	for key, pod := range q.failedPods {
		delete(q.failedPods, key)
		delete(q.retryCount, key)
		toRetry = append(toRetry, pod)
	}
	q.mu.Unlock()

	q.logger.Printf("🔄 Reconciliation: re-queuing %d previously failed pods", len(toRetry))
	queued := 0
	for _, pod := range toRetry {
		if q.Enqueue(pod) {
			queued++
		}
	}
	q.logger.Printf("🔄 Reconciliation: queued %d/%d pods", queued, len(toRetry))
}

// Stop stops the work queue and waits for workers to finish
func (q *WorkQueue) Stop() {
	q.logger.Printf("Stopping SBOM work queue...")
	q.cancel()
	// Don't close queue here - it may still be used by watcher
	// Workers will stop when ctx is cancelled
	q.wg.Wait()
	q.logger.Printf("✅ SBOM work queue stopped")
}

// QueueSize returns the current queue size
func (q *WorkQueue) QueueSize() int {
	return len(q.queue)
}

// ActiveCount returns the number of active (queued or processing) pods
func (q *WorkQueue) ActiveCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.active)
}
