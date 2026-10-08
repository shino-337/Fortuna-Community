package loader

import (
	"testing"

	"github.com/fortuna/core/pkg/cve/cvss"
)

func TestBuildAdvisory(t *testing.T) {
	raw := []byte(`{
	  "id": "RHSA-2024:1234",
	  "modified": "2024-05-01T00:00:00Z",
	  "aliases": ["GHSA-aaaa-bbbb-cccc"],
	  "upstream": ["CVE-2024-0001", "CVE-2024-0002", "GHSA-aaaa-bbbb-cccc"],
	  "related": ["CVE-2024-0001", "ALSA-2024:1234"],
	  "severity": [
	    {"type": "CVSS_V3", "score": "CVSS:3.1/AV:L/AC:L/PR:L/UI:R/S:U/C:H/I:H/A:H"},
	    {"type": "CVSS_V4", "score": "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N"}
	  ],
	  "affected": [
	    {"package": {"ecosystem": "Red Hat:enterprise_linux:9::appstream", "name": "openssl"},
	     "ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}, {"fixed": "1:3.0.7-27.el9"}]}]},
	    {"package": {"ecosystem": "Red Hat:enterprise_linux:9::baseos", "name": "openssl"},
	     "ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}, {"fixed": "1:3.0.7-27.el9"}]}]},
	    {"package": {"ecosystem": "Red Hat:enterprise_linux:8::baseos", "name": "openssl"},
	     "ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}, {"fixed": "1:1.1.1k-12.el8"}]}]}
	  ]
	}`)
	osv, err := ParseBytes(raw, "test")
	if err != nil {
		t.Fatal(err)
	}
	adv := BuildAdvisory(osv, raw)

	if adv.Source != "redhat" || adv.Kind != "vulnerability" {
		t.Errorf("source/kind = %s/%s", adv.Source, adv.Kind)
	}
	if adv.CVSSv3Score == nil || *adv.CVSSv3Score != 7.3 || adv.CVSSv4Score == nil || *adv.CVSSv4Score != 9.3 {
		t.Errorf("cvss = %v / %v", adv.CVSSv3Score, adv.CVSSv4Score)
	}
	wantRefs := []AdvisoryRef{
		{"GHSA-aaaa-bbbb-cccc", "alias", "ghsa"},
		{"CVE-2024-0001", "upstream", "cve"},
		{"CVE-2024-0002", "upstream", "cve"},
		{"ALSA-2024:1234", "related", "advisory"},
	}
	if len(adv.Refs) != len(wantRefs) {
		t.Fatalf("refs = %+v", adv.Refs)
	}
	for i, r := range wantRefs {
		if adv.Refs[i] != r {
			t.Errorf("ref %d = %+v, want %+v", i, adv.Refs[i], r)
		}
	}
	// appstream and baseos of RHEL 9 collapse into one range.
	if len(adv.Affected) != 2 {
		t.Fatalf("affected = %+v", adv.Affected)
	}
	if a := adv.Affected[0]; a.Ecosystem != "redhat" || a.Release != "9" || a.Fixed != "1:3.0.7-27.el9" || a.Introduced != "0" {
		t.Errorf("affected[0] = %+v", a)
	}
	if adv.Affected[1].Release != "8" {
		t.Errorf("affected[1] = %+v", adv.Affected[1])
	}

	again := BuildAdvisory(osv, raw)
	if again.ContentSHA256 != adv.ContentSHA256 || len(adv.ContentSHA256) != 64 {
		t.Errorf("content hash not stable: %s vs %s", adv.ContentSHA256, again.ContentSHA256)
	}
	changed := BuildAdvisory(osv, append([]byte(" "), raw...))
	if changed.ContentSHA256 == adv.ContentSHA256 {
		t.Error("content hash ignores the file content")
	}
}

func TestBuildAdvisoryRatings(t *testing.T) {
	raw := []byte(`{
	  "id": "DEBIAN-CVE-2024-1111",
	  "upstream": ["CVE-2024-1111"],
	  "affected": [
	    {"package": {"ecosystem": "Debian:12", "name": "curl"}, "ecosystem_specific": {"urgency": "low"},
	     "ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}, {"fixed": "7.88.1-10+deb12u6"}]}]},
	    {"package": {"ecosystem": "Debian:11", "name": "curl"}, "ecosystem_specific": {"urgency": "unimportant"},
	     "ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}]}]},
	    {"package": {"ecosystem": "Debian:13", "name": "curl"}, "ecosystem_specific": {"urgency": "not yet assigned"},
	     "ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}]}]}
	  ]
	}`)
	osv, err := ParseBytes(raw, "test")
	if err != nil {
		t.Fatal(err)
	}
	adv := BuildAdvisory(osv, raw)
	if adv.SourceSeverity != cvss.Low {
		t.Errorf("source severity = %q, want LOW (highest urgency)", adv.SourceSeverity)
	}
	got := map[string]string{}
	for _, a := range adv.Affected {
		got[a.Release] = a.VendorSeverity
	}
	if got["12"] != cvss.Low || got["11"] != cvss.Negligible || got["13"] != "" {
		t.Errorf("vendor severities = %v", got)
	}

	// No CVSS: the legacy row keeps MEDIUM but no made-up score, and the vendor rating wins.
	legacy, err := ConvertToCVE(osv)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Severity != cvss.Low || legacy.CVSSScore != 0 {
		t.Errorf("legacy severity = %s %.1f", legacy.Severity, legacy.CVSSScore)
	}

	mal := []byte(`{"id": "MAL-2024-1", "affected": [{"package": {"ecosystem": "npm", "name": "evil"}, "ranges": [{"type": "SEMVER", "events": [{"introduced": "0"}]}]}]}`)
	osv, err = ParseBytes(mal, "test")
	if err != nil {
		t.Fatal(err)
	}
	adv = BuildAdvisory(osv, mal)
	if adv.Kind != "malware" || adv.Source != "ossf-malicious-packages" || adv.SourceSeverity != "" {
		t.Errorf("malware advisory = %+v", adv)
	}
	legacy, _ = ConvertToCVE(osv)
	if legacy.Severity != cvss.Medium || legacy.CVSSScore != 0 {
		t.Errorf("legacy severity without any rating = %s %.1f", legacy.Severity, legacy.CVSSScore)
	}
}

func TestLegacyCVSSUsesOfficialFormula(t *testing.T) {
	score, vector, version, severity := parseCVSS([]OSVSeverity{
		{Type: "CVSS_V4", Score: "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N"},
		{Type: "CVSS_V3", Score: "CVSS:3.1/AV:L/AC:L/PR:L/UI:R/S:U/C:H/I:H/A:H"},
	})
	if score != 7.3 || version != "V3" || severity != cvss.High || vector == "" {
		t.Errorf("parseCVSS = %.1f %s %s %s", score, vector, version, severity)
	}
	if score, _, _, sev := parseCVSS([]OSVSeverity{{Type: "CVSS_V3", Score: "garbage"}}); score != 0 || sev != "" {
		t.Errorf("invalid vector scored %.1f %s", score, sev)
	}
}
