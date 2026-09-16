package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type agentPodEvidenceClaims struct {
	PodUID    string `json:"podUid"`
	ClusterID string `json:"clusterId"`
	Namespace string `json:"namespace"`
}

type agentEventEvidenceClaims struct {
	ClusterID string `json:"clusterId"`
	Events    []struct {
		InvolvedKind string `json:"involvedKind"`
		InvolvedUID  string `json:"involvedUid"`
		Namespace    string `json:"namespace"`
	} `json:"events"`
}

// requireScopedPodEvidenceOwnership validates request ownership before the
// downstream handler can perform idempotency checks or writes. Legacy shared-token
// mode is intentionally preserved until migration is complete.
func requireScopedPodEvidenceOwnership(db *gorm.DB, eventBatch bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, scoped := middleware.AgentPrincipal(c)
		if !scoped {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid ingest body", "code": "invalid_agent_payload"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if eventBatch {
			var req agentEventEvidenceClaims
			if err := json.Unmarshal(body, &req); err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid ingest body", "code": "invalid_agent_payload"})
				return
			}
			if strings.TrimSpace(req.ClusterID) == "" || strings.TrimSpace(req.ClusterID) != principal.ClusterID {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "authenticated agent cannot write requested cluster", "code": "agent_cluster_mismatch"})
				return
			}
			if err := validatePodEventBatchOwnership(db, principal.ClusterID, req); err != nil {
				abortPodOwnershipError(c, err)
				return
			}
			c.Next()
			return
		}

		var req agentPodEvidenceClaims
		if err := json.Unmarshal(body, &req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid ingest body", "code": "invalid_agent_payload"})
			return
		}
		req.PodUID = strings.TrimSpace(req.PodUID)
		req.ClusterID = strings.TrimSpace(req.ClusterID)
		req.Namespace = strings.TrimSpace(req.Namespace)
		if req.PodUID == "" || req.PodUID == "0" || req.ClusterID == "" || req.Namespace == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "podUid, clusterId and namespace are required", "code": "invalid_pod_evidence_claims"})
			return
		}
		if req.ClusterID != principal.ClusterID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "authenticated agent cannot write requested cluster", "code": "agent_cluster_mismatch"})
			return
		}
		if err := validateOnePodOwnership(db, principal.ClusterID, req.PodUID, req.Namespace); err != nil {
			abortPodOwnershipError(c, err)
			return
		}
		c.Next()
	}
}

var errPodOwnershipMismatch = gorm.ErrRecordNotFound

type podOwnershipStorageError struct{ err error }

func (e podOwnershipStorageError) Error() string { return e.err.Error() }

func validateOnePodOwnership(db *gorm.DB, clusterID, podUID, namespace string) error {
	var pod models.Pod
	err := db.Select("uid", "cluster_id", "namespace").Where("cluster_id = ? AND uid = ?", clusterID, podUID).First(&pod).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errPodOwnershipMismatch
		}
		return podOwnershipStorageError{err: err}
	}
	if strings.TrimSpace(pod.Namespace) != strings.TrimSpace(namespace) {
		return errPodOwnershipMismatch
	}
	return nil
}

func validatePodEventBatchOwnership(db *gorm.DB, clusterID string, req agentEventEvidenceClaims) error {
	expected := map[string]string{}
	for _, event := range req.Events {
		if !strings.EqualFold(strings.TrimSpace(event.InvolvedKind), "Pod") {
			continue
		}
		uid := strings.TrimSpace(event.InvolvedUID)
		namespace := strings.TrimSpace(event.Namespace)
		if uid == "" || uid == "0" || namespace == "" {
			return errPodOwnershipMismatch
		}
		if existing, ok := expected[uid]; ok && existing != namespace {
			return errPodOwnershipMismatch
		}
		expected[uid] = namespace
	}
	if len(expected) == 0 {
		return nil
	}
	uids := make([]string, 0, len(expected))
	for uid := range expected {
		uids = append(uids, uid)
	}
	var pods []models.Pod
	if err := db.Select("uid", "cluster_id", "namespace").Where("cluster_id = ? AND uid IN ?", clusterID, uids).Find(&pods).Error; err != nil {
		return podOwnershipStorageError{err: err}
	}
	if len(pods) != len(expected) {
		return errPodOwnershipMismatch
	}
	for _, pod := range pods {
		if expected[pod.UID] != strings.TrimSpace(pod.Namespace) {
			return errPodOwnershipMismatch
		}
	}
	return nil
}

func abortPodOwnershipError(c *gin.Context, err error) {
	if _, ok := err.(podOwnershipStorageError); ok {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "pod ownership verification unavailable", "code": "pod_ownership_unavailable"})
		return
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "pod does not belong to authenticated agent cluster", "code": "pod_ownership_mismatch"})
}
