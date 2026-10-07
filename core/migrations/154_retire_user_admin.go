package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/securityaudit"
)

// retiredUserAdminScope grants no cluster (see authorization.ScopeNoClusters).
const retiredUserAdminScope = `{"clusters":[]}`

// Migration154_RetireUserAdmin moves accounts off the removed user_admin role.
// They become viewers that see no cluster, so the change grants no data the
// account could not read before; an admin then picks clusters or another role.
// Each change is written to the security activity log.
func Migration154_RetireUserAdmin(db *gorm.DB) error {
	type row struct {
		ID        uint
		Username  string
		ScopeJSON string `gorm:"column:scope_json"`
	}
	var rows []row
	if err := db.Table("users").Select("id, username, scope_json").Where("LOWER(TRIM(role)) = ?", "user_admin").Find(&rows).Error; err != nil {
		return fmt.Errorf("[Migration 154] list user_admin accounts: %w", err)
	}
	for _, r := range rows {
		err := db.Transaction(func(tx *gorm.DB) error {
			return tx.Table("users").Where("id = ?", r.ID).Updates(map[string]any{
				"role":       "viewer",
				"scope_json": retiredUserAdminScope,
			}).Error
		})
		if err != nil {
			return fmt.Errorf("[Migration 154] retire user_admin %d: %w", r.ID, err)
		}
		id := r.ID
		securityaudit.Append(db, &securityaudit.Event{
			ActorUsername: "system",
			ActorRole:     "system",
			Action:        "user_rbac_change",
			ResourceType:  "user",
			ResourceID:    fmt.Sprintf("%d", r.ID),
			TargetUserID:  &id,
			BeforeState:   map[string]any{"role": "user_admin", "scopeJson": r.ScopeJSON},
			AfterState:    map[string]any{"role": "viewer", "scopeJson": retiredUserAdminScope},
			Details:       map[string]any{"reason": "user_admin role retired", "username": r.Username},
			Severity:      "medium",
			AuthMethod:    "migration",
			Result:        "success",
		})
		log.Printf("[Migration 154] user %q: user_admin -> viewer with no cluster", r.Username)
	}
	return nil
}
