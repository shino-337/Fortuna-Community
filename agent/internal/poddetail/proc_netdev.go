package poddetail

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// NetDevCounters are cumulative interface counters from /proc/<pid>/net/dev.
// Values are summed across all interfaces in the pod network namespace.
type NetDevCounters struct {
	RxBytes   int64
	TxBytes   int64
	RxPackets int64
	TxPackets int64
}

// ParseProcNetDev reads /proc/<pid>/net/dev from the given procRoot and returns
// summed counters across interfaces. By default excludes loopback ("lo").
func ParseProcNetDev(procRoot string, pid int) (NetDevCounters, error) {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "net", "dev")
	f, err := os.Open(path)
	if err != nil {
		return NetDevCounters{}, err
	}
	defer f.Close()

	var out NetDevCounters
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		lineNo++
		if line == "" {
			continue
		}
		// First 2 lines are headers in /proc/net/dev.
		if lineNo <= 2 {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if iface == "" || iface == "lo" {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(parts[1]))
		// Format: rx_bytes rx_packets rx_errs rx_drop rx_fifo rx_frame rx_compressed rx_multicast tx_bytes tx_packets ...
		if len(fields) < 10 {
			continue
		}
		rxBytes, err1 := strconv.ParseInt(fields[0], 10, 64)
		rxPkts, err2 := strconv.ParseInt(fields[1], 10, 64)
		txBytes, err3 := strconv.ParseInt(fields[8], 10, 64)
		txPkts, err4 := strconv.ParseInt(fields[9], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}
		out.RxBytes += rxBytes
		out.RxPackets += rxPkts
		out.TxBytes += txBytes
		out.TxPackets += txPkts
	}
	if err := sc.Err(); err != nil {
		return NetDevCounters{}, fmt.Errorf("scan net/dev: %w", err)
	}
	return out, nil
}

