package api

import (
	"context"

	"github.com/fortuna/core/internal/k8s"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/mutations"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
)

func auditUserID(c *gin.Context) uint {
	if v, ok := c.Get("userID"); ok {
		if id, ok := v.(uint); ok && id > 0 {
			return id
		}
	}
	return 0
}

func deleteServiceAccountResource(db *gorm.DB, c *gin.Context, sa *models.ServiceAccount) (int, string) {
	if sa.UID == "" {
		return 409, "Kubernetes UID is required; inventory record retained"
	}
	var cluster models.Cluster
	if err := db.WithContext(c.Request.Context()).Where("id = ?", sa.ClusterID).First(&cluster).Error; err != nil {
		return 503, "Target cluster is unavailable; inventory record retained"
	}
	if cluster.Kubeconfig == "" {
		return 503, "cluster-specific Kubernetes credentials are required for deletion"
	}
	client, err := k8s.NewClientFromKubeconfig(cluster.Kubeconfig)
	if err != nil {
		return 502, "failed to initialize the target cluster client"
	}
	job, err := mutations.QueueDeletion(db.WithContext(c.Request.Context()), *sa, auditUserID(c), c.GetString("username"))
	if err != nil {
		return 503, "Unable to persist deletion intent; no Kubernetes changes made"
	}
	factory := func(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
		if clusterID != sa.ClusterID {
			return nil, mutations.ErrDrift
		}
		return client.Clientset, nil
	}
	if err = mutations.Process(c.Request.Context(), db, factory, job.ID); err != nil {
		return 503, "Deletion intent retained; worker will retry after persistence recovery"
	}
	if err = db.First(&job, "id = ?", job.ID).Error; err != nil {
		return 503, "Deletion status unavailable; durable intent retained"
	}
	if job.Status == "succeeded" {
		return 200, "ServiceAccount deleted from Kubernetes and inventory"
	}
	if job.Status == "blocked" {
		return 409, "Kubernetes identity or permission changed; review a new mutation preview"
	}
	return 502, "Kubernetes deletion pending durable retry; inventory record retained"
}
