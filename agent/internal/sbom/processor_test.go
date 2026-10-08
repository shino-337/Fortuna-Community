package sbom

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/fortuna/agent/pkg/sbom/extractor"
)

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		name     string
		imageRef string
		wantName string
		wantTag  string
	}{
		{
			name:     "registry with port and tag",
			imageRef: "registry.example.com:5000/ns/img:v1.0",
			wantName: "registry.example.com:5000/ns/img",
			wantTag:  "v1.0",
		},
		{
			name:     "digest reference",
			imageRef: "nginx@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantName: "index.docker.io/library/nginx",
			wantTag:  "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple tag",
			imageRef: "nginx:latest",
			wantName: "index.docker.io/library/nginx",
			wantTag:  "latest",
		},
		{
			name:     "no tag defaults to latest",
			imageRef: "nginx",
			wantName: "index.docker.io/library/nginx",
			wantTag:  "latest",
		},
		{
			name:     "qualified repo with tag",
			imageRef: "gcr.io/myproject/myimage:tag",
			wantName: "gcr.io/myproject/myimage",
			wantTag:  "tag",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotTag := parseImageRef(tt.imageRef)
			if gotName != tt.wantName {
				t.Errorf("parseImageRef(%q) name = %q, want %q", tt.imageRef, gotName, tt.wantName)
			}
			if gotTag != tt.wantTag {
				t.Errorf("parseImageRef(%q) tag = %q, want %q", tt.imageRef, gotTag, tt.wantTag)
			}
		})
	}
}

func TestRunningImageDigestRef(t *testing.T) {
	const d = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	pod := func(imageID string) *corev1.Pod {
		return &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{Name: "other", ImageID: "docker.io/library/redis@" + d},
			{Name: "app", ImageID: imageID},
		}}}
	}
	tests := []struct {
		name    string
		imageID string
		want    string
	}{
		{"containerd repo digest", "docker.io/library/nginx@" + d, "index.docker.io/library/nginx@" + d},
		{"cri-dockerd prefix", "docker-pullable://ghcr.io/acme/api@" + d, "ghcr.io/acme/api@" + d},
		{"config id only is not pullable", d, ""},
		{"not reported yet", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runningImageDigestRef(pod(tt.imageID), "app"); got != tt.want {
				t.Errorf("runningImageDigestRef = %q, want %q", got, tt.want)
			}
		})
	}
	if got := runningImageDigestRef(pod(""), "missing"); got != "" {
		t.Errorf("unknown container: got %q, want empty", got)
	}
}

func TestConvertToProtoDropsAnnotations(t *testing.T) {
	p := &Processor{}
	pod := &corev1.Pod{}
	pod.Labels = map[string]string{"app": "api"}
	pod.Annotations = map[string]string{"kubectl.kubernetes.io/last-applied-configuration": `{"env":[{"name":"DB_PASSWORD","value":"s3cret"}]}`}
	out := p.convertToProto(pod, corev1.Container{Name: "app", Image: "nginx:1.27"}, &extractor.RawSBOM{})
	if len(out.Annotations) != 0 {
		t.Fatalf("annotations sent to Core: %v", out.Annotations)
	}
	if out.Labels["app"] != "api" {
		t.Fatalf("labels = %v, want app=api kept", out.Labels)
	}
}
