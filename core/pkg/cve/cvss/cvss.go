// Package cvss scores CVSS vectors with the official FIRST formulas (v2, v3.0, v3.1, v4.0)
// and maps scores and vendor ratings to Fortuna severities.
package cvss

import (
	"math"
	"strings"

	gocvss20 "github.com/pandatix/go-cvss/20"
	gocvss30 "github.com/pandatix/go-cvss/30"
	gocvss31 "github.com/pandatix/go-cvss/31"
	gocvss40 "github.com/pandatix/go-cvss/40"
)

// Severities, highest first. Unknown means no source rated the vulnerability; it is never
// replaced by a guessed MEDIUM.
const (
	Critical   = "CRITICAL"
	High       = "HIGH"
	Medium     = "MEDIUM"
	Low        = "LOW"
	Negligible = "NEGLIGIBLE"
	Unknown    = "UNKNOWN"
)

// BaseScore returns the base score of a CVSS vector and its version ("2.0", "3.0", "3.1",
// "4.0"). ok is false when the vector is not a valid CVSS vector.
func BaseScore(vector string) (score float64, version string, ok bool) {
	v := strings.TrimSpace(vector)
	switch {
	case strings.HasPrefix(v, "CVSS:4.0/"):
		c, err := gocvss40.ParseVector(v)
		if err != nil {
			return 0, "", false
		}
		return round1(c.Score()), "4.0", true
	case strings.HasPrefix(v, "CVSS:3.1/"):
		c, err := gocvss31.ParseVector(v)
		if err != nil {
			return 0, "", false
		}
		return c.BaseScore(), "3.1", true
	case strings.HasPrefix(v, "CVSS:3.0/"):
		c, err := gocvss30.ParseVector(v)
		if err != nil {
			return 0, "", false
		}
		return c.BaseScore(), "3.0", true
	case strings.HasPrefix(v, "AV:"):
		c, err := gocvss20.ParseVector(v)
		if err != nil {
			return 0, "", false
		}
		return c.BaseScore(), "2.0", true
	}
	return 0, "", false
}

// SeverityFromScore maps a CVSS v3/v4 base score to its qualitative rating. A score of 0 has
// no impact and is rated Negligible.
func SeverityFromScore(score float64) string {
	switch {
	case score >= 9.0:
		return Critical
	case score >= 7.0:
		return High
	case score >= 4.0:
		return Medium
	case score > 0:
		return Low
	}
	return Negligible
}

// SeverityFromV2Score maps a CVSS v2 base score to the v2 rating, which has no Critical.
func SeverityFromV2Score(score float64) string {
	switch {
	case score >= 7.0:
		return High
	case score >= 4.0:
		return Medium
	}
	return Low
}

// NormalizeRating maps a vendor or database rating (GitHub "MODERATE", Debian urgency
// "low**", Ubuntu priority "negligible", Red Hat "Important") to a Fortuna severity. Ratings
// that say nothing about impact ("not yet assigned", "end-of-life", "") return "".
func NormalizeRating(rating string) string {
	r := strings.ToLower(strings.TrimSpace(rating))
	r = strings.TrimRight(r, "*")
	r = strings.TrimSpace(r)
	switch r {
	case "critical":
		return Critical
	case "high", "important":
		return High
	case "medium", "moderate":
		return Medium
	case "low":
		return Low
	case "negligible", "unimportant", "none":
		return Negligible
	}
	return ""
}

// Rank orders severities: Critical 5 … Negligible 1, Unknown and "" 0.
func Rank(severity string) int {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case Critical:
		return 5
	case High:
		return 4
	case Medium:
		return 3
	case Low:
		return 2
	case Negligible:
		return 1
	}
	return 0
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
