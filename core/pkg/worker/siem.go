package worker

import (
	"encoding/json"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"github.com/nats-io/nats.go"
)

// PublishSIEMEvents publishes critical/high insights to fortuna.siem.events for SIEM adapters.
func PublishSIEMEvents(js nats.JetStreamContext, insights []*models.Insight) {
	if js == nil || len(insights) == 0 {
		return
	}
	for _, i := range insights {
		if i == nil {
			continue
		}
		sev := strings.ToLower(strings.TrimSpace(i.Severity))
		if sev != "critical" && sev != "high" {
			continue
		}
		payload := map[string]interface{}{
			"insightId": i.ID, "severity": i.Severity, "title": i.Title,
			"resourceUid": i.ResourceUID, "resourceType": i.ResourceType,
			"resourceName": i.ResourceName, "resourceNamespace": i.ResourceNamespace,
			"insightType": i.InsightType, "cveId": i.CVEID, "detectedAt": i.DetectedAt,
		}
		if b, err := json.Marshal(payload); err == nil {
			_, _ = js.Publish(SubjectSIEMEvents, b)
		}
	}
}
