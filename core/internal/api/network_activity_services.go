package api

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type networkActivityServiceRef struct {
	Name      string
	Namespace string
	FQDN      string
}

var networkActivityServiceCache = struct {
	sync.Mutex
	expiresAt time.Time
	byIP      map[string]networkActivityServiceRef
	lastErrAt time.Time
}{}

const networkActivityServiceCacheTTL = 30 * time.Second

func enrichNetworkActivityEdgeServices(ctx context.Context, rows []networkActivityEdgeRow) {
	ips := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.DestIP) != "" {
			ips = append(ips, row.DestIP)
		}
	}
	refs := lookupNetworkActivityServicesByIP(ctx, ips)
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

func enrichNetworkActivityDestinationServices(ctx context.Context, rows []clusterDestinationRow) {
	ips := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.DestIP) != "" {
			ips = append(ips, row.DestIP)
		}
	}
	refs := lookupNetworkActivityServicesByIP(ctx, ips)
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

func lookupNetworkActivityServicesByIP(ctx context.Context, ips []string) map[string]networkActivityServiceRef {
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

	byIP := getNetworkActivityServiceCache(ctx)
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

func getNetworkActivityServiceCache(ctx context.Context) map[string]networkActivityServiceRef {
	now := time.Now()
	networkActivityServiceCache.Lock()
	if now.Before(networkActivityServiceCache.expiresAt) && networkActivityServiceCache.byIP != nil {
		cached := networkActivityServiceCache.byIP
		networkActivityServiceCache.Unlock()
		return cached
	}
	networkActivityServiceCache.Unlock()

	byIP, err := loadNetworkActivityServices(ctx)
	if err != nil {
		networkActivityServiceCache.Lock()
		if now.Sub(networkActivityServiceCache.lastErrAt) > time.Minute {
			log.Printf("WARN network activity service enrichment unavailable: %v", err)
			networkActivityServiceCache.lastErrAt = now
		}
		cached := networkActivityServiceCache.byIP
		networkActivityServiceCache.Unlock()
		return cached
	}

	networkActivityServiceCache.Lock()
	networkActivityServiceCache.byIP = byIP
	networkActivityServiceCache.expiresAt = now.Add(networkActivityServiceCacheTTL)
	networkActivityServiceCache.Unlock()
	return byIP
}

func loadNetworkActivityServices(ctx context.Context) (map[string]networkActivityServiceRef, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	byIP := make(map[string]networkActivityServiceRef, len(list.Items))
	for _, svc := range list.Items {
		ip := strings.TrimSpace(svc.Spec.ClusterIP)
		if ip == "" || ip == "None" {
			continue
		}
		byIP[ip] = networkActivityServiceRef{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			FQDN:      svc.Name + "." + svc.Namespace + ".svc",
		}
	}
	return byIP, nil
}
