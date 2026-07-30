package api

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// joinPodsForNetworkActivity INNER JOINs active inventory pod as source (alias p).
// Keeps pod_network_connections rows only when the pod still exists in pods (deleted_at IS NULL) — avoids showing UIDs removed from the cluster.
func joinPodsForNetworkActivity(db *gorm.DB) *gorm.DB {
	return db.Joins(`INNER JOIN pods p ON p.uid = n.pod_uid AND p.cluster_id = n.cluster_id AND p.deleted_at IS NULL`)
}

// applyNetworkActivityNamespaceFilter filters rows by source pod namespace on pod_network_connections (alias n).
// Matching is case-insensitive. Suffix "*" means prefix match (e.g. kube* → kube%).
// applyNetworkActivityPodUidFilter restricts to a single source pod (exact UID). Index-friendly vs broad q LIKE.
func applyNetworkActivityPodUidFilter(db *gorm.DB, podUID string) *gorm.DB {
	uid := strings.TrimSpace(podUID)
	if uid == "" {
		return db
	}
	return db.Where("n.pod_uid = ?", uid)
}

func applyNetworkActivityNamespaceFilter(db *gorm.DB, namespace string) *gorm.DB {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return db
	}
	if strings.HasSuffix(ns, "*") {
		prefix := strings.TrimSpace(strings.TrimSuffix(ns, "*"))
		if prefix == "" {
			return db
		}
		// Escape ILIKE wildcards in user prefix (Kubernetes names rarely use % or _)
		prefix = strings.ReplaceAll(prefix, "\\", "\\\\")
		prefix = strings.ReplaceAll(prefix, "%", "\\%")
		prefix = strings.ReplaceAll(prefix, "_", "\\_")
		return db.Where("n.namespace ILIKE ? ESCAPE '\\'", prefix+"%")
	}
	return db.Where("LOWER(TRIM(COALESCE(n.namespace,''))) = LOWER(?)", ns)
}

// applyNetworkActivityTopologyFlowFilter removes socket-state rows that are useful in raw evidence
// tables but misleading in the topology graph. The graph is an active traffic view, so it keeps
// established remote flows and drops local listeners, loopback endpoints, stale close states, and
// host-mode server-side sockets such as postgres:5432 -> core:ephemeral. Those accepted sockets are
// real from the server process view, but drawing them reverses the user-facing dependency direction.
func applyNetworkActivityTopologyFlowFilter(db *gorm.DB) *gorm.DB {
	return db.Where(`
		UPPER(COALESCE(n.state, '')) = 'ESTABLISHED'
		AND LOWER(COALESCE(n.protocol, '')) IN ('tcp', 'udp')
		AND COALESCE(n.dest_ip, '') NOT IN ('', '0.0.0.0', '127.0.0.1', '::1')
		AND COALESCE(n.source_ip, '') NOT IN ('127.0.0.1', '::1')
		AND NOT (
			LOWER(COALESCE(n.runtime_source, '')) = 'host'
			AND COALESCE(n.source_ip, '') = COALESCE(p.pod_ip, '')
			AND n.source_port > 0
			AND n.source_port < 32768
			AND n.dest_port >= 32768
		)
	`)
}

// applyNetworkActivityConnectionsSearch adds WHERE for q on alias n and p.
// Caller must already apply joinPodsForNetworkActivity (INNER JOIN pods p) so p exists.
// If q is a valid port number (1–65535), adds equality on dest_port/source_port in addition to text LIKEs.
func applyNetworkActivityConnectionsSearch(db *gorm.DB, q string) *gorm.DB {
	q = strings.TrimSpace(q)
	if q == "" {
		return db
	}
	like := "%" + strings.ToLower(q) + "%"
	port, err := strconv.Atoi(q)
	if err == nil && port > 0 && port <= 65535 {
		return db.Where(`(n.dest_port = ? OR n.source_port = ? OR LOWER(COALESCE(p.name,'')) LIKE ? OR LOWER(n.namespace) LIKE ? OR LOWER(COALESCE(n.dest_ip,'')) LIKE ? OR CAST(n.dest_port AS TEXT) LIKE ? OR CAST(n.source_port AS TEXT) LIKE ? OR LOWER(CAST(n.pod_uid AS TEXT)) LIKE ? OR LOWER(CAST(p.uid AS TEXT)) LIKE ?)`,
			port, port, like, like, like, like, like, like, like)
	}
	return db.Where(`(LOWER(COALESCE(p.name,'')) LIKE ? OR LOWER(n.namespace) LIKE ? OR LOWER(COALESCE(n.dest_ip,'')) LIKE ? OR CAST(n.dest_port AS TEXT) LIKE ? OR CAST(n.source_port AS TEXT) LIKE ? OR LOWER(CAST(n.pod_uid AS TEXT)) LIKE ? OR LOWER(CAST(p.uid AS TEXT)) LIKE ?)`,
		like, like, like, like, like, like, like)
}

// applyNetworkActivityPodsSubSearch filters the grouped subquery (alias n, join p) by q.
func applyNetworkActivityPodsSubSearch(sub *gorm.DB, q string) *gorm.DB {
	q = strings.TrimSpace(q)
	if q == "" {
		return sub
	}
	like := "%" + strings.ToLower(q) + "%"
	port, err := strconv.Atoi(q)
	if err == nil && port > 0 && port <= 65535 {
		return sub.Where(`(n.dest_port = ? OR n.source_port = ? OR LOWER(COALESCE(p.name,'')) LIKE ? OR LOWER(n.namespace) LIKE ? OR LOWER(CAST(n.pod_uid AS TEXT)) LIKE ? OR LOWER(CAST(p.uid AS TEXT)) LIKE ?)`,
			port, port, like, like, like, like)
	}
	return sub.Where(`(LOWER(COALESCE(p.name,'')) LIKE ? OR LOWER(n.namespace) LIKE ? OR LOWER(CAST(n.pod_uid AS TEXT)) LIKE ? OR LOWER(CAST(p.uid AS TEXT)) LIKE ?)`,
		like, like, like, like)
}
