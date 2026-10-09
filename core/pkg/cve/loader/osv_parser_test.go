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
		"Red Hat:enterprise_linux:9::baseos": "9",
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

func TestAffectedRanges(t *testing.T) {
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
	type row struct{ eco, rel, name, start, endEx, endIn string }
	var rows []row
	for _, r := range affectedRanges(doc) {
		rows = append(rows, row{r.Ecosystem, r.Release, r.PackageName, r.Introduced, r.Fixed, r.LastAffected})
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
}

func TestRedHatEcosystemAndRelease(t *testing.T) {
	for eco, want := range map[string]string{
		"Red Hat:enterprise_linux:9::appstream":                "9",
		"Red Hat:enterprise_linux:8":                           "8",
		"Red Hat:rhel_eus:9.4::baseos":                         "9",
		"Red Hat:enterprise_linux_eus:10.0":                    "10",
		"Red Hat:openshift:4.14::el9":                          "9",
		"Red Hat:jboss_enterprise_application_platform:7::el8": "8",
		"Red Hat:devtools:2020":                                "",
		"Red Hat:hummingbird:1":                                "",
	} {
		if got := OSVEcosystemRelease(eco); got != want {
			t.Errorf("OSVEcosystemRelease(%q) = %q, want %q", eco, got, want)
		}
		if got := normalizeEcosystem(eco); got != "redhat" {
			t.Errorf("normalizeEcosystem(%q) = %q, want redhat", eco, got)
		}
	}
	if got := NormalizeDistroRelease("redhat", "9.4"); got != "9" {
		t.Errorf("NormalizeDistroRelease(redhat, 9.4) = %q", got)
	}
}
