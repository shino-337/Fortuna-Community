package risk

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestComputePathInfluence_ProgressSaturationMonotonic(t *testing.T) {
	path := models.AttackPath{
		Nodes:      `[{"id":"step:p:RECON","type":"attack_step","properties":{"stepId":"RECON"}},{"id":"step:p:CRED_ACCESS","type":"attack_step","properties":{"stepId":"CRED_ACCESS"}}]`,
		TotalRisk:  8,
		Length:     2,
		Description: "LATERAL_PATH test",
	}
	signalMap := map[string][]string{
		"S1": {"RECON"},
		"S2": {"CRED_ACCESS"},
	}
	low := computePathInfluence([]models.AttackPath{path}, []models.RuntimeSignal{{SignalType: "S1"}}, signalMap, 0.8, nil)
	high := computePathInfluence([]models.AttackPath{path}, []models.RuntimeSignal{{SignalType: "S1"}, {SignalType: "S2"}}, signalMap, 0.8, nil)
	if !(low.ProgressSaturated > 0 && high.ProgressSaturated > low.ProgressSaturated) {
		t.Fatalf("expected monotonic saturation, low=%v high=%v", low.ProgressSaturated, high.ProgressSaturated)
	}
	if !(high.ProgressSaturated < 1.0 && high.ProgressSaturated > 0) {
		t.Fatalf("expected saturated progress in (0,1), raw=%v sat=%v", high.ProgressRaw, high.ProgressSaturated)
	}
}

func TestLoadRuntimeSignalStepMappingsWithMeta_HashStableOnReorder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.RuntimeSignalStepMapping{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Minute)
	rows := []models.RuntimeSignalStepMapping{
		{SignalType: "PROC_EXEC", StepID: "EXEC", Enabled: true, EffectiveFrom: &now},
		{SignalType: "PROC_EXEC", StepID: "PRIV_ESC", Enabled: true, EffectiveFrom: &now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	_, meta1 := loadRuntimeSignalStepMappingsWithMeta(db)
	if err := db.Exec("DELETE FROM runtime_signal_step_mappings").Error; err != nil {
		t.Fatal(err)
	}
	rows2 := []models.RuntimeSignalStepMapping{
		{SignalType: "PROC_EXEC", StepID: "PRIV_ESC", Enabled: true, EffectiveFrom: &now},
		{SignalType: "PROC_EXEC", StepID: "EXEC", Enabled: true, EffectiveFrom: &now},
	}
	if err := db.Create(&rows2).Error; err != nil {
		t.Fatal(err)
	}
	_, meta2 := loadRuntimeSignalStepMappingsWithMeta(db)
	if meta1.MappingHash == "" || meta1.MappingHash != meta2.MappingHash {
		t.Fatalf("expected stable hash on reorder, meta1=%+v meta2=%+v", meta1, meta2)
	}
}

func TestComputePathInfluence_InjectScaleByStrength(t *testing.T) {
	signalMap := map[string][]string{"S1": {"EXEC"}}
	weak := models.AttackPath{
		Nodes:      `[{"id":"step:p:EXEC","type":"attack_step","properties":{"stepId":"EXEC"}}]`,
		TotalRisk:  2.0,
		Length:     1,
		Description: "LATERAL_PATH weak",
	}
	strong := weak
	strong.TotalRisk = 9.0
	piWeak := computePathInfluence([]models.AttackPath{weak}, []models.RuntimeSignal{{SignalType: "S1"}}, signalMap, 0.8, nil)
	piStrong := computePathInfluence([]models.AttackPath{strong}, []models.RuntimeSignal{{SignalType: "S1"}}, signalMap, 0.8, nil)
	if !(piStrong.InjectScale > piWeak.InjectScale) {
		t.Fatalf("expected strong inject scale > weak, weak=%v strong=%v", piWeak.InjectScale, piStrong.InjectScale)
	}
}

