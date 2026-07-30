package graph

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestFilterPodCapabilitiesForCKDB_respectsPathFlow(t *testing.T) {
	nodes := `[{"id":"SA_TOKEN","type":"` + NodeTypeCapability + `","properties":{}}]`
	paths := []models.AttackPath{
		{Nodes: nodes, Edges: "[]", PathID: "p0", PodUID: "u1"},
	}
	caps := []models.PodCapability{
		{CapabilityID: "SA_TOKEN"},
		{CapabilityID: "ORPHAN_CAP_NOT_ON_PATH"},
	}
	out := FilterPodCapabilitiesForCKDB(paths, caps)
	if len(out) != 1 || out[0] != "SA_TOKEN" {
		t.Fatalf("want only on-path cap, got %#v", out)
	}
}

func TestFilterPodCapabilitiesForCKDB_emptyPathFallsBackToAll(t *testing.T) {
	caps := []models.PodCapability{
		{CapabilityID: "A"},
		{CapabilityID: "B"},
	}
	out := FilterPodCapabilitiesForCKDB(nil, caps)
	if len(out) != 2 {
		t.Fatalf("want all caps when no paths, got %#v", out)
	}
}
