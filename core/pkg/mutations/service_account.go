package mutations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/fortuna/core/internal/k8s"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

var ErrDrift = errors.New("Kubernetes identity or preview changed; create a fresh preview")

type Step struct {
	Kind            string `json:"kind"`
	Namespace       string `json:"namespace,omitempty"`
	Name            string `json:"name"`
	UID             string `json:"uid"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	Before, After   []rbacv1.Subject
	RoleRef         rbacv1.RoleRef
}
type Plan struct {
	Version     int      `json:"version"`
	Namespace   string   `json:"namespace"`
	Name        string   `json:"name"`
	Steps       []Step   `json:"steps"`
	Limitations []string `json:"limitations"`
}

func removeSubject(subjects []rbacv1.Subject, sa models.ServiceAccount, defaultNS string) ([]rbacv1.Subject, bool) {
	after := []rbacv1.Subject{}
	changed := false
	for _, s := range subjects {
		ns := s.Namespace
		if ns == "" {
			ns = defaultNS
		}
		if s.Kind == "ServiceAccount" && s.APIGroup == "" && s.Name == sa.Name && ns == sa.Namespace {
			changed = true
			continue
		}
		after = append(after, s)
	}
	return after, changed
}
func Preview(ctx context.Context, client kubernetes.Interface, sa models.ServiceAccount, action string) (Plan, error) {
	plan := Plan{Version: 1, Namespace: sa.Namespace, Name: sa.Name, Steps: []Step{}, Limitations: []string{}}
	if sa.ClusterID == "" || sa.UID == "" || sa.Name == "" || sa.Namespace == "" {
		return plan, ErrDrift
	}
	actual, err := client.CoreV1().ServiceAccounts(sa.Namespace).Get(ctx, sa.Name, metav1.GetOptions{})
	if err != nil {
		return plan, err
	}
	if string(actual.UID) != sa.UID {
		return plan, ErrDrift
	}
	if action == "delete" {
		plan.Steps = append(plan.Steps, Step{Kind: "ServiceAccount", Namespace: sa.Namespace, Name: sa.Name, UID: sa.UID})
		plan.Limitations = []string{"Deletes the Kubernetes ServiceAccount and reconciles only its exact inventory identity. Existing Pods are not deleted."}
		return plan, nil
	}
	if action != "revoke" {
		return plan, fmt.Errorf("action must be revoke or delete")
	}
	bindings, err := client.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return plan, err
	}
	if bindings.Continue != "" {
		return plan, fmt.Errorf("RBAC preview exceeds 500 objects; use a narrower operator workflow")
	}
	for _, b := range bindings.Items {
		after, changed := removeSubject(b.Subjects, sa, b.Namespace)
		if changed {
			plan.Steps = append(plan.Steps, Step{Kind: "RoleBinding", Namespace: b.Namespace, Name: b.Name, UID: string(b.UID), ResourceVersion: b.ResourceVersion, Before: b.Subjects, After: after, RoleRef: b.RoleRef})
		}
	}
	clusterBindings, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return plan, err
	}
	if clusterBindings.Continue != "" {
		return plan, fmt.Errorf("cluster RBAC preview exceeds 500 objects")
	}
	for _, b := range clusterBindings.Items {
		after, changed := removeSubject(b.Subjects, sa, "")
		if changed {
			plan.Steps = append(plan.Steps, Step{Kind: "ClusterRoleBinding", Name: b.Name, UID: string(b.UID), ResourceVersion: b.ResourceVersion, Before: b.Subjects, After: after, RoleRef: b.RoleRef})
		}
	}
	secrets, err := client.CoreV1().Secrets(sa.Namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return plan, err
	}
	if secrets.Continue != "" {
		return plan, fmt.Errorf("legacy token preview exceeds 500 objects")
	}
	for _, s := range secrets.Items {
		if s.Type == corev1.SecretTypeServiceAccountToken && s.Annotations[corev1.ServiceAccountNameKey] == sa.Name && s.Annotations[corev1.ServiceAccountUIDKey] == sa.UID {
			plan.Steps = append(plan.Steps, Step{Kind: "Secret", Namespace: s.Namespace, Name: s.Name, UID: string(s.UID), ResourceVersion: s.ResourceVersion})
		}
	}
	for _, s := range plan.Steps {
		if s.UID == "" || s.ResourceVersion == "" {
			return plan, ErrDrift
		}
	}
	plan.Limitations = []string{"Removes only this ServiceAccount's direct subjects from the listed bindings; other subjects and roles remain unchanged.", "Deletes only the listed legacy ServiceAccount token Secrets. Bound/TokenRequest tokens and existing Pods are not revoked or deleted.", "Group/inherited permissions, newly added bindings/tokens, external credentials and admission policy are outside this preview. This operation does not disable the ServiceAccount."}
	return plan, nil
}
func SavePreview(db *gorm.DB, sa models.ServiceAccount, actorID uint, actor, action string, plan Plan) (models.ServiceAccountMutation, error) {
	raw, err := json.Marshal(plan)
	if err != nil {
		return models.ServiceAccountMutation{}, err
	}
	hash := sha256.Sum256(raw)
	now := time.Now().UTC()
	job := models.ServiceAccountMutation{ID: rand.Text(), ClusterID: sa.ClusterID, ServiceAccountID: sa.ID, UID: sa.UID, ActorID: actorID, Actor: actor, Action: action, Plan: string(raw), Digest: hex.EncodeToString(hash[:]), Status: "preview", ExpiresAt: now.Add(10 * time.Minute), RetryAt: now}
	err = db.Create(&job).Error
	return job, err
}
func Queue(db *gorm.DB, id, digest string, actorID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var job models.ServiceAccountMutation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND digest = ? AND actor_id = ? AND status = ? AND expires_at > ?", id, digest, actorID, "preview", time.Now().UTC()).First(&job).Error; err != nil {
			return ErrDrift
		}
		if err := tx.Model(&job).Updates(map[string]any{"status": "queued", "retry_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]any{"operation_id": job.ID, "status": "queued", "plan_digest": job.Digest})
		audit := models.AuditLog{ClusterID: job.ClusterID, UserID: job.ActorID, User: job.Actor, Action: job.Action + "_intent", Resource: "serviceaccount_mutation", ResourceID: job.ID, Details: string(details)}
		if actorID == 0 {
			return tx.Omit("UserID").Create(&audit).Error
		}
		return tx.Create(&audit).Error
	})
}

// apply is idempotent across a crash after a successful Kubernetes write. UID
// and optimistic version checks prevent replacement/deviating objects from being touched.
func sameSubjects(a, b []rbacv1.Subject) bool {
	return (len(a) == 0 && len(b) == 0) || reflect.DeepEqual(a, b)
}
func apply(ctx context.Context, client kubernetes.Interface, job models.ServiceAccountMutation, plan Plan, step Step) error {
	if job.Action == "revoke" {
		sa, err := client.CoreV1().ServiceAccounts(plan.Namespace).Get(ctx, plan.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return ErrDrift
		}
		if err != nil {
			return err
		}
		if string(sa.UID) != job.UID {
			return ErrDrift
		}
	}
	uid := types.UID(step.UID)
	opts := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}
	if step.Kind == "ServiceAccount" {
		err := client.CoreV1().ServiceAccounts(step.Namespace).Delete(ctx, step.Name, opts)
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if step.Kind == "Secret" {
		rv := step.ResourceVersion
		opts.Preconditions.ResourceVersion = &rv
		err := client.CoreV1().Secrets(step.Namespace).Delete(ctx, step.Name, opts)
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if step.Kind == "RoleBinding" {
		b, err := client.RbacV1().RoleBindings(step.Namespace).Get(ctx, step.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if string(b.UID) != step.UID || b.RoleRef != step.RoleRef {
			return ErrDrift
		}
		if sameSubjects(b.Subjects, step.After) {
			return nil
		}
		if b.ResourceVersion != step.ResourceVersion || !sameSubjects(b.Subjects, step.Before) {
			return ErrDrift
		}
		b.Subjects = step.After
		_, err = client.RbacV1().RoleBindings(step.Namespace).Update(ctx, b, metav1.UpdateOptions{})
		return err
	}
	if step.Kind == "ClusterRoleBinding" {
		b, err := client.RbacV1().ClusterRoleBindings().Get(ctx, step.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if string(b.UID) != step.UID || b.RoleRef != step.RoleRef {
			return ErrDrift
		}
		if sameSubjects(b.Subjects, step.After) {
			return nil
		}
		if b.ResourceVersion != step.ResourceVersion || !sameSubjects(b.Subjects, step.Before) {
			return ErrDrift
		}
		b.Subjects = step.After
		_, err = client.RbacV1().ClusterRoleBindings().Update(ctx, b, metav1.UpdateOptions{})
		return err
	}
	return ErrDrift
}

type ClientFactory func(context.Context, string) (kubernetes.Interface, error)

func ClusterClient(db *gorm.DB) ClientFactory {
	return func(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
		var cluster models.Cluster
		if err := db.WithContext(ctx).First(&cluster, "id = ?", clusterID).Error; err != nil {
			return nil, err
		}
		if cluster.Kubeconfig == "" {
			return nil, fmt.Errorf("cluster-specific credentials required")
		}
		client, err := k8s.NewClientFromKubeconfig(cluster.Kubeconfig)
		if err != nil {
			return nil, err
		}
		return client.Clientset, nil
	}
}
func ProcessOne(ctx context.Context, db *gorm.DB, factory ClientFactory) error {
	return Process(ctx, db, factory, "")
}
func Process(ctx context.Context, db *gorm.DB, factory ClientFactory, id string) error {
	now := time.Now().UTC()
	var job models.ServiceAccountMutation
	claimed := false
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if id != "" {
			query = query.Where("id = ?", id)
		}
		err := query.Where("(status IN ('queued','retry') AND retry_at <= ?) OR (status = 'running' AND lease_until <= ?)", now, now).Order("created_at ASC").First(&job).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		job.LeaseToken = rand.Text()
		lease := now.Add(45 * time.Second)
		job.LeaseUntil = &lease
		job.Status = "running"
		job.Attempts++
		if err = tx.Model(&job).Updates(map[string]any{"lease_token": job.LeaseToken, "lease_until": lease, "status": "running", "attempts": job.Attempts}).Error; err != nil {
			return err
		}
		claimed = true
		return nil
	})
	if err != nil || !claimed {
		return err
	}
	var plan Plan
	applyErr := json.Unmarshal([]byte(job.Plan), &plan)
	hash := sha256.Sum256([]byte(job.Plan))
	if applyErr == nil && (plan.Version != 1 || job.Digest != hex.EncodeToString(hash[:]) || job.NextStep < 0 || job.NextStep > len(plan.Steps)) {
		applyErr = ErrDrift
	}
	done := job.NextStep >= len(plan.Steps)
	if applyErr == nil && !done {
		requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		client, err := factory(requestCtx, job.ClusterID)
		if err == nil {
			err = apply(requestCtx, client, job, plan, plan.Steps[job.NextStep])
		}
		cancel()
		applyErr = err
		if err == nil {
			job.NextStep++
			done = job.NextStep == len(plan.Steps)
		}
	}
	status, message := "queued", ""
	if done && applyErr == nil {
		status = "succeeded"
	}
	if applyErr != nil {
		status = "retry"
		message = "Kubernetes operation unavailable; retry scheduled"
		if errors.Is(applyErr, ErrDrift) || apierrors.IsConflict(applyErr) || apierrors.IsForbidden(applyErr) || apierrors.IsUnauthorized(applyErr) {
			status = "blocked"
			message = "Identity, preview or permission changed; review a new preview"
		}
	}
	retryAt := time.Now().UTC()
	if status == "retry" {
		retryAt = retryAt.Add(30 * time.Second)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.ServiceAccountMutation{}).Where("id = ? AND lease_token = ? AND status = 'running'", job.ID, job.LeaseToken).Updates(map[string]any{"status": status, "next_step": job.NextStep, "last_error": message, "lease_until": nil, "lease_token": "", "retry_at": retryAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("mutation lease lost")
		}
		if applyErr == nil && job.Action == "delete" && done {
			if err := tx.Where("id = ? AND cluster_id = ? AND uid = ?", job.ServiceAccountID, job.ClusterID, job.UID).Delete(&models.ServiceAccount{}).Error; err != nil {
				return err
			}
		}
		detail, _ := json.Marshal(map[string]any{"operation_id": job.ID, "status": status, "completed_steps": job.NextStep, "plan_digest": job.Digest})
		audit := models.AuditLog{ClusterID: job.ClusterID, UserID: job.ActorID, User: job.Actor, Action: job.Action, Resource: "serviceaccount_mutation", ResourceID: job.ID, Details: string(detail)}
		if job.ActorID == 0 {
			return tx.Omit("UserID").Create(&audit).Error
		}
		return tx.Create(&audit).Error
	})
}
func Start(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	factory := ClusterClient(db)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = ProcessOne(ctx, db, factory)
		}
	}
}

// QueueDeletion keeps the existing explicit DELETE action durable too. The
// cluster/UID key makes concurrent/retried requests share the original intent.
func QueueDeletion(db *gorm.DB, sa models.ServiceAccount, actorID uint, actor string) (models.ServiceAccountMutation, error) {
	plan := Plan{Version: 1, Namespace: sa.Namespace, Name: sa.Name, Steps: []Step{{Kind: "ServiceAccount", Namespace: sa.Namespace, Name: sa.Name, UID: sa.UID}}, Limitations: []string{"Deletes only the exact ServiceAccount UID; Pods remain unchanged."}}
	raw, err := json.Marshal(plan)
	if err != nil {
		return models.ServiceAccountMutation{}, err
	}
	digest := sha256.Sum256(raw)
	identity := sha256.Sum256([]byte(sa.ClusterID + "\x00" + sa.UID))
	now := time.Now().UTC()
	job := models.ServiceAccountMutation{ID: "delete-" + hex.EncodeToString(identity[:24]), ClusterID: sa.ClusterID, ServiceAccountID: sa.ID, UID: sa.UID, ActorID: actorID, Actor: actor, Action: "delete", Plan: string(raw), Digest: hex.EncodeToString(digest[:]), Status: "queued", RetryAt: now, ExpiresAt: now.Add(10 * time.Minute)}
	err = db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&job)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return tx.First(&job, "id = ?", job.ID).Error
		}
		details, _ := json.Marshal(map[string]any{"operation_id": job.ID, "plan_digest": job.Digest, "status": "queued"})
		audit := models.AuditLog{ClusterID: sa.ClusterID, UserID: actorID, User: actor, Action: "delete_intent", Resource: "serviceaccount_mutation", ResourceID: job.ID, Details: string(details)}
		if actorID == 0 {
			return tx.Omit("UserID").Create(&audit).Error
		}
		return tx.Create(&audit).Error
	})
	return job, err
}
