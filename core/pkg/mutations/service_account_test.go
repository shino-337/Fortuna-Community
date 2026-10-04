package mutations

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kt "k8s.io/client-go/testing"
)

func mutationFixture(t *testing.T) (*gorm.DB, *fake.Clientset, models.ServiceAccount) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { pool.Close() })
	require.NoError(t, db.AutoMigrate(&models.Cluster{}, &models.ServiceAccount{}, &models.ServiceAccountMutation{}, &models.AuditLog{}))
	require.NoError(t, db.Create(&models.Cluster{ID: "cluster-a", Name: "a"}).Error)
	sa := models.ServiceAccount{ClusterID: "cluster-a", UID: "sa-old", Namespace: "test", Name: "target"}
	require.NoError(t, db.Create(&sa).Error)
	target := rbacv1.Subject{Kind: "ServiceAccount", Name: sa.Name, Namespace: sa.Namespace}
	other := rbacv1.Subject{Kind: "Group", APIGroup: rbacv1.GroupName, Name: "operators"}
	client := fake.NewSimpleClientset(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{UID: "sa-old", Namespace: sa.Namespace, Name: sa.Name}}, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{UID: "binding-old", Namespace: sa.Namespace, Name: "grant", ResourceVersion: "1"}, Subjects: []rbacv1.Subject{target, other}, RoleRef: rbacv1.RoleRef{Kind: "Role", APIGroup: rbacv1.GroupName, Name: "reader"}}, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{UID: "token-old", Namespace: sa.Namespace, Name: "legacy", ResourceVersion: "1", Annotations: map[string]string{corev1.ServiceAccountNameKey: sa.Name, corev1.ServiceAccountUIDKey: sa.UID}}, Type: corev1.SecretTypeServiceAccountToken}, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{UID: "foreign-token", Namespace: sa.Namespace, Name: "foreign", Annotations: map[string]string{corev1.ServiceAccountNameKey: sa.Name, corev1.ServiceAccountUIDKey: "sa-foreign"}}, Type: corev1.SecretTypeServiceAccountToken})
	return db, client, sa
}
func fakeFactory(client kubernetes.Interface) ClientFactory {
	return func(ctx context.Context, id string) (kubernetes.Interface, error) {
		if id != "cluster-a" {
			return nil, ErrDrift
		}
		return client, nil
	}
}
func jobState(t *testing.T, db *gorm.DB, id string) models.ServiceAccountMutation {
	t.Helper()
	var j models.ServiceAccountMutation
	require.NoError(t, db.First(&j, "id = ?", id).Error)
	return j
}
func TestMutationIdentityReplayAndDurability(t *testing.T) {
	db, client, sa := mutationFixture(t)
	ctx := context.Background()
	plan, err := Preview(ctx, client, sa, "revoke")
	require.NoError(t, err)
	require.Len(t, plan.Steps, 2)
	require.Len(t, plan.Limitations, 3)
	job, err := SavePreview(db, sa, 0, "operator", "revoke", plan)
	require.NoError(t, err)
	require.Error(t, Queue(db, job.ID, "changed-digest", 0))
	require.NoError(t, Queue(db, job.ID, job.Digest, 0))
	require.Error(t, Queue(db, job.ID, job.Digest, 0))
	// Kubernetes succeeds, then persistence fails. The intent remains claimable
	// after lease expiry and the same subject removal is safely replayed.
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_mutation_audit BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(ABORT, 'injected audit failure'); END`).Error)
	require.Error(t, Process(ctx, db, fakeFactory(client), job.ID))
	state := jobState(t, db, job.ID)
	require.Equal(t, "running", state.Status)
	require.Zero(t, state.NextStep)
	b, err := client.RbacV1().RoleBindings(sa.Namespace).Get(ctx, "grant", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, b.Subjects, 1)
	require.Equal(t, "operators", b.Subjects[0].Name)
	require.NoError(t, db.Exec(`DROP TRIGGER fail_mutation_audit`).Error)
	require.NoError(t, db.Model(&state).Update("lease_until", time.Now().Add(-time.Second)).Error)
	require.NoError(t, Process(ctx, db, fakeFactory(client), job.ID))
	state = jobState(t, db, job.ID)
	require.Equal(t, 1, state.NextStep)
	require.NoError(t, db.Model(&state).Update("retry_at", time.Now().Add(-time.Second)).Error)
	require.NoError(t, Process(ctx, db, fakeFactory(client), job.ID))
	require.Equal(t, "succeeded", jobState(t, db, job.ID).Status)
	_, err = client.CoreV1().Secrets(sa.Namespace).Get(ctx, "legacy", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().Secrets(sa.Namespace).Get(ctx, "foreign", metav1.GetOptions{})
	require.NoError(t, err)
	var remaining int64
	require.NoError(t, db.Model(&models.ServiceAccount{}).Count(&remaining).Error)
	require.EqualValues(t, 1, remaining, "revocation never deletes the ServiceAccount inventory")
}
func TestMutationBlocksReplacementAndChangedBindings(t *testing.T) {
	for _, kind := range []string{"serviceaccount", "binding"} {
		t.Run(kind, func(t *testing.T) {
			db, client, sa := mutationFixture(t)
			ctx := context.Background()
			plan, err := Preview(ctx, client, sa, "revoke")
			require.NoError(t, err)
			job, err := SavePreview(db, sa, 0, "operator", "revoke", plan)
			require.NoError(t, err)
			require.NoError(t, Queue(db, job.ID, job.Digest, 0))
			if kind == "serviceaccount" {
				actual, err := client.CoreV1().ServiceAccounts(sa.Namespace).Get(ctx, sa.Name, metav1.GetOptions{})
				require.NoError(t, err)
				actual.UID = "replacement"
				_, err = client.CoreV1().ServiceAccounts(sa.Namespace).Update(ctx, actual, metav1.UpdateOptions{})
				require.NoError(t, err)
			} else {
				b, err := client.RbacV1().RoleBindings(sa.Namespace).Get(ctx, "grant", metav1.GetOptions{})
				require.NoError(t, err)
				b.ResourceVersion = "2"
				b.Subjects = append(b.Subjects, rbacv1.Subject{Kind: "User", Name: "new-user"})
				_, err = client.RbacV1().RoleBindings(sa.Namespace).Update(ctx, b, metav1.UpdateOptions{})
				require.NoError(t, err)
			}
			require.NoError(t, Process(ctx, db, fakeFactory(client), job.ID))
			require.Equal(t, "blocked", jobState(t, db, job.ID).Status)
			b, err := client.RbacV1().RoleBindings(sa.Namespace).Get(ctx, "grant", metav1.GetOptions{})
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(b.Subjects), 2)
		})
	}
}
func TestDurableDeletionRetainsUIDPrecondition(t *testing.T) {
	db, client, sa := mutationFixture(t)
	job, err := QueueDeletion(db, sa, 0, "operator")
	require.NoError(t, err)
	same, err := QueueDeletion(db, sa, 0, "operator")
	require.NoError(t, err)
	require.Equal(t, job.ID, same.ID)
	client.PrependReactor("delete", "serviceaccounts", func(action kt.Action) (bool, runtime.Object, error) {
		opts := action.(kt.DeleteAction).GetDeleteOptions()
		require.NotNil(t, opts.Preconditions)
		require.Equal(t, sa.UID, string(*opts.Preconditions.UID))
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "serviceaccounts"}, sa.Name, ErrDrift)
	})
	require.NoError(t, Process(context.Background(), db, fakeFactory(client), job.ID))
	require.Equal(t, "blocked", jobState(t, db, job.ID).Status)
	var count int64
	require.NoError(t, db.Model(&models.ServiceAccount{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestMutationDigestSurvivesJSONBRepresentation(t *testing.T) {
	db, client, sa := mutationFixture(t)
	job, err := QueueDeletion(db, sa, 0, "operator")
	require.NoError(t, err)
	var value any
	require.NoError(t, json.Unmarshal([]byte(job.Plan), &value))
	stored, err := json.MarshalIndent(value, "", "  ")
	require.NoError(t, err)
	require.NotEqual(t, job.Plan, string(stored))
	require.NoError(t, db.Model(&job).Update("plan", string(stored)).Error)
	require.NoError(t, Process(context.Background(), db, fakeFactory(client), job.ID))
	require.Equal(t, "succeeded", jobState(t, db, job.ID).Status)
}

func TestMutationStopsRetryingAfterMaxAttempts(t *testing.T) {
	db, client, sa := mutationFixture(t)
	ctx := context.Background()
	plan, err := Preview(ctx, client, sa, "revoke")
	require.NoError(t, err)
	job, err := SavePreview(db, sa, 0, "operator", "revoke", plan)
	require.NoError(t, err)
	require.NoError(t, Queue(db, job.ID, job.Digest, 0))

	unreachable := func(context.Context, string) (kubernetes.Interface, error) {
		return nil, context.DeadlineExceeded
	}
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		require.NoError(t, Process(ctx, db, unreachable, job.ID))
		state := jobState(t, db, job.ID)
		if attempt < MaxAttempts {
			require.Equal(t, "retry", state.Status, "attempt %d", attempt)
			require.NoError(t, db.Model(&state).Update("retry_at", time.Now().Add(-time.Second)).Error)
			continue
		}
		require.Equal(t, "blocked", state.Status)
		require.Equal(t, MaxAttempts, state.Attempts)
	}
	// A blocked job is never claimed again.
	require.NoError(t, Process(ctx, db, unreachable, job.ID))
	require.Equal(t, MaxAttempts, jobState(t, db, job.ID).Attempts)
}
