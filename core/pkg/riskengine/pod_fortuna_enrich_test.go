package riskengine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnrichPodFortunaContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeSignal{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	podUID := "11111111-1111-1111-1111-111111111111"
	if err := db.Create(&models.Pod{
		ClusterID: "c1", Name: "p", Namespace: "ns", ServiceAccount: "sa",
		UID: podUID, HostNetwork: true, HostPID: false,
	}).Error; err != nil {
		t.Fatalf("pod: %v", err)
	}
	if err := db.Create(&models.RuntimeSignal{
		PodUID: podUID, SignalType: "NETWORK_QUEUE_ANOMALY", Category: "NETWORK",
		Evidence: "{}", CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("signal: %v", err)
	}
	if err := db.Create(&models.RuntimeSignal{
		PodUID: podUID, SignalType: "PROC_ROOT_PIVOT", Category: "ESCAPE",
		Evidence: "{}", CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("signal2: %v", err)
	}

	e := &Engine{db: db}
	enriched := map[string]interface{}{"uid": podUID, "kind": "Pod"}
	e.enrichPodFortunaContext(context.Background(), enriched)
	f, ok := enriched["fortuna"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected fortuna map, got %T", enriched["fortuna"])
	}
	if f["signal_total_24h"].(int64) != 2 {
		t.Fatalf("signal_total_24h: %v", f["signal_total_24h"])
	}
	if f["has_network_queue_anomaly"] != true || f["has_escape_related"] != true {
		t.Fatalf("flags: %+v", f)
	}
	if f["host_network"] != true {
		t.Fatalf("host_network: %v", f["host_network"])
	}
	if f["service_account_bound_to_cluster_admin"] != false {
		t.Fatalf("expected no cluster-admin binding, got %v", f["service_account_bound_to_cluster_admin"])
	}
}

func TestEnrichPodFortunaContext_SecurityStateAvailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.AssetSecurityState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	podUID := "99999999-9999-9999-9999-999999999999"
	now := time.Now().UTC()

	if err := db.Create(&models.Pod{
		ClusterID: "c1", Name: "p", Namespace: "ns", ServiceAccount: "sa",
		UID: podUID, HostNetwork: false, HostPID: false, HostIPC: false,
	}).Error; err != nil {
		t.Fatalf("pod: %v", err)
	}

	if err := db.Create(&models.AssetSecurityState{
		AssetType:                         "pod",
		PodUID:                            podUID,
		Namespace:                         "ns",
		ClusterID:                         "c1",
		HostNetwork:                       true,
		HostPID:                           true,
		HostIPC:                           false,
		ServiceAccountBoundToClusterAdmin: true,
		SignalTotal24h:                    7,
		HasSuspiciousExec:                 true,
		HasNetworkQueueAnomaly:            true,
		HasEscapeRelated:                  false,
		RuntimeSignalsByType:              `{"NETWORK_QUEUE_ANOMALY":2,"SUSPICIOUS_EXEC_FROM_SNAPSHOT":5}`,
		EffectiveCapabilities:             `["ESC_RUNTIME_PROBE","ESC_RUNTIME_ACTIVE"]`,
		LastRuntimeActivityAt:             &now,
		CreatedAt:                         now,
		UpdatedAt:                         now,
	}).Error; err != nil {
		t.Fatalf("asset security state: %v", err)
	}

	e := &Engine{db: db}
	enriched := map[string]interface{}{"uid": podUID, "kind": "Pod"}
	e.enrichPodFortunaContext(context.Background(), enriched)

	sec, ok := enriched["securityState"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected securityState map, got %T", enriched["securityState"])
	}
	if sec["signal_total_24h"].(int64) != 7 {
		t.Fatalf("signal_total_24h mismatch: %v", sec["signal_total_24h"])
	}
	if sec["host_network"] != true {
		t.Fatalf("host_network expected true, got %v", sec["host_network"])
	}
	rst, ok := sec["runtime_signals_by_type"].(map[string]int64)
	if !ok {
		// CEL input can still work with map[string]interface{}; this assert targets the parsing shape.
		t.Fatalf("runtime_signals_by_type wrong type: %T", sec["runtime_signals_by_type"])
	}
	if rst["NETWORK_QUEUE_ANOMALY"] != 2 {
		t.Fatalf("NETWORK_QUEUE_ANOMALY mismatch: %v", rst["NETWORK_QUEUE_ANOMALY"])
	}
	eff, ok := sec["effective_capabilities"].([]string)
	if !ok {
		t.Fatalf("effective_capabilities wrong type: %T", sec["effective_capabilities"])
	}
	if len(eff) != 2 {
		t.Fatalf("effective_capabilities len expected 2, got %d", len(eff))
	}
}

func TestRuntimeHostNetworkNetworkAnomalyRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}

	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	var expr string
	for _, r := range ye.rules {
		if r.ID != "runtime-hostnetwork-network-anomaly" {
			continue
		}
		for _, c := range r.Conditions {
			if c.Type == CondTypeExpression && strings.TrimSpace(c.Expression) != "" {
				expr = c.Expression
				break
			}
		}
		break
	}
	if strings.TrimSpace(expr) == "" {
		t.Fatalf("rule expression not found for runtime-hostnetwork-network-anomaly")
	}

	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"securityState": map[string]interface{}{
				"host_network":              true,
				"has_network_queue_anomaly": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected rule to match with securityState only")
	}

	// Backward-compat fallback: securityState absent, fortuna present.
	matched2, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"fortuna": map[string]interface{}{
				"host_network":              true,
				"has_network_queue_anomaly": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (fortuna fallback) error: %v", err)
	}
	if !matched2 {
		t.Fatalf("expected rule to match via fortuna fallback")
	}
}

