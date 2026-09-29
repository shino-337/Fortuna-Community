package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sourcehealth"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errSourceHealthConflict = errors.New("source-health session, replay or ordering conflict")

func PostRuntimeSourceHealth(db *gorm.DB, registryPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, scoped := middleware.AgentPrincipal(c)
		if !scoped || principal.ClusterID == "" || principal.AgentID == "" {
			c.JSON(401, gin.H{"code": "source_health_identity_required"})
			return
		}
		var signed collection.SignedRuntimeSourceHealth
		decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16384))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&signed); err != nil {
			c.JSON(400, gin.H{"code": "invalid_source_health"})
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			c.JSON(400, gin.H{"code": "invalid_source_health"})
			return
		}
		r := signed.Report
		r.SourceStartedAt = r.SourceStartedAt.UTC().Truncate(time.Microsecond)
		r.WindowStart = r.WindowStart.UTC().Truncate(time.Microsecond)
		r.WindowEnd = r.WindowEnd.UTC().Truncate(time.Microsecond)
		r.ValidUntil = r.ValidUntil.UTC().Truncate(time.Microsecond)
		signed.Report = r
		now := time.Now().UTC().Truncate(time.Microsecond)
		if err := r.Validate(now); err != nil {
			c.JSON(400, gin.H{"code": "invalid_source_health", "error": err.Error()})
			return
		}
		if r.ClusterID != principal.ClusterID || r.AgentID != principal.AgentID {
			c.JSON(403, gin.H{"code": "source_health_ownership_mismatch"})
			return
		}
		if err := sourcehealth.Verify(registryPath, signed, now); err != nil {
			c.JSON(403, gin.H{"code": "source_health_untrusted"})
			return
		}
		payload, err := r.SigningBytes()
		if err != nil {
			c.JSON(400, gin.H{"code": "invalid_source_health"})
			return
		}
		hash := sha256.Sum256(payload)
		receipt := models.RuntimeSourceHealthReceipt{ClusterID: r.ClusterID, AgentID: r.AgentID, ProducerID: r.ProducerID, SessionID: r.SessionID, SourceSessionID: r.SourceSessionID, Sequence: r.Sequence, KeyID: r.KeyID, PayloadHash: hex.EncodeToString(hash[:]), Payload: string(payload), Signature: signed.Signature, ReceivedAt: now}
		replay := false
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var producer models.RuntimeProducerState
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", r.ClusterID, r.AgentID, r.ProducerID).First(&producer).Error; err != nil {
				return err
			}
			if producer.SessionID != r.SessionID || producer.SourceKind != r.SourceKind || !producer.Enabled || !producer.LeaseFresh(now) || producer.State == collection.RuntimeProducerStopped || producer.State == collection.RuntimeProducerDisabled || r.WindowStart.Before(producer.SessionStartedAt) {
				return errSourceHealthConflict
			}
			var prior models.RuntimeSourceHealthReceipt
			found := tx.Where("cluster_id = ? AND agent_id = ? AND producer_id = ? AND session_id = ? AND source_session_id = ? AND sequence = ?", r.ClusterID, r.AgentID, r.ProducerID, r.SessionID, r.SourceSessionID, r.Sequence).First(&prior).Error
			if found == nil {
				if prior.PayloadHash != receipt.PayloadHash {
					return errSourceHealthConflict
				}
				replay = true
				return nil // Exact replay never renews the health lease or authority.
			}
			if !errors.Is(found, gorm.ErrRecordNotFound) {
				return found
			}
			sameSource := producer.SourceSessionID == r.SourceSessionID
			if sameSource {
				if producer.SourceStartedAt == nil || !producer.SourceStartedAt.Equal(r.SourceStartedAt) || r.Sequence <= producer.SourceHealthSequence || (producer.SourceHealthEnd != nil && r.WindowStart.Before(*producer.SourceHealthEnd)) {
					return errSourceHealthConflict
				}
			} else if producer.SourceStartedAt != nil && !r.SourceStartedAt.After(*producer.SourceStartedAt) {
				return errSourceHealthConflict
			}
			var since *time.Time
			if r.Status == "healthy" {
				start := r.WindowStart
				if sameSource && r.Sequence == producer.SourceHealthSequence+1 && producer.SourceHealthSince != nil && producer.SourceHealthEnd != nil && r.WindowStart.Equal(*producer.SourceHealthEnd) && producer.SourceHealthCovers(*producer.SourceHealthEnd, *producer.SourceHealthEnd, now) {
					start = *producer.SourceHealthSince
				}
				since = &start
			}
			if err := tx.Create(&receipt).Error; err != nil {
				return err
			}
			changes := map[string]any{"source_session_id": r.SourceSessionID, "source_started_at": r.SourceStartedAt, "source_health_since": since, "source_health_end": r.WindowEnd, "source_health_received_at": now, "source_health_sequence": r.Sequence, "source_health_expires_at": r.ValidUntil, "authoritative": r.Status == "healthy"}
			if !sameSource || r.Status == "failed" {
				changes["gap_since"] = r.WindowStart
				changes["gap_reason"] = "source_restart"
				if r.Status == "failed" {
					changes["gap_reason"] = "source_health_failed"
				}
				// A source restart/failure breaks event coverage even if later health
				// arrives before the next event window.
				if err := tx.Model(&models.RuntimeCoverage{}).Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", r.ClusterID, r.AgentID, r.ProducerID).Update("continuous_since", nil).Error; err != nil {
					return err
				}
			}
			return tx.Model(&producer).Updates(changes).Error
		})
		if err != nil {
			if errors.Is(err, errSourceHealthConflict) || errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(409, gin.H{"code": "source_health_conflict"})
				return
			}
			c.JSON(503, gin.H{"code": "source_health_persistence_unavailable", "retryable": true})
			return
		}
		c.JSON(200, gin.H{"success": true, "replay": replay})
	}
}
