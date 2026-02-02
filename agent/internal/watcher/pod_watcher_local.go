package watcher

import (
	"context"
	"log"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// PodHandler processes pod events
type PodHandler func(ctx context.Context, pod *corev1.Pod) error

// LocalPodWatcher watches pods on the local node only
type LocalPodWatcher struct {
	clientset     *kubernetes.Clientset
	nodeName      string
	handler       PodHandler
	queue         chan *corev1.Pod // Queue for async processing (optional)
	informer      cache.SharedIndexInformer
	stopCh        chan struct{}
	logger        *log.Logger
	processedPods map[string]time.Time // Track processed pod UIDs with timestamp to prevent duplicates and enable cleanup
	mu            sync.RWMutex          // Mutex for processedPods map access
}

// NewLocalPodWatcher creates a new local pod watcher
// If queue is provided, pods will be queued for async processing instead of calling handler directly
func NewLocalPodWatcher(clientset *kubernetes.Clientset, nodeName string, handler PodHandler, queue chan *corev1.Pod) *LocalPodWatcher {
	watcher := &LocalPodWatcher{
		clientset:     clientset,
		nodeName:      nodeName,
		handler:       handler,
		queue:         queue,
		stopCh:        make(chan struct{}),
		logger:        log.New(log.Writer(), "[LocalPodWatcher] ", log.LstdFlags),
		processedPods: make(map[string]time.Time),
	}
	
	return watcher
}

// Start starts the pod watcher
func (w *LocalPodWatcher) Start(ctx context.Context) error {
	w.logger.Printf("Starting pod watcher for node: %s", w.nodeName)

	// Create field selector to watch only pods on this node
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", w.nodeName).String()

	// Create informer factory with field selector
	// Reduced resync period from 30s to 5s for faster pod detection
	// Watch all namespaces using metav1.NamespaceAll ("")
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientset,
		5*time.Second, // Resync period (reduced from 30s)
		informers.WithNamespace(metav1.NamespaceAll),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fieldSelector
		}),
	)

	// Get pod informer
	w.informer = factory.Core().V1().Pods().Informer()

	// Add event handlers
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			// Log all pod add events for debugging
			w.logger.Printf("🆕 Pod added: %s/%s (phase: %s, node: %s, containers: %d)",
				pod.Namespace, pod.Name, pod.Status.Phase, pod.Spec.NodeName, len(pod.Spec.Containers))

			// Process all pods (handler will decide if SBOM extraction is needed)
			// This ensures we don't miss pods that are already Running when added
			if len(pod.Spec.Containers) > 0 {
				// Process immediately if Running, otherwise UpdateFunc will catch it
				if pod.Status.Phase == corev1.PodRunning {
					// Check if already processed to prevent duplicates
					podUID := string(pod.UID)
					w.mu.RLock()
					_, alreadyProcessed := w.processedPods[podUID]
					w.mu.RUnlock()
					if alreadyProcessed {
						w.logger.Printf("   → Skipping pod %s/%s (already processed)", pod.Namespace, pod.Name)
						return
					}

					// If queue is available, enqueue for async processing
					// This prevents blocking the informer during slow SBOM extraction (2-3 min per pod)
					if w.queue != nil {
						select {
						case w.queue <- pod:
							w.logger.Printf("   → Queued pod %s/%s for async processing", pod.Namespace, pod.Name)
							w.mu.Lock()
							w.processedPods[podUID] = time.Now() // Mark as processed with timestamp
							w.mu.Unlock()
						default:
							w.logger.Printf("   ⚠️  Queue full, processing pod %s/%s synchronously", pod.Namespace, pod.Name)
							// Fallback to synchronous processing if queue is full
							if err := w.handler(ctx, pod); err != nil {
								w.logger.Printf("⚠️  Handler error for pod %s/%s: %v", pod.Namespace, pod.Name, err)
							} else {
								w.mu.Lock()
								w.processedPods[podUID] = time.Now()
								w.mu.Unlock()
							}
						}
					} else {
						// No queue, process synchronously (backward compatibility)
						w.logger.Printf("   → Processing pod %s/%s synchronously", pod.Namespace, pod.Name)
						if err := w.handler(ctx, pod); err != nil {
							w.logger.Printf("⚠️  Handler error for pod %s/%s: %v", pod.Namespace, pod.Name, err)
						} else {
							w.mu.Lock()
							w.processedPods[podUID] = time.Now()
							w.mu.Unlock()
						}
					}
				} else {
					w.logger.Printf("   → Waiting for pod %s/%s to reach Running state (current: %s)",
						pod.Namespace, pod.Name, pod.Status.Phase)
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)

			// Log all significant pod updates for debugging
			if oldPod.Status.Phase != newPod.Status.Phase {
				w.logger.Printf("🔄 Pod phase changed: %s/%s (%s → %s)",
					newPod.Namespace, newPod.Name, oldPod.Status.Phase, newPod.Status.Phase)
			}

			// Process any pod that is now in Running state (not just transitions)
			// This catches pods that may have been missed during AddFunc
			if newPod.Status.Phase == corev1.PodRunning && len(newPod.Spec.Containers) > 0 {
			// Check if already processed to prevent duplicates
			podUID := string(newPod.UID)
			w.mu.RLock()
			_, alreadyProcessed := w.processedPods[podUID]
			w.mu.RUnlock()
			if alreadyProcessed {
				// Already processed, skip (prevents duplicate SBOM extractions during resyncs)
				return
			}

				// Only process if this is a new transition to Running
				if oldPod.Status.Phase != corev1.PodRunning {
					// If queue is available, enqueue for async processing
					if w.queue != nil {
						select {
						case w.queue <- newPod:
							w.logger.Printf("   → Queued pod %s/%s for async processing (transitioned to Running)", newPod.Namespace, newPod.Name)
							w.mu.Lock()
							w.processedPods[podUID] = time.Now() // Mark as processed with timestamp
							w.mu.Unlock()
						default:
							w.logger.Printf("   ⚠️  Queue full, processing pod %s/%s synchronously", newPod.Namespace, newPod.Name)
							// Fallback to synchronous processing if queue is full
							if err := w.handler(ctx, newPod); err != nil {
								w.logger.Printf("⚠️  Handler error for pod %s/%s: %v", newPod.Namespace, newPod.Name, err)
							} else {
								w.mu.Lock()
								w.processedPods[podUID] = time.Now()
								w.mu.Unlock()
							}
						}
					} else {
						// No queue, process synchronously (backward compatibility)
						w.logger.Printf("   → Processing pod %s/%s synchronously (transitioned to Running)", newPod.Namespace, newPod.Name)
						if err := w.handler(ctx, newPod); err != nil {
							w.logger.Printf("⚠️  Handler error for pod %s/%s: %v", newPod.Namespace, newPod.Name, err)
						} else {
							w.mu.Lock()
							w.processedPods[podUID] = time.Now()
							w.mu.Unlock()
						}
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			w.logger.Printf("🗑️  Pod deleted: %s/%s", pod.Namespace, pod.Name)
			// Clean up processed pods map to prevent memory leaks
			podUID := string(pod.UID)
			w.mu.Lock()
			delete(w.processedPods, podUID)
			w.mu.Unlock()
		},
	})

	// Start informer
	go w.informer.Run(w.stopCh)

	// Wait for cache sync
	w.logger.Printf("Waiting for cache sync...")
	if !cache.WaitForCacheSync(ctx.Done(), w.informer.HasSynced) {
		return ctx.Err()
	}

	w.logger.Printf("✅ Cache synced, watching pods on node %s", w.nodeName)

	// Explicitly process any pods that may have been added during cache sync
	// This ensures we don't miss pods that became Running while informer was starting
	w.logger.Printf("📋 Checking for pods that may have been missed during startup...")
	items := w.informer.GetStore().List()
	processedCount := 0
	for _, item := range items {
		pod := item.(*corev1.Pod)
		if pod.Status.Phase == corev1.PodRunning && len(pod.Spec.Containers) > 0 {
			podUID := string(pod.UID)
			w.mu.RLock()
			_, alreadyProcessed := w.processedPods[podUID]
			w.mu.RUnlock()
			if alreadyProcessed {
				w.logger.Printf("   → Skipping pod %s/%s (already processed)", pod.Namespace, pod.Name)
				continue
			}

			// If queue is available, enqueue for async processing
			if w.queue != nil {
				select {
				case w.queue <- pod:
					w.logger.Printf("   → Queued pod %s/%s for async processing", pod.Namespace, pod.Name)
					w.mu.Lock()
					w.processedPods[podUID] = time.Now()
					w.mu.Unlock()
					processedCount++
				default:
					w.logger.Printf("   ⚠️  Queue full, processing pod %s/%s synchronously", pod.Namespace, pod.Name)
					if err := w.handler(ctx, pod); err != nil {
						w.logger.Printf("⚠️  Handler error for pod %s/%s: %v", pod.Namespace, pod.Name, err)
					} else {
						w.mu.Lock()
						w.processedPods[podUID] = time.Now()
						w.mu.Unlock()
						processedCount++
					}
				}
			} else {
				// No queue, process synchronously (backward compatibility)
				w.logger.Printf("   → Found Running pod: %s/%s (processing synchronously...)", pod.Namespace, pod.Name)
				if err := w.handler(ctx, pod); err != nil {
					w.logger.Printf("⚠️  Handler error for pod %s/%s: %v", pod.Namespace, pod.Name, err)
				} else {
					w.mu.Lock()
					w.processedPods[podUID] = time.Now()
					w.mu.Unlock()
					processedCount++
				}
			}
		}
	}
	w.logger.Printf("✅ Startup check complete: processed %d pods", processedCount)

	return nil
}

