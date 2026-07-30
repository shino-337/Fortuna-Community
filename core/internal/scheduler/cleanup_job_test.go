package scheduler

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodCleanupJobScopesGhostDeletionToLocalCluster(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	localCluster := "local-cluster"
	remoteCluster := "remote-cluster"
	seedPods := []models.Pod{
		{ClusterID: localCluster, UID: "local-live", Name: "local-live", Namespace: "default"},
		{ClusterID: localCluster, UID: "local-ghost", Name: "local-ghost", Namespace: "default"},
		{ClusterID: remoteCluster, UID: "remote-live", Name: "remote-live", Namespace: "default"},
	}
	if err := db.Create(&seedPods).Error; err != nil {
		t.Fatalf("seed pods: %v", err)
	}

	job := &PodCleanupJob{
		db: db,
		k8sClient: fake.NewSimpleClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       types.UID("local-live"),
				Name:      "local-live",
				Namespace: "default",
			},
		}),
	}
	job.run()

	var localGhostActive int64
	if err := db.Model(&models.Pod{}).Where("cluster_id = ? AND uid = ?", localCluster, "local-ghost").Count(&localGhostActive).Error; err != nil {
		t.Fatalf("count local ghost: %v", err)
	}
	if localGhostActive != 0 {
		t.Fatalf("local ghost active=%d, want 0", localGhostActive)
	}

	var remoteActive int64
	if err := db.Model(&models.Pod{}).Where("cluster_id = ?", remoteCluster).Count(&remoteActive).Error; err != nil {
		t.Fatalf("count remote: %v", err)
	}
	if remoteActive != 1 {
		t.Fatalf("remote active=%d, want 1", remoteActive)
	}
}
