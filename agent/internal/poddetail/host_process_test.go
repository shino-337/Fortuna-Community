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
