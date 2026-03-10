package poddetail

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// CollectProcessesFromPod runs "ps" in each container of the pod and returns process list.
// Uses real exec into the pod; no mock/seed. If a container has no "ps" (e.g. distroless), that container is skipped.
// Control-plane pods (kube-apiserver, etc.) often lack ps; we skip without spamming logs.
func CollectProcessesFromPod(ctx context.Context, clientset kubernetes.Interface, restConfig *rest.Config, pod *corev1.Pod) ([]processPayload, error) {
	if restConfig == nil {
		return nil, nil
	}
	var all []processPayload
	observedAt := time.Now().Format(time.RFC3339)
	for _, c := range pod.Spec.Containers {
		list, err := execPsInContainer(ctx, clientset, restConfig, pod.Namespace, pod.Name, c.Name, observedAt)
		if err != nil {
			if isExecToolNotFound(err) {
				logExecToolNotFoundDebug(pod.Namespace, pod.Name, c.Name, "process")
			} else {
				log.Printf("[PodDetail] process collect %s/%s/%s: %v (skipping container)", pod.Namespace, pod.Name, c.Name, err)
			}
			continue
		}
		all = append(all, list...)
	}
	return all, nil
}

// execPsInContainer runs "ps -eo pid,ppid,user,%cpu,%mem,comm" in the container and parses output.
func execPsInContainer(ctx context.Context, clientset kubernetes.Interface, restConfig *rest.Config, namespace, podName, containerName, observedAt string) ([]processPayload, error) {
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"ps", "-eo", "pid,ppid,user,%cpu,%mem,comm"},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return nil, err
	}
	out := stdout.String()
	// Do not fallback to "sh -c ps ...": distroless/control-plane images often have no sh,
	// which would produce "sh: executable file not found" and spam logs.
	return parsePsOutput(out, containerName, observedAt)
}

// parsePsOutput parses "ps -eo pid,ppid,user,%cpu,%mem,comm" style output (header + lines).
func parsePsOutput(out, containerName, observedAt string) ([]processPayload, error) {
	var list []processPayload
	scanner := bufio.NewScanner(strings.NewReader(out))
	var header bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			// Maybe header or short line
			if !header && len(fields) >= 1 {
				// Could be PID-only format
				if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 {
					list = append(list, processPayload{
						ContainerName: containerName,
						PID:           pid,
						PPID:          0,
						UserName:      "",
						CPUPercent:    0,
						MemoryPercent: 0,
						Command:       strings.Join(fields[1:], " "),
						ObservedAt:    observedAt,
					})
				}
			}
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			// Likely header line (e.g. "PID PPID USER %CPU %MEM COMMAND")
			header = true
			continue
		}
		user := fields[2]
		cpu := 0.0
		if len(fields) > 3 {
			cpu, _ = strconv.ParseFloat(fields[3], 64)
		}
		mem := 0.0
		if len(fields) > 4 {
			mem, _ = strconv.ParseFloat(fields[4], 64)
		}
		comm := ""
		if len(fields) > 5 {
			comm = strings.Join(fields[5:], " ")
		}
		if len(comm) > 1024 {
			comm = comm[:1024]
		}
		list = append(list, processPayload{
			ContainerName: containerName,
			PID:           pid,
			PPID:          ppid,
			UserName:      truncate(user, 128),
			CPUPercent:    cpu,
			MemoryPercent: mem,
			Command:       comm,
			BinaryPath:    comm,
			ObservedAt:    observedAt,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return list, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
