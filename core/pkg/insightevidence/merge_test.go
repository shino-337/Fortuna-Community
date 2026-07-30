package insightevidence

import (
	"strings"
	"testing"
)

func TestMerge(t *testing.T) {
	s := Merge(`{"a":1}`, map[string]interface{}{
		"b": 2,
		"a": 3,
	})
	if s == "" {
		t.Fatal("empty")
	}
	if !strings.Contains(s, `"a":3`) || !strings.Contains(s, `"b":2`) {
		t.Fatalf("%s", s)
	}
}
