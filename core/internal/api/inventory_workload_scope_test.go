package api

import (
	"encoding/json"
	"github.com/fortuna/core/pkg/models"
	"testing"
	"time"
)

func TestInventoryWorkloadCapabilityScope(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	if err := db.AutoMigrate(&models.Deployment{}, &models.ReplicaSet{}, &models.Pod{}, &models.PodCapability{}); err != nil {
		t.Fatal(err)
	}
	for _, cl := range []string{"a", "b"} {
		for _, row := range []any{
			&models.Deployment{UID: "dep-" + cl, ClusterID: cl, Name: "shared", Namespace: "shared"},
			&models.ReplicaSet{UID: "rs-" + cl, ClusterID: cl, Name: "shared", Namespace: "shared"},
			&models.Pod{UID: "pod-" + cl, ClusterID: cl, Name: "shared", Namespace: "shared"},
			&models.PodCapability{PodUID: "pod-" + cl, Namespace: "shared", CapabilityID: "TEST", Severity: "high", CreatedAt: time.Now().UTC()},
		} {
			if err := db.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	paths := []string{"/inventory/deployments", "/inventory/replicasets", "/inventory/pod-capabilities", "/inventory/pod-capabilities/summary", "/inventory/pod-capabilities/summary/cluster", "/inventory/pod-capabilities/summary/namespace", "/inventory/pod-capabilities/summary/capability", "/inventory/pod-capabilities/summary/severity", "/inventory/pod-capabilities/trends"}
	for _, path := range paths {
		for _, tc := range []struct {
			suffix, who string
			code        int
		}{{"", "a", 200}, {"?clusterId=b", "a", 403}, {"?cluster=b", "a", 403}, {"?cluster=a&clusterId=b", "admin", 400}, {"", "missing", 401}} {
			w := request("GET", path+tc.suffix, tc.who, "")
			if w.Code != tc.code {
				t.Errorf("%s %+v: %d %s", path, tc, w.Code, w.Body)
			}
		}
		w := request("GET", path, "a", "")
		var out map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if path == "/inventory/pod-capabilities/trends" {
			var points []struct{ High int }
			json.Unmarshal(out["points"], &points)
			count := 0
			for _, p := range points {
				count += p.High
			}
			if count != 1 {
				t.Errorf("trend leaked: %s", w.Body)
			}
		} else {
			var total int
			json.Unmarshal(out["total"], &total)
			if total != 1 {
				t.Errorf("scope count: %s %s", path, w.Body)
			}
		}
	}
	for _, kind := range []string{"deployments", "replicasets"} {
		for _, tc := range []struct {
			tail string
			code int
		}{{"/1", 200}, {"/2", 404}, {"/invalid", 400}, {"?page=9223372036854775807", 200}} {
			w := request("GET", "/inventory/"+kind+tc.tail, "a", "")
			if w.Code != tc.code {
				t.Errorf("%s %+v: %d %s", kind, tc, w.Code, w.Body)
			}
		}
	}
	if err := db.Migrator().DropTable(&models.PodCapability{}); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/inventory/pod-capabilities", "a", ""); w.Code != 503 {
		t.Fatalf("missing schema: %d %s", w.Code, w.Body)
	}
}
