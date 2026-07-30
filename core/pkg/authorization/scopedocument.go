package authorization

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ScopeDocument is the v2 scope JSON on users (cluster-only today; extended fields reserved).
// Backward compatible: {"cluster_ids":["a"]} or {"clusters":["a"],"namespaces":[],"environments":[],"tenants":[],"labels":{}}.
type ScopeDocument struct {
	Clusters          []string          `json:"clusters"`
	Namespaces        []string          `json:"namespaces"`
	Environments      []string          `json:"environments"`
	Tenants           []string          `json:"tenants"`
	BusinessServices  []string          `json:"business_services"`
	CrownJewels       []string          `json:"crown_jewels"`
	RegulatoryDomains []string          `json:"regulatory_domains"`
	Labels            map[string]string `json:"labels"`
	LegacyCluster     []string          `json:"cluster_ids"`
}

// ParseScopeDocument parses user scope JSON. Malformed JSON returns restrictive empty document with RestrictsClusters true.
func ParseScopeDocument(scopeJSON string) ScopeDocument {
	s := strings.TrimSpace(scopeJSON)
	if s == "" || s == "{}" {
		return ScopeDocument{}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return ScopeDocument{Clusters: []string{"__invalid_scope__"}}
	}
	var d ScopeDocument
	if b, ok := raw["clusters"]; ok {
		_ = json.Unmarshal(b, &d.Clusters)
	}
	if b, ok := raw["cluster_ids"]; ok {
		_ = json.Unmarshal(b, &d.LegacyCluster)
	}
	if b, ok := raw["namespaces"]; ok {
		_ = json.Unmarshal(b, &d.Namespaces)
	}
	if b, ok := raw["environments"]; ok {
		_ = json.Unmarshal(b, &d.Environments)
	}
	if b, ok := raw["tenants"]; ok {
		_ = json.Unmarshal(b, &d.Tenants)
	}
	if b, ok := raw["labels"]; ok {
		_ = json.Unmarshal(b, &d.Labels)
	}
	if b, ok := raw["business_services"]; ok {
		_ = json.Unmarshal(b, &d.BusinessServices)
	}
	if b, ok := raw["crown_jewels"]; ok {
		_ = json.Unmarshal(b, &d.CrownJewels)
	}
	if b, ok := raw["regulatory_domains"]; ok {
		_ = json.Unmarshal(b, &d.RegulatoryDomains)
	}
	return d
}

func (d ScopeDocument) clusterIDs() []string {
	var out []string
	seen := make(map[string]struct{})
	for _, id := range d.Clusters {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range d.LegacyCluster {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// ClusterIDs returns the normalized effective cluster allow-list.
func (d ScopeDocument) ClusterIDs() []string {
	return d.clusterIDs()
}

// RestrictsClusters is true when the user must be checked against an explicit cluster allow-list.
func (d ScopeDocument) RestrictsClusters() bool {
	ids := d.clusterIDs()
	if len(ids) == 0 {
		return false
	}
	// Malformed scope marker: deny all cluster-scoped routes.
	if len(ids) == 1 && ids[0] == "__invalid_scope__" {
		return true
	}
	return true
}

// ClusterAllowed reports whether clusterKey is in scope (exact string match).
func (d ScopeDocument) ClusterAllowed(clusterKey string) bool {
	ids := d.clusterIDs()
	if len(ids) == 1 && ids[0] == "__invalid_scope__" {
		return false
	}
	for _, id := range ids {
		if id == clusterKey {
			return true
		}
	}
	return false
}

// ClusterAllowListSize returns how many cluster identifiers are in the effective allow-list (0 = unrestricted).
func (d ScopeDocument) ClusterAllowListSize() int {
	return len(d.clusterIDs())
}

// Validate checks structural limits for governance (future ABAC expansion).
func (d ScopeDocument) Validate() error {
	if len(d.Labels) > 64 {
		return fmt.Errorf("labels: too many keys (%d > 64)", len(d.Labels))
	}
	for k, v := range d.Labels {
		if len(k) > 128 || len(v) > 256 {
			return errors.New("labels: key or value too long")
		}
	}
	if len(d.Namespaces) > 512 || len(d.Environments) > 128 || len(d.Tenants) > 128 {
		return errors.New("scope arrays exceed maximum supported size")
	}
	return nil
}

// IntersectClusters returns cluster IDs present in both scope documents (intersection primitive).
func IntersectClusters(a, b ScopeDocument) []string {
	ma := make(map[string]struct{})
	for _, id := range a.clusterIDs() {
		if id == "" || id == "__invalid_scope__" {
			continue
		}
		ma[id] = struct{}{}
	}
	var out []string
	for _, id := range b.clusterIDs() {
		if _, ok := ma[id]; ok {
			out = append(out, id)
		}
	}
	return out
}
