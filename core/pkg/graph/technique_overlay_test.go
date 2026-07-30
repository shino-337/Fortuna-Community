package graph

import "testing"

func TestMitreRefsForTechnique_escape(t *testing.T) {
	refs := MitreRefsForTechnique("ESCAPE_HOSTPATH")
	if len(refs) == 0 {
		t.Fatal("expected MITRE refs for ESCAPE_HOSTPATH")
	}
	if refs[0].ID != "T1611" {
		t.Errorf("got %q want T1611", refs[0].ID)
	}
	if refs[0].URL == "" {
		t.Error("expected URL on overlay")
	}
}

func TestTechniqueByID_mergesMitre(t *testing.T) {
	tech, ok := TechniqueByID("RBAC_PRIV_ESC")
	if !ok {
		t.Fatal("expected technique")
	}
	if len(tech.MitreTechniques) == 0 {
		t.Fatal("expected merged MITRE from overlay")
	}
	found := false
	for _, m := range tech.MitreTechniques {
		if m.ID == "T1098.006" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("want T1098.006 in %#v", tech.MitreTechniques)
	}
	if tech.RiskWeight < 1.4 {
		t.Errorf("expected risk_weight from overlay for RBAC_PRIV_ESC, got %v", tech.RiskWeight)
	}
}

func TestEffectiveMitreWeight_usesTacticModifier(t *testing.T) {
	w := EffectiveMitreWeight("T1098.006")
	if w < 1.0 {
		t.Errorf("expected positive weight, got %v", w)
	}
}
