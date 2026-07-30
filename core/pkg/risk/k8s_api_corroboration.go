package risk

import (
	"time"

	"github.com/fortuna/core/pkg/k8scorroboration"
	"github.com/fortuna/core/pkg/models"
)

// CorroboratesK8sAPIAccess delegates to k8scorroboration (shared with graph enrichment).
func CorroboratesK8sAPIAccess(events []models.RuntimeEvent, signals []models.RuntimeSignal) bool {
	return k8scorroboration.CorroboratesK8sAPIAccess(events, signals)
}

func runtimeSatisfiedRequirements(events []models.RuntimeEvent, signals []models.RuntimeSignal) []string {
	w := k8scorroboration.SATokenSatisfactionWeight(events, signals, time.Now().UTC(), k8scorroboration.DefaultSATokenSatisfactionHalfLife)
	if w >= 0.28 {
		return []string{"SA_TOKEN"}
	}
	return nil
}
