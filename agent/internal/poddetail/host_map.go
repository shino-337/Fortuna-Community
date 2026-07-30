package poddetail

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// PodContainerInfo identifies a pod and container for a given container ID (short).
type PodContainerInfo struct {
	PodUID        string
	Namespace     string
	ContainerName string
}

// BuildContainerIDToPodMap builds a map from container ID (short, last-12-chars) to pod/container info.
// K8s ContainerID is like "containerd://a1b2c3d4e5f6..." or "cri-o://...".
// Stores both the short (last-12) and full raw ID so lookup succeeds regardless of
// which form the cgroup parser returns.
func BuildContainerIDToPodMap(pods []corev1.Pod) map[string]PodContainerInfo {
	out := make(map[string]PodContainerInfo)
	for i := range pods {
		pod := &pods[i]
		uid := string(pod.UID)
		if uid == "" || uid == "0" {
			continue
		}
		for j := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[j]
			cid := cs.ContainerID
			if cid == "" {
				continue
			}
			info := PodContainerInfo{
				PodUID:        uid,
				Namespace:     pod.Namespace,
				ContainerName: cs.Name,
			}
			raw := trimContainerIDPrefix(cid)
			if raw == "" {
				continue
			}
			out[raw] = info
			short := normalizeContainerIDShort(cid)
			out[short] = info
		}
	}
	return out
}

// normalizeContainerIDShort strips the runtime prefix and returns the cgroup-style
// short ID (last 12 hex chars when the trimmed ID is longer than 12).
func normalizeContainerIDShort(containerID string) string {
	raw := trimContainerIDPrefix(containerID)
	if raw == "" {
		return ""
	}
	if len(raw) > 12 {
		return raw[len(raw)-12:]
	}
	return raw
}

func trimContainerIDPrefix(containerID string) string {
	s := containerID
	for _, prefix := range []string{"containerd://", "cri-o://", "docker://"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	return s
}
