package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fortuna/core/pkg/models"
)

// Scorer calculates risk scores using improved V2 formula
// Formula: TotalScore = (BaseScore + ExploitabilityScore + BusinessImpactScore) × TimeDecay
type Scorer struct {
	db *gorm.DB
}

// NewScorer creates a new risk scorer (V2)
func NewScorer(db *gorm.DB) *Scorer {
	return &Scorer{
		db: db,
	}
}

// RiskScoreV2 represents a calculated risk score with V2 components
type RiskScoreV2 struct {
	ResourceType        string
	ResourceUID         string
	ResourceName        string
	Namespace           string
	ClusterID           string
	BaseScore           float64 // 0-40
	ExploitabilityScore float64 // 0-30
	BusinessImpactScore float64 // 0-30
	TimeDecay           float64 // 0.7-1.0
	TotalScore          float64 // 0-100
	PriorityLevel       string  // P0-P4
	Factors             map[string]interface{}
	InsightsCount       int
	HighestSeverity     string
	ScorerVersion       string // "v2"
}

// CalculateScore calculates risk score for a resource using V2 formula
func (s *Scorer) CalculateScore(ctx context.Context, resourceUID string) (*RiskScoreV2, error) {
	// Step 1: Get all active insights for resource
	var insights []models.Insight
	err := s.db.WithContext(ctx).
		Where("resource_uid = ? AND status IN (?) AND deleted_at IS NULL",
			resourceUID,
			[]string{"active", "acknowledged"}).
		Find(&insights).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch insights: %w", err)
	}

	if len(insights) == 0 {
		return &RiskScoreV2{
			ResourceUID:   resourceUID,
			TotalScore:    0,
			PriorityLevel: "P4", // Minimal
			ScorerVersion: "v2",
		}, nil
	}

	// Step 2: Get resource metadata
	resourceInfo := s.getResourceInfo(ctx, resourceUID, insights)

	// Step 3: Separate CVE and policy insights
	cveInsights, policyInsights := s.separateInsights(insights)

	// Step 4: Calculate components
	// For CVE insights: Use CVSS-based scoring
	// For policy insights: Use existing V2 scoring
	cveBaseScore := s.calculateCVEBaseScore(cveInsights)
	policyBaseScore := s.calculateBaseScore(policyInsights)
	baseScore := cveBaseScore + policyBaseScore
	if baseScore > 40 {
		baseScore = 40 // Cap at 40
	}

	// Exploitability: Enhanced for CVE insights
	exploitScore := s.calculateExploitabilityScore(append(cveInsights, policyInsights...), resourceInfo)

	// Business impact: Same for both
	businessScore := s.calculateBusinessImpactScore(append(cveInsights, policyInsights...), resourceInfo)
	timeDecay := s.calculateTimeDecay(append(cveInsights, policyInsights...))

	// Step 4: Calculate total score using new formula
	// TotalScore = (BaseScore + ExploitabilityScore + BusinessImpactScore) × TimeDecay
	totalScore := (baseScore + exploitScore + businessScore) * timeDecay

	// Cap at 100 (shouldn't exceed by design, but safety check)
	if totalScore > 100 {
		totalScore = 100
	}

	// Get highest severity
	highestSeverity := s.getHighestSeverity(insights)

	// Build factors
	factors := map[string]interface{}{
		"insight_types":      s.getInsightTypes(insights),
		"age_oldest_hours":   s.getOldestInsightAgeHours(insights),
		"affected_resources": len(insights),
		"reasons":            s.getRiskReasons(insights),
		"base_components": map[string]interface{}{
			"severity_core":       s.getSeverityCoreScore(insights),
			"vulnerability_bonus": s.getVulnerabilityTypeBonus(insights),
		},
		"exploitability_components":  s.getExploitabilityBreakdown(insights, resourceInfo),
		"business_impact_components": s.getBusinessImpactBreakdown(insights, resourceInfo),
	}

	s.attachScoreV3Preview(ctx, factors, resourceUID, resourceInfo)

	priorityLevel := s.determinePriority(totalScore)

	return &RiskScoreV2{
		ResourceType:        resourceInfo.ResourceType,
		ResourceUID:         resourceUID,
		ResourceName:        resourceInfo.ResourceName,
		Namespace:           resourceInfo.Namespace,
		ClusterID:           resourceInfo.ClusterID,
		BaseScore:           math.Round(baseScore*100) / 100,
		ExploitabilityScore: math.Round(exploitScore*100) / 100,
		BusinessImpactScore: math.Round(businessScore*100) / 100,
		TimeDecay:           math.Round(timeDecay*100) / 100,
		TotalScore:          math.Round(totalScore*100) / 100,
		PriorityLevel:       priorityLevel,
		Factors:             factors,
		InsightsCount:       len(insights),
		HighestSeverity:     highestSeverity,
		ScorerVersion:       "v2",
	}, nil
}

