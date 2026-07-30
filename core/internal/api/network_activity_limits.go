package api

import (
	"os"
	"strconv"
	"strings"
)

const (
	networkActivityStandardMaxHardCap   = 500
	networkActivityTopologyEdgesHardCap = 8000
)

// networkActivityStandardMaxPageSize caps pageSize for views: connections, pods, destinations, talkers.
// Default 200; NETWORK_ACTIVITY_MAX_PAGE_SIZE raises up to 500.
func networkActivityStandardMaxPageSize() int {
	const def = 200
	return parseEnvIntBounded("NETWORK_ACTIVITY_MAX_PAGE_SIZE", def, 1, networkActivityStandardMaxHardCap)
}

// networkActivityTopologyEdgesMaxPageSize caps pageSize for view=edges (aggregated pod→dest tuples for topology).
// Default 2500; NETWORK_ACTIVITY_TOPOLOGY_EDGES_MAX up to 8000.
func networkActivityTopologyEdgesMaxPageSize() int {
	const def = 2500
	return parseEnvIntBounded("NETWORK_ACTIVITY_TOPOLOGY_EDGES_MAX", def, 1, networkActivityTopologyEdgesHardCap)
}

func parseEnvIntBounded(key string, defaultVal, minV, maxV int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < minV {
		return defaultVal
	}
	if n > maxV {
		return maxV
	}
	return n
}

func networkActivityPageSizeCap(view string) int {
	if view == "edges" {
		return networkActivityTopologyEdgesMaxPageSize()
	}
	return networkActivityStandardMaxPageSize()
}
