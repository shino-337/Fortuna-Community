package authorization_test

import (
	"encoding/json"
	"testing"

	"github.com/fortuna/core/pkg/authorization"
)

func TestRedactViewerValue_StripsContainersAndPaths(t *testing.T) {
	in := map[string]interface{}{
		"name": "p",
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "c",
					"image": "nginx",
					"env":   []interface{}{map[string]interface{}{"name": "SECRET", "value": "x"}},
				},
			},
		},
		"paths": []interface{}{map[string]interface{}{"step": "exploit"}},
	}
	out := authorization.RedactViewerValue(in)
	b, _ := json.Marshal(out)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	spec := m["spec"].(map[string]interface{})
	if spec["containers"] != "[REDACTED]" {
		t.Fatalf("containers not redacted: %v", spec["containers"])
	}
	if m["paths"] != "[REDACTED]" {
		t.Fatalf("paths not redacted: %v", m["paths"])
	}
}

func TestRedactViewerValue_SensitiveKeys(t *testing.T) {
	in := map[string]interface{}{"password": "secret123", "ok": "v"}
	out := authorization.RedactViewerValue(in).(map[string]interface{})
	if out["password"] != "[REDACTED]" {
		t.Fatalf("got %v", out["password"])
	}
	if out["ok"] != "v" {
		t.Fatal("non-sensitive changed")
	}
}
