package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/fortuna/agent/internal/cluster"
)

type PodPayload struct {
	Name                         string                   `json:"name"`
	Namespace                    string                   `json:"namespace"`
	UID                          string                   `json:"uid"`
	ServiceAccountName           string                   `json:"serviceAccountName"`
	NodeName                     string                   `json:"nodeName"`
	HostNetwork                  bool                     `json:"hostNetwork"`
	HostPID                      bool                     `json:"hostPID"`
	HostIPC                      bool                     `json:"hostIPC"`
	AutomountServiceAccountToken bool                     `json:"automountServiceAccountToken"`
	PodSecurityContext           interface{}              `json:"podSecurityContext,omitempty"`
	Containers                   []ContainerPayload       `json:"containers,omitempty"`
	Volumes                      []VolumePayload          `json:"volumes,omitempty"`
	Tolerations                  []map[string]interface{} `json:"tolerations,omitempty"`
	Affinity                     interface{}              `json:"affinity,omitempty"`
}

type ContainerPayload struct {
	Name            string                   `json:"name"`
	SecurityContext interface{}              `json:"securityContext,omitempty"`
	VolumeMounts    []map[string]interface{} `json:"volumeMounts,omitempty"`
}

type VolumePayload struct {
	Name     string      `json:"name"`
	HostPath interface{} `json:"hostPath,omitempty"`
}

type ServiceAccountPayload struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	UID        string            `json:"uid"`
	Labels     map[string]string `json:"labels"`
	Secrets    []string          `json:"secrets"`
	LinkedPods []string          `json:"linkedPods"`
}

type RolePayload struct {
	Name      string              `json:"name"`
	Namespace string              `json:"namespace"`
	UID       string              `json:"uid"`
	Rules     []rbacv1.PolicyRule `json:"rules"`
}

type RoleBindingPayload struct {
	Name      string           `json:"name"`
	Namespace string           `json:"namespace"`
	UID       string           `json:"uid"`
	RoleRef   rbacv1.RoleRef   `json:"roleRef"`
	Subjects  []rbacv1.Subject `json:"subjects"`
}

type ClusterRolePayload struct {
	Name  string              `json:"name"`
	UID   string              `json:"uid"`
	Rules []rbacv1.PolicyRule `json:"rules"`
}

type ClusterRoleBindingPayload struct {
	Name     string           `json:"name"`
	UID      string           `json:"uid"`
	RoleRef  rbacv1.RoleRef   `json:"roleRef"`
	Subjects []rbacv1.Subject `json:"subjects"`
}

type SyncData struct {
	IsFullSync          bool                        `json:"isFullSync"`
	IsDeltaSync         bool                        `json:"isDeltaSync"`
	Pods                []PodPayload                `json:"pods"`
	ServiceAccounts     []ServiceAccountPayload     `json:"serviceAccounts"`
	Roles               []RolePayload               `json:"roles"`
	RoleBindings        []RoleBindingPayload        `json:"roleBindings"`
	ClusterRoles        []ClusterRolePayload        `json:"clusterRoles"`
	ClusterRoleBindings []ClusterRoleBindingPayload `json:"clusterRoleBindings"`
}

// ClusterPayload is the contract agent → core (SSOT).
type ClusterPayload struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"` // "auto" | "env"
	K8sVersion   string `json:"k8s_version,omitempty"`
	Distribution string `json:"distribution,omitempty"` // "eks" | "gke" | "aks" | "kubeadm" | "unknown"
}

type SyncPayload struct {
	ClusterID   string          `json:"clusterId"`             // backward compat; must be from cluster.Discover (s.clusterInfo.ID) only
	ClusterName string          `json:"clusterName,omitempty"` // backward compat
	Cluster     *ClusterPayload `json:"cluster,omitempty"`     // full contract for Core SSOT
	Agent       *AgentPayload   `json:"agent,omitempty"`
	Data        SyncData        `json:"data"`
}

type AgentPayload struct {
	AgentID  string `json:"agentId"`
	NodeName string `json:"nodeName"`
	Version  string `json:"version,omitempty"`
}

type Syncer struct {
	client      kubernetes.Interface
	endpoint    string
	clusterInfo *cluster.Info
	interval    time.Duration
	namespace   string
	agentID     string
	nodeName    string
	version     string
	httpClient  *http.Client
}

