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

// ParseScopeDocumentStrict parses and validates the persisted scope schema.
// Scope is authorization input: malformed types or unknown fields must never be
// silently ignored because that could turn an intended restriction into an
// unrestricted document.
func ParseScopeDocumentStrict(scopeJSON string) (ScopeDocument, error) {
	s := strings.TrimSpace(scopeJSON)
	if s == "" || s == "{}" {
		return ScopeDocument{}, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return ScopeDocument{}, fmt.Errorf("invalid scope JSON: %w", err)
	}
	if raw == nil {
		return ScopeDocument{}, errors.New("scope JSON must be an object")
	}

	allowed := map[string]struct{}{
		"clusters": {}, "cluster_ids": {}, "namespaces": {}, "environments": {},
		"tenants": {}, "business_services": {}, "crown_jewels": {},
		"regulatory_domains": {}, "labels": {},
	}
	for key, value := range raw {
		if _, ok := allowed[key]; !ok {
			return ScopeDocument{}, fmt.Errorf("unsupported scope field %q", key)
		}
		if strings.TrimSpace(string(value)) == "null" {
			return ScopeDocument{}, fmt.Errorf("scope field %q cannot be null", key)
		}
	}

	var d ScopeDocument
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return ScopeDocument{}, fmt.Errorf("invalid scope field type: %w", err)
	}
	// An explicit empty cluster allow-list would otherwise mean "no restriction",
	// so clearing the last cluster would grant every cluster. Unrestricted access
	// is expressed only by omitting the field ("{}").
	if _, ok := raw["clusters"]; ok && len(d.Clusters) == 0 {
		return ScopeDocument{}, errors.New("clusters: list must not be empty; omit the field for access to all clusters")
	}
	if _, ok := raw["cluster_ids"]; ok && len(d.LegacyCluster) == 0 {
		return ScopeDocument{}, errors.New("cluster_ids: list must not be empty; omit the field for access to all clusters")
	}
	if err := d.Validate(); err != nil {
		return ScopeDocument{}, err
	}
	return d, nil
}

// ParseScopeDocument is the read-path fail-closed wrapper. Legacy malformed rows
// remain denied while write paths can use ParseScopeDocumentStrict to return a
// validation error to the caller.
func ParseScopeDocument(scopeJSON string) ScopeDocument {
	d, err := ParseScopeDocumentStrict(scopeJSON)
	if err != nil {
		return ScopeDocument{Clusters: []string{"__invalid_scope__"}}
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

// HasUnenforcedRestrictions reports whether the document asks for an ABAC
// dimension that is stored/reserved but not yet enforced by every security-data
// read path. Such a document must fail closed for non-admin authorization.
func (d ScopeDocument) HasUnenforcedRestrictions() bool {
	return len(d.Namespaces) > 0 ||
		len(d.Environments) > 0 ||
		len(d.Tenants) > 0 ||
		len(d.BusinessServices) > 0 ||
		len(d.CrownJewels) > 0 ||
		len(d.RegulatoryDomains) > 0 ||
		len(d.Labels) > 0
}

// ClusterIDs returns the effective cluster allow-list. An unsupported persisted
// restriction becomes a deny-all marker rather than silently broadening access.
func (d ScopeDocument) ClusterIDs() []string {
	if d.HasUnenforcedRestrictions() {
		return []string{"__invalid_scope__"}
	}
	return d.clusterIDs()
}

// RestrictsClusters is true when the user must be checked against an explicit
// allow-list or when an unenforced persisted restriction must fail closed.
func (d ScopeDocument) RestrictsClusters() bool {
	if d.HasUnenforcedRestrictions() {
		return true
	}
	return len(d.clusterIDs()) > 0
}

// ClusterAllowed reports whether clusterKey is in scope (exact string match).
func (d ScopeDocument) ClusterAllowed(clusterKey string) bool {
	if d.HasUnenforcedRestrictions() {
		return false
	}
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

// ClusterAllowListSize returns how many identifiers are in the effective allow-list.
func (d ScopeDocument) ClusterAllowListSize() int {
	return len(d.ClusterIDs())
}

// Validate checks structural limits for governance (future ABAC expansion).
func (d ScopeDocument) Validate() error {
	validateList := func(name string, values []string, maxItems int) error {
		if len(values) > maxItems {
			return fmt.Errorf("%s: too many values (%d > %d)", name, len(values), maxItems)
		}
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return fmt.Errorf("%s: empty values are not allowed", name)
			}
			if len(trimmed) > 256 {
				return fmt.Errorf("%s: value too long", name)
			}
		}
		return nil
	}

	for _, spec := range []struct {
		name   string
		values []string
		max    int
	}{
		{"clusters", d.Clusters, 512},
		{"cluster_ids", d.LegacyCluster, 512},
		{"namespaces", d.Namespaces, 512},
		{"environments", d.Environments, 128},
		{"tenants", d.Tenants, 128},
		{"business_services", d.BusinessServices, 256},
		{"crown_jewels", d.CrownJewels, 256},
		{"regulatory_domains", d.RegulatoryDomains, 256},
	} {
		if err := validateList(spec.name, spec.values, spec.max); err != nil {
			return err
		}
	}

	if len(d.Labels) > 64 {
		return fmt.Errorf("labels: too many keys (%d > 64)", len(d.Labels))
	}
	for k, v := range d.Labels {
		if strings.TrimSpace(k) == "" {
			return errors.New("labels: empty key is not allowed")
		}
		if len(k) > 128 || len(v) > 256 {
			return errors.New("labels: key or value too long")
		}
	}
	return nil
}
