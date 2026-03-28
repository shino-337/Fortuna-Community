package risk

import "github.com/fortuna/core/pkg/models"

// buildScoreV3PreviewMap builds a JSON-serializable snapshot for risk_scores.factors.score_v3_preview (G-RE-01 bridge).
// Kept in package risk to avoid import cycles with riskengine.
func buildScoreV3PreviewMap(s *models.AssetSecurityState) map[string]interface{} {
	if s == nil {
		return map[string]interface{}{
			"scorerVersion": "v3-preview-0.1",
			"total":         0.0,
			"byDimension":   map[string]float64{},
			"provenance":    []map[string]interface{}{},
		}
	}
	by := map[string]float64{}
	var prov []map[string]interface{}
	add := func(dim string, w float64, reason, detail string) {
		if w <= 0 {
			return
		}
		by[dim] += w
		prov = append(prov, map[string]interface{}{
			"dimension": dim, "reason": reason, "weight": w, "detail": detail,
		})
	}
	if s.HostNetwork {
		add("exposure", 12, "host_network", "asset_security_state.host_network")
	}
	if s.HostPID || s.HostIPC {
		add("exposure", 10, "host_ns", "host_pid or host_ipc")
	}
	if s.ServiceAccountBoundToClusterAdmin {
		add("privilege", 18, "cluster_admin_sa", "service account bound to cluster-admin")
	}
	if s.HasSuspiciousExec {
		add("runtime_threat", 14, "suspicious_exec", "runtime flag")
	}
	if s.HasNetworkQueueAnomaly {
		add("runtime_threat", 8, "net_queue_anomaly", "runtime flag")
	}
	if s.HasEscapeRelated {
		add("runtime_threat", 12, "escape_related", "runtime flag")
	}
	if s.SignalTotal24h > 0 {
		sig := float64(s.SignalTotal24h)
		if sig > 50 {
			sig = 50
		}
		add("runtime_threat", sig*0.2, "signal_volume_24h", "signal_total_24h scaled")
	}
	var total float64
	for _, v := range by {
		if v > 0 {
			total += v
		}
	}
	return map[string]interface{}{
		"scorerVersion": "v3-preview-0.1",
		"total":         total,
		"byDimension":   by,
		"provenance":    prov,
	}
}
