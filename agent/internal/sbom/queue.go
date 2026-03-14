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
	active     map[string]bool    // Track active pods to prevent duplicates
	retryCount map[string]int     // Per-pod send retry count (transient failures)
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
	}
}

// Queue returns the pod queue channel for use by watcher
func (q *WorkQueue) Queue() chan *corev1.Pod {
	return q.queue
}

// Enqueue adds a pod to the work queue
// Returns true if pod was queued, false if already queued/processing
// Note: This method is kept for backward compatibility, but the watcher
// now writes directly to the queue channel returned by Queue()
func (q *WorkQueue) Enqueue(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	// Create unique key for pod
	key := pod.Namespace + "/" + pod.Name

	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if pod is already queued or being processed
	if q.active[key] {
		q.logger.Printf("Pod %s already queued/processing, skipping", key)
		return false
	}

	// Mark as active
	q.active[key] = true

	// Try to enqueue (non-blocking)
	select {
	case q.queue <- pod:
		q.logger.Printf("✅ Queued pod %s for SBOM extraction", key)
		return true
	default:
		// Queue is full, remove from active and log warning
		delete(q.active, key)
		q.logger.Printf("⚠️  Queue full, dropping pod %s", key)
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

			key := pod.Namespace + "/" + pod.Name

			// Mark as active (if not already marked by Enqueue)
			q.mu.Lock()
			if !q.active[key] {
				q.active[key] = true
			}
			q.mu.Unlock()

			q.logger.Printf("[Worker %d] Processing pod %s", id, key)

			// Process pod (this is the slow SBOM extraction - 2-3 minutes)
			// This runs asynchronously, so it doesn't block the informer
			start := time.Now()
			err := q.processor.ProcessPod(q.ctx, pod)
			if err != nil {
				q.logger.Printf("[Worker %d] ⚠️  Failed to process pod %s: %v", id, key, err)
				// On transient send failure (e.g. Core restart), re-queue so we retry after Core is back
				if isTransientSendError(err) {
					q.mu.Lock()
					n := q.retryCount[key] + 1
					q.retryCount[key] = n
					q.mu.Unlock()
					if n <= maxSendRetries {
						q.logger.Printf("[Worker %d] 🔄 Re-queuing pod %s for retry %d/%d in %v (Core may have restarted)", id, key, n, maxSendRetries, sendRetryDelay)
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
									q.logger.Printf("[Worker] ⚠️  Queue full, gave up retry for pod %s", key)
								}
							}
						}(pod)
						continue // do not remove from active; pod will be processed again
					}
					q.mu.Lock()
					delete(q.retryCount, key)
					q.mu.Unlock()
					q.logger.Printf("[Worker %d] ⚠️  Gave up pod %s after %d send retries", id, key, maxSendRetries)
				}
			} else {
				duration := time.Since(start)
				q.logger.Printf("[Worker %d] ✅ Completed pod %s in %v", id, key, duration)
				q.mu.Lock()
				delete(q.retryCount, key)
				q.mu.Unlock()
			}

			// Remove from active set
			q.mu.Lock()
			delete(q.active, key)
			q.mu.Unlock()
		}
	}
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
