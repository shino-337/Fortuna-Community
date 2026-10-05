package api

// Row limits for list endpoints that used to return unbounded collections.
// Parsing goes through listlimit.Parse (missing/invalid -> default, above max
// -> max). Defaults for config- or inventory-shaped lists that the dashboard
// renders in full (users, clusters, rules, templates) are deliberately at or
// near the hard maximum so current screens are not cut; the dashboard passes
// an explicit limit where it needs "everything" (see dashboard/lib/api.ts).
const (
	usersListDefaultLimit = 1000
	usersListMaxLimit     = 5000

	// maxClustersListed bounds /inventory/clusters and /inventory/clusters/stats.
	// The dashboard builds its cluster selector from these, so the cap is far
	// above any realistic fleet rather than a page size.
	maxClustersListed = 1000

	clusterInventoryDefaultLimit = 5000 // distinct node names / namespaces per cluster
	clusterInventoryMaxLimit     = 20000
	clusterAgentsDefaultLimit    = 1000
	clusterAgentsMaxLimit        = 5000
	agentStatusDefaultLimit      = 1000
	agentStatusMaxLimit          = 5000

	resourcesDefaultLimit = 5000
	resourcesMaxLimit     = 20000

	capabilityMetadataUnpaginatedMax = 2000 // same as the paginated max
	promotionRulesDefaultLimit       = 1000
	promotionRulesMaxLimit           = 5000
	accessReviewMaxSignals           = 1000

	auditReportsDefaultLimit = 500
	auditReportsMaxLimit     = 2000

	investigationCasesDefaultLimit    = 500
	investigationCasesMaxLimit        = 2000
	investigationCasesScanCap         = 20000
	investigationTimelineDefaultLimit = 1000
	investigationTimelineMaxLimit     = 5000

	policyRulesDefaultLimit  = 1000
	policyRulesMaxLimit      = 5000
	ruleMatchesDefaultLimit  = 50
	ruleMatchesMaxLimit      = 100
	riskRulesDefaultLimit    = 1000
	riskRulesMaxLimit        = 5000
	riskRulesExportMaxRules  = 5000
	exceptionsDefaultLimit   = 500
	exceptionsMaxLimit       = 2000
	podReportMaxInsights     = 5000
	podAttackStepsDefault    = 500
	podAttackStepsMax        = 2000
	attackStepsSummaryMax    = 1000
	stepMappingsDefaultLimit = 1000
	stepMappingsMaxLimit     = 5000
	podCapabilitiesDefault   = 1000
	podCapabilitiesMax       = 5000
	capabilitySummaryMaxRows = 5000

	graphMaxNodes          = 5000
	graphMaxLinks          = 20000
	attackBundleMaxPaths   = 5000
	attackChainsMax        = 1000
	podAttackPathsDefault  = 500
	podAttackPathsMaxLimit = 2000
)
