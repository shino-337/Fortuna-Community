package models

import "testing"

func TestValidateAssetSecurityStateJSON(t *testing.T) {
	if err := ValidateAssetSecurityStateJSON(&AssetSecurityState{
		RuntimeSignalsByType:  `{"INTERACTIVE_SHELL_EXEC":1}`,
		EffectiveCapabilities: `["CAP_A"]`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAssetSecurityStateJSON(&AssetSecurityState{
		RuntimeSignalsByType: "not-json",
	}); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateAssetSecurityStateJSON(&AssetSecurityState{
		EffectiveCapabilities: `{"x":1}`,
	}); err == nil {
		t.Fatal("expected error for non-array effectiveCapabilities")
	}
}

func TestAssetSecurityStateColumnGroupCompleteness(t *testing.T) {
	// Every ADR group should appear at least once (software_risk reserved for future columns).
	for _, g := range []string{
		GroupIdentityContext, GroupExposureContext, GroupRuntimeSecurityContext, GroupEffectiveCapabilityContext,
	} {
		found := false
		for _, v := range AssetSecurityStateColumnGroup {
			if v == g {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("group %q not mapped to any column", g)
		}
	}
}