// ResourceInfoV2 holds extended resource information for V2 scoring
type ResourceInfoV2 struct {
	ResourceType         string
	ResourceName         string
	Namespace            string
	ClusterID            string
	IsInternetFacing     bool
	NetworkZone          string
	HasLoadBalancer      bool
	HasIngress           bool
	ServiceType          string
	HasServiceAccount    bool
	AssetTier            string
	Environment          string
	DataClassifications  []string
	ComplianceFrameworks []string
}

// getResourceInfo extracts resource info from insights and database
func (s *Scorer) getResourceInfo(ctx context.Context, resourceUID string, insights []models.Insight) ResourceInfoV2 {
	info := ResourceInfoV2{
		DataClassifications:  []string{},
		ComplianceFrameworks: []string{},
	}

	// Use direct resource fields from Insight model (no JSONB parsing)
	if len(insights) > 0 {
		insight := insights[0]
		if insight.ResourceName != "" {
			info.ResourceName = insight.ResourceName
		}
		if insight.ResourceNamespace != "" {
			info.Namespace = insight.ResourceNamespace
		}
		if insight.ResourceType != "" {
			info.ResourceType = insight.ResourceType
		}
	}

	// Try to get cluster ID and additional info from database
	if info.ClusterID == "" {
		var pod models.Pod
		if err := s.db.WithContext(ctx).Where("uid = ?", resourceUID).First(&pod).Error; err == nil {
			info.ClusterID = pod.ClusterID
			info.Namespace = pod.Namespace
			info.ResourceName = pod.Name
			info.ResourceType = "Pod"
			// Check if pod has service account (check spec if available)
			// Check if pod has service account
			info.HasServiceAccount = pod.ServiceAccount != "" && pod.ServiceAccount != "default"
		} else {
			var sa models.ServiceAccount
			if err := s.db.WithContext(ctx).Where("uid = ?", resourceUID).First(&sa).Error; err == nil {
				info.ClusterID = sa.ClusterID
				info.Namespace = sa.Namespace
				info.ResourceName = sa.Name
				info.ResourceType = "ServiceAccount"
				info.HasServiceAccount = true
			}
		}
	}

	// Determine environment from namespace
	nsLower := strings.ToLower(info.Namespace)
	if strings.Contains(nsLower, "prod") || nsLower == "production" {
		info.Environment = "production"
		info.AssetTier = "tier-1"
	} else if strings.Contains(nsLower, "staging") || strings.Contains(nsLower, "preprod") {
		info.Environment = "staging"
		info.AssetTier = "tier-2"
	} else if strings.Contains(nsLower, "dev") || strings.Contains(nsLower, "test") || strings.Contains(nsLower, "qa") {
		info.Environment = "development"
		info.AssetTier = "tier-3"
	} else {
		info.Environment = "unknown"
		info.AssetTier = "tier-3"
	}

	// Determine network zone (simplified - can be enhanced)
	if info.Environment == "production" {
		info.NetworkZone = "internal"
	} else {
		info.NetworkZone = "internal"
	}

	// Default service type
	info.ServiceType = "ClusterIP"

	return info
}

// calculateBaseScore calculates base score (0-40)
// BaseScore = SeverityCoreScore (5-30) + VulnerabilityTypeBonus (0-10)
func (s *Scorer) calculateBaseScore(insights []models.Insight) float64 {
	severityCore := s.getSeverityCoreScore(insights)
	vulnerabilityBonus := s.getVulnerabilityTypeBonus(insights)
	return severityCore + vulnerabilityBonus
}

