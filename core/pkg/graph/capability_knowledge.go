package graph

import (
	_ "embed"
	"math"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed capability_knowledge.yaml
var capabilityKnowledgeYAML []byte

type ckFile struct {
	Version int `yaml:"version"`
	Entries map[string]ckEntry `yaml:"entries"`
}

type ckEntry struct {
	ExplanationShort string  `yaml:"explanation_short"`
	BlastHint        string  `yaml:"blast_hint"`
	RiskPoints       float64 `yaml:"risk_points"`
}

var ckByCap map[string]ckEntry

func init() {
	ckByCap = make(map[string]ckEntry)
	var f ckFile
	if err := yaml.Unmarshal(capabilityKnowledgeYAML, &f); err != nil {
		panic("capability_knowledge.yaml: " + err.Error())
	}
	for k, v := range f.Entries {
		ckByCap[strings.ToUpper(strings.TrimSpace(k))] = v
	}
}

// CapabilityKnowledgeShort returns a one-line explanation for a legacy capability id, if present.
func CapabilityKnowledgeShort(capID string) string {
	c := strings.ToUpper(strings.TrimSpace(capID))
	if e, ok := ckByCap[c]; ok {
		return e.ExplanationShort
	}
	c2 := CanonicalCapability(capID)
	if e, ok := ckByCap[strings.ToUpper(c2)]; ok {
		return e.ExplanationShort
	}
	return ""
}

// CKDBRiskPointsForCapability returns CKDB-derived risk contribution for a pod capability id (0 if unknown).
func CKDBRiskPointsForCapability(capID string) float64 {
	c := strings.ToUpper(strings.TrimSpace(capID))
	if e, ok := ckByCap[c]; ok && e.RiskPoints > 0 {
		return e.RiskPoints
	}
	c2 := CanonicalCapability(capID)
	if e, ok := ckByCap[strings.ToUpper(c2)]; ok && e.RiskPoints > 0 {
		return e.RiskPoints
	}
	return 0
}

// SumCKDBRiskForCapabilityIDs sums distinct capability CKDB risk_points, capped for attack_path dimension injection.
func SumCKDBRiskForCapabilityIDs(capIDs []string) float64 {
	seen := map[string]bool{}
	var sum float64
	for _, raw := range capIDs {
		k := strings.ToUpper(strings.TrimSpace(raw))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		sum += CKDBRiskPointsForCapability(k)
	}
	inj := math.Log1p(sum)
	return math.Min(4.0, inj)
}
