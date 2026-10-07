package api

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestAttackPathNotificationRouteKeepsClusterAndPath(t *testing.T) {
	got := attackPathNotificationRoute(models.AttackPath{ClusterID: "prod-eu-1", PathID: "ap_3f9c1e7a0b2d4e5f"})
	if want := "/attack-paths?clusterId=prod-eu-1&path=ap_3f9c1e7a0b2d4e5f"; got != want {
		t.Fatalf("route = %q, want %q", got, want)
	}
	if got := attackPathNotificationRoute(models.AttackPath{PathID: "ap_1"}); got != "/attack-paths?path=ap_1" {
		t.Fatalf("route without cluster = %q", got)
	}
}
