package migrations

import (
	"log"
	"strings"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration121_AddRuntimeSignalStepMappings creates configurable mapping
// between runtime signal_type and attack step_id for path-progress reasoning.
func Migration121_AddRuntimeSignalStepMappings(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.RuntimeSignalStepMapping{}); err != nil {
		return err
	}

	seeds := []models.RuntimeSignalStepMapping{
		{SignalType: "PROC_ROOT_PIVOT", StepID: "PROC_ROOT_PIVOT", Enabled: true},
		{SignalType: "NAMESPACE_ESCAPE", StepID: "PROC_NAMESPACE_ACCESS", Enabled: true},
		{SignalType: "NAMESPACE_ESCAPE", StepID: "IPC_NAMESPACE_ACCESS", Enabled: true},
		{SignalType: "FS_ESCAPE_ATTEMPT", StepID: "NODE_FS_WRITE", Enabled: true},
		{SignalType: "CAPABILITY_MISUSE", StepID: "NODE_KERNEL_ACCESS", Enabled: true},
		{SignalType: "CAPABILITY_MISUSE", StepID: "NETWORK_SNIFFING", Enabled: true},
		{SignalType: "EBPF_EXEC_ACTIVITY", StepID: "NODE_PERSISTENCE", Enabled: true},
		{SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT", StepID: "NODE_PERSISTENCE", Enabled: true},
	}
	for _, s := range seeds {
		s.SignalType = strings.ToUpper(strings.TrimSpace(s.SignalType))
		s.StepID = strings.ToUpper(strings.TrimSpace(s.StepID))
		if s.SignalType == "" || s.StepID == "" {
			continue
		}
		row := models.RuntimeSignalStepMapping{}
		if err := db.Where("signal_type = ? AND step_id = ?", s.SignalType, s.StepID).First(&row).Error; err == nil {
			if !row.Enabled {
				_ = db.Model(&row).Update("enabled", true).Error
			}
			continue
		}
		if err := db.Create(&s).Error; err != nil {
			log.Printf("Migration121: seed mapping %s -> %s failed: %v", s.SignalType, s.StepID, err)
		}
	}

	log.Println("Migration121: runtime_signal_step_mappings created/seeded")
	return nil
}