// getSeverityCoreScore returns highest severity core score (5-30)
func (s *Scorer) getSeverityCoreScore(insights []models.Insight) float64 {
	maxScore := 5.0 // Default to low

	for _, insight := range insights {
		var score float64
		switch strings.ToLower(insight.Severity) {
		case "critical":
			score = 30.0
		case "high":
			score = 20.0
		case "medium":
			score = 10.0
		case "low":
			score = 5.0
		default:
			score = 5.0
		}

		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore
}

// getVulnerabilityTypeBonus returns highest vulnerability type bonus (0-10)
func (s *Scorer) getVulnerabilityTypeBonus(insights []models.Insight) float64 {
	maxBonus := 0.0

	for _, insight := range insights {
		desc := strings.ToLower(insight.Description)
		iType := strings.ToLower(insight.InsightType)
		bonus := 0.0

		// Check for vulnerability types (highest to lowest)
		if strings.Contains(desc, "privilege") && strings.Contains(desc, "escalation") {
			bonus = 10.0
		} else if strings.Contains(desc, "remote code execution") || strings.Contains(desc, "rce") {
			bonus = 8.0
		} else if strings.Contains(desc, "secret") || strings.Contains(desc, "credential") || strings.Contains(desc, "password") {
			bonus = 7.0
		} else if strings.Contains(desc, "data exposure") || strings.Contains(desc, "sensitive data") {
			bonus = 6.0
		} else if strings.Contains(desc, "denial of service") || strings.Contains(desc, "dos") || strings.Contains(desc, "ddos") {
			bonus = 4.0
		} else if strings.Contains(iType, "misconfiguration") || strings.Contains(desc, "misconfig") {
			bonus = 3.0
		} else if strings.Contains(desc, "information disclosure") {
			bonus = 2.0
		} else if iType == "supply_chain_malware" || strings.Contains(desc, "malicious") && strings.Contains(desc, "package") {
			bonus = 8.0
		}

		if bonus > maxBonus {
			maxBonus = bonus
		}
	}

	return maxBonus
}

// calculateExploitabilityScore calculates exploitability score (0-30)
// Sum of 5 components, each 0-6 points.
// RP-1: When a CVSS vector is available, its AV/AC/PR components supplement
// the heuristic sub-scores for attack vector, complexity, and auth.
func (s *Scorer) calculateExploitabilityScore(insights []models.Insight, resourceInfo ResourceInfoV2) float64 {
	// RP-1: Try to extract CVSS vector from the highest-severity CVE insight.
	cvssVec := bestCVSSVector(insights)

	attackVector := s.scoreAttackVector(resourceInfo)      // 0-6
	complexity := s.scoreAttackComplexity(insights)        // 0-6
	auth := s.scoreAuthRequirement(resourceInfo, insights) // 0-6
	exposure := s.scoreNetworkExposure(resourceInfo)       // 0-6
	exploit := s.scoreExploitAvailability(insights)        // 0-6

	// RP-1: Override heuristic sub-scores with authoritative CVSS vector data
	// when available.  We take the max of (heuristic, vector-derived) so the
	// vector can only raise, never lower, the score.
	if cvssVec != nil {
		if vecAV := cvssVectorAttackVector(cvssVec); vecAV > attackVector {
			attackVector = vecAV
		}
		if vecAC := cvssVectorAttackComplexity(cvssVec); vecAC > complexity {
			complexity = vecAC
		}
		if vecPR := cvssVectorPrivilegesRequired(cvssVec); vecPR > auth {
			auth = vecPR
		}
	}

	return attackVector + complexity + auth + exposure + exploit
}

// cvssVectorAttackVector maps CVSS AV metric to 0-6 score.  RP-1.
func cvssVectorAttackVector(v *CVSSVector) float64 {
	switch v.AttackVector {
	case "N": // Network
		return 6.0
	case "A": // Adjacent
		return 4.5
	case "L": // Local
		return 3.0
	case "P": // Physical
		return 1.0
	default:
		return 0.0
	}
}

// cvssVectorAttackComplexity maps CVSS AC metric to 0-6 score.  RP-1.
func cvssVectorAttackComplexity(v *CVSSVector) float64 {
	switch v.AttackComplexity {
	case "L": // Low complexity → easy to exploit → high score
		return 6.0
	case "H": // High complexity → hard to exploit → low score
		return 2.0
	default:
		return 0.0
	}
}

// cvssVectorPrivilegesRequired maps CVSS PR metric to 0-6 score.  RP-1.
func cvssVectorPrivilegesRequired(v *CVSSVector) float64 {
	switch v.PrivilegesRequired {
	case "N": // None
		return 6.0
	case "L": // Low
		return 4.0
	case "H": // High
		return 2.0
	default:
		return 0.0
	}
}

// scoreAttackVector scores attack vector (0-6)
func (s *Scorer) scoreAttackVector(resourceInfo ResourceInfoV2) float64 {
	if resourceInfo.IsInternetFacing {
		return 6.0
	}

	switch resourceInfo.NetworkZone {
	case "public", "internet":
		return 6.0
	case "dmz":
		return 5.0
	case "internal":
		return 4.0
	case "management":
		return 3.0
	case "isolated":
		return 1.0
	default:
		return 4.0 // Assume internal
	}
}

// scoreAttackComplexity scores attack complexity (0-6)
func (s *Scorer) scoreAttackComplexity(insights []models.Insight) float64 {
	maxComplexity := 4.0 // Default to medium

	for _, insight := range insights {
		desc := strings.ToLower(insight.Description)

		// Low complexity indicators (easy to exploit)
		if strings.Contains(desc, "no authentication") ||
			strings.Contains(desc, "default credentials") ||
			strings.Contains(desc, "hardcoded") ||
			strings.Contains(desc, "weak") {
			return 6.0 // Easy to exploit
		}

		// High complexity indicators (hard to exploit)
		if strings.Contains(desc, "race condition") ||
			strings.Contains(desc, "timing") ||
			strings.Contains(desc, "requires admin") ||
			strings.Contains(desc, "requires root") {
			if maxComplexity > 2.0 {
				maxComplexity = 2.0
			}
		}
	}

	return maxComplexity
}

// scoreAuthRequirement scores authentication requirement (0-6)
func (s *Scorer) scoreAuthRequirement(resourceInfo ResourceInfoV2, insights []models.Insight) float64 {
	// Check insights for auth requirements
	for _, insight := range insights {
		desc := strings.ToLower(insight.Description)

		if strings.Contains(desc, "no authentication") ||
			strings.Contains(desc, "unauthenticated") ||
			strings.Contains(desc, "public access") {
			return 6.0
		}

		if strings.Contains(desc, "requires authentication") ||
			strings.Contains(desc, "auth required") {
			return 4.0
		}

		if strings.Contains(desc, "multi-factor") ||
			strings.Contains(desc, "mfa") ||
			strings.Contains(desc, "two-factor") {
			return 2.0
		}
	}

	// Check resource type
	switch resourceInfo.ResourceType {
	case "ServiceAccount":
		return 4.0 // Token-based auth
	case "Pod":
		if resourceInfo.HasServiceAccount {
			return 4.0
		}
		return 6.0 // No explicit auth
	default:
		return 4.0 // Assume some auth required
	}
}

// scoreNetworkExposure scores network exposure (0-6)
func (s *Scorer) scoreNetworkExposure(resourceInfo ResourceInfoV2) float64 {
	if resourceInfo.HasLoadBalancer {
		return 6.0
	}

	if resourceInfo.HasIngress {
		return 5.0
	}

	switch resourceInfo.ServiceType {
	case "LoadBalancer":
		return 6.0
	case "NodePort":
		return 5.0
	case "ClusterIP":
		return 3.0
	default:
		return 3.0
	}
}

// scoreExploitAvailability scores exploit availability (0-6)
// RP-3: Uses structured exploit_available/exploit_maturity fields from Evidence JSON
// in addition to description heuristics and EPSS/KEV.
func (s *Scorer) scoreExploitAvailability(insights []models.Insight) float64 {
	maxScore := 0.0

	for _, insight := range insights {
		score := 0.0

		// For CVE insights: Check exploit maturity from structured fields, description + EPSS.
		if insight.InsightType == "vulnerability" {
			// RP-3: First check structured exploit fields from Evidence JSON.
			if ea, ok := parseExploitAvailableFromEvidence(insight.Evidence); ok && ea {
				score = 5.5 // Known exploit available
				if em, ok := parseExploitMaturityFromEvidence(insight.Evidence); ok {
					switch strings.ToLower(em) {
					case "high", "functional":
						score = 6.0
					case "poc", "proof-of-concept":
						score = 4.5
					}
				}
			} else {
				// Fallback: heuristic from description text
				desc := strings.ToLower(insight.Description)
				if strings.Contains(desc, "functional") || strings.Contains(desc, "high") {
					score = 6.0
				} else if strings.Contains(desc, "poc") || strings.Contains(desc, "proof of concept") {
					score = 4.0
				} else {
					score = 5.0
				}
			}

			// Blend EPSS signal
			if e, ok := parseEPSSFromInsightEvidence(insight.Evidence); ok {
				switch {
				case e >= 0.75:
					score = math.Max(score, 6.0)
				case e <= 0.05:
					score = math.Min(score, 4.5)
				default:
					score = math.Max(score, 4.0+e*2.5)
				}
			}
			// CISA KEV overrides to max
			if kev, ok := parseCISAKEVFromInsightEvidence(insight.Evidence); ok && kev {
				score = math.Max(score, 6.0)
			}
		} else if insight.InsightType == "supply_chain_malware" {
			score = 5.5
			if strings.EqualFold(strings.TrimSpace(insight.Severity), "critical") {
				score = 6.0
			}
		} else {
			// For policy insights: Parse from description
			desc := strings.ToLower(insight.Description)
			if strings.Contains(desc, "actively exploited") ||
				strings.Contains(desc, "in the wild") ||
				strings.Contains(desc, "active exploitation") {
				score = 6.0
			} else if strings.Contains(desc, "metasploit") ||
				strings.Contains(desc, "public exploit") ||
				strings.Contains(desc, "exploit available") {
				score = 5.0
			} else if strings.Contains(desc, "proof of concept") ||
				strings.Contains(desc, "poc") ||
				strings.Contains(desc, "proof-of-concept") {
				score = 4.0
			} else if strings.Contains(desc, "exploit-db") ||
				strings.Contains(desc, "exploit db") {
				score = 3.0
			}
		}

		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore
}

// parseEPSSFromInsightEvidence reads "epss" from Insight.evidence JSON (matcher EPSS enrichment).
func parseEPSSFromInsightEvidence(evidence string) (epss float64, ok bool) {
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return 0, false
	}
	var v struct {
		EPSS *float64 `json:"epss"`
	}
	if err := json.Unmarshal([]byte(evidence), &v); err != nil || v.EPSS == nil {
		return 0, false
	}
	e := *v.EPSS
	if e < 0 || e > 1 {
		return 0, false
	}
	return e, true
}

func parseCISAKEVFromInsightEvidence(evidence string) (kev bool, ok bool) {
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return false, false
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(evidence), &m); err != nil {
		return false, false
	}
	v, ok := m["cisa_kev"].(bool)
	return v, ok
}