func NewSyncer(client kubernetes.Interface, endpoint string, clusterInfo *cluster.Info, interval time.Duration, namespace, agentID, nodeName, version string) *Syncer {
	return &Syncer{
		client:      client,
		endpoint:    endpoint,
		clusterInfo: clusterInfo,
		interval:    interval,
		namespace:   namespace,
		agentID:     agentID,
		nodeName:    nodeName,
		version:     version,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Syncer) Start(ctx context.Context) {
	if s.endpoint == "" {
		log.Printf("[Syncer] CORE_HTTP_ENDPOINT is empty, skipping auto-sync")
		return
	}
	if s.interval <= 0 {
		log.Printf("[Syncer] SYNC_INTERVAL is invalid (%v), skipping auto-sync", s.interval)
		return
	}

	go func() {
		log.Printf("[Syncer] Starting auto-sync: endpoint=%s interval=%s", s.endpoint, s.interval)

		// Run immediately on start
		if err := s.SyncOnce(ctx); err != nil {
			log.Printf("[Syncer] Initial sync failed: %v", err)
		}

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Printf("[Syncer] Stopping auto-sync")
				return
			case <-ticker.C:
				if err := s.SyncOnce(ctx); err != nil {
					log.Printf("[Syncer] Sync failed: %v", err)
				}
			}
		}
	}()
}

func (s *Syncer) SyncOnce(ctx context.Context) error {
	payload, err := s.buildPayload(ctx)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/agent/sync", s.endpoint), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post sync: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sync failed: status=%d", resp.StatusCode)
	}

	log.Printf("[Syncer] ✅ Full sync completed")
	return nil
}

