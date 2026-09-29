package migrations

import (
	"fmt"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

func EnsureServiceAccountMutations(db *gorm.DB) error {
	model := &models.ServiceAccountMutation{}
	if !db.Migrator().HasTable(model) {
		return db.Migrator().CreateTable(model)
	}
	if err := ensureModelColumns(db, model, []string{"ID", "ClusterID", "ServiceAccountID", "UID", "ActorID", "Actor", "Action", "Plan", "Digest", "Status", "NextStep", "Attempts", "LastError", "LeaseToken", "LeaseUntil", "RetryAt", "ExpiresAt", "CreatedAt", "UpdatedAt"}); err != nil {
		return err
	}
	var invalid int64
	if err := db.Model(model).Where("id IS NULL OR id = '' OR cluster_id IS NULL OR cluster_id = '' OR uid IS NULL OR uid = '' OR plan IS NULL OR digest IS NULL OR status IS NULL").Count(&invalid).Error; err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("mutation history contains %d incomplete records", invalid)
	}
	return ensureIndex(db, "idx_service_account_mutation_identity", "service_account_mutations", "id", true)
}
