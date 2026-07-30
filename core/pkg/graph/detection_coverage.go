package graph

import (
	_ "embed"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed detection_coverage.yaml
var detectionCoverageYAML []byte

type detectionCoverageFile struct {
	Version                  int      `yaml:"version"`
	MitreWithDetection       []string `yaml:"mitre_with_detection"`
	MitreExplicitNotCovered  []string `yaml:"mitre_explicit_not_covered"`
}

var mitreDetectionAllow map[string]bool
var mitreExplicitNotCovered map[string]bool

func init() {
	mitreDetectionAllow = make(map[string]bool)
	mitreExplicitNotCovered = make(map[string]bool)
	var f detectionCoverageFile
	if err := yaml.Unmarshal(detectionCoverageYAML, &f); err != nil {
		panic("detection_coverage.yaml: " + err.Error())
	}
	for _, id := range f.MitreWithDetection {
		n := normalizeMitreIDForOverlay(id)
		if n != "" {
			mitreDetectionAllow[n] = true
		}
	}
	for _, id := range f.MitreExplicitNotCovered {
		n := normalizeMitreIDForOverlay(id)
		if n != "" {
			mitreExplicitNotCovered[n] = true
		}
	}
}

// MitreHasDetectionRule is true when the ID is listed in mitre_with_detection.
func MitreHasDetectionRule(mitreID string) bool {
	return mitreDetectionAllow[normalizeMitreIDForOverlay(mitreID)]
}

// MitreExplicitNotCovered is true when listed as explicitly not covered by product rules.
func MitreExplicitNotCovered(mitreID string) bool {
	return mitreExplicitNotCovered[normalizeMitreIDForOverlay(mitreID)]
}

// BuildMitreCoverage assigns observed | inferred | not_covered | unknown per path MITRE id.
// Inference confidence = chainRealism × avgStepGrounding (when status uses product reasoning).
func BuildMitreCoverage(pathMitres []string, runtimeHas map[string]bool, chainRealism float64, avgStepGrounding float64) []MitreCoverageItem {
	seen := map[string]bool{}
	var out []MitreCoverageItem
	baseConf := mathRound3(chainRealism * avgStepGrounding)
	if baseConf < 0 {
		baseConf = 0
	}
	if baseConf > 1 {
		baseConf = 1
	}
	for _, id := range pathMitres {
		n := normalizeMitreID(id)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		var st string
		switch {
		case runtimeHas[n]:
			st = "observed"
		case MitreHasDetectionRule(n):
			st = "inferred"
		case MitreExplicitNotCovered(n):
			st = "not_covered"
		default:
			st = "unknown"
		}
		var conf *float64
		switch st {
		case "observed", "inferred", "not_covered":
			c := baseConf
			conf = &c
		default:
			conf = nil
		}
		prio := mitreCoveragePriority(st, conf)
		out = append(out, MitreCoverageItem{MitreID: n, Status: st, Confidence: conf, Priority: prio})
	}
	return out
}

func mitreCoveragePriority(status string, conf *float64) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "not_covered":
		if conf != nil && *conf > 0.7 {
			return "HIGH"
		}
		return "LOW"
	case "inferred":
		return "MEDIUM"
	case "observed":
		return "LOW"
	default:
		return "LOW"
	}
}
