package migrations

import (
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func TestMigration161_FixMitreMappings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.CapabilityMetadata{}, &models.RiskRule{}); err != nil {
		t.Fatal(err)
	}
	seed := []models.CapabilityMetadata{
		{CapabilityID: "ESC_HOSTPID_POD", Domain: "ESC", Category: "c", Description: "d", SeverityBase: "HIGH", MitreTechnique: "T1611.001"},
		{CapabilityID: "ESC_HOSTPATH_NODE", Domain: "ESC", Category: "c", Description: "d", SeverityBase: "CRITICAL", MitreTechnique: "T1610"},
		{CapabilityID: "CTRL_CONTROL_PLANE_POD", Domain: "CTRL", Category: "c", Description: "d", SeverityBase: "MEDIUM", MitreTechnique: "T1496"},
		// Operator-edited value is kept.
		{CapabilityID: "API_RBAC_WRITE_CLUSTER", Domain: "API", Category: "c", Description: "d", SeverityBase: "HIGH", MitreTechnique: "T1078"},
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatal(err)
	}
	rules := []models.RiskRule{
		{RuleID: "pss-host-namespaces", Name: "n", Conditions: "[]", Tags: `["pss","T1614","T1611.001","T1611.002"]`},
		{RuleID: "cis-5.1.6", Name: "n", Conditions: "[]", Tags: `["cis","T1612","T1552"]`},
		{RuleID: "rbac-pods-exec-attach-portforward", Name: "n", Conditions: "[]", Tags: `["T1612","T1059"]`},
	}
	if err := db.Create(&rules).Error; err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ { // idempotent
		if err := Migration161_FixMitreMappings(db); err != nil {
			t.Fatal(err)
		}
	}

	technique := func(id string) string {
		var m models.CapabilityMetadata
		if err := db.First(&m, "capability_id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		return m.MitreTechnique
	}
	for id, want := range map[string]string{
		"ESC_HOSTPID_POD":        "T1611",
		"ESC_HOSTPATH_NODE":      "T1611",
		"CTRL_CONTROL_PLANE_POD": "",
		"API_RBAC_WRITE_CLUSTER": "T1078",
	} {
		if got := technique(id); got != want {
			t.Errorf("%s: technique %q want %q", id, got, want)
		}
	}

	tags := func(ruleID string) []string {
		var r models.RiskRule
		if err := db.First(&r, "rule_id = ?", ruleID).Error; err != nil {
			t.Fatal(err)
		}
		var out []string
		_ = json.Unmarshal([]byte(r.Tags), &out)
		return out
	}
	assertTags := func(ruleID string, want ...string) {
		got := tags(ruleID)
		if len(got) != len(want) {
			t.Fatalf("%s: tags %v want %v", ruleID, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: tags %v want %v", ruleID, got, want)
			}
		}
	}
	assertTags("pss-host-namespaces", "pss", "T1614", "T1611")
	assertTags("cis-5.1.6", "cis", "T1552")
	assertTags("rbac-pods-exec-attach-portforward", "T1609", "T1059")
}