func TestRuntimeNetworkQueueAnomalySignalRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "runtime-network-queue-anomaly-signal")
	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"securityState": map[string]interface{}{
				"has_network_queue_anomaly": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected match with securityState only")
	}

	matched2, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"fortuna": map[string]interface{}{
				"has_network_queue_anomaly": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (fortuna fallback) error: %v", err)
	}
	if !matched2 {
		t.Fatalf("expected match via fortuna fallback")
	}
}

func TestRuntimeSuspiciousExecSignalRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "runtime-suspicious-exec-signal")
	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"securityState": map[string]interface{}{
				"has_suspicious_exec": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected match with securityState only")
	}

	matched2, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"fortuna": map[string]interface{}{
				"has_suspicious_exec": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (fortuna fallback) error: %v", err)
	}
	if !matched2 {
		t.Fatalf("expected match via fortuna fallback")
	}
}

func TestRuntimeSignalsRecentRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "runtime-signals-recent")
	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"securityState": map[string]interface{}{
				"signal_total_24h": int64(1),
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected match with securityState only")
	}

	matched2, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"fortuna": map[string]interface{}{
				"signal_total_24h": int64(1),
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (fortuna fallback) error: %v", err)
	}
	if !matched2 {
		t.Fatalf("expected match via fortuna fallback")
	}
}

func TestRuntimePrivilegedWithSignalsRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "runtime-privileged-with-signals")
	objBase := map[string]interface{}{
		"kind": "Pod",
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					"securityContext": map[string]interface{}{
						"privileged": true,
					},
				},
			},
		},
	}

	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			// merge base
			"kind": objBase["kind"],
			"spec": objBase["spec"],
			"securityState": map[string]interface{}{
				"signal_total_24h": int64(1),
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected match with securityState only")
	}

	matched2, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": objBase["kind"],
			"spec": objBase["spec"],
			"fortuna": map[string]interface{}{
				"signal_total_24h": int64(1),
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (fortuna fallback) error: %v", err)
	}
	if !matched2 {
		t.Fatalf("expected match via fortuna fallback")
	}
}

func TestRuntimeEscapeClassSignalsRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "runtime-escape-class-signals")
	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"securityState": map[string]interface{}{
				"has_escape_related": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected match with securityState only")
	}

	matched2, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"fortuna": map[string]interface{}{
				"has_escape_related": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (fortuna fallback) error: %v", err)
	}
	if !matched2 {
		t.Fatalf("expected match via fortuna fallback")
	}
}

func TestClusterAdminPodRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "cluster-admin-pod")
	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"spec": map[string]interface{}{},
			"securityState": map[string]interface{}{
				"service_account_bound_to_cluster_admin": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected match with securityState only")
	}

	matched2, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"spec": map[string]interface{}{},
			"fortuna": map[string]interface{}{
				"service_account_bound_to_cluster_admin": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (fortuna fallback) error: %v", err)
	}
	if !matched2 {
		t.Fatalf("expected match via fortuna fallback")
	}
}

func TestToxicComboClusterAdminHostNetworkRuntimeRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "toxic-combo-clusteradmin-hostnetwork-runtime")
	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"securityState": map[string]interface{}{
				"service_account_bound_to_cluster_admin": true,
				"host_network":                           true,
				"signal_total_24h":                       int64(3),
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected toxic-combo rule to match with securityState")
	}
}

func TestToxicComboClusterAdminEscapeRuntimeRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "toxic-combo-clusteradmin-escape-runtime")
	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"securityState": map[string]interface{}{
				"service_account_bound_to_cluster_admin": true,
				"has_escape_related":                     true,
				"signal_total_24h":                       int64(2),
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected toxic combo clusteradmin+escape to match with securityState")
	}
}

func TestToxicComboHostNamespacesEscapeRuntimeRule_UsesSecurityState(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	expr := ruleExpressionForID(t, ye, "toxic-combo-hostns-escape-runtime")
	matched, err := ye.celCompiler.Evaluate(expr, map[string]interface{}{
		"object": map[string]interface{}{
			"kind": "Pod",
			"securityState": map[string]interface{}{
				"host_network":       true,
				"has_escape_related": true,
				"signal_total_24h":   int64(1),
			},
		},
	})
	if err != nil {
		t.Fatalf("CEL eval (securityState only) error: %v", err)
	}
	if !matched {
		t.Fatalf("expected toxic combo hostns+escape to match with securityState")
	}
}

