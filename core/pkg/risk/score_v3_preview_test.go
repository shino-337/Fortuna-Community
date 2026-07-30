package risk

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestBuildScoreV3PreviewMap(t *testing.T) {
	m := buildScoreV3PreviewMap(&models.AssetSecurityState{
		HostNetwork: true, SignalTotal24h: 10,
	})
	total, ok := m["total"].(float64)
	if !ok || total <= 0 {
		t.Fatalf("expected total: %#v", m["total"])
	}
	by, ok := m["byDimension"].(map[string]float64)
	if !ok || len(by) < 1 {
		t.Fatalf("byDimension: %#v", m["byDimension"])
	}
}
