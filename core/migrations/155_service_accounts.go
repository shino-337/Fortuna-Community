package migrations

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"gorm.io/gorm"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

// Migration155_SystemUserHasNoRole takes the admin role away from the "system"
// account Core created for agent-sync audit rows. It held admin with a random
// password nobody knew, so it was an admin account without an owner. It now has
// no permissions and cannot sign in. The account is matched by the username and
// email Core gave it, and left alone if it is the only active admin.
func Migration155_SystemUserHasNoRole(db *gorm.DB) error {
	var u models.User
	err := db.Where("username = ? AND email = ? AND LOWER(role) = ?", "system", "system@fortuna.local", models.RoleAdmin).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("[Migration 155] find system user: %w", err)
	}
	var otherAdmins int64
	if err := db.Model(&models.User{}).
		Where("id <> ? AND LOWER(role) = ? AND active = ?", u.ID, models.RoleAdmin, true).
		Count(&otherAdmins).Error; err != nil {
		return fmt.Errorf("[Migration 155] count admins: %w", err)
	}
	if otherAdmins == 0 {
		log.Printf("[Migration 155] system user is the only active admin; left unchanged")
		return nil
	}
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).Updates(map[string]any{
		"role":       models.RoleSystem,
		"active":     false,
		"scope_json": `{"clusters":[]}`,
	}).Error; err != nil {
		return fmt.Errorf("[Migration 155] update system user: %w", err)
	}
	id := u.ID
	securityaudit.Append(db, &securityaudit.Event{
		ActorUsername: "system",
		ActorRole:     models.RoleSystem,
		Action:        "user_rbac_change",
		ResourceType:  "user",
		ResourceID:    fmt.Sprintf("%d", u.ID),
		TargetUserID:  &id,
		BeforeState:   map[string]any{"role": u.Role, "active": u.Active},
		AfterState:    map[string]any{"role": models.RoleSystem, "active": false},
		Details:       map[string]any{"reason": "system account for audit rows holds no permissions"},
		Severity:      "medium",
		AuthMethod:    "migration",
		Result:        "success",
	})
	log.Printf("[Migration 155] system user: admin -> no permissions, cannot sign in")
	return nil
}

// DefaultRiskEvaluatorUsername is the account the risk evaluation CronJob signs in as.
const DefaultRiskEvaluatorUsername = "fortuna-risk-evaluator"

// EnsureRiskEvaluatorAccount creates or updates the account of the scheduled
// risk evaluation job from FORTUNA_RISK_EVALUATOR_PASSWORD. It holds only
// auth.session and risk.evaluate. Global evaluation needs every cluster, so its
// scope is unrestricted. Nothing happens without the variable, and an existing
// account of another role is never taken over.
func EnsureRiskEvaluatorAccount(db *gorm.DB) error {
	password := strings.TrimSpace(os.Getenv("FORTUNA_RISK_EVALUATOR_PASSWORD"))
	if password == "" {
		return nil
	}
	username := strings.TrimSpace(os.Getenv("FORTUNA_RISK_EVALUATOR_USERNAME"))
	if username == "" {
		username = DefaultRiskEvaluatorUsername
	}
	if err := auth.ValidatePasswordStrength(password); err != nil {
		return fmt.Errorf("FORTUNA_RISK_EVALUATOR_PASSWORD does not meet password policy: %w", err)
	}

	var u models.User
	err := db.Where("username = ?", username).First(&u).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		u = models.User{
			Username:  username,
			Email:     username + "@fortuna.local",
			Password:  hash,
			Role:      models.RoleRiskEvaluator,
			ScopeJSON: "{}",
			Active:    true,
		}
		if err := db.Create(&u).Error; err != nil {
			return fmt.Errorf("create risk evaluator account: %w", err)
		}
		log.Printf("Created risk evaluator service account: %s", username)
		return nil
	case err != nil:
		return fmt.Errorf("find risk evaluator account: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(u.Role), models.RoleRiskEvaluator) {
		log.Printf("[Security] WARNING: account %q exists with role %q; not using it as the risk evaluator service account", username, u.Role)
		return nil
	}
	updates := map[string]any{}
	if u.ScopeJSON != "{}" {
		updates["scope_json"] = "{}"
	}
	if !auth.CheckPasswordHash(password, u.Password) {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		updates["password"] = hash
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&models.User{}).Where("id = ?", u.ID).Updates(updates).Error
}
