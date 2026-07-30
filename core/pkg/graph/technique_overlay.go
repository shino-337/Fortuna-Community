package graph

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed technique_overlay.yaml
var techniqueOverlayYAML []byte

type overlayDefaults struct {
	RiskWeight      float64 `yaml:"risk_weight"`
	HalfLifeHours   float64 `yaml:"half_life_hours"`
	MitreBoostCap   float64 `yaml:"mitre_boost_cap"`
	MitreBoostScale float64 `yaml:"mitre_boost_scale"`
}

type techniqueOverlayFile struct {
	Version         int                            `yaml:"version"`
	Defaults        overlayDefaults                `yaml:"defaults"`
	TacticModifiers map[string]float64             `yaml:"tactic_modifiers"`
	Overlays        map[string]techniqueOverlayRow `yaml:"overlays"`
}

type techniqueOverlayRow struct {
	MitreTechniques []MitreTechniqueRef `yaml:"mitre_techniques"`
	RiskWeight      float64             `yaml:"risk_weight,omitempty"`
	ContextScope    []string            `yaml:"context_scope,omitempty"`
}

// TechniqueOverlayLoadDigest is the last successfully loaded overlay identity (audit / regression).
type TechniqueOverlayLoadDigest struct {
	Version         int       `json:"version"`
	SHA256          string    `json:"sha256"`
	TechniquesCount int       `json:"techniques_count"`
	MitreRefsCount  int       `json:"mitre_refs_count"`
	Bytes           int       `json:"bytes"`
	LoadedAt        time.Time `json:"loaded_at"`
}

var (
	overlayReloadMu sync.Mutex

	overlayDigestMu  sync.RWMutex
	overlayDigestVal TechniqueOverlayLoadDigest

	techniqueOverlayByID map[string]techniqueOverlayRow
	overlayDefaultsVals  overlayDefaults
	tacticModifierByKey  map[string]float64
	// mitreRiskIndex: normalized MITRE id → max risk_weight & tactic for modifier lookup
	mitreRiskIndex map[string]mitreRiskRecord
	// mitreContextScopes: union of overlay context_scope labels per normalized MITRE id
	mitreContextScopes map[string][]string
)

type mitreRiskRecord struct {
	RiskWeight float64
	Tactic     string
}

func init() {
	if err := applyTechniqueOverlayYAML(techniqueOverlayYAML, true); err != nil {
		panic(err.Error())
	}
}

// TechniqueOverlayDigest returns the digest of the in-memory technique overlay (YAML load).
func TechniqueOverlayDigest() TechniqueOverlayLoadDigest {
	overlayDigestMu.RLock()
	defer overlayDigestMu.RUnlock()
	return overlayDigestVal
}

func clearTechniqueOverlayGlobals() {
	techniqueOverlayByID = nil
	tacticModifierByKey = nil
	mitreRiskIndex = nil
	mitreContextScopes = nil
	overlayDefaultsVals = overlayDefaults{}
}

// overlaySemanticFromYAMLFileEnabled is false when FORTUNA_TECHNIQUE_SOURCE=go (overlay MITRE/risk/tactic semantics off; defaults still load from YAML bytes).
func overlaySemanticFromYAMLFileEnabled() bool {
	return strings.TrimSpace(strings.ToLower(os.Getenv("FORTUNA_TECHNIQUE_SOURCE"))) != "go"
}

// TechniqueOverlayEmbeddedYAML returns the embedded technique_overlay.yaml bytes (baseline for reload/tests).
func TechniqueOverlayEmbeddedYAML() []byte {
	cp := make([]byte, len(techniqueOverlayYAML))
	copy(cp, techniqueOverlayYAML)
	return cp
}

// ReloadTechniqueOverlay replaces in-memory overlay indices from YAML bytes (dual-read / admin tooling).
// It clears overlay-derived maps first to avoid stale cross-refs (Phase 4.2).
// Serialize callers that mutate overlay-backed globals (tests should use t.Cleanup to restore embedded YAML).
func ReloadTechniqueOverlay(yamlBytes []byte) error {
	overlayReloadMu.Lock()
	defer overlayReloadMu.Unlock()
	return applyTechniqueOverlayYAML(yamlBytes, false)
}

// ResetTechniqueRegistry clears overlay-derived state and reloads from yamlBytes.
// The static Go techniqueRegistry map is immutable; only YAML-applied overlay indices are reset.
func ResetTechniqueRegistry(yamlBytes []byte) error {
	return ReloadTechniqueOverlay(yamlBytes)
}