// RP-1: CVSSVector holds parsed CVSS v3 vector components.
// Vector format: CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H
type CVSSVector struct {
	AttackVector          string // N(etwork), A(djacent), L(ocal), P(hysical)
	AttackComplexity      string // L(ow), H(igh)
	PrivilegesRequired    string // N(one), L(ow), H(igh)
	UserInteraction       string // N(one), R(equired)
	Scope                 string // U(nchanged), C(hanged)
	ConfidentialityImpact string // N(one), L(ow), H(igh)
	IntegrityImpact       string // N(one), L(ow), H(igh)
	AvailabilityImpact    string // N(one), L(ow), H(igh)
}

// parseCVSSVectorFromEvidence extracts and parses the CVSS vector string from
// Insight.Evidence JSON.  RP-1: allows sub-scores (AV, AC, PR, UI) to be used
// directly instead of relying on description heuristics.
func parseCVSSVectorFromEvidence(evidence string) (*CVSSVector, bool) {
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return nil, false
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(evidence), &m); err != nil {
		return nil, false
	}
	vectorStr, _ := m["cvss_vector"].(string)
	if vectorStr == "" {
		return nil, false
	}
	return parseCVSSVectorString(vectorStr)
}

// parseCVSSVectorString parses a CVSS v3.x vector string.
func parseCVSSVectorString(vs string) (*CVSSVector, bool) {
	if vs == "" {
		return nil, false
	}
	// e.g. "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
	parts := strings.Split(vs, "/")
	vec := &CVSSVector{}
	found := 0
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "AV":
			vec.AttackVector = kv[1]
			found++
		case "AC":
			vec.AttackComplexity = kv[1]
			found++
		case "PR":
			vec.PrivilegesRequired = kv[1]
			found++
		case "UI":
			vec.UserInteraction = kv[1]
			found++
		case "S":
			vec.Scope = kv[1]
			found++
		case "C":
			vec.ConfidentialityImpact = kv[1]
			found++
		case "I":
			vec.IntegrityImpact = kv[1]
			found++
		case "A":
			vec.AvailabilityImpact = kv[1]
			found++
		}
	}
	if found < 3 { // Need at least a few components to be useful
		return nil, false
	}
	return vec, true
}

