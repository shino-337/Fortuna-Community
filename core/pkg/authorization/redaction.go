package authorization

import (
	"encoding/json"
	"regexp"
	"strings"
)

// viewerSensitiveKey matches JSON/object keys whose values must not be exposed to viewer role (RBAC.md §14).
var viewerSensitiveKey = regexp.MustCompile(`(?i)^(password|passwd|pwd|token|secret|api_?key|authorization|auth|credential|private_?key|bearer|kubeconfig|tls|clientcertificate|clientkey|bootstrap|serviceaccounttoken)$`)

// viewerStructuralStripKeys removes entire subtrees for viewer (env, commands, full spec blobs).
var viewerStructuralStripKeys = map[string]struct{}{
	"env":                       {},
	"envfrom":                   {},
	"args":                      {},
	"command":                   {},
	"containers":                {},
	"initcontainers":            {},
	"volumes":                   {},
	"volumemounts":              {},
	"podsecuritycontext":        {},
	"containersecuritycontexts": {},
	"tolerations":               {},
	"affinity":                  {},
	"imagedigests":              {},
	"paths":                     {},
	"links":                     {},
	"edges":                     {},
	"results":                   {},
}

// RedactViewerValue returns a deep-copied JSON value safe for the viewer role.
func RedactViewerValue(v interface{}) interface{} {
	return RedactViewerValuePreservingStructuralKeys(v, nil)
}

// RedactViewerValuePreservingStructuralKeys is used by graph-style APIs where
// keys such as edges/links/paths are the public data model, not sensitive spec
// subtrees. Callers should keep this scoped to endpoints with explicit graph
// permission checks.
func RedactViewerValuePreservingStructuralKeys(v interface{}, preserve map[string]struct{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var root interface{}
	if err := json.Unmarshal(b, &root); err != nil {
		return v
	}
	return redactWalk(root, preserve)
}

func redactWalk(v interface{}, preserve map[string]struct{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			kNorm := strings.ToLower(strings.TrimSpace(k))
			if _, strip := viewerStructuralStripKeys[kNorm]; strip {
				if _, keep := preserve[kNorm]; keep {
					out[k] = redactWalk(val, preserve)
					continue
				}
				out[k] = "[REDACTED]"
				continue
			}
			if viewerSensitiveKey.MatchString(k) {
				out[k] = "[REDACTED]"
				continue
			}
			// Heuristic: nested env arrays
			if kNorm == "items" {
				if arr, ok := val.([]interface{}); ok {
					out[k] = redactItemsArray(arr, preserve)
					continue
				}
			}
			out[k] = redactWalk(val, preserve)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, val := range x {
			out[i] = redactWalk(val, preserve)
		}
		return out
	default:
		return v
	}
}

func redactItemsArray(arr []interface{}, preserve map[string]struct{}) []interface{} {
	out := make([]interface{}, len(arr))
	for i, el := range arr {
		if m, ok := el.(map[string]interface{}); ok {
			cm := make(map[string]interface{}, len(m))
			for k, val := range m {
				kn := strings.ToLower(strings.TrimSpace(k))
				if kn == "args" || kn == "command" || kn == "env" || kn == "envfrom" || kn == "cmdline" || kn == "commandline" || kn == "argv" {
					cm[k] = "[REDACTED]"
					continue
				}
				if viewerSensitiveKey.MatchString(k) {
					cm[k] = "[REDACTED]"
					continue
				}
				cm[k] = redactWalk(val, preserve)
			}
			out[i] = cm
			continue
		}
		out[i] = redactWalk(el, preserve)
	}
	return out
}