// OverlayDerivedMapsIdentity returns opaque table pointers for overlay-derived maps (Phase 4.2 A/B: assert new allocations after Reset/Reload).
func OverlayDerivedMapsIdentity() (overlayByTechniquePtr, mitreRiskIndexPtr uintptr) {
	overlayReloadMu.Lock()
	defer overlayReloadMu.Unlock()
	if techniqueOverlayByID != nil {
		overlayByTechniquePtr = reflect.ValueOf(techniqueOverlayByID).Pointer()
	}
	if mitreRiskIndex != nil {
		mitreRiskIndexPtr = reflect.ValueOf(mitreRiskIndex).Pointer()
	}
	return overlayByTechniquePtr, mitreRiskIndexPtr
}

// CanonicalMitreTechniqueIDs returns sorted unique normalized ATT&CK IDs for drift-safe comparisons (Phase 4.1 shadow).
// Alias/duplicate refs collapse to one id to avoid double-counting in parity checks.
func CanonicalMitreTechniqueIDs(refs []MitreTechniqueRef) []string {
	seen := make(map[string]struct{})
	for _, r := range refs {
		id := normalizeMitreIDForOverlay(r.ID)
		if id == "" {
			continue
		}
		seen[id] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func applyTechniqueOverlayYAML(yamlBytes []byte, logReload bool) error {
	clearTechniqueOverlayGlobals()
	var f techniqueOverlayFile
	if err := yaml.Unmarshal(yamlBytes, &f); err != nil {
		return fmt.Errorf("technique_overlay.yaml: %w", err)
	}

	techniqueOverlayByID = make(map[string]techniqueOverlayRow)
	tacticModifierByKey = make(map[string]float64)
	mitreRiskIndex = make(map[string]mitreRiskRecord)
	mitreContextScopes = make(map[string][]string)

	overlayDefaultsVals = f.Defaults
	if overlayDefaultsVals.RiskWeight <= 0 {
		overlayDefaultsVals.RiskWeight = 1.0
	}
	if overlayDefaultsVals.HalfLifeHours <= 0 {
		overlayDefaultsVals.HalfLifeHours = 72
	}
	if overlayDefaultsVals.MitreBoostCap <= 0 {
		overlayDefaultsVals.MitreBoostCap = 5.0
	}
	if overlayDefaultsVals.MitreBoostScale <= 0 {
		overlayDefaultsVals.MitreBoostScale = 0.42
	}
	for k, v := range f.TacticModifiers {
		tacticModifierByKey[strings.ToLower(strings.TrimSpace(k))] = v
	}
	for id, row := range f.Overlays {
		key := strings.ToUpper(strings.TrimSpace(id))
		techniqueOverlayByID[key] = row
		rw := row.RiskWeight
		if rw <= 0 {
			rw = overlayDefaultsVals.RiskWeight
		}
		for _, mt := range row.MitreTechniques {
			mid := normalizeMitreIDForOverlay(mt.ID)
			if mid == "" {
				continue
			}
			prev, ok := mitreRiskIndex[mid]
			if !ok || rw > prev.RiskWeight {
				mitreRiskIndex[mid] = mitreRiskRecord{RiskWeight: rw, Tactic: mt.Tactic}
			}
			mitreContextScopes[mid] = mergeUniqueScopeLabels(mitreContextScopes[mid], row.ContextScope)
		}
	}
	sum := sha256.Sum256(yamlBytes)
	var mitreRefCount int
	for _, row := range f.Overlays {
		mitreRefCount += len(row.MitreTechniques)
	}
	overlayDigestMu.Lock()
	overlayDigestVal = TechniqueOverlayLoadDigest{
		Version:         f.Version,
		SHA256:          hex.EncodeToString(sum[:]),
		TechniquesCount: len(f.Overlays),
		MitreRefsCount:  mitreRefCount,
		Bytes:           len(yamlBytes),
		LoadedAt:        time.Now().UTC(),
	}
	overlayDigestMu.Unlock()
	if logReload {
		avgMitrePerTech := 0.0
		if len(f.Overlays) > 0 {
			avgMitrePerTech = float64(mitreRefCount) / float64(len(f.Overlays))
		}
		log.Printf("[technique_overlay] techniques=%d mitre_refs=%d avg_mitre_per_technique=%.3f version=%d sha256=%s bytes=%d",
			len(f.Overlays), mitreRefCount, avgMitrePerTech, f.Version, hex.EncodeToString(sum[:]), len(yamlBytes))
	}
	return nil
}

func mergeUniqueScopeLabels(base []string, add []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range base {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, strings.TrimSpace(s))
	}
	for _, s := range add {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

// ContextScopesForMitreID returns merged overlay context_scope hints for a normalized MITRE id.
func ContextScopesForMitreID(mitreID string) []string {
	mid := normalizeMitreIDForOverlay(mitreID)
	if mid == "" {
		return nil
	}
	out := mitreContextScopes[mid]
	if len(out) == 0 {
		return nil
	}
	cp := make([]string, len(out))
	copy(cp, out)
	return cp
}

func eventMatchesContextScopes(ev runtimeEventLite, scopes []string) bool {
	for _, s := range scopes {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "pod":
			continue
		case "node", "control_plane":
			if strings.TrimSpace(ev.NodeName) == "" {
				return false
			}
		default:
			continue
		}
	}
	return true
}

// MitreRuntimeScopeTierMultiplier refines MITRE runtime weight using overlay scope match, then namespace-tier fallbacks.
func MitreRuntimeScopeTierMultiplier(ev runtimeEventLite, mitreNorm, targetPodNamespace string) float64 {
	scopes := ContextScopesForMitreID(mitreNorm)
	if len(scopes) == 0 {
		return 1.0
	}
	if eventMatchesContextScopes(ev, scopes) {
		return 1.2
	}
	tn := strings.TrimSpace(targetPodNamespace)
	evn := strings.TrimSpace(ev.PodNamespace)
	if tn != "" && evn != "" && strings.EqualFold(evn, tn) {
		return 1.05
	}
	if evn != "" {
		return 0.9
	}
	return 0.7
}

func normalizeMitreIDForOverlay(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func overlayForTechnique(id string) (techniqueOverlayRow, bool) {
	if !overlaySemanticFromYAMLFileEnabled() {
		return techniqueOverlayRow{}, false
	}
	r, ok := techniqueOverlayByID[strings.ToUpper(strings.TrimSpace(id))]
	return r, ok
}

// MitreRefsForTechnique returns embedded ATT&CK refs for an internal technique ID.
func MitreRefsForTechnique(id string) []MitreTechniqueRef {
	if r, ok := overlayForTechnique(id); ok {
		out := make([]MitreTechniqueRef, len(r.MitreTechniques))
		copy(out, r.MitreTechniques)
		return out
	}
	return nil
}

// DefaultHalfLifeHours for MITRE runtime decay (from overlay defaults).
func DefaultHalfLifeHours() float64 {
	return overlayDefaultsVals.HalfLifeHours
}

// MitreBoostCap returns max points for attack_path MITRE boost.
func MitreBoostCap() float64 {
	return overlayDefaultsVals.MitreBoostCap
}

// MitreBoostScale returns scaler from weighted sum → dimension points.
func MitreBoostScale() float64 {
	return overlayDefaultsVals.MitreBoostScale
}

// TacticModifier returns tactic_weight for a tactic label (default 1.0).
func TacticModifier(tactic string) float64 {
	if !overlaySemanticFromYAMLFileEnabled() {
		return 1.0
	}
	if tactic == "" {
		return 1.0
	}
	k := strings.ToLower(strings.TrimSpace(tactic))
	if v, ok := tacticModifierByKey[k]; ok {
		return v
	}
	return 1.0
}

// MitreRiskRecord returns aggregated risk+tactic for a normalized MITRE id (for runtime weighting).
func MitreRiskRecord(mitreID string) (riskWeight float64, tactic string, ok bool) {
	if !overlaySemanticFromYAMLFileEnabled() {
		return overlayDefaultsVals.RiskWeight, "", false
	}
	mid := normalizeMitreIDForOverlay(mitreID)
	rec, ok := mitreRiskIndex[mid]
	if !ok {
		return overlayDefaultsVals.RiskWeight, "", false
	}
	return rec.RiskWeight, rec.Tactic, true
}

// ContextScopesForTechnique lists execution-surface hints from overlay.
func ContextScopesForTechnique(techniqueID string) []string {
	if r, ok := overlayForTechnique(techniqueID); ok && len(r.ContextScope) > 0 {
		out := make([]string, len(r.ContextScope))
		copy(out, r.ContextScope)
		return out
	}
	return nil
}

// EffectiveMitreWeight = technique_risk_weight × tactic_modifier(tactic).
func EffectiveMitreWeight(mitreID string) float64 {
	rw, tactic, ok := MitreRiskRecord(mitreID)
	if !ok {
		rw = overlayDefaultsVals.RiskWeight
	}
	tw := TacticModifier(tactic)
	return math.Max(0.1, rw*tw)
}

func defaultOverlayRiskWeight() float64 {
	if overlayDefaultsVals.RiskWeight > 0 {
		return overlayDefaultsVals.RiskWeight
	}
	return 1.0
}
