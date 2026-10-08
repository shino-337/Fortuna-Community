package loader

import (
	"reflect"
	"testing"
)

func TestOSVRangeIntervals(t *testing.T) {
	cases := []struct {
		name   string
		events []OSVEvent
		want   []OSVInterval
	}{
		{
			name:   "introduced then fixed",
			events: []OSVEvent{{Introduced: "0"}, {Fixed: "1.2"}},
			want:   []OSVInterval{{Introduced: "0", Fixed: "1.2"}},
		},
		{
			// Before, the second interval lost its lower bound and matched 1.x too.
			name:   "two intervals keep their lower bounds",
			events: []OSVEvent{{Introduced: "0"}, {Fixed: "1.2"}, {Introduced: "2.0"}, {LastAffected: "2.3"}},
			want:   []OSVInterval{{Introduced: "0", Fixed: "1.2"}, {Introduced: "2.0", LastAffected: "2.3"}},
		},
		{
			// Before, an advisory with no fix produced no row and never matched.
			name:   "no fix yet",
			events: []OSVEvent{{Introduced: "0"}},
			want:   []OSVInterval{{Introduced: "0"}},
		},
		{
			name:   "open interval after a fixed one",
			events: []OSVEvent{{Introduced: "1.0"}, {Fixed: "1.5"}, {Introduced: "3.0"}},
			want:   []OSVInterval{{Introduced: "1.0", Fixed: "1.5"}, {Introduced: "3.0"}},
		},
	}
	for _, tc := range cases {
		if got := OSVRangeIntervals(tc.events); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestOSVEcosystemRelease(t *testing.T) {
	cases := map[string]string{
		"Debian:12":                          "12",
		"Debian:11":                          "11",
		"Alpine:v3.20":                       "3.20",
		"Ubuntu:22.04:LTS":                   "22.04",
		"Ubuntu:Pro:18.04:LTS":               "18.04",
		"Ubuntu:24.10":                       "24.10",
		"Rocky Linux:8":                      "8",
		"AlmaLinux:9":                        "9",
		"Debian":                             "",
		"PyPI":                               "",
		"Red Hat:enterprise_linux:9::baseos": "",
	}
	for in, want := range cases {
		if got := OSVEcosystemRelease(in); got != want {
			t.Errorf("OSVEcosystemRelease(%q) = %q, want %q", in, got, want)
		}
	}
	if got := NormalizeDistroRelease("alpine", "3.20.3"); got != "3.20" {
		t.Errorf("alpine 3.20.3 → %q", got)
	}
	if got := NormalizeDistroRelease("debian", "12.7"); got != "12" {
		t.Errorf("debian 12.7 → %q", got)
	}
}

func TestConvertToPackageVulnerabilities(t *testing.T) {
	doc := &OSVVulnerability{
		ID: "DEBIAN-CVE-2025-0001",
		Affected: []OSVAffected{
			{
				Package: OSVPackage{Ecosystem: "Debian:11", Name: "openssl"},
				Ranges:  []OSVRange{{Type: "ECOSYSTEM", Events: []OSVEvent{{Introduced: "0"}, {Fixed: "1.1.1w-0+deb11u2"}}}},
			},
			{
				Package: OSVPackage{Ecosystem: "Debian:12", Name: "openssl"},
				Ranges:  []OSVRange{{Type: "ECOSYSTEM", Events: []OSVEvent{{Introduced: "0"}}}},
			},
			{
				// Only a GIT range: the explicit versions are what can be matched.
				Package:  OSVPackage{Ecosystem: "PyPI", Name: "demo"},
				Ranges:   []OSVRange{{Type: "GIT", Events: []OSVEvent{{Introduced: "0"}, {Fixed: "abc123"}}}},
				Versions: []string{"1.0", "1.1", "1.0"},
			},
		},
	}
	got, err := ConvertToPackageVulnerabilities(doc)
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ eco, rel, name, start, endEx, endIn string }
	var rows []row
	for _, pv := range got {
		rows = append(rows, row{pv.Ecosystem, pv.EcosystemRelease, pv.PackageName, pv.VersionStartIncluding, pv.VersionEndExcluding, pv.VersionEndIncluding})
	}
	want := []row{
		{"debian", "11", "openssl", "0", "1.1.1w-0+deb11u2", ""},
		{"debian", "12", "openssl", "0", "", ""},
		{"pypi", "", "demo", "1.0", "", "1.0"},
		{"pypi", "", "demo", "1.1", "", "1.1"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows:\n got %+v\nwant %+v", rows, want)
	}

	doc.Withdrawn = "2025-06-01T00:00:00Z"
	got, _ = ConvertToPackageVulnerabilities(doc)
	if len(got) != 0 {
		t.Fatalf("withdrawn advisory produced %d rows", len(got))
	}
}
