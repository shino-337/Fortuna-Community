package poddetail

import (
	"testing"
)

func TestExtractContainerIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"0::/kubepods.slice/kubepods-pod123.slice/cri-containerd-abc123def456.scope", "abc123def456"},
		{"/kubepods.slice/kubepods-podxyz.slice/containerd-a1b2c3d4e5f6.scope", "a1b2c3d4e5f6"},
		{"1:name=systemd:/kubepods/burstable/pod123/abc123def456", "abc123def456"},
		{"", ""},
		{"/some/other/path", ""},
	}
	for _, tt := range tests {
		got := extractContainerIDFromPath(tt.path)
		if got != tt.want {
			t.Errorf("extractContainerIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsHexOrAlpha(t *testing.T) {
	if !isHexOrAlpha("abc123def456") {
		t.Error("expected true for hex string")
	}
	if isHexOrAlpha("ghijk") {
		t.Error("expected false for non-hex")
	}
	if !isHexOrAlpha("0123456789abcdef") {
		t.Error("expected true")
	}
}
