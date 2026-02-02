package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
	"strings"

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
type Info struct {
	ID           string // stable, immutable (hash or env override)
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
func discoverFromAPI(ctx context.Context, client kubernetes.Interface, kubeconfigPath string) (clusterID, clusterName string, err error) {
	// 1. kube-system namespace UID = stable cluster identity
	ns, err := client.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	uid := string(ns.UID)
	hash := sha256.Sum256([]byte(uid))
	clusterID = "sha256-" + hex.EncodeToString(hash[:])[:16] // short stable id

	// 2. Display name: kubeconfig context.cluster if available, else inferred
	clusterName = inferClusterName(kubeconfigPath)
	if clusterName == "" {
		clusterName = "inferred-k8s-cluster"
	}
	return clusterID, clusterName, nil
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

