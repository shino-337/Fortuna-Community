package poddetail

// HostNetworkItem pairs a connection with its pod/container for grouping.
type HostNetworkItem struct {
	PodUID        string
	Namespace     string
	ContainerName string
	Connection    connectionPayload
}

// CollectNetworkFromHost reads /proc/<pid>/net/tcp and udp for each PID that belongs to a container
// (from containerMap via cgroup). Returns list with pod/container info; reporter groups by PodUID.
func CollectNetworkFromHost(procRoot string, containerMap map[string]PodContainerInfo) []HostNetworkItem {
	pids := ListPIDs(procRoot)
	var result []HostNetworkItem
	for _, pid := range pids {
		cid, err := ContainerIDFromCgroup(procRoot, pid)
		if err != nil {
			continue
		}
		if cid == "" {
			continue
		}
		info, ok := containerMap[cid]
		if !ok {
			continue
		}
		for _, proto := range []string{"tcp", "udp"} {
			conns, err := ParseProcNetFile(procRoot, proto, pid)
			if err != nil {
				continue
			}
			for i := range conns {
				conns[i].ContainerName = info.ContainerName
				result = append(result, HostNetworkItem{
					PodUID:        info.PodUID,
					Namespace:     info.Namespace,
					ContainerName: info.ContainerName,
					Connection:    conns[i],
				})
			}
		}
	}
	return result
}
