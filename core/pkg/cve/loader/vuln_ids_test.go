package loader

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalVulnIDs(t *testing.T) {
	ref := func(id, relation, kind string) AdvisoryRef {
		return AdvisoryRef{RefID: id, Relation: relation, RefKind: kind}
	}
	cases := []struct {
		name, id, source string
		refs             []AdvisoryRef
		want             []string
	}{
		{"cve itself", "CVE-2024-1234", "nvd", nil, []string{"CVE-2024-1234"}},
		{"ghsa alias", "GHSA-aaaa-bbbb-cccc", "ghsa", []AdvisoryRef{ref("CVE-2024-1", "alias", "cve"), ref("PYSEC-1", "alias", "advisory")}, []string{"CVE-2024-1"}},
		{"debian upstream", "DEBIAN-CVE-2024-2", "debian", []AdvisoryRef{ref("CVE-2024-2", "upstream", "cve")}, []string{"CVE-2024-2"}},
		{"erratum fans out", "RHSA-2024:1", "redhat", []AdvisoryRef{ref("CVE-2024-9", "upstream", "cve"), ref("CVE-2024-3", "upstream", "cve")}, []string{"CVE-2024-3", "CVE-2024-9"}},
		{"alma lists related", "ALSA-2024:1", "almalinux", []AdvisoryRef{ref("CVE-2024-4", "related", "cve")}, []string{"CVE-2024-4"}},
		{"related cve is another vuln", "GO-2024-1", "go", []AdvisoryRef{ref("CVE-2024-5", "related", "cve"), ref("GHSA-xxxx-yyyy-zzzz", "alias", "ghsa")}, []string{"GHSA-xxxx-yyyy-zzzz"}},
		{"ghsa without cve", "GHSA-aaaa-bbbb-dddd", "ghsa", []AdvisoryRef{ref("GO-2024-2", "alias", "advisory")}, []string{"GHSA-aaaa-bbbb-dddd"}},
		{"nothing to map", "MAL-2024-1", "ossf-malicious-packages", nil, []string{"MAL-2024-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, CanonicalVulnIDs(tc.id, tc.source, tc.refs))
		})
	}
}
