package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/inventoryevidence"
	"github.com/fortuna/core/pkg/lifecycle"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidCollection = errors.New("invalid inventory collection")
var ErrCollectionConflict = errors.New("inventory collection replay or ordering conflict")

// WithAgentRecord stages Agent liveness/metadata so it commits only with the
// inventory transaction that justified the heartbeat.
func (s *AgentService) WithAgentRecord(row *models.Agent) *AgentService {
	s.agentRecord = row
	return s
}

func (s *AgentService) persistAgentRecord(ctx context.Context, db *gorm.DB, clusterID, requiredAgentID string) error {
	if s.agentRecord == nil {
		return nil
	}
	record := *s.agentRecord
	if record.ClusterID != clusterID || (requiredAgentID != "" && record.AgentID != requiredAgentID) {
		return fmt.Errorf("%w: staged agent identity does not match inventory principal", ErrInvalidCollection)
	}
	if err := agentidentity.UpsertRecord(ctx, db, &record); err != nil {
		return fmt.Errorf("persist agent identity: %w", err)
	}
	return nil
}

func (s *AgentService) launchAfterSync(fn func()) {
	if s.deferWork {
		s.afterCommit = append(s.afterCommit, fn)
		return
	}
	go fn()
}
func (s *AgentService) scopeInventory(db *gorm.DB) *gorm.DB {
	if s.namespaceScope != "" {
		return db.Where("namespace = ?", s.namespaceScope)
	}
	return db
}

// ValidateInventoryPayload distinguishes missing/null arrays from successful empty
// lists and validates the whole batch before any persistence or pruning occurs.
func ValidateInventoryPayload(c collection.Inventory, data map[string]interface{}, now time.Time) error {
	if err := c.Validate(now); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCollection, err)
	}
	if c.Status == "failed" {
		return nil
	}
	if data["isFullSync"] != true || data["isDeltaSync"] != false {
		return fmt.Errorf("%w: complete observations require explicit full sync", ErrInvalidCollection)
	}
	for _, kind := range collection.InventoryKinds {
		rows, ok := data[kind].([]interface{})
		if !ok || len(rows) != c.Counts[kind] {
			return fmt.Errorf("%w: %s absent or count mismatch", ErrInvalidCollection, kind)
		}
		seen := map[string]bool{}
		for _, raw := range rows {
			row, ok := raw.(map[string]interface{})
			if !ok {
				return fmt.Errorf("%w: malformed %s", ErrInvalidCollection, kind)
			}
			uid, _ := row["uid"].(string)
			name, _ := row["name"].(string)
			if uid == "" || name == "" || seen[uid] {
				return fmt.Errorf("%w: missing/duplicate identity in %s", ErrInvalidCollection, kind)
			}
			seen[uid] = true
			if kind != "clusterRoles" && kind != "clusterRoleBindings" {
				ns, _ := row["namespace"].(string)
				if ns == "" || c.Namespace != "" && ns != c.Namespace {
					return fmt.Errorf("%w: namespace outside collection scope", ErrInvalidCollection)
				}
			}
			if kind == "roles" || kind == "clusterRoles" {
				rules, ok := row["rules"].([]interface{})
				if !ok {
					return fmt.Errorf("%w: missing role rules", ErrInvalidCollection)
				}
				for _, rawRule := range rules {
					r, ok := rawRule.(map[string]interface{})
					if !ok {
						return fmt.Errorf("%w: malformed rule", ErrInvalidCollection)
					}
					verbs, ok := r["verbs"].([]interface{})
					if !ok || len(verbs) == 0 {
						return fmt.Errorf("%w: missing verbs", ErrInvalidCollection)
					}
					for _, key := range []string{"resources", "apiGroups", "resourceNames", "nonResourceURLs"} {
						if raw, exists := r[key]; exists {
							values, ok := raw.([]interface{})
							if !ok {
								return fmt.Errorf("%w: malformed rule %s", ErrInvalidCollection, key)
							}
							for _, value := range values {
								if _, ok := value.(string); !ok {
									return fmt.Errorf("%w: invalid rule %s value", ErrInvalidCollection, key)
								}
							}
						}
					}
					resources, _ := r["resources"].([]interface{})
					urls, _ := r["nonResourceURLs"].([]interface{})
					groups, _ := r["apiGroups"].([]interface{})
					if len(resources) == 0 && len(urls) == 0 || len(resources) > 0 && len(groups) == 0 {
						return fmt.Errorf("%w: incomplete rule targets", ErrInvalidCollection)
					}
					for _, v := range verbs {
						if str, ok := v.(string); !ok || str == "" {
							return fmt.Errorf("%w: invalid verb", ErrInvalidCollection)
						}
					}
				}
			}
			if kind == "pods" {
				for _, key := range []string{"hostNetwork", "hostPID", "hostIPC", "automountServiceAccountToken"} {
					if _, ok := row[key].(bool); !ok {
						return fmt.Errorf("%w: missing Pod %s", ErrInvalidCollection, key)
					}
				}
				if cs, ok := row["containers"].([]interface{}); !ok || len(cs) == 0 {
					return fmt.Errorf("%w: incomplete Pod containers", ErrInvalidCollection)
				}
			}
			if kind == "roleBindings" || kind == "clusterRoleBindings" {
				ref, ok := row["roleRef"].(map[string]interface{})
				if !ok || ref["apiGroup"] != "rbac.authorization.k8s.io" || ref["name"] == "" || ref["name"] == nil {
					return fmt.Errorf("%w: invalid role reference", ErrInvalidCollection)
				}
				if name, ok := ref["name"].(string); !ok || name == "" {
					return fmt.Errorf("%w: invalid role name", ErrInvalidCollection)
				}
				if ref["kind"] != "Role" && ref["kind"] != "ClusterRole" || kind == "clusterRoleBindings" && ref["kind"] != "ClusterRole" {
					return fmt.Errorf("%w: invalid role kind", ErrInvalidCollection)
				}
				if _, ok := row["subjects"].([]interface{}); !ok {
					return fmt.Errorf("%w: missing binding subjects", ErrInvalidCollection)
				}
			}
		}
	}
	return nil
}

