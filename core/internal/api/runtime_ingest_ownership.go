package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
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

// requireScopedRuntimeOwnership validates every runtime batch before the
// handler can persist any event and attaches the trusted cluster identity to the
// request context. Registry-backed agents are pinned to their authenticated
// cluster. Legacy agents may continue only when the authoritative Pod inventory
// resolves the complete batch to exactly one cluster; ambiguous ownership fails
// closed rather than falling back to a UID-only write.
func requireScopedRuntimeOwnership(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		principal, scoped := middleware.AgentPrincipal(c)
		clusterID := ""
		if scoped {
			clusterID = principal.ClusterID
			if err := validateRuntimeBatchOwnership(db, clusterID, claims); err != nil {
				abortRuntimeOwnershipError(c, err)
				return
			}
		} else {
			clusterID, err = resolveRuntimeBatchCluster(db, claims)
			if err != nil {
				abortRuntimeOwnershipError(c, err)
				return
			}
		}

		c.Request = c.Request.WithContext(resourceidentity.WithClusterID(c.Request.Context(), clusterID))
		c.Next()
	}
}

func abortRuntimeOwnershipError(c *gin.Context, err error) {
	if _, ok := err.(podOwnershipStorageError); ok {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "runtime ownership verification unavailable", "code": "runtime_ownership_unavailable"})
		return
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "runtime event pod ownership could not be verified", "code": "runtime_ownership_mismatch"})
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

func runtimeExpectedNamespaces(claims []runtimeOwnershipClaim) (map[string]string, error) {
	if len(claims) == 0 {
		return nil, errPodOwnershipMismatch
	}
	expected := make(map[string]string, len(claims))
	for _, claim := range claims {
		uid, namespace := runtimeClaimIdentity(claim)
		if uid == "" || uid == "0" {
			return nil, errPodOwnershipMismatch
		}
		if previous, ok := expected[uid]; ok && previous != "" && namespace != "" && previous != namespace {
			return nil, errPodOwnershipMismatch
		}
		if previous := expected[uid]; previous == "" || namespace != "" {
			expected[uid] = namespace
		}
	}
	return expected, nil
}

func validateRuntimeBatchOwnership(db *gorm.DB, clusterID string, claims []runtimeOwnershipClaim) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return errPodOwnershipMismatch
	}
	expectedNamespace, err := runtimeExpectedNamespaces(claims)
	if err != nil {
		return err
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

// resolveRuntimeBatchCluster is the legacy-mode bridge. It never guesses from
// the first Pod row: every UID must map to one authoritative Pod identity and
// all identities in the batch must belong to the same cluster.
func resolveRuntimeBatchCluster(db *gorm.DB, claims []runtimeOwnershipClaim) (string, error) {
	expectedNamespace, err := runtimeExpectedNamespaces(claims)
	if err != nil {
		return "", err
	}

	uids := make([]string, 0, len(expectedNamespace))
	for uid := range expectedNamespace {
		uids = append(uids, uid)
	}
	var pods []models.Pod
	if err := db.Select("uid", "cluster_id", "namespace").Where("uid IN ?", uids).Find(&pods).Error; err != nil {
		return "", podOwnershipStorageError{err: err}
	}

	byUID := make(map[string][]models.Pod, len(expectedNamespace))
	for _, pod := range pods {
		if strings.TrimSpace(pod.ClusterID) == "" {
			continue
		}
		if expected := expectedNamespace[pod.UID]; expected != "" && expected != strings.TrimSpace(pod.Namespace) {
			continue
		}
		byUID[pod.UID] = append(byUID[pod.UID], pod)
	}

	clusterID := ""
	for uid := range expectedNamespace {
		candidates := byUID[uid]
		if len(candidates) != 1 {
			return "", errPodOwnershipMismatch
		}
		candidateCluster := strings.TrimSpace(candidates[0].ClusterID)
		if clusterID == "" {
			clusterID = candidateCluster
			continue
		}
		if candidateCluster != clusterID {
			return "", errPodOwnershipMismatch
		}
	}
	if clusterID == "" {
		return "", errPodOwnershipMismatch
	}
	return clusterID, nil
}
