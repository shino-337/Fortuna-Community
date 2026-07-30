package authorization

// OperationalScope is the server-normalized operational domain for UX (not security authority).
// Populated on login and GET /auth/me so clients do not infer scope from heuristics alone.
type OperationalScope struct {
	Clusters           []string `json:"clusters"`
	Namespaces         []string `json:"namespaces"`
	Teams              []string `json:"teams"`
	Environments       []string `json:"environments"`
	BusinessServices   []string `json:"business_services"`
	CrownJewels        []string `json:"crown_jewels"`
	RegulatoryDomains  []string `json:"regulatory_domains"`
	// Restricted is true when any explicit allow-list dimension is non-empty.
	Restricted bool `json:"restricted"`
}

// OperationalScopeFromDocument builds the API-facing scope from stored scope JSON.
func OperationalScopeFromDocument(scopeJSON string) OperationalScope {
	doc := ParseScopeDocument(scopeJSON)
	clusters := doc.clusterIDs()
	out := OperationalScope{
		Clusters:          append([]string(nil), clusters...),
		Namespaces:        append([]string(nil), doc.Namespaces...),
		Teams:             append([]string(nil), doc.Tenants...),
		Environments:      append([]string(nil), doc.Environments...),
		BusinessServices:  append([]string(nil), doc.BusinessServices...),
		CrownJewels:       append([]string(nil), doc.CrownJewels...),
		RegulatoryDomains: append([]string(nil), doc.RegulatoryDomains...),
	}
	if len(out.Clusters) > 0 || len(out.Namespaces) > 0 || len(out.Teams) > 0 || len(out.Environments) > 0 ||
		len(out.BusinessServices) > 0 || len(out.CrownJewels) > 0 || len(out.RegulatoryDomains) > 0 {
		out.Restricted = true
	}
	if len(out.Clusters) == 1 && out.Clusters[0] == "__invalid_scope__" {
		out.Clusters = nil
		out.Restricted = true
	}
	return out
}