// SyncObservedData serializes by cluster and commits the inventory projection and
// receipt together. A receipt proves this payload was persisted, not runtime
// sensor coverage, authoritative deletion, or auto-resolution eligibility.
func (s *AgentService) SyncObservedData(ctx context.Context, clusterID, clusterName, source, k8sVersion, distribution, agentID string, data map[string]interface{}, traceID string, c collection.Inventory) (*models.InventoryCollection, error) {
	if agentID == "" || clusterID == "" {
		return nil, fmt.Errorf("%w: trusted identity required", ErrInvalidCollection)
	}
	c.StartedAt = c.StartedAt.UTC().Truncate(time.Microsecond)
	c.ObservedAt = c.ObservedAt.UTC().Truncate(time.Microsecond)
	normalizedKindStartedAt := make(map[string]time.Time, len(c.KindStartedAt))
	for kind, observed := range c.KindStartedAt {
		normalizedKindStartedAt[kind] = observed.UTC().Truncate(time.Microsecond)
	}
	c.KindStartedAt = normalizedKindStartedAt
	if err := ValidateInventoryPayload(c, data, time.Now().UTC()); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCollection, err)
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	counts, _ := json.Marshal(c.Counts)
	kindStartedAt, _ := json.Marshal(c.KindStartedAt)
	result := models.InventoryCollection{ClusterID: clusterID, AgentID: agentID, CollectionID: c.ID, Namespace: c.Namespace, Status: c.Status, StartedAt: c.StartedAt, ObservedAt: c.ObservedAt, ReceivedAt: time.Now().UTC().Truncate(time.Microsecond), PayloadSHA256: digest, Counts: string(counts), KindStartedAt: string(kindStartedAt)}
	if c.Status == "failed" {
		result.FailureStage = "collection"
	}
	hashes := map[string]string{}
	if c.Status == "complete" {
		for _, kind := range []string{"roles", "clusterRoles"} {
			typ := "Role"
			if kind == "clusterRoles" {
				typ = "ClusterRole"
			}
			for _, raw := range data[kind].([]interface{}) {
				row := raw.(map[string]interface{})
				rules, _ := json.Marshal(row["rules"])
				uid := row["uid"].(string)
				ns, _ := row["namespace"].(string)
				hash, err := inventoryevidence.RoleDigest(clusterID, typ, uid, row["name"].(string), ns, string(rules))
				if err != nil {
					return nil, err
				}
				hashes[inventoryevidence.Key(typ, uid)] = hash
			}
		}
	}
	hashJSON, _ := json.Marshal(hashes)
	result.RoleDigests = string(hashJSON)
	var child *AgentService
	replay := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.persistAgentRecord(ctx, tx, clusterID, agentID); err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.InventoryCollection{ClusterID: clusterID, Status: "unknown"}).Error; err != nil {
			return err
		}
		var prior models.InventoryCollection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cluster_id = ?", clusterID).First(&prior).Error; err != nil {
			return err
		}
		if prior.CollectionID == c.ID {
			if prior.PayloadSHA256 != digest || prior.AgentID != agentID || prior.Namespace != c.Namespace || prior.Status != c.Status || prior.KindStartedAt != result.KindStartedAt || !prior.StartedAt.Equal(c.StartedAt) || !prior.ObservedAt.Equal(c.ObservedAt) {
				return ErrCollectionConflict
			}
			result = prior
			replay = true
			return nil
		}
		if !prior.StartedAt.IsZero() && !c.StartedAt.After(prior.StartedAt) {
			return ErrCollectionConflict
		}
		if c.Status == "complete" {
			child = NewAgentService(tx)
			child.deferWork = true
			child.namespaceScope = c.Namespace
			if err := child.applySyncData(clusterID, clusterName, source, k8sVersion, distribution, data, traceID); err != nil {
				return err
			}
		}
		return tx.Save(&result).Error
	})
	if err != nil {
		// If inventory rolled back, invalidate older success only when this attempt is
		// newer. Never overwrite a concurrently completed newer observation.
		if !errors.Is(err, ErrCollectionConflict) {
			failureCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			failed := result
			failed.Status = "failed"
			failed.FailureStage = "persistence"
			persistErr := s.db.WithContext(failureCtx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "cluster_id"}}, DoUpdates: clause.AssignmentColumns([]string{"agent_id", "collection_id", "namespace", "status", "started_at", "observed_at", "received_at", "payload_sha256", "counts", "kind_started_at", "role_digests", "failure_stage"}), Where: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "inventory_collections.started_at < excluded.started_at"}}}}).Create(&failed).Error
			if persistErr != nil {
				return nil, fmt.Errorf("inventory sync: %v; failed to record collection failure: %w", err, persistErr)
			}
		}
		return nil, err
	}
	if child != nil && !replay {
		child.db = s.db
		child.podInstanceManager = lifecycle.NewPodInstanceManager(s.db)
		for _, fn := range child.afterCommit {
			go fn()
		}
	}
	return &result, nil
}

