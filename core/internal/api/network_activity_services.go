package api

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/fortuna/core/internal/k8s"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type networkActivityServiceRef struct {
	Name      string
	Namespace string
	FQDN      string
}

type networkServiceCacheEntry struct {
	expiresAt time.Time
	byIP      map[string]networkActivityServiceRef
}

var networkActivityServiceCache = struct {
	sync.Mutex
	entries map[string]networkServiceCacheEntry
}{entries: map[string]networkServiceCacheEntry{}}

const networkActivityServiceCacheTTL = 30 * time.Second

func enrichNetworkActivityEdgeServices(ctx context.Context, db *gorm.DB, clusterID string, rows []networkActivityEdgeRow) {
	ips := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.DestIP) != "" {
			ips = append(ips, row.DestIP)
		}
	}
	refs := lookupNetworkActivityServicesByIP(ctx, db, clusterID, ips)
	if len(refs) == 0 {
		return
	}
	for i := range rows {
		if ref, ok := refs[rows[i].DestIP]; ok {
			rows[i].DestServiceName = ref.Name
			rows[i].DestServiceNamespace = ref.Namespace
			rows[i].DestServiceFQDN = ref.FQDN
		}
	}
}

func enrichNetworkActivityDestinationServices(ctx context.Context, db *gorm.DB, clusterID string, rows []clusterDestinationRow) {
	ips := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.DestIP) != "" {
			ips = append(ips, row.DestIP)
		}
	}
	refs := lookupNetworkActivityServicesByIP(ctx, db, clusterID, ips)
	if len(refs) == 0 {
		return
	}
	for i := range rows {
		if ref, ok := refs[rows[i].DestIP]; ok {
			rows[i].DestServiceName = ref.Name
			rows[i].DestServiceNS = ref.Namespace
			rows[i].DestServiceFQDN = ref.FQDN
		}
	}
}

func lookupNetworkActivityServicesByIP(ctx context.Context, db *gorm.DB, clusterID string, ips []string) map[string]networkActivityServiceRef {
	need := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip != "" && ip != "0.0.0.0" && ip != "None" {
			need[ip] = struct{}{}
		}
	}
	if len(need) == 0 {
		return nil
	}

	byIP := getNetworkActivityServiceCache(ctx, db, clusterID)
	if len(byIP) == 0 {
		return nil
	}

	matched := make(map[string]networkActivityServiceRef, len(need))
	for ip := range need {
		if ref, ok := byIP[ip]; ok {
			matched[ip] = ref
		}
	}
	return matched
}

func getNetworkActivityServiceCache(ctx context.Context, db *gorm.DB, clusterID string) map[string]networkActivityServiceRef {
	if clusterID == "" {
		return nil
	}
	var cluster models.Cluster
	if err := db.WithContext(ctx).Where("id = ?", clusterID).First(&cluster).Error; err != nil || cluster.Kubeconfig == "" {
		return nil
	}
	key := fmt.Sprintf("%s:%x", clusterID, sha256.Sum256([]byte(cluster.Kubeconfig)))
	now := time.Now()
	networkActivityServiceCache.Lock()
	entry, ok := networkActivityServiceCache.entries[key]
	networkActivityServiceCache.Unlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.byIP
	}
	// Only credentials belonging to this cluster may populate its cache.
	client, err := k8s.NewClientFromKubeconfig(cluster.Kubeconfig)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	list, err := client.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	} // Never reuse an expired successful response after a failure.
	byIP := map[string]networkActivityServiceRef{}
	for _, svc := range list.Items {
		for _, ip := range svc.Spec.ClusterIPs {
			if ip != "" && ip != "None" {
				byIP[ip] = networkActivityServiceRef{Name: svc.Name, Namespace: svc.Namespace, FQDN: svc.Name + "." + svc.Namespace + ".svc"}
			}
		}
		if ip := svc.Spec.ClusterIP; ip != "" && ip != "None" {
			byIP[ip] = networkActivityServiceRef{Name: svc.Name, Namespace: svc.Namespace, FQDN: svc.Name + "." + svc.Namespace + ".svc"}
		}
	}
	networkActivityServiceCache.Lock()
	for key, entry := range networkActivityServiceCache.entries {
		if now.After(entry.expiresAt) {
			delete(networkActivityServiceCache.entries, key)
		}
	}
	networkActivityServiceCache.entries[key] = networkServiceCacheEntry{expiresAt: now.Add(networkActivityServiceCacheTTL), byIP: byIP}
	networkActivityServiceCache.Unlock()
	return byIP
}
