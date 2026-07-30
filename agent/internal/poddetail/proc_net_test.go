package poddetail

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseHexAddr(t *testing.T) {
	// 0100007F = 127.0.0.1 (LE: 7F 00 00 01), 1F90 = 8080
	ip, port := parseHexAddr("0100007F:1F90")
	if ip != "127.0.0.1" || port != 8080 {
		t.Errorf("parseHexAddr(0100007F:1F90) = %q, %d; want 127.0.0.1, 8080", ip, port)
	}
	// 00000000:0 = 0.0.0.0:0
	ip, port = parseHexAddr("00000000:0000")
	if ip != "0.0.0.0" || port != 0 {
		t.Errorf("got %q %d", ip, port)
	}
}

func TestParseProcNetFile(t *testing.T) {
	dir := t.TempDir()
	// Create fake /proc/1/net/tcp: header + one data line
	netDir := filepath.Join(dir, "1", "net")
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345
`
	if err := os.WriteFile(filepath.Join(netDir, "tcp"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	list, err := ParseProcNetFile(dir, "tcp", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(list))
	}
	c := list[0]
	if c.SourceIP != "127.0.0.1" || c.SourcePort != 8080 || c.Protocol != "tcp" {
		t.Errorf("got %+v", c)
	}
	if c.State != "LISTEN" {
		t.Errorf("state = %q, want LISTEN", c.State)
	}
}
