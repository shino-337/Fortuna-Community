package matcher

import (
	"strings"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/cve/cvss"
	"github.com/fortuna/core/pkg/models"
)

// One finding is reported per (component, vulnerability). Several advisories can match it (the
// Debian CVE entry and a DSA, a GHSA and a Go advisory, a Red Hat erratum per stream); the
// finding lists them all and takes its rating from the best severity source among them.

var severityTierBySource = map[string]int{
	"vendor":        1,
	"advisory_cvss": 2,
	"nvd":           3,
	"cve_cvss":      4,
	"errata_cvss":   5,
	"default":       6,
}

// tierOrder ranks a severity tier; rows of the legacy catalog carry no tier and rank last.
func tierOrder(tier int) int {
	if tier <= 0 {
		return 100
	}
	return tier
}

// rateMatch copies the rating and the advisory of a matched range onto a new finding.
func rateMatch(m *models.CVEMatch, c *cve.CVE) {
	m.Severity = strings.ToUpper(c.Severity)
	m.CVSS = float32(c.CVSSScore)
	m.SeveritySource = c.SeveritySource
	addAdvisoryID(m, c.AdvisoryID)
}

// mergeMatch folds another matched range of the same vulnerability into finding m.
func mergeMatch(m *models.CVEMatch, c *cve.CVE) {
	addAdvisoryID(m, c.AdvisoryID)
	have, offered := tierOrder(severityTierBySource[m.SeveritySource]), tierOrder(c.SeverityTier)
	if offered < have || (offered == have && cvss.Rank(c.Severity) > cvss.Rank(m.Severity)) {
		m.Severity = strings.ToUpper(c.Severity)
		m.CVSS = float32(c.CVSSScore)
		m.SeveritySource = c.SeveritySource
	}
	if strings.TrimSpace(m.FixedVersion) == "" {
		m.FixedVersion = c.FixedVersion
	}
}

func addAdvisoryID(m *models.CVEMatch, id string) {
	id = strings.TrimSpace(id)
	if id == "" || id == m.CVEID {
		return
	}
	for _, have := range m.AdvisoryIDs {
		if have == id {
			return
		}
	}
	m.AdvisoryIDs = append(m.AdvisoryIDs, id)
}
