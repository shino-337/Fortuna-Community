package worker

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ksam/core/pkg/metrics"
)

// BackpressureConfig configures backpressure behavior
type BackpressureConfig struct {
	MaxConcurrent    int     // Maximum concurrent messages per worker type (default: 100)
	Threshold        float64 // Backpressure threshold (0.8 = trigger at 80% capacity)
	CheckInterval    time.Duration // How often to check backpressure (default: 1s)
	BackpressureCh   chan struct{}  // Channel to signal backpressure events
}

// DefaultBackpressureConfig returns default backpressure configuration
func DefaultBackpressureConfig() BackpressureConfig {
	return BackpressureConfig{
		MaxConcurrent:  100,
		Threshold:     0.8, // Trigger backpressure at 80% capacity
		CheckInterval: 1 * time.Second,
		BackpressureCh: make(chan struct{}, 10), // Buffered channel for signals
	}
}

// WorkerLoadTracker tracks load for a worker type
type WorkerLoadTracker struct {
	currentLoad int32 // Atomic counter for current concurrent processing
	maxConcurrent int32 // Maximum concurrent processing
	workerType   string
	backpressureCh chan struct{}
	backpressureStartTime *int64 // Unix timestamp when backpressure started (atomic)
}

// NewWorkerLoadTracker creates a new load tracker
func NewWorkerLoadTracker(workerType string, maxConcurrent int) *WorkerLoadTracker {
	return &WorkerLoadTracker{
		currentLoad:           0,
		maxConcurrent:         int32(maxConcurrent),
		workerType:            workerType,
		backpressureCh:        make(chan struct{}, 10),
		backpressureStartTime: new(int64),
	}
}

// TryAcquire attempts to acquire a processing slot
// Returns true if acquired, false if backpressure should be applied
func (t *WorkerLoadTracker) TryAcquire() bool {
	current := atomic.LoadInt32(&t.currentLoad)
	max := atomic.LoadInt32(&t.maxConcurrent)
	
	// Check if we're at capacity
	if current >= max {
		// Check if we just entered backpressure
		if atomic.CompareAndSwapInt64(t.backpressureStartTime, 0, time.Now().Unix()) {
			// Signal backpressure event
			select {
			case t.backpressureCh <- struct{}{}:
			default:
			}
			metrics.WorkerBackpressureTotal.WithLabelValues(t.workerType).Inc()
			log.Printf("[Backpressure] Worker %s entered backpressure (load: %d/%d)", 
				t.workerType, current, max)
		}
		return false
	}
	
	// Acquire slot
	atomic.AddInt32(&t.currentLoad, 1)
	
	// Update metrics
	metrics.WorkerConcurrentProcessing.WithLabelValues(t.workerType).Set(float64(atomic.LoadInt32(&t.currentLoad)))
	
	// Clear backpressure start time if we're below threshold
	threshold := int32(float64(max) * 0.8) // 80% threshold
	if current < threshold {
		if startTime := atomic.SwapInt64(t.backpressureStartTime, 0); startTime > 0 {
			// Backpressure cleared
			duration := time.Since(time.Unix(startTime, 0))
			metrics.WorkerBackpressureDuration.WithLabelValues(t.workerType).Observe(duration.Seconds())
			log.Printf("[Backpressure] Worker %s cleared backpressure after %v (load: %d/%d)", 
				t.workerType, duration, current, max)
		}
	}
	
	return true
}

// Release releases a processing slot
func (t *WorkerLoadTracker) Release() {
	current := atomic.AddInt32(&t.currentLoad, -1)
	
	// Update metrics
	metrics.WorkerConcurrentProcessing.WithLabelValues(t.workerType).Set(float64(current))
	
	// Clear backpressure if we're below threshold
	max := atomic.LoadInt32(&t.maxConcurrent)
	threshold := int32(float64(max) * 0.8)
	if current < threshold {
		if startTime := atomic.SwapInt64(t.backpressureStartTime, 0); startTime > 0 {
			// Backpressure cleared
			duration := time.Since(time.Unix(startTime, 0))
			metrics.WorkerBackpressureDuration.WithLabelValues(t.workerType).Observe(duration.Seconds())
			log.Printf("[Backpressure] Worker %s cleared backpressure after %v (load: %d/%d)", 
				t.workerType, duration, current, max)
		}
	}
}

// GetCurrentLoad returns current load
func (t *WorkerLoadTracker) GetCurrentLoad() int32 {
	return atomic.LoadInt32(&t.currentLoad)
}

// GetMaxConcurrent returns max concurrent
func (t *WorkerLoadTracker) GetMaxConcurrent() int32 {
	return atomic.LoadInt32(&t.maxConcurrent)
}

// IsBackpressured checks if worker is currently under backpressure
func (t *WorkerLoadTracker) IsBackpressured() bool {
	current := atomic.LoadInt32(&t.currentLoad)
	max := atomic.LoadInt32(&t.maxConcurrent)
	return current >= max
}

// ApplyBackpressure applies backpressure to a message by NAKing it
func ApplyBackpressure(ctx context.Context, msg *nats.Msg, workerType string, tracker *WorkerLoadTracker) {
	// NAK the message for redelivery later
	// NATS will redeliver with exponential backoff
	if err := msg.Nak(); err != nil {
		log.Printf("[Backpressure] Failed to NAK message for worker %s: %v", workerType, err)
	} else {
		log.Printf("[Backpressure] NAKed message for worker %s (load: %d/%d)", 
			workerType, tracker.GetCurrentLoad(), tracker.GetMaxConcurrent())
	}
	
	// Update metrics
	metrics.WorkerMessagesProcessedTotal.WithLabelValues(workerType, "backpressure").Inc()
}

