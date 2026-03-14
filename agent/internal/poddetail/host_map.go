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

// BuildContainerIDToPodMap builds a map from container ID (short, 12 chars typically) to pod/container info.
// K8s ContainerID is like "containerd://a1b2c3d4e5f6..." or "cri-o://..."; cgroup usually has the short suffix.
// We store by normalized short ID (last 12 chars or full if shorter) and also by full suffix so lookup works.
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
			short := normalizeContainerIDShort(cid)
			if short == "" {
				continue
			}
			out[short] = PodContainerInfo{
				PodUID:        uid,
				Namespace:     pod.Namespace,
				ContainerName: cs.Name,
			}
		}
	}
	return out
}

// normalizeContainerIDShort returns the short container ID (no scheme).
// containerd: "containerd://abc123..." -> "abc123..." (we take up to 64 chars, but cgroup usually 12).
func normalizeContainerIDShort(containerID string) string {
	s := trimContainerIDPrefix(containerID)
	// Keep last 12 chars for cgroup v2 (e.g. cri-containerd-<12>.scope)
	if len(s) > 12 {
		return s[len(s)-12:]
	}
	return s
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
