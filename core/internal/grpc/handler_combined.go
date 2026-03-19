package grpc

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SendCombinedFinding handles combined SBOM + CVE findings from Agent
// NOTE: According to LOGIC_FLOW_REFACTOR_IMPLEMENTATION.md, Agent should NOT send CVE findings
// This method is kept for backward compatibility but should be deprecated
func (s *SBOMServiceServer) SendCombinedFinding(ctx context.Context, req *pb.CombinedFinding) (*pb.CombinedFindingResponse, error) {
	if req.Sbom == nil {
		return &pb.CombinedFindingResponse{
			Success: false,
			Message: "SBOM finding is required",
		}, nil
	}

	log.Printf("📦 [CombinedFinding] Received: pod=%s/%s container=%s image=%s cves=%d",
		req.Sbom.Namespace, req.Sbom.PodName, req.Sbom.ContainerName,
		req.Sbom.ImageName, req.Cve.GetTotalMatches())

	// Step 1: Store SBOM
	sbomID, err := s.storeSBOM(ctx, req.Sbom)
	if err != nil {
		log.Printf("❌ Failed to store SBOM: %v", err)
		return &pb.CombinedFindingResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to store SBOM: %v", err),
		}, nil
	}

	log.Printf("✅ SBOM stored: id=%d digest=%s", sbomID, req.Sbom.ImageDigest)

	// Step 2: Store CVE matches and create insights (if CVE finding present)
	// NOTE: In new architecture, CVE matching is done in Core, not Agent
	insightsCreated := 0
	if req.Cve != nil && len(req.Cve.Matches) > 0 {
		count, err := s.storeCVEFindings(ctx, req.Cve, sbomID)
		if err != nil {
			log.Printf("⚠️  Failed to store CVE findings: %v", err)
			// Don't fail the request, SBOM is already stored
		} else {
			insightsCreated = count
			log.Printf("✅ Created %d insights from %d CVE matches",
				insightsCreated, len(req.Cve.Matches))
		}
	}

	return &pb.CombinedFindingResponse{
		Success:         true,
		Message:         "SBOM and CVE findings received",
		SbomId:          fmt.Sprintf("%d", sbomID),
		InsightsCreated: fmt.Sprintf("%d", insightsCreated),
	}, nil
}