func ruleExpressionForID(t *testing.T, ye *YAMLEngine, ruleID string) string {
	t.Helper()
	for _, r := range ye.rules {
		if r.ID != ruleID {
			continue
		}
		for _, c := range r.Conditions {
			if c.Type == CondTypeExpression && strings.TrimSpace(c.Expression) != "" {
				return c.Expression
			}
		}
		t.Fatalf("rule expression not found for %s", ruleID)
	}
	t.Fatalf("rule not found: %s", ruleID)
	return ""
}

func TestYAMLEngineClusterAdminPodRule_WithClusterRoleBinding(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.ClusterRoleBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&models.Cluster{ID: "c1", Name: "c1"}).Error; err != nil {
		t.Fatal(err)
	}
	podUID := "33333333-3333-3333-3333-333333333333"
	if err := db.Create(&models.Pod{
		ClusterID: "c1", Name: "vip", Namespace: "prod", ServiceAccount: "power-sa",
		UID: podUID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	crb := models.ClusterRoleBinding{
		ClusterID: "c1", Name: "power-sa-admin", UID: "crb-1",
		RoleRef:  `{"kind":"ClusterRole","name":"cluster-admin"}`,
		Subjects: `[{"kind":"ServiceAccount","name":"power-sa","namespace":"prod"}]`,
	}
	if err := db.Create(&crb).Error; err != nil {
		t.Fatal(err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"kind":"Pod","metadata":{"namespace":"prod","name":"vip"},"spec":{"serviceAccountName":"power-sa","containers":[{"name":"c","image":"nginx"}]}}`
	insights, err := ye.EvaluateResource(context.Background(), "Pod", map[string]interface{}{
		"kind": "Pod", "uid": podUID, "name": "vip", "namespace": "prod", "cluster_id": "c1", "raw_json": raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range insights {
		if in.Title == "Pod using ServiceAccount with cluster-admin access" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cluster-admin pod insight, got titles: %v", insightTitles(insights))
	}
}

func TestYAMLEnginePssHostNamespacesRule(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	podUID := "44444444-4444-4444-4444-444444444444"
	if err := db.Create(&models.Pod{
		ClusterID: "c1", Name: "hn", Namespace: "ns", ServiceAccount: "default",
		UID: podUID, HostNetwork: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"kind":"Pod","metadata":{"namespace":"ns","name":"hn"},"spec":{"hostNetwork":true,"containers":[{"name":"c","image":"nginx"}]}}`
	insights, err := ye.EvaluateResource(context.Background(), "Pod", map[string]interface{}{
		"kind": "Pod", "uid": podUID, "name": "hn", "namespace": "ns", "raw_json": raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range insights {
		if in.Title == "Pod uses host namespaces (hostNetwork, hostPID, and/or hostIPC)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected PSS host-namespaces insight, got: %v", insightTitles(insights))
	}
}

func insightTitles(insights []*models.Insight) []string {
	var s []string
	for _, in := range insights {
		s = append(s, in.Title)
	}
	return s
}

func TestYAMLEnginePodRuntimeCorrelationRules(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skip("rules dir not found:", rulesDir)
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeSignal{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	podUID := "22222222-2222-2222-2222-222222222222"
	if err := db.Create(&models.Pod{
		ClusterID: "c1", Name: "p2", Namespace: "ns", ServiceAccount: "sa",
		UID: podUID, HostNetwork: true,
	}).Error; err != nil {
		t.Fatalf("pod: %v", err)
	}
	if err := db.Create(&models.RuntimeSignal{
		PodUID: podUID, SignalType: "NETWORK_QUEUE_ANOMALY", Category: "NETWORK",
		Evidence: "{}", CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("signal: %v", err)
	}

	ye, err := NewYAMLEngine(db, rulesDir)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	raw := `{"kind":"Pod","metadata":{"namespace":"ns","name":"p2"},"spec":{"containers":[{"name":"c","image":"nginx"}]}}`
	insights, err := ye.EvaluateResource(context.Background(), "Pod", map[string]interface{}{
		"kind": "Pod", "uid": podUID, "name": "p2", "namespace": "ns", "raw_json": raw,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	foundHostNet := false
	foundRecent := false
	for _, in := range insights {
		switch in.Title {
		case "Host network pod with network anomaly signal":
			foundHostNet = true
		case "Runtime behavior signals observed for this pod":
			foundRecent = true
		}
	}
	if !foundHostNet {
		t.Fatalf("expected hostnetwork+anomaly insight, got %d insights", len(insights))
	}
	if !foundRecent {
		t.Fatalf("expected recent-signals insight, got %d insights", len(insights))
	}
}
