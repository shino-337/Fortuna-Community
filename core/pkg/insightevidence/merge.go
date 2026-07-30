// Package insightevidence merges structured JSON blobs for Insight.evidence (EPSS, KEV, etc.).
package insightevidence

import (
	"encoding/json"
	"strings"
)

// Merge parses base JSON (if any) and overlays patch keys. Empty base is ok.
func Merge(base string, patch map[string]interface{}) string {
	if patch == nil {
		patch = map[string]interface{}{}
	}
	m := make(map[string]interface{})
	base = strings.TrimSpace(base)
	if base != "" {
		_ = json.Unmarshal([]byte(base), &m)
	}
	for k, v := range patch {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		return base
	}
	return string(b)
}
