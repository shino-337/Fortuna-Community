package cvss

import "testing"

func TestBaseScore(t *testing.T) {
	cases := []struct {
		vector  string
		score   float64
		version string
		sev     string
	}{
		// The vector the old approximation stored as 9.0 CRITICAL.
		{"CVSS:3.1/AV:L/AC:L/PR:L/UI:R/S:U/C:H/I:H/A:H", 7.3, "3.1", High},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 9.8, "3.1", Critical},
		{"CVSS:3.1/AV:N/AC:L/PR:L/UI:N/S:C/C:H/I:L/A:N", 8.5, "3.1", High},
		{"CVSS:3.0/AV:L/AC:H/PR:N/UI:R/S:C/C:H/I:H/A:H", 7.7, "3.0", High},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N", 0, "3.1", Negligible},
		{"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N", 9.3, "4.0", Critical},
		{"CVSS:4.0/AV:L/AC:L/AT:N/PR:L/UI:N/VC:H/VI:N/VA:N/SC:N/SI:N/SA:N", 6.8, "4.0", Medium},
		{"AV:N/AC:L/Au:N/C:P/I:P/A:P", 7.5, "2.0", High},
	}
	for _, c := range cases {
		score, version, ok := BaseScore(c.vector)
		if !ok || score != c.score || version != c.version {
			t.Errorf("BaseScore(%q) = %v, %q, %v; want %v, %q", c.vector, score, version, ok, c.score, c.version)
			continue
		}
		sev := SeverityFromScore(score)
		if version == "2.0" {
			sev = SeverityFromV2Score(score)
		}
		if sev != c.sev {
			t.Errorf("severity of %q = %s, want %s", c.vector, sev, c.sev)
		}
	}
	for _, bad := range []string{"", "CVSS:3.1/AV:X", "garbage", "CVSS:3.1/AV:N/AC:L"} {
		if _, _, ok := BaseScore(bad); ok {
			t.Errorf("BaseScore(%q) accepted an invalid vector", bad)
		}
	}
}

func TestNormalizeRating(t *testing.T) {
	cases := map[string]string{
		"MODERATE":         Medium,
		"Important":        High,
		"low**":            Low,
		"unimportant":      Negligible,
		"negligible":       Negligible,
		"not yet assigned": "",
		"end-of-life":      "",
		"CRITICAL":         Critical,
		"":                 "",
	}
	for in, want := range cases {
		if got := NormalizeRating(in); got != want {
			t.Errorf("NormalizeRating(%q) = %q, want %q", in, got, want)
		}
	}
}
