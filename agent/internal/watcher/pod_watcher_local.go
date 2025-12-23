package watcher

import (
	"context"
	"log"
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
	clientset  *kubernetes.Clientset
	nodeName   string
	handler    PodHandler
	informer   cache.SharedIndexInformer
	stopCh     chan struct{}
	logger     *log.Logger
}

// NewLocalPodWatcher creates a new local pod watcher
func NewLocalPodWatcher(clientset *kubernetes.Clientset, nodeName string, handler PodHandler) *LocalPodWatcher {
	return &LocalPodWatcher{
		clientset:  clientset,
		nodeName:   nodeName,
		handler:    handler,
		stopCh:     make(chan struct{}),
		logger:     log.New(log.Writer(), "[LocalPodWatcher] ", log.LstdFlags),
	}
}

// Start starts the pod watcher
func (w *LocalPodWatcher) Start(ctx context.Context) error {
	w.logger.Printf("Starting pod watcher for node: %s", w.nodeName)

	// Create field selector to watch only pods on this node
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", w.nodeName).String()

	// Create informer factory with field selector
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientset,
		30*time.Second, // Resync period
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
			w.logger.Printf("🆕 Pod added: %s/%s (phase: %s)", pod.Namespace, pod.Name, pod.Status.Phase)
			
			// Only process running pods with containers
			if pod.Status.Phase == corev1.PodRunning && len(pod.Spec.Containers) > 0 {
				if err := w.handler(ctx, pod); err != nil {
					w.logger.Printf("⚠️  Handler error for pod %s/%s: %v", pod.Namespace, pod.Name, err)
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)

			// Only process if pod transitioned to Running state
			if oldPod.Status.Phase != corev1.PodRunning && newPod.Status.Phase == corev1.PodRunning {
				w.logger.Printf("🔄 Pod updated to Running: %s/%s", newPod.Namespace, newPod.Name)
				if len(newPod.Spec.Containers) > 0 {
					if err := w.handler(ctx, newPod); err != nil {
						w.logger.Printf("⚠️  Handler error for pod %s/%s: %v", newPod.Namespace, newPod.Name, err)
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			w.logger.Printf("🗑️  Pod deleted: %s/%s", pod.Namespace, pod.Name)
			// No action needed for deletion in Phase 1
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
	return nil
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
	return pods, nil
}

