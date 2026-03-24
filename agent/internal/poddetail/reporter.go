package poddetail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultReportInterval = 2 * time.Minute
)

// Reporter sends pod runtime metrics and process snapshots to Core (Pod Detail services).
// Data is collected from real pods on this node only; no seed or mock.
type Reporter struct {
	client      kubernetes.Interface
	restConfig  *rest.Config // optional: when set, real process collection via exec is enabled
	coreBaseURL string
	clusterID   string
	nodeName    string
	interval    time.Duration
	httpClient  *http.Client
}

// NewReporter creates a Reporter. coreBaseURL is the Core HTTP base (e.g. http://fortuna-core:8080).
// restConfig: when non-nil, process list is collected via exec into each container (real data); when nil, process payload is empty.
func NewReporter(client kubernetes.Interface, restConfig *rest.Config, coreBaseURL, clusterID, nodeName string, interval time.Duration) *Reporter {
	if interval <= 0 {
		interval = defaultReportInterval
	}
	return &Reporter{
		client:      client,
		restConfig:  restConfig,
		coreBaseURL: coreBaseURL,
		clusterID:   clusterID,
		nodeName:    nodeName,
		interval:    interval,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Start runs the report loop until ctx is done.
func (r *Reporter) Start(ctx context.Context) {
	if r.coreBaseURL == "" {
		log.Printf("[PodDetail] CORE_HTTP_ENDPOINT empty, skipping pod detail reporter")
		return
	}
	log.Printf("[PodDetail] Starting reporter: interval=%v node=%s", r.interval, r.nodeName)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if err := r.reportOnce(ctx); err != nil {
			log.Printf("[PodDetail] report once failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// useHostRuntime returns true when runtime collection should use host (/proc) instead of exec.
// - "host" or "1" or "true": always host.
// - "exec": always exec (skip host).
// - "auto" or unset: use host if hostProcRoot() is readable (e.g. DaemonSet with hostPID + /proc mount), else exec.
func useHostRuntime() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("POD_DETAIL_RUNTIME_SOURCE")))
	if v == "exec" || v == "0" || v == "false" {
		return false
	}
	if v == "host" || v == "1" || v == "true" {
		return true
	}
	// auto or empty: try host if proc root is available
	return canUseHostProc(hostProcRoot())
}

// canUseHostProc returns true if procRoot exists and we can read it (e.g. /host/proc when mounted).
func canUseHostProc(procRoot string) bool {
	if procRoot == "" {
		return false
	}
	f, err := os.Open(procRoot)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err == nil
}

// hostProcRoot returns the path to host /proc (e.g. /host/proc when mounted in DaemonSet).
func hostProcRoot() string {
	if p := os.Getenv("POD_DETAIL_PROC_ROOT"); p != "" {
		return p
	}
	return "/host/proc"
}

func (r *Reporter) reportOnce(ctx context.Context) error {
	pods, err := r.listPodsOnNode(ctx)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	usageByPod, err := collectPodRuntimeUsageFromKubelet(ctx, r.client, r.nodeName)
	if err != nil {
		log.Printf("[PodDetail] runtime usage source unavailable (node=%s): %v", r.nodeName, err)
	}

	var processesByPod map[string][]processPayload
	var connectionsByPod map[string][]connectionPayload
	if useHostRuntime() {
		procRoot := hostProcRoot()
		containerMap := BuildContainerIDToPodMap(pods)
		observedAt := time.Now().Format(time.RFC3339)
		procItems := CollectProcessesFromHost(procRoot, containerMap, observedAt)
		processesByPod = make(map[string][]processPayload)
		for _, it := range procItems {
			processesByPod[it.PodUID] = append(processesByPod[it.PodUID], it.Process)
		}
		netItems := CollectNetworkFromHost(procRoot, containerMap)
		connectionsByPod = make(map[string][]connectionPayload)
		for _, it := range netItems {
			connectionsByPod[it.PodUID] = append(connectionsByPod[it.PodUID], it.Connection)
		}
	}

	for i := range pods {
		pod := &pods[i]
		uid := string(pod.UID)
		if uid == "" || uid == "0" {
			continue
		}
		// Send runtime metrics (from pod status: container state, restart count)
		if err := r.sendRuntimeMetrics(ctx, pod, usageByPod[uid]); err != nil {
			log.Printf("[PodDetail] send metrics for %s/%s: %v", pod.Namespace, pod.Name, err)
		}
		// Process snapshot: host (from /proc) or exec
		if err := r.sendProcessSnapshotsForPod(ctx, pod, processesByPod[uid]); err != nil {
			log.Printf("[PodDetail] send processes for %s/%s: %v", pod.Namespace, pod.Name, err)
		}
		// Network: host (from /proc/<pid>/net/*) or exec
		if err := r.sendNetworkConnectionsForPod(ctx, pod, connectionsByPod[uid]); err != nil {
			log.Printf("[PodDetail] send network for %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}
	return nil
}

func (r *Reporter) listPodsOnNode(ctx context.Context) ([]corev1.Pod, error) {
	list, err := r.client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + r.nodeName,
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// runtimeMetric matches Core model for ingest (subset).
type runtimeMetricPayload struct {
	ContainerName     string `json:"containerName"`
	CPUUsageMillicore int    `json:"cpuUsageMillicore"`
	MemoryUsageBytes  int64  `json:"memoryUsageBytes"`
	MemoryLimitBytes  int64  `json:"memoryLimitBytes"`
	RestartCount      int    `json:"restartCount"`
	State             string `json:"state"`
}

func (r *Reporter) sendRuntimeMetrics(ctx context.Context, pod *corev1.Pod, usageByContainer map[string]containerRuntimeUsage) error {
	uid := string(pod.UID)
	if uid == "" || uid == "0" {
		return nil
	}
	metrics := make([]runtimeMetricPayload, 0, len(pod.Status.ContainerStatuses))
	for _, cs := range pod.Status.ContainerStatuses {
		state := "Unknown"
		if cs.State.Running != nil {
			state = "Running"
		} else if cs.State.Waiting != nil {
			state = "Waiting"
		} else if cs.State.Terminated != nil {
			state = "Terminated"
		}
		usage := usageByContainer[cs.Name]
		metrics = append(metrics, runtimeMetricPayload{
			ContainerName:     cs.Name,
			CPUUsageMillicore: usage.CPUUsageMillicore,
			MemoryUsageBytes:  usage.MemoryUsageBytes,
			MemoryLimitBytes:  memoryLimitBytesForContainer(pod, cs.Name),
			RestartCount:      int(cs.RestartCount),
			State:             state,
		})
	}
	if len(metrics) == 0 {
		return nil
	}
	body := map[string]interface{}{
		"podUid":    uid,
		"clusterId": r.clusterID,
		"namespace": pod.Namespace,
		"metrics":   metrics,
	}
	return r.post(ctx, "/api/v1/agent/pod-runtime-metrics", body)
}

// processPayload matches Core PodProcess JSON (pid/ppid).
type processPayload struct {
	ContainerName string  `json:"containerName"`
	PID           int     `json:"pid"`
	PPID          int     `json:"ppid"`
	UserName      string  `json:"userName"`
	UserID        int     `json:"userId,omitempty"`
	GroupID       int     `json:"groupId,omitempty"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
	Command       string  `json:"command"`
	BinaryPath    string  `json:"binaryPath"`
	WorkingDir    string  `json:"workingDir,omitempty"`
	CapEff        string  `json:"capEff,omitempty"`
	ObservedAt    string  `json:"observedAt"`
}

// sendProcessSnapshotsForPod sends process list for one pod. When hostProcesses is non-nil (host mode)
// it uses that; otherwise collects via exec when restConfig is set.
func (r *Reporter) sendProcessSnapshotsForPod(ctx context.Context, pod *corev1.Pod, hostProcesses []processPayload) error {
	uid := string(pod.UID)
	if uid == "" || uid == "0" {
		return nil
	}
	var processes []processPayload
	if hostProcesses != nil {
		processes = hostProcesses
	} else if r.restConfig != nil {
		var err error
		processes, err = CollectProcessesFromPod(ctx, r.client, r.restConfig, pod)
		if err != nil {
			return err
		}
	}
	body := map[string]interface{}{
		"podUid":    uid,
		"clusterId": r.clusterID,
		"namespace": pod.Namespace,
		"processes": processes,
	}
	if hostProcesses != nil {
		body["runtimeSource"] = "host"
	}
	return r.post(ctx, "/api/v1/agent/pod-processes", body)
}

func (r *Reporter) post(ctx context.Context, path string, body interface{}) error {
	return r.postWithRetry(ctx, path, body)
}

// postWithRetry sends POST with exponential backoff (Phase 2.2: avoid drop on transient failure).
func (r *Reporter) postWithRetry(ctx context.Context, path string, body interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := r.coreBaseURL + path
	backoff := time.Second
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := r.httpClient.Do(req)
		if err != nil {
			if attempt < maxRetries-1 {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
				continue
			}
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if attempt < maxRetries-1 && (resp.StatusCode == 429 || resp.StatusCode >= 500) {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return fmt.Errorf("POST %s: %d", path, resp.StatusCode)
	}
	return nil
}

// sendNetworkConnectionsForPod sends network connections for one pod. When hostConnections is non-nil (host mode)
// uses that; otherwise collects via exec when restConfig is set.
func (r *Reporter) sendNetworkConnectionsForPod(ctx context.Context, pod *corev1.Pod, hostConnections []connectionPayload) error {
	uid := string(pod.UID)
	if uid == "" || uid == "0" {
		return nil
	}
	var connections []connectionPayload
	if hostConnections != nil {
		connections = hostConnections
	} else if r.restConfig != nil {
		var err error
		connections, err = CollectNetworkFromPod(ctx, r.client, r.restConfig, pod)
		if err != nil {
			return err
		}
	}
	body := map[string]interface{}{
		"podUid":      uid,
		"clusterId":   r.clusterID,
		"namespace":   pod.Namespace,
		"connections": connections,
	}
	if hostConnections != nil {
		body["runtimeSource"] = "host"
	}
	return r.postWithRetry(ctx, "/api/v1/agent/pod-network-connections", body)
}
