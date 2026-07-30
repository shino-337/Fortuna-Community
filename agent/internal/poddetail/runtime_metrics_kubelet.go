package poddetail

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type containerRuntimeUsage struct {
	CPUUsageMillicore int
	MemoryUsageBytes  int64
}

type podRuntimeUsageMap map[string]map[string]containerRuntimeUsage

type kubeletSummary struct {
	Pods []kubeletSummaryPod `json:"pods"`
}

type kubeletSummaryPod struct {
	PodRef     kubeletSummaryPodRef      `json:"podRef"`
	Containers []kubeletSummaryContainer `json:"containers"`
}

type kubeletSummaryPodRef struct {
	UID string `json:"uid"`
}

type kubeletSummaryContainer struct {
	Name   string               `json:"name"`
	CPU    kubeletSummaryCPU    `json:"cpu"`
	Memory kubeletSummaryMemory `json:"memory"`
}

type kubeletSummaryCPU struct {
	UsageNanoCores *uint64 `json:"usageNanoCores"`
}

type kubeletSummaryMemory struct {
	WorkingSetBytes *uint64 `json:"workingSetBytes"`
}

func collectPodRuntimeUsageFromKubelet(ctx context.Context, client kubernetes.Interface, nodeName string) (podRuntimeUsageMap, error) {
	if client == nil || nodeName == "" {
		return nil, nil
	}
	raw, err := client.CoreV1().RESTClient().Get().
		Resource("nodes").
		Name(nodeName).
		SubResource("proxy").
		Suffix("stats", "summary").
		DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch kubelet summary: %w", err)
	}
	return buildPodRuntimeUsageMap(raw)
}

func buildPodRuntimeUsageMap(raw []byte) (podRuntimeUsageMap, error) {
	var summary kubeletSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		return nil, fmt.Errorf("decode kubelet summary: %w", err)
	}
	out := make(podRuntimeUsageMap)
	for i := range summary.Pods {
		p := summary.Pods[i]
		podUID := p.PodRef.UID
		if podUID == "" {
			continue
		}
		if _, ok := out[podUID]; !ok {
			out[podUID] = make(map[string]containerRuntimeUsage)
		}
		for j := range p.Containers {
			c := p.Containers[j]
			if c.Name == "" {
				continue
			}
			out[podUID][c.Name] = containerRuntimeUsage{
				CPUUsageMillicore: nanoCoresToMillicore(c.CPU.UsageNanoCores),
				MemoryUsageBytes:  uint64PtrToInt64(c.Memory.WorkingSetBytes),
			}
		}
	}
	return out, nil
}

func nanoCoresToMillicore(v *uint64) int {
	if v == nil {
		return 0
	}
	// 1 millicore = 1,000,000 nanocores.
	return int(*v / 1_000_000)
}

func uint64PtrToInt64(v *uint64) int64 {
	if v == nil {
		return 0
	}
	return int64(*v)
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
