package nvd

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestParseCPE(t *testing.T) {
	tests := []struct {
		cpe                      string
		wantVendor, wantProduct  string
		wantEco                  string
	}{
		{"cpe:2.3:a:openssl:openssl:1.1.1:*:*:*:*:*:*:*", "openssl", "openssl", "nvd"},
		{"cpe:2.3:a:debian:apt:1.0:*:*:*:*:*:*:*", "debian", "apt", "debian"},
		{"cpe:2.3:a:golang:go:1.20:*:*:*:*:*:*:*", "golang", "go", "go"},
		{"cpe:2.3:a:djangoproject:django:3.2:*:*:*:*:*:*:*", "djangoproject", "django", "pypi"},
		{"cpe:2.3:a:canonical:ubuntu_linux:22.04:*:*:*:*:*:*:*", "canonical", "ubuntu_linux", "ubuntu"},
		{"", "", "", ""},
	}

	for _, tt := range tests {
		v, p, e := parseCPE(tt.cpe)
		if v != tt.wantVendor || p != tt.wantProduct || e != tt.wantEco {
			t.Errorf("parseCPE(%q) = (%q,%q,%q), want (%q,%q,%q)",
				tt.cpe, v, p, e, tt.wantVendor, tt.wantProduct, tt.wantEco)
		}
	}
}

func TestExtractCVSS(t *testing.T) {
	m := &Metrics{
		CvssMetricV31: []CVSSMetric{{
			CVSSData: CVSSData{
				Version:      "3.1",
				VectorString: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
				BaseScore:    9.8,
				BaseSeverity: "CRITICAL",
			},
		}},
	}

	score, vector, version, severity := extractCVSS(m)
	if score != 9.8 {
		t.Errorf("score = %f, want 9.8", score)
	}
	if vector != "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H" {
		t.Errorf("vector = %q", vector)
	}
	if version != "3.1" {
		t.Errorf("version = %q", version)
	}
	if severity != "CRITICAL" {
		t.Errorf("severity = %q", severity)
	}
}

func TestExtractCVSSFallback(t *testing.T) {
	score, _, _, sev := extractCVSS(nil)
	if score != 0 || sev != "MEDIUM" {
		t.Errorf("nil metrics: score=%f sev=%q", score, sev)
	}
}

func TestExtractCWEIDs(t *testing.T) {
	ws := []Weakness{{
		Description: []LangString{
			{Lang: "en", Value: "CWE-79"},
			{Lang: "en", Value: "CWE-89"},
		},
	}}
	got := extractCWEIDs(ws)
	want := pq.StringArray{"CWE-79", "CWE-89"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildRefsJSON(t *testing.T) {
	refs := []Reference{
		{URL: "https://example.com/advisory", Tags: []string{"advisory"}},
	}
	got := buildRefsJSON(refs)
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(arr))
	}
}

func TestParseTime(t *testing.T) {
	tests := []string{
		"2024-01-15T10:00:00.000",
		"2024-01-15T10:00:00Z",
		"2024-01-15T10:00:00.000Z",
	}
	for _, ts := range tests {
		got := parseTime(ts)
		if got == nil {
			t.Errorf("parseTime(%q) = nil", ts)
			continue
		}
		if got.Year() != 2024 || got.Month() != time.January || got.Day() != 15 {
			t.Errorf("parseTime(%q) = %v", ts, got)
		}
	}
}

func TestScoreToSeverity(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{9.8, "CRITICAL"},
		{7.5, "HIGH"},
		{5.0, "MEDIUM"},
		{2.0, "LOW"},
		{0.0, "LOW"},
	}
	for _, tt := range tests {
		if got := scoreToSeverity(tt.score); got != tt.want {
			t.Errorf("scoreToSeverity(%f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestExtractEnglish(t *testing.T) {
	descs := []LangString{
		{Lang: "es", Value: "descripción en español"},
		{Lang: "en", Value: "english description"},
	}
	if got := extractEnglish(descs); got != "english description" {
		t.Errorf("got %q", got)
	}
}
