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

type runtimeOwnershipClaim struct {
	Pod struct {
		UID       string `json:"uid"`
		Namespace string `json:"namespace"`
	} `json:"pod"`
	PodUID    string `json:"pod_uid"`
	PodUid    string `json:"podUid"`
	Namespace string `json:"namespace"`
}

// requireScopedRuntimeOwnership validates the entire runtime batch against the
// authenticated principal's cluster before the handler can persist any event.
// It is intentionally not wired into routes until agent credential provisioning
// and retry behavior are ready for rollout (C2e2/C2f).
func requireScopedRuntimeOwnership(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, scoped := middleware.AgentPrincipal(c)
		if !scoped {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid runtime ingest body", "code": "invalid_runtime_payload"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		claims, err := decodeRuntimeOwnershipClaims(body)
		if err != nil || len(claims) == 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "runtime batch requires pod identity", "code": "invalid_runtime_ownership_claims"})
			return
		}
		if err := validateRuntimeBatchOwnership(db, principal.ClusterID, claims); err != nil {
			if _, ok := err.(podOwnershipStorageError); ok {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "runtime ownership verification unavailable", "code": "runtime_ownership_unavailable"})
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "runtime event pod does not belong to authenticated agent cluster", "code": "runtime_ownership_mismatch"})
			return
		}
		c.Next()
	}
}

func decodeRuntimeOwnershipClaims(body []byte) ([]runtimeOwnershipClaim, error) {
	var batch []runtimeOwnershipClaim
	if err := json.Unmarshal(body, &batch); err == nil {
		return batch, nil
	}
	var single runtimeOwnershipClaim
	if err := json.Unmarshal(body, &single); err != nil {
		return nil, err
	}
	return []runtimeOwnershipClaim{single}, nil
}

func runtimeClaimIdentity(c runtimeOwnershipClaim) (string, string) {
	uid := strings.TrimSpace(c.Pod.UID)
	if uid == "" {
		uid = strings.TrimSpace(c.PodUID)
	}
	if uid == "" {
		uid = strings.TrimSpace(c.PodUid)
	}
	namespace := strings.TrimSpace(c.Pod.Namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(c.Namespace)
	}
	return uid, namespace
}

func validateRuntimeBatchOwnership(db *gorm.DB, clusterID string, claims []runtimeOwnershipClaim) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" || len(claims) == 0 {
		return errPodOwnershipMismatch
	}

	expectedNamespace := make(map[string]string, len(claims))
	for _, claim := range claims {
		uid, namespace := runtimeClaimIdentity(claim)
		if uid == "" || uid == "0" {
			return errPodOwnershipMismatch
		}
		if previous, ok := expectedNamespace[uid]; ok && previous != "" && namespace != "" && previous != namespace {
			return errPodOwnershipMismatch
		}
		if previous := expectedNamespace[uid]; previous == "" || namespace != "" {
			expectedNamespace[uid] = namespace
		}
	}

	uids := make([]string, 0, len(expectedNamespace))
	for uid := range expectedNamespace {
		uids = append(uids, uid)
	}
	var pods []models.Pod
	if err := db.Select("uid", "cluster_id", "namespace").Where("cluster_id = ? AND uid IN ?", clusterID, uids).Find(&pods).Error; err != nil {
		return podOwnershipStorageError{err: err}
	}
	if len(pods) != len(expectedNamespace) {
		return errPodOwnershipMismatch
	}
	for _, pod := range pods {
		if expected := expectedNamespace[pod.UID]; expected != "" && expected != strings.TrimSpace(pod.Namespace) {
			return errPodOwnershipMismatch
		}
	}
	return nil
}
