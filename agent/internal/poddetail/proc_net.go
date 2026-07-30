package poddetail

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// TCP state constants (from kernel)
const (
	TcpStateEstablished = 0x01
	TcpStateListen      = 0x0A
)

var tcpStateNames = map[int]string{
	0x01: "ESTABLISHED",
	0x02: "SYN_SENT",
	0x03: "SYN_RECV",
	0x04: "FIN_WAIT1",
	0x05: "FIN_WAIT2",
	0x06: "TIME_WAIT",
	0x07: "CLOSE",
	0x08: "CLOSE_WAIT",
	0x09: "LAST_ACK",
	0x0A: "LISTEN",
	0x0B: "CLOSING",
}

// ParseProcNetFile reads /proc/<pid>/net/tcp or .../net/udp and returns connection entries.
// First line is header; data lines: sl local_address rem_address st tx_queue rx_queue ...
// local_address and rem_address are hex "AABBCCDD:PORT" (IPv4, little-endian).
func ParseProcNetFile(procRoot, proto string, pid int) ([]connectionPayload, error) {
	filename := proto
	if proto == "tcp" {
		filename = "tcp"
	} else if proto == "udp" {
		filename = "udp"
	} else {
		return nil, fmt.Errorf("unsupported proto %q", proto)
	}
	path := filepath.Join(procRoot, strconv.Itoa(pid), "net", filename)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var list []connectionPayload
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		// Skip header (st is numeric in data lines)
		if fields[0] == "sl" || fields[0] == "num" {
			continue
		}
		localAddr := fields[1]
		remAddr := fields[2]
		stStr := fields[3]
		txQueue, rxQueue := parseTxRxQueue(fields)
		st, _ := strconv.ParseInt(stStr, 16, 32)
		stateStr := tcpStateNames[int(st)]
		if stateStr == "" {
			stateStr = stStr
		}
		localIP, localPort := parseHexAddr(localAddr)
		remIP, remPort := parseHexAddr(remAddr)
		list = append(list, connectionPayload{
			SourceIP:   localIP,
			SourcePort: localPort,
			DestIP:     remIP,
			DestPort:   remPort,
			Protocol:   proto,
			State:      stateStr,
			BytesSent:  txQueue,
			BytesRecv:  rxQueue,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// parseTxRxQueue reads tx/rx queue bytes from proc net fields[4] format: "00000000:00000000" (hex).
func parseTxRxQueue(fields []string) (int64, int64) {
	if len(fields) < 5 {
		return 0, 0
	}
	parts := strings.Split(fields[4], ":")
	if len(parts) != 2 {
		return 0, 0
	}
	tx, err1 := strconv.ParseInt(parts[0], 16, 64)
	rx, err2 := strconv.ParseInt(parts[1], 16, 64)
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return tx, rx
}

// parseHexAddr parses "AABBCCDD:PORT" (hex, IPv4 LE) to (ip string, port int).
func parseHexAddr(addrPort string) (string, int) {
	idx := strings.LastIndex(addrPort, ":")
	if idx < 0 {
		return "", 0
	}
	hexIP := addrPort[:idx]
	hexPort := addrPort[idx+1:]
	if hexIP == "" || hexPort == "" {
		return "", 0
	}
	// IPv4: 4 bytes in kernel order (little-endian): 0100007F -> 7F,00,00,01 -> 127.0.0.1
	if len(hexIP) == 8 {
		var b [4]byte
		for i := 0; i < 4; i++ {
			s := hexIP[(3-i)*2 : (4-i)*2]
			v, err := strconv.ParseUint(s, 16, 8)
			if err != nil {
				return "", 0
			}
			b[i] = byte(v)
		}
		ip := net.IP(b[:]).String()
		port, _ := strconv.ParseUint(hexPort, 16, 16)
		return ip, int(port)
	}
	// IPv6: skip or simple placeholder
	port, _ := strconv.ParseUint(hexPort, 16, 16)
	return "", int(port)
}