// storeSBOM stores SBOM and components in database
func (s *SBOMServiceServer) storeSBOM(ctx context.Context, sbom *pb.SBOMFinding) (uint, error) {
	// Check for existing SBOM by image digest (cache reuse)
	var existingSBOM models.SBOM
	res := s.db.WithContext(ctx).Where("image_digest = ? AND deleted_at IS NULL", sbom.ImageDigest).First(&existingSBOM)
	if res.Error == nil {
		log.Printf("♻️  SBOM already exists for digest %s, reusing ID %d", sbom.ImageDigest, existingSBOM.ID)
		// Update usage stats
		existingSBOM.LastUsedAt = time.Now()
		existingSBOM.UseCount++
		s.db.WithContext(ctx).Save(&existingSBOM)
		return existingSBOM.ID, nil
	} else if res.Error != gorm.ErrRecordNotFound {
		return 0, fmt.Errorf("failed to query existing SBOM: %w", res.Error)
	}

	// Create new SBOM
	sbomModel := &models.SBOM{
		PodUID:         sbom.PodUid,
		PodName:        sbom.PodName,
		Namespace:     sbom.Namespace,
		ContainerName:  sbom.ContainerName,
		ImageName:      sbom.ImageName,
		ImageTag:       sbom.ImageTag,
		ImageDigest:    sbom.ImageDigest,
		OSName:         sbom.OsInfo.GetName(),
		OSVersion:      sbom.OsInfo.GetVersion(),
		OSArchitecture: sbom.OsInfo.GetArchitecture(),
		PackageCount:   int(len(sbom.Packages)),
		GeneratedAt:    sbom.GeneratedAt.AsTime(),
		AgentID:        sbom.AgentId,
		NodeID:         sbom.NodeId,
		Labels:         sbom.Labels, // Already map[string]string
		Annotations:    sbom.Annotations, // Already map[string]string
		LastUsedAt:     time.Now(),
		UseCount:       1,
	}

	// Upsert SBOM (unique by image_digest)
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "image_digest"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"pod_uid":         sbom.PodUid,
			"pod_name":        sbom.PodName,
			"namespace":       sbom.Namespace,
			"container_name":  sbom.ContainerName,
			"image_name":      sbom.ImageName,
			"image_tag":       sbom.ImageTag,
			"os_name":         sbom.OsInfo.GetName(),
			"os_version":      sbom.OsInfo.GetVersion(),
			"os_architecture": sbom.OsInfo.GetArchitecture(),
			"package_count":   int(len(sbom.Packages)),
			"generated_at":    sbom.GeneratedAt.AsTime(),
			"agent_id":        sbom.AgentId,
			"node_id":         sbom.NodeId,
			"labels":          models.ToJSONBString(sbom.Labels),
			"annotations":     models.ToJSONBString(sbom.Annotations),
			"last_used_at":    gorm.Expr("CURRENT_TIMESTAMP"),
			"use_count":       gorm.Expr("sboms.use_count + 1"),
			"updated_at":      gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(sbomModel).Error

	if err != nil {
		return 0, fmt.Errorf("failed to upsert SBOM: %w", err)
	}

	// Reload to get ID
	var persisted models.SBOM
	if err := s.db.WithContext(ctx).
		Where("image_digest = ? AND deleted_at IS NULL", sbom.ImageDigest).
		First(&persisted).Error; err != nil {
		return 0, fmt.Errorf("failed to reload SBOM: %w", err)
	}

	// Store components
	if len(sbom.Packages) > 0 {
		components := make([]models.SBOMComponent, 0, len(sbom.Packages))
		for _, pkg := range sbom.Packages {
			// Generate PURL from package info
			purl := s.generatePURL(pkg, sbom.OsInfo)
			if purl == "" {
				continue
			}

			components = append(components, models.SBOMComponent{
				SBOMID:           persisted.ID,
				ComponentType:    s.mapComponentType(pkg.Type),
				ComponentName:    pkg.Name,
				ComponentVersion: pkg.Version,
				PURL:             purl,
				Licenses:         models.ToJSONBString(pkg.Licenses),
				Source:           pkg.Source,
				Description:      pkg.Description,
				Homepage:         pkg.Homepage,
				Maintainer:       pkg.Maintainer,
			})
		}

		// Batch insert with dedup
		const batchSize = 500
		for i := 0; i < len(components); i += batchSize {
			end := i + batchSize
			if end > len(components) {
				end = len(components)
			}
			batch := components[i:end]
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "sbom_id"}, {Name: "purl"}},
				DoNothing: true,
			}).Create(&batch).Error; err != nil {
				log.Printf("⚠️  Failed to insert component batch: %v", err)
			}
		}
	}

	// Link pod to SBOM
	if err := s.linkPodToSBOM(ctx, sbom, persisted.ID); err != nil {
		log.Printf("⚠️  Failed to link pod to SBOM: %v", err)
	}

	return persisted.ID, nil
}

// storeCVEFindings stores CVE matches and creates insights
func (s *SBOMServiceServer) storeCVEFindings(ctx context.Context, cve *pb.CVEFinding, sbomID uint) (int, error) {
	insightsCreated := 0

	for _, match := range cve.Matches {
		// Create CVE match record
		cveMatch := models.CVEMatch{
			SBOMID:         sbomID,
			PodUID:         cve.PodUid,
			ContainerName:  cve.ContainerName,
			CVEID:          match.CveId,
			PackageName:    match.PackageName,
			PackageVersion: match.PackageVersion,
			PURL:           match.Purl,
			Severity:       match.Severity,
			CVSS:           float32(match.CvssScore), // Convert float64 to float32
			FixedVersion:   match.FixedIn,
			MatchedBy:      match.MatchedBy,
		}

		// Upsert CVE match (unique by sbom_id + cve_id + package_name + package_version)
		err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sbom_id"}, {Name: "cve_id"}, {Name: "package_name"}, {Name: "package_version"}},
			DoNothing: true, // Don't update if already exists
		}).Create(&cveMatch).Error

		if err != nil {
			log.Printf("⚠️  Failed to store CVE match %s: %v", match.CveId, err)
			continue
		}

		// Create insight
		insight := models.Insight{
			ResourceType:      "Pod",
			ResourceNamespace: cve.Namespace,
			ResourceName:      cve.PodName,
			ResourceUID:       cve.PodUid,
			InsightType:       "vulnerability",
			Severity:          s.mapSeverityToInsight(match.Severity),
			Title:             fmt.Sprintf("%s in %s", match.CveId, match.PackageName),
			Description:       fmt.Sprintf("Container '%s' image '%s:%s' contains vulnerable package %s@%s",
				cve.ContainerName, cve.ImageName, cve.ImageTag, match.PackageName, match.PackageVersion),
			Recommendation:    s.getRecommendation(match),
			CVEID:             match.CveId,
			AffectedComponent: match.PackageName,
			AffectedVersion:   match.PackageVersion,
			FixedVersion:      match.FixedIn,
			CVSS:              float32(match.CvssScore), // Convert float64 to float32
			DetectedAt:        time.Now(),
			Status:            "active",
		}

		// Upsert insight (unique by resource_uid + cve_id + affected_component + affected_version)
		err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "resource_uid"},
				{Name: "cve_id"},
				{Name: "affected_component"},
				{Name: "affected_version"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"detected_at": gorm.Expr("CURRENT_TIMESTAMP"),
				"status":      "active",
				"severity":    insight.Severity,
				"title":       insight.Title,
				"description": insight.Description,
				"recommendation": insight.Recommendation,
				"fixed_version": insight.FixedVersion,
				"cvss":          insight.CVSS,
			}),
		}).Create(&insight).Error

		if err != nil {
			log.Printf("⚠️  Failed to create insight for %s: %v", match.CveId, err)
			continue
		}

		insightsCreated++
	}

	return insightsCreated, nil
}

