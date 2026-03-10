package poddetail

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// connectionPayload matches Core PodNetworkConnection JSON for ingest.
type connectionPayload struct {
	ContainerName string `json:"containerName"`
	SourceIP      string `json:"sourceIp"`
	SourcePort    int    `json:"sourcePort"`
	DestIP        string `json:"destIp"`
	DestPort      int    `json:"destPort"`
	Protocol      string `json:"protocol"`
	State         string `json:"state"`
}

// CollectNetworkFromPod runs "ss" or "netstat" in each container and returns connection list.
// Real data from pod; no mock. Skips container if exec fails (e.g. no ss/netstat). Minimal images
// (etcd, kube-apiserver, etc.) often lack these tools; we skip without spamming logs.
func CollectNetworkFromPod(ctx context.Context, clientset kubernetes.Interface, restConfig *rest.Config, pod *corev1.Pod) ([]connectionPayload, error) {
	if restConfig == nil {
		return nil, nil
	}
	var all []connectionPayload
	for _, c := range pod.Spec.Containers {
		list, err := execSsInContainer(ctx, clientset, restConfig, pod.Namespace, pod.Name, c.Name)
		if err != nil {
			if !isExecToolNotFound(err) {
				log.Printf("[PodDetail] network collect %s/%s/%s: %v (skipping)", pod.Namespace, pod.Name, c.Name, err)
			}
			continue
		}
		for i := range list {
			list[i].ContainerName = c.Name
		}
		all = append(all, list...)
	}
	return all, nil
}

func execSsInContainer(ctx context.Context, clientset kubernetes.Interface, restConfig *rest.Config, namespace, podName, containerName string) ([]connectionPayload, error) {
	// Try "ss -tunap" first (most Linux); then "netstat -tunap". Do not use "sh -c" fallback:
	// many control-plane images (etcd, kube-apiserver, etc.) are distroless and have no sh.
	commands := [][]string{
		{"ss", "-tunap"},
		{"netstat", "-tunap"},
	}
	var out string
	var lastErr error
	for _, cmd := range commands {
		req := clientset.CoreV1().RESTClient().Post().
			Resource("pods").Namespace(namespace).Name(podName).SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: containerName,
				Command:   cmd,
				Stdin:     false, Stdout: true, Stderr: true,
			}, scheme.ParameterCodec)
		executor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
		if err != nil {
			lastErr = err
			continue
		}
		var stdout, stderr strings.Builder
		err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr})
		if err != nil {
			lastErr = err
			continue
		}
		out = stdout.String()
		if out != "" {
			break
		}
	}
	if out == "" {
		return nil, lastErr
	}
	return parseSsOrNetstat(out, containerName)
}

// parseSsOrNetstat parses "ss -tunap" or "netstat -tunap" style output.
// ss: "tcp  LISTEN  0  128  *:8080  *:*  users:(...)"
// netstat: "tcp  0  0  0.0.0.0:8080  0.0.0.0:*  LISTEN"
func parseSsOrNetstat(out, containerName string) ([]connectionPayload, error) {
	var list []connectionPayload
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Netid") || strings.HasPrefix(line, "Active") || strings.HasPrefix(line, "Proto") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		proto := strings.ToLower(fields[0])
		if proto != "tcp" && proto != "udp" {
			continue
		}
		state := ""
		var local, remote string
		for i := 1; i < len(fields); i++ {
			if strings.Contains(fields[i], ":") {
				local = fields[i]
				if i+1 < len(fields) && strings.Contains(fields[i+1], ":") {
					remote = fields[i+1]
					if i+2 < len(fields) && !strings.Contains(fields[i+2], ":") {
						state = fields[i+2]
					}
				} else if i+1 < len(fields) {
					state = fields[i+1]
				}
				break
			}
			if fields[i] == "LISTEN" || fields[i] == "ESTAB" || strings.HasPrefix(fields[i], "ESTAB") {
				state = fields[i]
			}
		}
		sip, sport := splitAddr(local)
		dip, dport := splitAddr(remote)
		list = append(list, connectionPayload{
			ContainerName: containerName,
			SourceIP:      sip,
			SourcePort:    sport,
			DestIP:        dip,
			DestPort:      dport,
			Protocol:      proto,
			State:         state,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return list, nil
}

func splitAddr(addr string) (ip string, port int) {
	addr = strings.TrimSpace(addr)
	if addr == "" || addr == "*" {
		return "", 0
	}
	// "*:*" means no specific address/port (e.g. peer for LISTEN)
	if addr == "*:*" {
		return "", 0
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		// Fallback: ss often outputs "*:3000" or "0.0.0.0:3000"; SplitHostPort may fail for "*" on some Go versions
		if idx := strings.LastIndex(addr, ":"); idx >= 0 && idx < len(addr)-1 {
			if p, e := strconv.Atoi(addr[idx+1:]); e == nil {
				hostPart := addr[:idx]
				if hostPart == "*" {
					hostPart = "0.0.0.0" // normalize for LISTEN on all interfaces
				}
				return hostPart, p
			}
		}
		if net.ParseIP(addr) != nil {
			return addr, 0
		}
		return "", 0
	}
	p, _ := strconv.Atoi(portStr)
	if host == "*" {
		host = "0.0.0.0"
	}
	return host, p
}