// Legacy requests still ingest during the explicit HTTP compatibility mode, but
// cannot preserve/promote a verified receipt. Production startup requires the
// table; the table check also supports old, focused service test schemas.
func (s *AgentService) SyncUnverifiedData(ctx context.Context, clusterID, clusterName, source, k8sVersion, distribution string, data map[string]interface{}, traceID string, c *collection.Inventory) error {
	if c != nil {
		if err := ValidateInventoryPayload(*c, data, time.Now()); err != nil {
			return err
		}
	}
	if !s.db.Migrator().HasTable(&models.InventoryCollection{}) {
		if c != nil {
			return fmt.Errorf("inventory collection schema unavailable")
		}
		var compatibilityChild *AgentService
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := s.persistAgentRecord(ctx, tx, clusterID, ""); err != nil {
				return err
			}
			compatibilityChild = NewAgentService(tx)
			compatibilityChild.deferWork = true
			if err := compatibilityChild.applySyncData(clusterID, clusterName, source, k8sVersion, distribution, data, traceID); err != nil {
				return err
			}
			return nil
		})
		if err == nil && compatibilityChild != nil {
			compatibilityChild.db = s.db
			compatibilityChild.podInstanceManager = lifecycle.NewPodInstanceManager(s.db)
			for _, fn := range compatibilityChild.afterCommit {
				go fn()
			}
		}
		return err
	}
	var child *AgentService
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.persistAgentRecord(ctx, tx, clusterID, ""); err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.InventoryCollection{ClusterID: clusterID, Status: "unknown"}).Error; err != nil {
			return err
		}
		var state models.InventoryCollection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cluster_id = ?", clusterID).First(&state).Error; err != nil {
			return err
		}
		if c == nil || c.Status != "failed" {
			child = NewAgentService(tx)
			child.deferWork = true
			if c != nil {
				child.namespaceScope = c.Namespace
			}
			if err := child.applySyncData(clusterID, clusterName, source, k8sVersion, distribution, data, traceID); err != nil {
				return err
			}
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		namespace := ""
		if c != nil {
			namespace = c.Namespace
		}
		return tx.Model(&state).Updates(map[string]interface{}{"status": "unknown", "failure_stage": "", "received_at": now, "started_at": now, "observed_at": time.Time{}, "agent_id": "", "collection_id": "", "namespace": namespace, "payload_sha256": "", "role_digests": "{}", "counts": "{}", "kind_started_at": "{}"}).Error
	})
	if err == nil && child != nil {
		child.db = s.db
		child.podInstanceManager = lifecycle.NewPodInstanceManager(s.db)
		for _, fn := range child.afterCommit {
			go fn()
		}
	}
	return err
}
