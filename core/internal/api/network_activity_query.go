package api

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// joinPodsForNetworkActivity INNER JOINs active inventory pod as source (alias p).
// Chỉ giữ dòng pod_network_connections khi pod còn tồn tại trong pods (deleted_at IS NULL) — tránh hiển thị UID đã mất khỏi cluster.
func joinPodsForNetworkActivity(db *gorm.DB) *gorm.DB {
	return db.Joins(`INNER JOIN pods p ON p.uid = n.pod_uid AND p.cluster_id = n.cluster_id AND p.deleted_at IS NULL`)
}

// applyNetworkActivityNamespaceFilter filters rows by source pod namespace on pod_network_connections (alias n).
// Matching is case-insensitive. Suffix "*" means prefix match (e.g. kube* → kube%).
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
