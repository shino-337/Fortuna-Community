//go:build legacy_trivy

package scanner

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/watch"

	"github.com/ksam/core/pkg/models"
	"gorm.io/gorm"
)

// PodWatcher watches Kubernetes pods and triggers image scans
type PodWatcher struct {
	db          *gorm.DB
	k8sClient   kubernetes.Interface
	scanner     *ImageScanner
	processor   *CVEProcessor
	clusterID   string
	logger      *log.Logger
}

// NewPodWatcher creates a new pod watcher
func NewPodWatcher(
	db *gorm.DB,
	k8sClient kubernetes.Interface,
	scanner *ImageScanner,
	processor *CVEProcessor,
	clusterID string,
) *PodWatcher {
	return &PodWatcher{
		db:        db,
		k8sClient: k8sClient,
		scanner:   scanner,
		processor: processor,
		clusterID: clusterID,
		logger:    log.New(log.Writer(), "[PodWatcher] ", log.LstdFlags),
	}
}

// Start watches for pod events
func (w *PodWatcher) Start(ctx context.Context) error {
	w.logger.Println("Starting pod watcher...")

	// Watch all namespaces
	watcher, err := w.k8sClient.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Stop()

	// Use watch.Until to handle reconnection
	_, err = watch.UntilWithSync(ctx, watcher, &corev1.Pod{}, nil, func(event watch.Event) (bool, error) {
		if event.Type == watch.Added || event.Type == watch.Modified {
			if pod, ok := event.Object.(*corev1.Pod); ok {
				if err := w.handlePodEvent(ctx, pod); err != nil {
					w.logger.Printf("Failed to handle pod event: %v", err)
					// Continue processing other events
				}
			}
		}
		return false, nil // Continue watching
	})

	return err
}

// handlePodEvent processes pod create/update
func (w *PodWatcher) handlePodEvent(ctx context.Context, pod *corev1.Pod) error {
	// Only process running pods
	if pod.Status.Phase != corev1.PodRunning {
		return nil
	}

	return w.scanPod(ctx, pod)
}

// scanPod scans all containers in a pod
func (w *PodWatcher) scanPod(ctx context.Context, pod *corev1.Pod) error {
	w.logger.Printf("Scanning pod %s/%s", pod.Namespace, pod.Name)

	// Scan each container
	for _, container := range pod.Spec.Containers {
		// 1. Scan image
		scanResult, err := w.scanner.ScanImage(ctx, container.Image)
		if err != nil {
			w.logger.Printf("Failed to scan %s: %v", container.Image, err)
			continue
		}

		// 2. Link pod to scan
		if err := w.linkPodToScan(ctx, pod, container.Name, scanResult.ID); err != nil {
			w.logger.Printf("Failed to link pod to scan: %v", err)
			continue
		}

		// 3. Process vulnerabilities (create insights) if critical/high found
		if scanResult.CriticalCount > 0 || scanResult.HighCount > 0 {
			if err := w.processor.ProcessScanResult(
				ctx,
				string(pod.UID),
				pod.Name,
				pod.Namespace,
				w.clusterID,
				container.Name,
				scanResult,
			); err != nil {
				w.logger.Printf("Failed to process scan result: %v", err)
			}
		}
	}

	return nil
}

// linkPodToScan creates pod_image_scans entry
func (w *PodWatcher) linkPodToScan(ctx context.Context, pod *corev1.Pod, containerName string, scanID uint) error {
	// Parse image reference
	container := findContainer(pod, containerName)
	if container == nil {
		return fmt.Errorf("container %s not found in pod", containerName)
	}

	imageName, imageTag := parseImageRefFromString(container.Image)
	podCreatedAt := pod.CreationTimestamp.Time

	podScan := &models.PodImageScan{
		PodUID:         string(pod.UID),
		PodName:        pod.Name,
		PodNamespace:   pod.Namespace,
		ClusterID:      w.clusterID,
		ContainerName:  containerName,
		ContainerImage: container.Image,
		ImageName:      imageName,
		ImageTag:       imageTag,
		ScanResultID:   &scanID,
		PodPhase:       string(pod.Status.Phase),
		PodCreatedAt:   &podCreatedAt,
	}

	// Upsert
	return w.db.WithContext(ctx).
		Where("pod_uid = ? AND container_name = ?", pod.UID, containerName).
		Assign(podScan).
		FirstOrCreate(podScan).Error
}

// findContainer finds container by name
func findContainer(pod *corev1.Pod, name string) *corev1.Container {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return &pod.Spec.Containers[i]
		}
	}
	return nil
}

