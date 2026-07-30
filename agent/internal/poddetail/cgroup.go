package poddetail

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ContainerIDFromCgroup reads /proc/<pid>/cgroup (or <procRoot>/<pid>/cgroup) and extracts
// the container ID for containerd/cri-o. Returns empty string if not a container process or on parse error.
// Supports cgroup v2 (single line 0::/path) and v1 (multiple lines controller:path).
func ContainerIDFromCgroup(procRoot string, pid int) (string, error) {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "cgroup")
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Cgroup v2: "0::/kubepods.slice/kubepods-pod<uid>.slice/cri-containerd-<id>.scope"
		if strings.HasPrefix(line, "0::") {
			pathPart := strings.TrimPrefix(line, "0::")
			return extractContainerIDFromPath(pathPart), nil
		}
		// Cgroup v1: "controller:path" e.g. "0::/kubepods/..." or "1:name=systemd:/kubepods.slice/..."
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		pathPart := strings.TrimSpace(line[idx+1:])
		if id := extractContainerIDFromPath(pathPart); id != "" {
			return id, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

// extractContainerIDFromPath finds containerd/cri-o container ID in a cgroup path.
// Examples:
//   - .../cri-containerd-<id>.scope  (cgroup v2 standard)
//   - .../containerd-<id>.scope
//   - .../crio-<id>.scope
//   - ...slice:cri-containerd:<id>   (cgroup v2 colon-separated variant, seen on some containerd+systemd nodes)
//   - .../pod<uid>/<id>  (v1)
func extractContainerIDFromPath(pathPart string) string {
	// Match both dash-separated (cri-containerd-<id>.scope) and colon-separated
	// (.slice:cri-containerd:<id>) cgroup formats.
	for _, prefix := range []string{"cri-containerd-", "cri-containerd:", "containerd-", "containerd:", "crio-", "crio:"} {
		i := strings.LastIndex(pathPart, prefix)
		if i < 0 {
			continue
		}
		after := pathPart[i+len(prefix):]
		if end := strings.IndexAny(after, "./:"); end > 0 {
			after = after[:end]
		}
		after = strings.Trim(after, "/")
		if len(after) >= 6 && len(after) <= 64 && isHexOrAlpha(after) {
			if len(after) > 12 {
				after = after[len(after)-12:]
			}
			return after
		}
	}
	// v1: last segment sometimes is the id (e.g. .../pod<uid>/<id>)
	segments := strings.Split(strings.Trim(pathPart, "/"), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		s := segments[i]
		if len(s) >= 12 && len(s) <= 64 && isHexOrAlpha(s) {
			if len(s) > 12 {
				s = s[len(s)-12:]
			}
			return s
		}
	}
	return ""
}

func isHexOrAlpha(s string) bool {
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}