// linkPodToSBOM creates pod_image_scan record
func (s *SBOMServiceServer) linkPodToSBOM(ctx context.Context, sbom *pb.SBOMFinding, sbomID uint) error {
	clusterID := clusterIDFromContext(ctx)
	if clusterID == "" && s.db != nil {
		var pod models.Pod
		if err := s.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", sbom.PodUid).First(&pod).Error; err == nil {
			clusterID = pod.ClusterID
		}
	}
	if clusterID == "" {
		if v := os.Getenv("DEFAULT_CLUSTER_ID"); v != "" {
			clusterID = v
		} else {
			clusterID = "unknown"
		}
	}

	scan := models.PodImageScan{
		PodUID:         sbom.PodUid,
		PodName:        sbom.PodName,
		PodNamespace:   sbom.Namespace,
		ClusterID:      clusterID,
		ContainerName:  sbom.ContainerName,
		ContainerImage: fmt.Sprintf("%s:%s", sbom.ImageName, sbom.ImageTag),
		ImageName:      sbom.ImageName,
		ImageTag:       sbom.ImageTag,
		SBOMID:         &sbomID,
		UpdatedAt:      time.Now(),
	}

	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "pod_uid"}, {Name: "container_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"sbom_id":    sbomID,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(&scan).Error
}

// Helper functions

func (s *SBOMServiceServer) generatePURL(pkg *pb.Package, osInfo *pb.OSInfo) string {
	// Simplified PURL generation
	var purlType string
	switch pkg.Type {
	case pb.PackageType_PACKAGE_TYPE_DEB:
		purlType = "deb"
	case pb.PackageType_PACKAGE_TYPE_RPM:
		purlType = "rpm"
	case pb.PackageType_PACKAGE_TYPE_APK:
		purlType = "apk"
	case pb.PackageType_PACKAGE_TYPE_NPM:
		purlType = "npm"
	case pb.PackageType_PACKAGE_TYPE_PYPI:
		purlType = "pypi"
	case pb.PackageType_PACKAGE_TYPE_GEM:
		purlType = "gem"
	case pb.PackageType_PACKAGE_TYPE_GO_MOD:
		purlType = "go"
	case pb.PackageType_PACKAGE_TYPE_MAVEN:
		purlType = "maven"
	case pb.PackageType_PACKAGE_TYPE_CARGO:
		purlType = "cargo"
	default:
		purlType = "generic"
	}
	return fmt.Sprintf("pkg:%s/%s@%s", purlType, pkg.Name, pkg.Version)
}

func (s *SBOMServiceServer) mapComponentType(t pb.PackageType) string {
	switch t {
	case pb.PackageType_PACKAGE_TYPE_DEB, pb.PackageType_PACKAGE_TYPE_RPM, pb.PackageType_PACKAGE_TYPE_APK:
		return "os-package"
	case pb.PackageType_PACKAGE_TYPE_NPM, pb.PackageType_PACKAGE_TYPE_PYPI, pb.PackageType_PACKAGE_TYPE_GEM,
		pb.PackageType_PACKAGE_TYPE_GO_MOD, pb.PackageType_PACKAGE_TYPE_MAVEN, pb.PackageType_PACKAGE_TYPE_CARGO:
		return "language-package"
	default:
		return "unknown"
	}
}

func (s *SBOMServiceServer) mapSeverityToInsight(cveSeverity string) string {
	switch strings.ToLower(cveSeverity) {
	case "critical":
		return "Critical"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return "Unknown"
	}
}

func (s *SBOMServiceServer) getRecommendation(match *pb.CVEMatch) string {
	if match.FixedIn != "" {
		return fmt.Sprintf("Upgrade package '%s' to version '%s' or higher.", match.PackageName, match.FixedIn)
	}
	return fmt.Sprintf("Monitor for updates to package '%s'. No fix available yet.", match.PackageName)
}
