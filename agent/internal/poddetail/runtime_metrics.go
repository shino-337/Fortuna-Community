package poddetail

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

type containerRuntimeUsage struct {
	CPUUsageMillicore int
	MemoryUsageBytes  int64
}

type podRuntimeUsageMap map[string]map[string]containerRuntimeUsage

// podMetricsList is the subset of metrics.k8s.io/v1beta1 PodMetricsList the Agent reads.
type podMetricsList struct {
	Items []podMetrics `json:"items"`
}

type podMetrics struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Containers []struct {
		Name  string            `json:"name"`
		Usage map[string]string `json:"usage"`
	} `json:"containers"`
}

// collectPodRuntimeUsage reads per-container CPU and memory for the given pods from the
// metrics.k8s.io API (metrics-server). It lists only the namespaces those pods run in.
//
// The Agent used to read the kubelet summary through nodes/{node}/proxy. That needs
// get on nodes/proxy, which the kubelet also accepts for exec, attach and run in any pod
// on the node, so the Agent must never hold it.
func collectPodRuntimeUsage(ctx context.Context, client kubernetes.Interface, pods []corev1.Pod) (podRuntimeUsageMap, error) {
	if client == nil || len(pods) == 0 {
		return nil, nil
	}
	rc := client.Discovery().RESTClient()
	if rc == nil {
		return nil, nil
	}
	uidByName := make(map[string]string, len(pods))
	var namespaces []string
	for i := range pods {
		p := &pods[i]
		if _, seen := uidByName[p.Namespace+"/"+p.Name]; !seen {
			uidByName[p.Namespace+"/"+p.Name] = string(p.UID)
		}
		if !containsString(namespaces, p.Namespace) {
			namespaces = append(namespaces, p.Namespace)
		}
	}
	sort.Strings(namespaces)
	out := make(podRuntimeUsageMap)
	for _, ns := range namespaces {
		raw, err := rc.Get().AbsPath("/apis/metrics.k8s.io/v1beta1/namespaces", ns, "pods").DoRaw(ctx)
		if err != nil {
			return out, fmt.Errorf("fetch pod metrics (namespace=%s): %w", ns, err)
		}
		if err := mergePodMetrics(out, raw, uidByName); err != nil {
			return out, err
		}
	}
	return out, nil
}

// mergePodMetrics adds the containers of the pods in uidByName (keyed namespace/name) to out.
// Pods not on this node are ignored.
func mergePodMetrics(out podRuntimeUsageMap, raw []byte, uidByName map[string]string) error {
	var list podMetricsList
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("decode pod metrics: %w", err)
	}
	for i := range list.Items {
		item := list.Items[i]
		uid := uidByName[item.Metadata.Namespace+"/"+item.Metadata.Name]
		if uid == "" {
			continue
		}
		if _, ok := out[uid]; !ok {
			out[uid] = make(map[string]containerRuntimeUsage)
		}
		for _, c := range item.Containers {
			if c.Name == "" {
				continue
			}
			out[uid][c.Name] = containerRuntimeUsage{
				CPUUsageMillicore: int(quantityValue(c.Usage["cpu"], true)),
				MemoryUsageBytes:  quantityValue(c.Usage["memory"], false),
			}
		}
	}
	return nil
}

// quantityValue parses a resource quantity; milli returns millicores (for CPU).
func quantityValue(s string, milli bool) int64 {
	if s == "" {
		return 0
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return 0
	}
	if milli {
		return q.MilliValue()
	}
	return q.Value()
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func memoryLimitBytesForContainer(pod *corev1.Pod, containerName string) int64 {
	if pod == nil || containerName == "" {
		return 0
	}
	for i := range pod.Spec.Containers {
		c := pod.Spec.Containers[i]
		if c.Name != containerName {
			continue
		}
		qty, ok := c.Resources.Limits[corev1.ResourceMemory]
		if !ok {
			return 0
		}
		return qty.Value()
	}
	return 0
}
