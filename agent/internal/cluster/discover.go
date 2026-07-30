package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Source is how cluster identity was determined.
const (
	SourceAuto = "auto"
	SourceEnv  = "env"
)

// Info holds discovered or overridden cluster identity (SSOT from agent).
// ID must always be from discovery (kube-system UID hash, "sha256-...") or env CLUSTER_ID; never use display name as ID.
type Info struct {
	ID           string // stable, immutable (hash or env override); never set to Name
	Name         string // display name, mutable
	Source       string // "auto" | "env"
	K8sVersion  string
	Distribution string // "eks" | "gke" | "aks" | "kubeadm" | "unknown"
}

// Discover resolves cluster identity: env override (LOW PRIORITY) or K8s API auto-discovery.
// Rule: if CLUSTER_ID is set, use env and log OVERRIDE; else auto-discover from API.
func Discover(ctx context.Context, client kubernetes.Interface, kubeconfigPath string) (*Info, error) {
	envID := os.Getenv("CLUSTER_ID")
	envName := os.Getenv("CLUSTER_NAME")

	if envID != "" {
		name := envName
		if name == "" {
			name = envID
		}
		log.Printf("[cluster] id=%s source=env_override name=%s (CLUSTER_ID set)", envID, name)
		return &Info{
			ID:           envID,
			Name:         name,
			Source:       SourceEnv,
			K8sVersion:  getVersionFromAPI(ctx, client),
			Distribution: inferDistribution(""),
		}, nil
	}

	// Auto-discover from Kubernetes API
	id, name, err := discoverFromAPI(ctx, client, kubeconfigPath)
	if err != nil {
		return nil, err
	}
	version := getVersionFromAPI(ctx, client)
	dist := inferDistribution(version)
	log.Printf("[cluster] id=%s source=auto name=%s k8s_version=%s distribution=%s", id, name, version, dist)
	return &Info{
		ID:           id,
		Name:         name,
		Source:       SourceAuto,
		K8sVersion:  version,
		Distribution: dist,
	}, nil
}

// discoverFromAPI derives stable cluster_id and display name from K8s API (no node/pod UID).
// Cluster name is taken from real cluster data when possible: kubeadm-config, node labels, default API service, or derived from clusterID.
func discoverFromAPI(ctx context.Context, client kubernetes.Interface, kubeconfigPath string) (clusterID, clusterName string, err error) {
	// 1. kube-system namespace UID = stable cluster identity
	ns, err := client.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	uid := string(ns.UID)
	hash := sha256.Sum256([]byte(uid))
	clusterID = "sha256-" + hex.EncodeToString(hash[:])[:16] // short stable id

	// 2. Display name: from kubeconfig if available (e.g. agent runs with KUBECONFIG), else from cluster API
	clusterName = inferClusterName(kubeconfigPath)
	if clusterName == "" {
		clusterName = inferClusterNameFromAPI(ctx, client, clusterID)
	}
	return clusterID, clusterName, nil
}

// inferClusterNameFromAPI derives cluster display name from cluster resources (in-cluster; no kubeconfig).
// Order: kubeadm-config ClusterConfiguration.clusterName → node labels (EKS/K3s) → default API service name → cluster-<shortID>.
func inferClusterNameFromAPI(ctx context.Context, client kubernetes.Interface, clusterID string) string {
	// 1. kubeadm-config ConfigMap (kube-system): ClusterConfiguration may contain clusterName
	if name := clusterNameFromKubeadmConfig(ctx, client); name != "" {
		return name
	}
	// 2. Node labels (EKS: eks.amazonaws.com/cluster-name, K3s: k3s.io/cluster, etc.)
	if name := clusterNameFromNodeLabels(ctx, client); name != "" {
		return name
	}
	// 3. Default API server Service name in default namespace (every cluster has "kubernetes")
	if name := clusterNameFromDefaultAPIService(ctx, client); name != "" {
		return name
	}
	// 4. Fallback: derive from cluster ID (real, stable id from kube-system UID)
	if len(clusterID) > 7 {
		return "cluster-" + clusterID[7:15] // e.g. cluster-4016171f
	}
	return "cluster-" + clusterID
}

var kubeadmClusterNameRE = regexp.MustCompile(`clusterName:\s*["']?([^"'\s\n\r]+)`)

func clusterNameFromKubeadmConfig(ctx context.Context, client kubernetes.Interface) string {
	cm, err := client.CoreV1().ConfigMaps("kube-system").Get(ctx, "kubeadm-config", metav1.GetOptions{})
	if err != nil {
		return ""
	}
	// ClusterConfiguration is often under key "ClusterConfiguration" (YAML block)
	for _, data := range cm.Data {
		if m := kubeadmClusterNameRE.FindStringSubmatch(data); len(m) >= 2 && strings.TrimSpace(m[1]) != "" {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func clusterNameFromNodeLabels(ctx context.Context, client kubernetes.Interface) string {
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil || len(nodeList.Items) == 0 {
		return ""
	}
	node := nodeList.Items[0]
	// EKS: eks.amazonaws.com/cluster-name
	if v, ok := node.Labels["eks.amazonaws.com/cluster-name"]; ok && v != "" {
		return v
	}
	// K3s: k3s.io/cluster (or similar)
	if v, ok := node.Labels["k3s.io/cluster"]; ok && v != "" {
		return v
	}
	return ""
}

func clusterNameFromDefaultAPIService(ctx context.Context, client kubernetes.Interface) string {
	svc, err := client.CoreV1().Services(corev1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return ""
	}
	// Every cluster has the default "kubernetes" Service for the API server; use its name as display
	if svc.Name != "" {
		return svc.Name
	}
	return ""
}

func inferClusterName(kubeconfigPath string) string {
	if kubeconfigPath == "" {
		return ""
	}
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return ""
	}
	ctx := config.CurrentContext
	if ctx == "" {
		return ""
	}
	kctx, ok := config.Contexts[ctx]
	if !ok || kctx.Cluster == "" {
		return ""
	}
	// cluster.local / kubernetes.default.svc -> "cluster.local" or keep as-is
	return kctx.Cluster
}

func getVersionFromAPI(ctx context.Context, client kubernetes.Interface) string {
	// GET /version via discovery or server version
	ver, err := client.Discovery().ServerVersion()
	if err != nil {
		return ""
	}
	if ver != nil && ver.GitVersion != "" {
		return ver.GitVersion
	}
	return ""
}

func inferDistribution(version string) string {
	v := strings.ToLower(version)
	switch {
	case strings.Contains(v, "eks"):
		return "eks"
	case strings.Contains(v, "gke"):
		return "gke"
	case strings.Contains(v, "aks"):
		return "aks"
	case strings.Contains(v, "k3s"):
		return "k3s"
	case strings.Contains(v, "kubeadm") || strings.Contains(v, "v1."):
		return "kubeadm"
	default:
		return "unknown"
	}
}

