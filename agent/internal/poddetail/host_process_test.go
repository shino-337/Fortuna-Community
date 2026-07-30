package poddetail

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadProcComm(t *testing.T) {
	dir := t.TempDir()
	pid := 99999
	pidDir := filepath.Join(dir, strconv.Itoa(pid))
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("nginx\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := readProcComm(dir, pid)
	if got != "nginx" {
		t.Errorf("readProcComm = %q, want nginx", got)
	}
}

func TestReadProcStat(t *testing.T) {
	dir := t.TempDir()
	pid := 88888
	pidDir := filepath.Join(dir, strconv.Itoa(pid))
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Format: pid (comm) state ppid ...
	statContent := "88888 (nginx) R 1 88888 88888 0 - 1 0 0 0 0 0 0\n"
	if err := os.WriteFile(filepath.Join(pidDir, "stat"), []byte(statContent), 0644); err != nil {
		t.Fatal(err)
	}
	comm, ppid := readProcStat(dir, pid)
	if comm != "nginx" || ppid != 1 {
		t.Errorf("readProcStat: comm=%q ppid=%d, want nginx 1", comm, ppid)
	}
}

func TestReadProcCmdline(t *testing.T) {
	dir := t.TempDir()
	pid := 77777
	pidDir := filepath.Join(dir, strconv.Itoa(pid))
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("/usr/bin/python3\x00-m\x00http.server\x008080\x00")
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	got := readProcCmdline(dir, pid)
	want := "/usr/bin/python3 -m http.server 8080"
	if got != want {
		t.Fatalf("readProcCmdline = %q, want %q", got, want)
	}
}

func TestReadProcStatusMeta(t *testing.T) {
	dir := t.TempDir()
	pid := 66666
	pidDir := filepath.Join(dir, strconv.Itoa(pid))
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatal(err)
	}
	status := "Name:\ttest\nUid:\t1001\t1001\t1001\t1001\nGid:\t2001\t2001\t2001\t2001\nCapEff:\t00000000a80425fb\n"
	if err := os.WriteFile(filepath.Join(pidDir, "status"), []byte(status), 0644); err != nil {
		t.Fatal(err)
	}
	uid, gid, capEff := readProcStatusMeta(dir, pid)
	if uid != 1001 || gid != 2001 || capEff != "00000000a80425fb" {
		t.Fatalf("readProcStatusMeta = uid=%d gid=%d capEff=%q", uid, gid, capEff)
	}
}

func TestParseProcStatCPUFields(t *testing.T) {
	stat := "123 (myproc) S 1 2 3 4 5 6 7 8 9 10 11 120 80 0 0 0 0 0 0 5000 0 0 0"
	jiffies, start, ok := parseProcStatCPUFields(stat)
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if jiffies <= 0 {
		t.Fatalf("unexpected non-positive proc jiffies: %v", jiffies)
	}
	if start < 0 {
		t.Fatalf("unexpected negative start jiffies: %v", start)
	}
}

func TestParseVmRssAndMemTotalKB(t *testing.T) {
	status := "Name:\tbash\nVmRSS:\t   10240 kB\n"
	meminfo := "MemTotal:       2048000 kB\nMemFree: 100 kB\n"
	if got := parseVmRssKB(status); got != 10240 {
		t.Fatalf("unexpected VmRSS: %d", got)
	}
	if got := parseMemTotalKB(meminfo); got != 2048000 {
		t.Fatalf("unexpected MemTotal: %d", got)
	}
}

func TestReadHostCPUCoreCount(t *testing.T) {
	dir := t.TempDir()
	stat := "cpu  1 2 3 4\ncpu0 1 1 1 1\ncpu1 2 2 2 2\nintr 0\n"
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0644); err != nil {
		t.Fatal(err)
	}
	if got := readHostCPUCoreCount(dir); got != 2 {
		t.Fatalf("unexpected core count: %d", got)
	}
}

func TestEnableApproxCPUFromProc(t *testing.T) {
	key := "POD_DETAIL_PROCESS_CPU_APPROX_FROM_PROC"
	_ = os.Unsetenv(key)
	if !enableApproxCPUFromProc() {
		t.Fatalf("default should be enabled")
	}
	_ = os.Setenv(key, "false")
	if enableApproxCPUFromProc() {
		t.Fatalf("expected disabled when env=false")
	}
	_ = os.Setenv(key, "true")
	if !enableApproxCPUFromProc() {
		t.Fatalf("expected enabled when env=true")
	}
	_ = os.Unsetenv(key)
}