// bestCVSSVector returns the "worst-case" CVSS vector across all CVE insights.
// RP-1: Used to supplement heuristic-based sub-scores with authoritative vector data.
func bestCVSSVector(insights []models.Insight) *CVSSVector {
	var best *CVSSVector
	var bestScore float64
	for _, ins := range insights {
		if ins.InsightType != "vulnerability" {
			continue
		}
		vec, ok := parseCVSSVectorFromEvidence(ins.Evidence)
		if !ok {
			continue
		}
		score := float64(ins.CVSS)
		if score > bestScore {
			bestScore = score
			best = vec
		}
	}
	return best
}

// separateInsights separates CVE-like insights (CVSS path) from policy/runtime insights.
// supply_chain_malware uses the same base-score path as CVE (severity → pseudo-CVSS when CVSS=0).
func (s *Scorer) separateInsights(insights []models.Insight) ([]models.Insight, []models.Insight) {
	cveInsights := []models.Insight{}
	policyInsights := []models.Insight{}

	for _, insight := range insights {
		if insight.InsightType == "vulnerability" && strings.TrimSpace(insight.CVEID) != "" {
			cveInsights = append(cveInsights, insight)
		} else if insight.InsightType == "supply_chain_malware" && strings.TrimSpace(insight.CVEID) != "" {
			cveInsights = append(cveInsights, insight)
		} else {
			policyInsights = append(policyInsights, insight)
		}
	}

	return cveInsights, policyInsights
}