// cleanupProcessedPods periodically cleans up old entries from processedPods map
// to prevent memory leaks when pods are not deleted (e.g., long-running pods)
func (w *LocalPodWatcher) cleanupProcessedPods(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour) // Run cleanup every hour
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.mu.Lock()
			now := time.Now()
			cleaned := 0
			for podUID, processedTime := range w.processedPods {
				// Remove entries older than 24 hours
				if now.Sub(processedTime) > 24*time.Hour {
					delete(w.processedPods, podUID)
					cleaned++
				}
			}
			w.mu.Unlock()
			if cleaned > 0 {
				w.logger.Printf("🧹 Cleaned up %d old entries from processedPods map", cleaned)
			}
		}
	}
}

// Stop stops the watcher
func (w *LocalPodWatcher) Stop() {
	w.logger.Printf("Stopping pod watcher")
	close(w.stopCh)
}

// ListCurrentPods lists all current pods on the node (for initial sync)
func (w *LocalPodWatcher) ListCurrentPods(ctx context.Context) ([]*corev1.Pod, error) {
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", w.nodeName).String()

	podList, err := w.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	pods := make([]*corev1.Pod, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		// Only include running pods with containers
		if pod.Status.Phase == corev1.PodRunning && len(pod.Spec.Containers) > 0 {
			pods = append(pods, pod)
		}
	}

	w.logger.Printf("Found %d running pods on node %s", len(pods), w.nodeName)

	// Queue all existing pods for async processing instead of processing synchronously
	// This prevents blocking during initial sync when there are many pods
	if w.queue != nil {
		queued := 0
		for _, pod := range pods {
			podUID := string(pod.UID)
			// Skip if already processed
			w.mu.RLock()
			_, alreadyProcessed := w.processedPods[podUID]
			w.mu.RUnlock()
			if alreadyProcessed {
				w.logger.Printf("   → Skipping pod %s/%s (already processed)", pod.Namespace, pod.Name)
				continue
			}

			select {
			case w.queue <- pod:
				w.mu.Lock()
				w.processedPods[podUID] = time.Now() // Mark as processed with timestamp
				w.mu.Unlock()
				queued++
			default:
				w.logger.Printf("⚠️  Queue full during initial sync, skipping pod %s/%s", pod.Namespace, pod.Name)
			}
		}
		w.logger.Printf("✅ Queued %d/%d existing pods for async processing", queued, len(pods))
		return nil, nil // Return empty list since pods are queued
	}

	return pods, nil
}
