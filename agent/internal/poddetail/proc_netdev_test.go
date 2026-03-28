package poddetail

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProcNetDev_SumsInterfacesExcludesLo(t *testing.T) {
	dir := t.TempDir()
	netDir := filepath.Join(dir, "123", "net")
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  lo: 10 1 0 0 0 0 0 0  20 2 0 0 0 0 0 0
eth0: 100 3 0 0 0 0 0 0  200 4 0 0 0 0 0 0
eth1: 7  5 0 0 0 0 0 0  9   6 0 0 0 0 0 0
`
	if err := os.WriteFile(filepath.Join(netDir, "dev"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ParseProcNetDev(dir, 123)
	if err != nil {
		t.Fatal(err)
	}
	if got.RxBytes != 107 || got.TxBytes != 209 || got.RxPackets != 8 || got.TxPackets != 10 {
		t.Fatalf("got=%+v", got)
	}
}