func (s *Syncer) buildPayload(ctx context.Context) (*SyncPayload, error) {
	namespace := s.namespace
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	pods, err := s.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	serviceAccounts, err := s.client.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list serviceaccounts: %w", err)
	}
	roles, err := s.client.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	roleBindings, err := s.client.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list rolebindings: %w", err)
	}
	clusterRoles, err := s.client.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list clusterroles: %w", err)
	}
	clusterRoleBindings, err := s.client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list clusterrolebindings: %w", err)
	}

	linkedPodsBySA := make(map[string][]string)
	podPayloads := make([]PodPayload, 0, len(pods.Items))
	for _, p := range pods.Items {
		automount := true
		if p.Spec.AutomountServiceAccountToken != nil {
			automount = *p.Spec.AutomountServiceAccountToken
		}

		containers := make([]ContainerPayload, 0, len(p.Spec.Containers))
		for _, c := range p.Spec.Containers {
			volumeMounts := make([]map[string]interface{}, 0, len(c.VolumeMounts))
			for _, vm := range c.VolumeMounts {
				entry := map[string]interface{}{
					"name":      vm.Name,
					"mountPath": vm.MountPath,
					"readOnly":  vm.ReadOnly,
					"subPath":   vm.SubPath,
				}
				if vm.MountPropagation != nil {
					entry["mountPropagation"] = string(*vm.MountPropagation)
				}
				volumeMounts = append(volumeMounts, entry)
			}
			containers = append(containers, ContainerPayload{
				Name:            c.Name,
				SecurityContext: c.SecurityContext,
				VolumeMounts:    volumeMounts,
			})
		}

		volumes := make([]VolumePayload, 0, len(p.Spec.Volumes))
		for _, v := range p.Spec.Volumes {
			var hostPath interface{}
			if v.HostPath != nil {
				hostPath = map[string]interface{}{
					"path": v.HostPath.Path,
					"type": v.HostPath.Type,
				}
			}
			volumes = append(volumes, VolumePayload{
				Name:     v.Name,
				HostPath: hostPath,
			})
		}

		tolerations := make([]map[string]interface{}, 0, len(p.Spec.Tolerations))
		for _, t := range p.Spec.Tolerations {
			tolerations = append(tolerations, map[string]interface{}{
				"key":      t.Key,
				"operator": string(t.Operator),
				"value":    t.Value,
				"effect":   string(t.Effect),
				"seconds":  t.TolerationSeconds,
			})
		}

		saName := p.Spec.ServiceAccountName
		key := fmt.Sprintf("%s/%s", p.Namespace, saName)
		if saName != "" {
			linkedPodsBySA[key] = append(linkedPodsBySA[key], string(p.UID))
		}
		podPayloads = append(podPayloads, PodPayload{
			Name:                         p.Name,
			Namespace:                    p.Namespace,
			UID:                          string(p.UID),
			ServiceAccountName:           saName,
			NodeName:                     p.Spec.NodeName,
			HostNetwork:                  p.Spec.HostNetwork,
			HostPID:                      p.Spec.HostPID,
			HostIPC:                      p.Spec.HostIPC,
			AutomountServiceAccountToken: automount,
			PodSecurityContext:           p.Spec.SecurityContext,
			Containers:                   containers,
			Volumes:                      volumes,
			Tolerations:                  tolerations,
			Affinity:                     p.Spec.Affinity,
		})
	}

	saPayloads := make([]ServiceAccountPayload, 0, len(serviceAccounts.Items))
	for _, sa := range serviceAccounts.Items {
		key := fmt.Sprintf("%s/%s", sa.Namespace, sa.Name)
		linked := linkedPodsBySA[key]
		if linked == nil {
			linked = []string{}
		}
		secrets := make([]string, 0, len(sa.Secrets))
		for _, sec := range sa.Secrets {
			if sec.Name != "" {
				secrets = append(secrets, sec.Name)
			}
		}
		saPayloads = append(saPayloads, ServiceAccountPayload{
			Name:       sa.Name,
			Namespace:  sa.Namespace,
			UID:        string(sa.UID),
			Labels:     sa.Labels,
			Secrets:    secrets,
			LinkedPods: linked,
		})
	}

	rolePayloads := make([]RolePayload, 0, len(roles.Items))
	for _, role := range roles.Items {
		rolePayloads = append(rolePayloads, RolePayload{
			Name:      role.Name,
			Namespace: role.Namespace,
			UID:       string(role.UID),
			Rules:     role.Rules,
		})
	}

	roleBindingPayloads := make([]RoleBindingPayload, 0, len(roleBindings.Items))
	for _, rb := range roleBindings.Items {
		roleBindingPayloads = append(roleBindingPayloads, RoleBindingPayload{
			Name:      rb.Name,
			Namespace: rb.Namespace,
			UID:       string(rb.UID),
			RoleRef:   rb.RoleRef,
			Subjects:  rb.Subjects,
		})
	}

	clusterRolePayloads := make([]ClusterRolePayload, 0, len(clusterRoles.Items))
	for _, cr := range clusterRoles.Items {
		clusterRolePayloads = append(clusterRolePayloads, ClusterRolePayload{
			Name:  cr.Name,
			UID:   string(cr.UID),
			Rules: cr.Rules,
		})
	}

	clusterRoleBindingPayloads := make([]ClusterRoleBindingPayload, 0, len(clusterRoleBindings.Items))
	for _, crb := range clusterRoleBindings.Items {
		clusterRoleBindingPayloads = append(clusterRoleBindingPayloads, ClusterRoleBindingPayload{
			Name:     crb.Name,
			UID:      string(crb.UID),
			RoleRef:  crb.RoleRef,
			Subjects: crb.Subjects,
		})
	}

	data := SyncData{
		IsFullSync:          true,
		IsDeltaSync:         false,
		Pods:                podPayloads,
		ServiceAccounts:     saPayloads,
		Roles:               rolePayloads,
		RoleBindings:        roleBindingPayloads,
		ClusterRoles:        clusterRolePayloads,
		ClusterRoleBindings: clusterRoleBindingPayloads,
	}

	cp := &ClusterPayload{
		ID:           s.clusterInfo.ID,
		Name:         s.clusterInfo.Name,
		Source:       s.clusterInfo.Source,
		K8sVersion:   s.clusterInfo.K8sVersion,
		Distribution: s.clusterInfo.Distribution,
	}
	return &SyncPayload{
		ClusterID:   s.clusterInfo.ID,
		ClusterName: s.clusterInfo.Name,
		Cluster:     cp,
		Agent: &AgentPayload{
			AgentID:  s.agentID,
			NodeName: s.nodeName,
			Version:  s.version,
		},
		Data: data,
	}, nil
}
