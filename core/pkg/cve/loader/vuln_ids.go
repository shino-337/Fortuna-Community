package loader

import (
	"sort"
	"strings"
)

// erratumSources publish one advisory per package update that fixes several CVEs and list
// those CVEs as "related" rather than as aliases or upstream IDs.
var erratumSources = map[string]bool{"almalinux": true, "rocky": true, "redhat": true}

// CanonicalVulnIDs returns the vulnerability IDs a finding from this advisory is reported
// under: its CVEs when it has any, otherwise its GHSA ID, otherwise the advisory ID itself.
// An erratum fixing several CVEs yields one ID per CVE.
func CanonicalVulnIDs(advisoryID, source string, refs []AdvisoryRef) []string {
	advisoryID = strings.TrimSpace(advisoryID)
	if cveIDRE.MatchString(advisoryID) {
		return []string{advisoryID}
	}
	cves := refIDs(refs, "cve", "alias", "upstream")
	if len(cves) == 0 && erratumSources[source] {
		cves = refIDs(refs, "cve", "related")
	}
	if len(cves) > 0 {
		return cves
	}
	if strings.HasPrefix(advisoryID, "GHSA-") {
		return []string{advisoryID}
	}
	if ghsa := refIDs(refs, "ghsa", "alias"); len(ghsa) > 0 {
		return ghsa[:1]
	}
	return []string{advisoryID}
}

func refIDs(refs []AdvisoryRef, kind string, relations ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range refs {
		if r.RefKind != kind || seen[r.RefID] {
			continue
		}
		for _, rel := range relations {
			if r.Relation == rel {
				seen[r.RefID] = true
				out = append(out, r.RefID)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
