package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/internal/config"
	corek8s "github.com/fortuna/core/internal/k8s"
	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/internal/storage"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/mutations"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/fortuna/core/pkg/worker"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	appsv1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sync"
)

func livePostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("FORTUNA_TEST_POSTGRES_URL required by live gate")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := admin.DB()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	name := fmt.Sprintf("two_cluster_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+name).Error)
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + name + " CASCADE") })
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", name)
	u.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(25)
	t.Cleanup(func() { sqlDB.Close() })
	require.NoError(t, storage.Migrate(db))
	if !db.Migrator().HasTable(&models.UserSession{}) {
		require.NoError(t, db.Migrator().CreateTable(&models.UserSession{}))
	}
	return db
}

type liveCluster struct {
	id         string
	client     kubernetes.Interface
	kubeconfig string
	nodes      []string
	tokens     map[string]map[string]string
}

func createLiveFixture(t *testing.T, path, id string) liveCluster {
	t.Helper()
	ctx := context.Background()
	client, err := corek8s.NewClientFromPath(path)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	nodes, err := client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	cluster := liveCluster{id: id, client: client.Clientset, kubeconfig: string(raw), tokens: map[string]map[string]string{}}
	for _, node := range nodes.Items {
		cluster.nodes = append(cluster.nodes, node.Name)
	}
	for _, ns := range []string{"audit-scope-a", "audit-scope-b", "audit-agents"} {
		_, err = cluster.client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}, metav1.CreateOptions{})
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = cluster.client.CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{})
		})
	}
	_, err = cluster.client.CoreV1().ServiceAccounts("audit-agents").Create(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "reader"}}, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = cluster.client.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "audit-reader"}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{"", "apps", "rbac.authorization.k8s.io"}, Resources: []string{"pods", "nodes", "events", "serviceaccounts", "roles", "rolebindings", "clusterroles", "clusterrolebindings", "deployments", "replicasets"}, Verbs: []string{"get", "list", "watch"}}, {NonResourceURLs: []string{"/version"}, Verbs: []string{"get"}}}}, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cluster.client.RbacV1().ClusterRoles().Delete(context.Background(), "audit-reader", metav1.DeleteOptions{})
	})
	_, err = cluster.client.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "audit-reader"}, RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "audit-reader"}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "audit-agents", Name: "reader"}}}, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cluster.client.RbacV1().ClusterRoleBindings().Delete(context.Background(), "audit-reader", metav1.DeleteOptions{})
	})
	for _, scope := range []string{"a", "b"} {
		ns := "audit-scope-" + scope
		cluster.tokens[scope] = map[string]string{}
		for _, node := range cluster.nodes {
			cluster.tokens[scope][node] = rand.Text() + rand.Text()
		}
		_, err = cluster.client.CoreV1().ServiceAccounts(ns).Create(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "first-investigation"}}, metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = cluster.client.CoreV1().Pods(ns).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "same-pod-name"}, Spec: corev1.PodSpec{ServiceAccountName: "first-investigation", Containers: []corev1.Container{{Name: "pause", Image: "registry.k8s.io/pause:3.10.1"}}}}, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	_, err = cluster.client.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "first-investigation-admin"}, RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "audit-scope-a", Name: "first-investigation"}}}, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cluster.client.RbacV1().ClusterRoleBindings().Delete(context.Background(), "first-investigation-admin", metav1.DeleteOptions{})
	})
	return cluster
}
func deployLiveAgents(t *testing.T, cluster liveCluster, scope, image, endpoint string) {
	t.Helper()
	ctx := context.Background()
	data := map[string][]byte{}
	for node, token := range cluster.tokens[scope] {
		data[node] = []byte(token)
	}
	_, err := cluster.client.CoreV1().Secrets("audit-agents").Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tokens-" + scope}, Data: data}, metav1.CreateOptions{})
	require.NoError(t, err)
	labels := map[string]string{"app": "audit-agent-" + scope}
	env := []corev1.EnvVar{{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}}, {Name: "AGENT_ID", Value: "$(NODE_NAME)-scope-" + scope + "-agent"}, {Name: "CLUSTER_ID", Value: cluster.id}, {Name: "CLUSTER_NAME", Value: cluster.id}, {Name: "CORE_HTTP_ENDPOINT", Value: endpoint}, {Name: "CORE_GRPC_ENDPOINT", Value: "127.0.0.1:1"}, {Name: "TLS_ENABLED", Value: "false"}, {Name: "WATCH_NAMESPACE", Value: "audit-scope-" + scope}, {Name: "SYNC_INTERVAL", Value: "30s"}, {Name: "FORTUNA_AGENT_TOKEN_FILE", Value: "/etc/fortuna/agent-token"}}
	_, err = cluster.client.AppsV1().DaemonSets("audit-agents").Create(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent-" + scope}, Spec: appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: labels}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: corev1.PodSpec{ServiceAccountName: "reader", Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}}, Containers: []corev1.Container{{Name: "agent", Image: image, ImagePullPolicy: corev1.PullNever, Env: env, VolumeMounts: []corev1.VolumeMount{{Name: "token", MountPath: "/etc/fortuna/agent-token", SubPathExpr: "$(NODE_NAME)", ReadOnly: true}}}}, Volumes: []corev1.Volume{{Name: "token", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tokens-" + scope}}}}}}}}, metav1.CreateOptions{})
	require.NoError(t, err)
}
func liveJWT(t *testing.T, db *gorm.DB, secret, role, scope string) (string, uint) {
	t.Helper()
	user := models.User{Username: "audit-" + rand.Text(), Email: rand.Text() + "@test.local", Password: "unused", Role: role, Active: true, ScopeJSON: scope}
	require.NoError(t, db.Create(&user).Error)
	sid, err := sessions.CreateLoginSession(db, user.ID, 1, "test", "127.0.0.1", "integration")
	require.NoError(t, err)
	token, err := auth.GenerateToken(user.ID, user.Username, user.Role, nil, sid, secret, 1)
	require.NoError(t, err)
	return token, user.ID
}
func liveRequest(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(method, path, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, request)
	return w
}
func liveCanReadSecrets(t *testing.T, cluster liveCluster) bool {
	t.Helper()
	review, err := cluster.client.AuthorizationV1().SubjectAccessReviews().Create(context.Background(), &authv1.SubjectAccessReview{Spec: authv1.SubjectAccessReviewSpec{User: "system:serviceaccount:audit-scope-a:first-investigation", ResourceAttributes: &authv1.ResourceAttributes{Verb: "get", Resource: "secrets", Namespace: "audit-scope-a"}}}, metav1.CreateOptions{})
	require.NoError(t, err)
	return review.Status.Allowed
}
func TestTwoClusterDaemonSetLive(t *testing.T) {
	if os.Getenv("FORTUNA_RUN_LIVE_INTEGRATION") != "1" {
		t.Skip("explicit disposable-cluster live gate")
	}
	image, host := os.Getenv("FORTUNA_TEST_AGENT_IMAGE"), os.Getenv("FORTUNA_TEST_CORE_HOST_IP")
	require.NotEmpty(t, image)
	require.NotEmpty(t, host)
	t.Chdir(filepath.Join("..", ".."))
	t.Setenv("FORTUNA_ATTACK_PATH_BUILD_CACHE_TTL", "0s")
	t.Setenv("FORTUNA_SECURITY_STATE_CACHE_TTL", "0s")
	t.Setenv("FORTUNA_ADMIN_PASSWORD", "Isolated-Audit-Only-27!")
	db := livePostgresDB(t)
	a := createLiveFixture(t, os.Getenv("FORTUNA_TEST_KUBECONFIG_A"), "audit-cluster-a")
	b := createLiveFixture(t, os.Getenv("FORTUNA_TEST_KUBECONFIG_B"), "audit-cluster-b")
	require.GreaterOrEqual(t, len(a.nodes), 2)
	require.GreaterOrEqual(t, len(b.nodes), 1)
	credentials := []agentidentity.Credential{}
	for _, cluster := range []liveCluster{a, b} {
		for scope, tokens := range cluster.tokens {
			for node, token := range tokens {
				hash := sha256.Sum256([]byte(token))
				credentials = append(credentials, agentidentity.Credential{ID: cluster.id + "-" + scope + "-" + node, ClusterID: cluster.id, AgentID: node + "-scope-" + scope + "-agent", TokenSHA256: hex.EncodeToString(hash[:]), ExpiresAt: time.Now().Add(time.Hour)})
			}
		}
	}
	raw, err := json.Marshal(map[string]any{"credentials": credentials})
	require.NoError(t, err)
	registry := filepath.Join(t.TempDir(), "registry.json")
	require.NoError(t, os.WriteFile(registry, raw, 0600))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	secret := rand.Text() + rand.Text()
	SetupRoutesWithCertManager(router, db, &config.Config{AuthEnabled: true, JWTSecret: secret, AgentCredentialRegistryPath: registry, TokenExpirationHours: 1}, nil, nil, nil)
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	server := &http.Server{Handler: router, ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	endpoint := fmt.Sprintf("http://%s:%d", host, listener.Addr().(*net.TCPAddr).Port)
	for _, cluster := range []liveCluster{a, b} {
		for _, scope := range []string{"a", "b"} {
			deployLiveAgents(t, cluster, scope, image, endpoint)
		}
	}
	require.Eventually(t, func() bool {
		for _, cluster := range []liveCluster{a, b} {
			var agents int64
			if db.Model(&models.Agent{}).Where("cluster_id = ? AND last_seen_at IS NOT NULL", cluster.id).Count(&agents).Error != nil || agents < int64(2*len(cluster.nodes)) {
				return false
			}
			var pods int64
			if db.Model(&models.Pod{}).Where("cluster_id = ? AND name = ?", cluster.id, "same-pod-name").Count(&pods).Error != nil || pods != 2 {
				return false
			}
		}
		return true
	}, 150*time.Second, time.Second, "real DaemonSet inventory did not commit for every scoped Agent")
	for _, cluster := range []liveCluster{a, b} {
		require.NoError(t, db.Model(&models.Cluster{}).Where("id = ?", cluster.id).Update("kubeconfig", cluster.kubeconfig).Error)
	}
	scoped, _ := liveJWT(t, db, secret, models.RoleOperator, `{"cluster_ids":["audit-cluster-a"]}`)
	admin, adminID := liveJWT(t, db, secret, models.RoleAdmin, "")
	w := liveRequest(t, router, "GET", "/api/v1/inventory/pods?clusterId="+a.id, scoped, nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), b.id)
	w = liveRequest(t, router, "GET", "/api/v1/graph?clusterId="+b.id, scoped, nil)
	require.Equal(t, 403, w.Code, w.Body.String())
	// First investigation: real RBAC permission, synchronized relational path,
	// explicit revocation, authorization denied, and fresh inventory recovery.
	require.True(t, liveCanReadSecrets(t, a))
	require.True(t, liveCanReadSecrets(t, b))
	paths, err := graph.NewRelationalPathBuilder(db).BuildAllPaths(context.Background(), a.id, false)
	require.NoError(t, err)
	found := false
	for _, path := range paths {
		for _, node := range path.Nodes {
			if strings.Contains(node.ID, "first-investigation-admin") {
				found = true
			}
		}
	}
	require.True(t, found, "synchronized inventory must produce the first investigation path")
	pod, err := a.client.CoreV1().Pods("audit-scope-a").Get(context.Background(), "same-pod-name", metav1.GetOptions{})
	require.NoError(t, err)
	podJSON, err := json.Marshal(pod)
	require.NoError(t, err)
	engine, err := riskengine.NewYAMLEngine(db, "rules")
	require.NoError(t, err)
	resource := map[string]any{"kind": "Pod", "uid": string(pod.UID), "name": pod.Name, "namespace": pod.Namespace, "cluster_id": a.id, "raw_json": string(podJSON)}
	detected, err := engine.EvaluateResource(context.Background(), "Pod", resource)
	require.NoError(t, err)
	var finding *models.Insight
	for _, insight := range detected {
		if insight.Title == "Pod using ServiceAccount with cluster-admin access" {
			finding = insight
		}
	}
	require.NotNil(t, finding, "a real first-finding rule must match synchronized RBAC evidence")
	require.NoError(t, riskengine.NewInsightManager(db).CreateOrUpdatePodInsight(finding))
	w = liveRequest(t, router, "POST", fmt.Sprintf("/api/v1/risk/insights/%d/acknowledge?clusterId=%s", finding.ID, a.id), admin, map[string]string{"reason": "investigating synchronized cluster-admin grant"})
	require.Equal(t, 200, w.Code, w.Body.String())
	// Concurrent evaluation upserts must retain a manual acknowledgement.
	var writes sync.WaitGroup
	writeErrors := make(chan error, 6)
	for i := 0; i < 6; i++ {
		writes.Add(1)
		go func() {
			defer writes.Done()
			copy := *finding
			writeErrors <- riskengine.NewInsightManager(db).CreateOrUpdatePodInsight(&copy)
		}()
	}
	writes.Wait()
	close(writeErrors)
	for err := range writeErrors {
		require.NoError(t, err)
	}
	var manualFinding models.Insight
	require.NoError(t, db.First(&manualFinding, finding.ID).Error)
	require.Equal(t, "acknowledged", manualFinding.Status)
	var sa models.ServiceAccount
	require.NoError(t, db.Where("cluster_id = ? AND namespace = ? AND name = ?", a.id, "audit-scope-a", "first-investigation").First(&sa).Error)
	w = liveRequest(t, router, "POST", "/api/v1/inventory/serviceaccounts/"+sa.UID+"/mutations/preview?clusterId="+a.id, admin, map[string]string{"action": "revoke"})
	require.Equal(t, 200, w.Code, w.Body.String())
	var preview struct {
		Operation models.ServiceAccountMutation `json:"operation"`
		Plan      mutations.Plan                `json:"plan"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &preview))
	require.Equal(t, adminID, preview.Operation.ActorID)
	require.NotEmpty(t, preview.Plan.Steps)
	w = liveRequest(t, router, "POST", "/api/v1/inventory/serviceaccount-mutations/"+preview.Operation.ID+"/execute?clusterId="+a.id, admin, map[string]string{"digest": preview.Operation.Digest})
	require.Equal(t, 202, w.Code, w.Body.String())
	workerCtx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); mutations.Start(workerCtx, db) }()
	defer cancel()
	require.Eventually(t, func() bool {
		var job models.ServiceAccountMutation
		return db.First(&job, "id = ?", preview.Operation.ID).Error == nil && job.Status == "succeeded"
	}, 30*time.Second, time.Second)
	require.Eventually(t, func() bool { return !liveCanReadSecrets(t, a) }, 15*time.Second, time.Second)
	require.True(t, liveCanReadSecrets(t, b), "cluster A revocation must not affect B")
	require.Eventually(t, func() bool {
		var binding models.ClusterRoleBinding
		if db.Where("cluster_id = ? AND name = ?", a.id, "first-investigation-admin").First(&binding).Error != nil {
			return false
		}
		var subjects []rbacv1.Subject
		return json.Unmarshal([]byte(binding.Subjects), &subjects) == nil && len(subjects) == 0
	}, 60*time.Second, time.Second)
	reevaluated, err := engine.EvaluateResource(context.Background(), "Pod", resource)
	require.NoError(t, err)
	for _, insight := range reevaluated {
		require.NotEqual(t, "Pod using ServiceAccount with cluster-admin access", insight.Title)
	}
	reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), 60*time.Second)
	reconciliationErr := worker.NewInsightStatusUpdater(db).UpdateStatusForResolvedRisks(reconcileCtx)
	require.NoError(t, reconcileCtx.Err(), "worker gate must finish within its deadline")
	reconcileCancel()
	if reconciliationErr != nil {
		t.Logf("worker reports incomplete evidence: %v", reconciliationErr)
	}
	require.NoError(t, db.First(&manualFinding, finding.ID).Error)
	if manualFinding.Status == "resolved" {
		var resolutionAudit int64
		require.NoError(t, db.Model(&models.AuditLog{}).Where("resource = 'insight' AND action = 'auto_resolve' AND resource_id = ? AND cluster_id = ?", fmt.Sprint(finding.ID), a.id).Count(&resolutionAudit).Error)
		require.Greater(t, resolutionAudit, int64(0), "verified resolution requires transactional evidence audit")
	} else {
		require.Equal(t, "acknowledged", manualFinding.Status, "unavailable evidence must preserve the user's manual state")
		require.Error(t, reconciliationErr, "ineligible evidence must be reported")
	}
	cancel()
	<-workerDone
	// A queued UID deletion must not remove an object replaced before execution.
	target, err := a.client.CoreV1().ServiceAccounts("audit-scope-a").Create(context.Background(), &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "replace-before-delete"}}, metav1.CreateOptions{})
	require.NoError(t, err)
	inventory := models.ServiceAccount{ClusterID: a.id, UID: string(target.UID), Namespace: target.Namespace, Name: target.Name, Labels: "{}", Secrets: "[]", LinkedPods: "[]"}
	require.NoError(t, db.Create(&inventory).Error)
	deleteJob, err := mutations.QueueDeletion(db, inventory, adminID, "admin")
	require.NoError(t, err)
	cancel()
	require.NoError(t, a.client.CoreV1().ServiceAccounts(target.Namespace).Delete(context.Background(), target.Name, metav1.DeleteOptions{}))
	replacement, err := a.client.CoreV1().ServiceAccounts(target.Namespace).Create(context.Background(), &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: target.Name}}, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, mutations.Process(context.Background(), db, mutations.ClusterClient(db), deleteJob.ID))
	var deletion models.ServiceAccountMutation
	require.NoError(t, db.First(&deletion, "id = ?", deleteJob.ID).Error)
	require.Equal(t, "blocked", deletion.Status)
	current, err := a.client.CoreV1().ServiceAccounts(target.Namespace).Get(context.Background(), target.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, replacement.UID, current.UID)
	// A Kubernetes delete followed by a real PostgreSQL audit failure must
	// recover through NotFound replay without losing its inventory/audit intent.
	retryTarget, err := a.client.CoreV1().ServiceAccounts("audit-scope-a").Create(context.Background(), &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "retry-after-persistence"}}, metav1.CreateOptions{})
	require.NoError(t, err)
	retryInventory := models.ServiceAccount{ClusterID: a.id, UID: string(retryTarget.UID), Namespace: retryTarget.Namespace, Name: retryTarget.Name, Labels: "{}", Secrets: "[]", LinkedPods: "[]"}
	require.NoError(t, db.Create(&retryInventory).Error)
	retryJob, err := mutations.QueueDeletion(db, retryInventory, adminID, "admin")
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE FUNCTION reject_mutation_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.resource = 'serviceaccount_mutation' AND NEW.action = 'delete' THEN RAISE EXCEPTION 'injected audit persistence failure'; END IF; RETURN NEW; END $$`).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_mutation_audit BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION reject_mutation_audit()`).Error)
	require.Error(t, mutations.Process(context.Background(), db, mutations.ClusterClient(db), retryJob.ID))
	_, err = a.client.CoreV1().ServiceAccounts(retryTarget.Namespace).Get(context.Background(), retryTarget.Name, metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	require.NoError(t, db.First(&retryJob, "id = ?", retryJob.ID).Error)
	require.Equal(t, "running", retryJob.Status)
	require.Zero(t, retryJob.NextStep)
	require.NoError(t, db.Exec(`DROP TRIGGER reject_mutation_audit ON audit_logs`).Error)
	require.NoError(t, db.Model(&retryJob).Update("lease_until", time.Now().Add(-time.Second)).Error)
	require.NoError(t, mutations.Process(context.Background(), db, mutations.ClusterClient(db), retryJob.ID))
	require.NoError(t, db.First(&retryJob, "id = ?", retryJob.ID).Error)
	require.Equal(t, "succeeded", retryJob.Status)
	// A real Agent Pod restart advances its execution session and still cannot
	// claim source health from reader lifecycle or successful empty inventory.
	agentID := a.nodes[0] + "-scope-a-agent"
	var priorProducer models.RuntimeProducerState
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", a.id, agentID, "falco").First(&priorProducer).Error)
	agentPods, err := a.client.CoreV1().Pods("audit-agents").List(context.Background(), metav1.ListOptions{LabelSelector: "app=audit-agent-a", FieldSelector: "spec.nodeName=" + a.nodes[0]})
	require.NoError(t, err)
	require.Len(t, agentPods.Items, 1)
	// Force an abrupt restart so the acceptance gate exercises lost in-memory
	// state without spending most of its deadline on the default Pod grace period.
	grace := int64(0)
	require.NoError(t, a.client.CoreV1().Pods("audit-agents").Delete(context.Background(), agentPods.Items[0].Name, metav1.DeleteOptions{GracePeriodSeconds: &grace}))
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		var producer models.RuntimeProducerState
		if assert.NoError(collect, db.Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", a.id, agentID, "falco").First(&producer).Error) {
			assert.NotEqual(collect, priorProducer.SessionID, producer.SessionID)
			assert.False(collect, producer.Authoritative)
		}
	}, 90*time.Second, time.Second)
	// Ownership check uses an actual credential from A against a forged B sync.
	tokenA := a.tokens["a"][a.nodes[0]]
	request := httptest.NewRequest("POST", "/api/v1/agent/sync", strings.NewReader(`{"clusterId":"audit-cluster-b","agent":{"agentId":"foreign"},"data":{}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Fortuna-Ingest-Token", tokenA)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request)
	require.Equal(t, 403, w.Code, w.Body.String())
	// Receipt is deliberately latest-per-cluster, not a fabricated aggregate of
	// the two namespace scopes. Every retained namespace remains explicit.
	var receipts []models.InventoryCollection
	require.NoError(t, db.Find(&receipts).Error)
	require.Len(t, receipts, 2)
	for _, receipt := range receipts {
		require.Contains(t, []string{"audit-scope-a", "audit-scope-b"}, receipt.Namespace)
	}
	// Avoid background HTTP/goroutines retaining a database scheduled for removal.
	for _, cluster := range []liveCluster{a, b} {
		for _, scope := range []string{"a", "b"} {
			_ = cluster.client.AppsV1().DaemonSets("audit-agents").Delete(context.Background(), "agent-"+scope, metav1.DeleteOptions{})
		}
	}
	if folder := os.Getenv("FORTUNA_TEST_EVIDENCE_DIR"); folder != "" {
		report := map[string]any{"passed": true, "clusters": 2, "nodes": len(a.nodes) + len(b.nodes), "daemonSets": 4, "namespaceScopes": 2, "agentInventory": true, "scopedHTTPJWT": true, "firstInvestigationRBAC": true, "firstFindingRule": true, "concurrentManualAck": true, "workerReconciliation": true, "replacementUIDProtected": true, "deletePersistenceRecovery": true, "agentRestart": true, "securityStateCacheTTL": "0s", "agentSyncInterval": "30s", "maxOpenConnections": 25, "runtimeAutoResolutionEnabled": false, "dashboardBrowserVerified": false}
		data, _ := json.MarshalIndent(report, "", "  ")
		require.NoError(t, os.WriteFile(filepath.Join(folder, "two-cluster.json"), data, 0600))
	}
}
