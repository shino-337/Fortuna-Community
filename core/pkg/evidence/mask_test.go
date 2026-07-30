package evidence

import (
	"strings"
	"testing"
)

func TestMaskSensitiveInJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCont string
	}{
		{"empty", "", ""},
		{"empty object", "{}", "{}"},
		{"password redacted", `{"user":"x","password":"secret123"}`, "[REDACTED]"},
		{"token redacted", `{"token":"abc"}`, "[REDACTED]"},
		{"nested", `{"a":{"api_key":"xyz"}}`, "[REDACTED]"},
		{"array of objects", `[{"password":"p"}]`, "[REDACTED]"},
		{"case insensitive", `{"PASSWORD":"x","Token":"y"}`, "[REDACTED]"},
		{"non-sensitive kept", `{"name":"n","value":1}`, "name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSensitiveInJSON(tt.input)
			if tt.input == "" && got == "" {
				return
			}
			if tt.wantCont != "" && !strings.Contains(got, tt.wantCont) {
				t.Errorf("MaskSensitiveInJSON() = %q, want containing %q", got, tt.wantCont)
			}
		})
	}
}