// calculateCVEBaseScore calculates base score for CVE insights using CVSS.
// RP-12: Uses **max** strategy instead of weighted average so a single critical
// CVE is not diluted by co-located lower-severity ones.  A small secondary-CVE
// penalty (up to +4 pts for volume/diversity) rewards breadth without hurting
// the primary signal.
func (s *Scorer) calculateCVEBaseScore(cveInsights []models.Insight) float64 {
	if len(cveInsights) == 0 {
		return 0.0
	}

	confMultiplier := func(conf string) float64 {
		switch strings.ToUpper(strings.TrimSpace(conf)) {
		case "HIGH":
			return 1.0
		case "MEDIUM":
			return 0.7
		case "LOW":
			return 0.4
		case "VERY_LOW":
			return 0.2
		default:
			return 1.0
		}
	}

	var maxScore float64

	for _, insight := range cveInsights {
		var cvssScore float64
		if insight.CVSS > 0 {
			cvssScore = float64(insight.CVSS)
		} else {
			switch strings.ToUpper(insight.Severity) {
			case "CRITICAL":
				cvssScore = 9.0
			case "HIGH":
				cvssScore = 7.5
			case "MEDIUM":
				cvssScore = 5.0
			case "LOW":
				cvssScore = 2.5
			default:
				cvssScore = 5.0
			}
		}

		baseCVSS := cvssScore * 4.0

		// RP-3: Use structured exploit fields from Evidence JSON when available,
		// fall back to description heuristic.
		exploitWeight := 1.0
		if em, ok := parseExploitMaturityFromEvidence(insight.Evidence); ok {
			switch strings.ToLower(em) {
			case "high", "functional":
				exploitWeight = 1.5
			case "poc", "proof-of-concept":
				exploitWeight = 1.3
			}
		} else {
			desc := strings.ToLower(insight.Description)
			if strings.Contains(desc, "exploit") || strings.Contains(desc, "poc") {
				exploitWeight = 1.5
			}
		}

		age := time.Since(insight.CreatedAt).Hours() / 24.0
		freshnessWeight := 1.0
		if age < 7 {
			freshnessWeight = 1.2
		} else if age > 90 {
			freshnessWeight = 0.9
		}

		score := baseCVSS * exploitWeight * freshnessWeight * confMultiplier(insight.FinalRiskConfidence)
		if score > maxScore {
			maxScore = score
		}
	}

	// Secondary-CVE volume penalty: +1 pt per additional CVE, up to +4 pts.
	volumeBonus := math.Min(4.0, float64(len(cveInsights)-1))
	result := maxScore + volumeBonus

	if result > 40 {
		return 40.0
	}
	return result
}

// parseExploitMaturityFromEvidence reads the "exploit_maturity" field from the
// Insight.Evidence JSON (populated by cve_processor during matching).  RP-3.
func parseExploitMaturityFromEvidence(evidence string) (string, bool) {
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return "", false
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(evidence), &v); err != nil {
		return "", false
	}
	if em, ok := v["exploit_maturity"].(string); ok && em != "" {
		return em, true
	}
	return "", false
}

// parseExploitAvailableFromEvidence reads the "exploit_available" boolean from
// Insight.Evidence JSON.  RP-3.
func parseExploitAvailableFromEvidence(evidence string) (bool, bool) {
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return false, false
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(evidence), &v); err != nil {
		return false, false
	}
	if ea, ok := v["exploit_available"].(bool); ok {
		return ea, true
	}
	return false, false
}

// calculateBusinessImpactScore calculates business impact score (0-30)
// Sum of 4 components, each 0-7.5 points
func (s *Scorer) calculateBusinessImpactScore(insights []models.Insight, resourceInfo ResourceInfoV2) float64 {
	assetCrit := s.scoreAssetCriticality(resourceInfo)  // 0-7.5
	dataSens := s.scoreDataSensitivity(resourceInfo)    // 0-7.5
	compliance := s.scoreComplianceImpact(resourceInfo) // 0-7.5
	blastRadius := s.scoreBlastRadius(insights)         // 0-7.5

	return assetCrit + dataSens + compliance + blastRadius
}

// scoreAssetCriticality scores asset criticality (0-7.5)
func (s *Scorer) scoreAssetCriticality(resourceInfo ResourceInfoV2) float64 {
	// Check asset tier
	switch resourceInfo.AssetTier {
	case "tier-1", "mission-critical":
		return 7.5
	case "tier-2", "business-critical":
		return 5.0
	case "tier-3", "supporting":
		return 2.5
	default:
		break
	}

	// Check environment
	switch resourceInfo.Environment {
	case "production", "prod":
		return 7.5
	case "staging", "preprod":
		return 5.0
	case "development", "dev", "test", "qa":
		return 0.0
	default:
		return 2.5
	}
}

// scoreDataSensitivity scores data sensitivity (0-7.5)
func (s *Scorer) scoreDataSensitivity(resourceInfo ResourceInfoV2) float64 {
	maxScore := 0.0

	// Check data classifications
	for _, classification := range resourceInfo.DataClassifications {
		score := 0.0
		classLower := strings.ToLower(classification)

		switch {
		case strings.Contains(classLower, "secret") ||
			strings.Contains(classLower, "top-secret") ||
			strings.Contains(classLower, "restricted"):
			score = 7.5
		case strings.Contains(classLower, "confidential") ||
			strings.Contains(classLower, "pii") ||
			strings.Contains(classLower, "phi") ||
			strings.Contains(classLower, "pci"):
			score = 5.0
		case strings.Contains(classLower, "internal") ||
			strings.Contains(classLower, "internal-use"):
			score = 2.5
		case strings.Contains(classLower, "public"):
			score = 0.0
		default:
			score = 2.5
		}

		if score > maxScore {
			maxScore = score
		}
	}

	// If no classification, check namespace
	if maxScore == 0.0 {
		ns := strings.ToLower(resourceInfo.Namespace)
		if strings.Contains(ns, "prod") ||
			ns == "kube-system" ||
			ns == "kube-public" ||
			ns == "default" {
			return 5.0
		}
		return 2.5
	}

	return maxScore
}

