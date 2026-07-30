package evidence

import (
	"encoding/json"
	"regexp"
	"strings"
)

// SensitiveKeyPattern matches keys that should have values redacted (case-insensitive).
var SensitiveKeyPattern = regexp.MustCompile(`(?i)^(password|passwd|pwd|token|secret|api_key|apikey|auth_header|authorization|private_key|credential)$`)

// MaskSensitiveInJSON redacts values for keys matching SensitiveKeyPattern in JSON (e.g. evidence, violated_rules).
// Returns the masked JSON string, or original if unmarshal fails (to avoid breaking save).
func MaskSensitiveInJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	masked := maskValue(v)
	b, err := json.Marshal(masked)
	if err != nil {
		return raw
	}
	return string(b)
}

func maskValue(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			if SensitiveKeyPattern.MatchString(k) {
				out[k] = "[REDACTED]"
			} else {
				out[k] = maskValue(val)
			}
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, val := range x {
			out[i] = maskValue(val)
		}
		return out
	default:
		return v
	}
}