// scoreComplianceImpact scores compliance impact (0-7.5)
func (s *Scorer) scoreComplianceImpact(resourceInfo ResourceInfoV2) float64 {
	maxScore := 0.0

	for _, framework := range resourceInfo.ComplianceFrameworks {
		score := 0.0
		frameworkLower := strings.ToLower(framework)

		switch {
		case strings.Contains(frameworkLower, "pci-dss") ||
			strings.Contains(frameworkLower, "pci") ||
			strings.Contains(frameworkLower, "hipaa"):
			score = 7.5
		case strings.Contains(frameworkLower, "sox") ||
			strings.Contains(frameworkLower, "gdpr") ||
			strings.Contains(frameworkLower, "ccpa"):
			score = 5.0
		case strings.Contains(frameworkLower, "iso27001") ||
			strings.Contains(frameworkLower, "nist"):
			score = 3.0
		case strings.Contains(frameworkLower, "internal"):
			score = 2.5
		default:
			score = 0.0
		}

		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore
}

// scoreBlastRadius scores blast radius (0-7.5)
func (s *Scorer) scoreBlastRadius(insights []models.Insight) float64 {
	maxScore := 1.0 // Default: single resource

	for _, insight := range insights {
		desc := strings.ToLower(insight.Description)
		score := 0.0

		// Check for cluster-wide impact
		if strings.Contains(desc, "cluster-admin") ||
			strings.Contains(desc, "cluster-wide") ||
			strings.Contains(desc, "all clusters") {
			score = 7.5
		} else if strings.Contains(desc, "namespace-admin") ||
			strings.Contains(desc, "all resources in namespace") ||
			strings.Contains(desc, "namespace-wide") {
			score = 5.0
		} else {
			// Count affected resources by checking distinct resource UIDs from all insights
			affectedCount := 0
			resourceUIDs := make(map[string]bool)
			for _, ins := range insights {
				if ins.ResourceUID != "" {
					resourceUIDs[ins.ResourceUID] = true
				}
			}
			affectedCount = len(resourceUIDs)
			if affectedCount > 10 {
				score = 3.0
			} else if affectedCount > 1 {
				score = 2.0
			} else {
				score = 1.0
			}
		}

		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore
}

// calculateTimeDecay calculates time decay using weighted average (0.7-1.0)
func (s *Scorer) calculateTimeDecay(insights []models.Insight) float64 {
	if len(insights) == 0 {
		return 1.0
	}

	// Calculate weighted average decay
	totalWeight := 0.0
	weightedDecay := 0.0

	for _, insight := range insights {
		// Get severity weight
		weight := s.getSeverityWeight(insight.Severity)

		// Calculate age-based decay
		age := time.Since(insight.CreatedAt)
		decay := s.getDecayForAge(age)

		totalWeight += weight
		weightedDecay += decay * weight
	}

	if totalWeight == 0 {
		return 1.0
	}

	return weightedDecay / totalWeight
}

// getSeverityWeight returns weight for severity (for time decay calculation)
func (s *Scorer) getSeverityWeight(severity string) float64 {
	switch strings.ToLower(severity) {
	case "critical":
		return 1.0
	case "high":
		return 0.7
	case "medium":
		return 0.4
	case "low":
		return 0.2
	default:
		return 0.4
	}
}

// getDecayForAge returns decay factor for age (gentler curve)
func (s *Scorer) getDecayForAge(age time.Duration) float64 {
	days := age.Hours() / 24

	if days < 7 {
		return 1.00 // No decay
	} else if days < 30 {
		return 0.95 // 5% reduction
	} else if days < 90 {
		return 0.85 // 15% reduction
	} else {
		return 0.70 // 30% reduction (max)
	}
}

// determinePriority determines priority level based on total score
func (s *Scorer) determinePriority(totalScore float64) string {
	if totalScore >= 80.0 {
		return "P0" // Critical
	} else if totalScore >= 60.0 {
		return "P1" // High
	} else if totalScore >= 35.0 {
		return "P2" // Medium
	} else if totalScore >= 10.0 {
		return "P3" // Low
	} else {
		return "P4" // Minimal
	}
}

// Helper functions (reused from scorer.go)

func (s *Scorer) getHighestSeverity(insights []models.Insight) string {
	severityOrder := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
	}

	maxOrder := 0
	highest := "low"

	for _, insight := range insights {
		order := severityOrder[strings.ToLower(insight.Severity)]
		if order > maxOrder {
			maxOrder = order
			highest = strings.ToLower(insight.Severity)
		}
	}

	return highest
}

func (s *Scorer) getInsightTypes(insights []models.Insight) []string {
	types := make(map[string]bool)
	for _, insight := range insights {
		types[insight.InsightType] = true
	}

	result := make([]string, 0, len(types))
	for t := range types {
		result = append(result, t)
	}
	return result
}

func (s *Scorer) getOldestInsightAgeHours(insights []models.Insight) int {
	oldest := time.Now()
	for _, insight := range insights {
		if insight.CreatedAt.Before(oldest) {
			oldest = insight.CreatedAt
		}
	}

	return int(time.Since(oldest).Hours())
}

func (s *Scorer) getRiskReasons(insights []models.Insight) []string {
	reasons := make([]string, 0)
	seen := make(map[string]bool)

	for _, insight := range insights {
		desc := strings.ToLower(insight.Description)
		if strings.Contains(desc, "cluster-admin") && !seen["cluster-admin access"] {
			reasons = append(reasons, "cluster-admin access")
			seen["cluster-admin access"] = true
		}
		if strings.Contains(desc, "privileged") && !seen["privileged container"] {
			reasons = append(reasons, "privileged container")
			seen["privileged container"] = true
		}
		if strings.Contains(desc, "hostnetwork") && !seen["hostNetwork enabled"] {
			reasons = append(reasons, "hostNetwork enabled")
			seen["hostNetwork enabled"] = true
		}
		if strings.Contains(desc, "wildcard") && !seen["wildcard permissions"] {
			reasons = append(reasons, "wildcard permissions")
			seen["wildcard permissions"] = true
		}
	}

	return reasons
}

// getExploitabilityBreakdown returns breakdown of exploitability components
func (s *Scorer) getExploitabilityBreakdown(insights []models.Insight, resourceInfo ResourceInfoV2) map[string]interface{} {
	return map[string]interface{}{
		"attack_vector":        s.scoreAttackVector(resourceInfo),
		"complexity":           s.scoreAttackComplexity(insights),
		"auth_requirement":     s.scoreAuthRequirement(resourceInfo, insights),
		"network_exposure":     s.scoreNetworkExposure(resourceInfo),
		"exploit_availability": s.scoreExploitAvailability(insights),
	}
}

// getBusinessImpactBreakdown returns breakdown of business impact components
func (s *Scorer) getBusinessImpactBreakdown(insights []models.Insight, resourceInfo ResourceInfoV2) map[string]interface{} {
	return map[string]interface{}{
		"asset_criticality": s.scoreAssetCriticality(resourceInfo),
		"data_sensitivity":  s.scoreDataSensitivity(resourceInfo),
		"compliance_impact": s.scoreComplianceImpact(resourceInfo),
		"blast_radius":      s.scoreBlastRadius(insights),
	}
}

// SaveScore saves risk score to database (V2 format)
func (s *Scorer) SaveScore(ctx context.Context, score *RiskScoreV2) error {
	// Convert factors map to JSONB
	factorsJSON, err := json.Marshal(score.Factors)
	if err != nil {
		return fmt.Errorf("failed to marshal factors: %w", err)
	}

	riskScore := &models.RiskScore{
		ResourceType:        score.ResourceType,
		ResourceUID:         score.ResourceUID,
		ResourceName:        score.ResourceName,
		Namespace:           score.Namespace,
		ClusterID:           score.ClusterID,
		TotalScore:          score.TotalScore,
		BaseScore:           score.BaseScore,
		SeverityWeight:      0, // Not used in V2, but keep for compatibility
		ImpactMultiplier:    0, // Not used in V2, but keep for compatibility
		TimeDecay:           score.TimeDecay,
		ExploitabilityScore: score.ExploitabilityScore,
		BusinessImpactScore: score.BusinessImpactScore,
		ScorerVersion:       score.ScorerVersion,
		Factors:             string(factorsJSON),
		InsightsCount:       score.InsightsCount,
		HighestSeverity:     score.HighestSeverity,
		PriorityLevel:       score.PriorityLevel,
		CalculatedAt:        time.Now(),
	}

	// Upsert on (resource_type, resource_uid, cluster_id). The table has a UNIQUE on those
	// columns without excluding deleted_at, so soft-deleted rows still occupy the key; GORM's
	// default FirstOrCreate skips deleted rows and would INSERT → 23505. ON CONFLICT also
	// makes concurrent SaveScore from InsightManager goroutines safe.
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "resource_type"},
			{Name: "resource_uid"},
			{Name: "cluster_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"resource_name",
			"namespace",
			"total_score",
			"base_score",
			"severity_weight",
			"impact_multiplier",
			"time_decay",
			"exploitability_score",
			"business_impact_score",
			"scorer_version",
			"factors",
			"insights_count",
			"highest_severity",
			"priority_level",
			"calculated_at",
			"updated_at",
			"deleted_at",
		}),
	}).Create(riskScore).Error
}
