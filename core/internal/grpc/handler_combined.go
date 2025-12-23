package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/models"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SendCombinedFinding handles combined SBOM + CVE findings from Agent
func (s *Server) SendCombinedFinding(ctx context.Context, req *pb.CombinedFinding) (*pb.CombinedFindingResponse, error) {
	if req.Sbom == nil {
		return &pb.CombinedFindingResponse{
			Success: false,
			Message: "SBOM finding is required",
		}, nil
	}

	log.Printf("📦 [CombinedFinding] Received: pod=%s/%s container=%s image=%s cves=%d",
		req.Sbom.Namespace, req.Sbom.PodName, req.Sbom.ContainerName,
		req.Sbom.ImageName, req.Cve.TotalMatches)

	// Step 1: Store SBOM
	sbomID, err := s.storeSBOM(ctx, req.Sbom)
	if err != nil {
		log.Printf("❌ Failed to store SBOM: %v", err)
		return &pb.CombinedFindingResponse{
			Success:     false,
			Message:     fmt.Sprintf("Failed to store SBOM: %v", err),
			ReceivedAt:  timestamppb.Now(),
		}, nil
	}

	log.Printf("✅ SBOM stored: id=%d digest=%s", sbomID, req.Sbom.ImageDigest)

	// Step 2: Store CVE matches and create insights (if CVE finding present)
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
		ReceivedAt:      timestamppb.Now(),
	}, nil
}

// storeSBOM stores SBOM and components in database
func (s *SBOMServiceServer) storeSBOM(ctx context.Context, sbom *pb.SBOMFinding) (uint, error) {
	// Check for existing SBOM by image digest
	var existingSBOM models.SBOM
	res := s.db.WithContext(ctx).Where("image_digest = ?", sbom.ImageDigest).First(&existingSBOM)
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

	// Create new SBOM model with all fields from Agent
	sbomModel := &models.SBOM{
		ImageName:      sbom.ImageName,
		ImageTag:       sbom.ImageTag,
		ImageDigest:    sbom.ImageDigest,
		PodUID:         sbom.PodUid,
		PodName:        sbom.PodName,
		Namespace:      sbom.Namespace,
		ContainerName:  sbom.ContainerName,
		OSName:         sbom.OsInfo.GetName(),
		OSVersion:      sbom.OsInfo.GetVersion(),
		OSArchitecture: sbom.OsInfo.GetArchitecture(),
		PackageCount:   len(sbom.Packages),
		SBOMFormat:     "fortuna-agent",
		GeneratedAt:    sbom.GeneratedAt.AsTime(),
		AgentID:        sbom.AgentId,
		NodeID:         sbom.NodeId,
		Labels:         sbom.Labels,
		Annotations:    sbom.Annotations,
		LastUsedAt:     time.Now(),
		UseCount:       1,
	}

	// Upsert SBOM (unique by image_digest)
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "image_digest"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_used_at": gorm.Expr("CURRENT_TIMESTAMP"),
			"use_count":    gorm.Expr("sboms.use_count + 1"),
			"updated_at":   gorm.Expr("CURRENT_TIMESTAMP"),
			"image_tag":    sbom.ImageTag, // Update tag if changed
		}),
	}).Create(sbomModel).Error

	if err != nil {
		return 0, fmt.Errorf("failed to upsert SBOM: %w", err)
	}

	// Create SBOM
	if err := s.db.WithContext(ctx).Create(sbomModel).Error; err != nil {
		return 0, fmt.Errorf("failed to create SBOM: %w", err)
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
				SBOMID:           sbomModel.ID,
				ComponentType:    s.mapComponentType(pkg.Type),
				ComponentName:    pkg.Name,
				ComponentVersion: pkg.Version,
				PURL:             purl,
				Licenses:         strings.Join(pkg.Licenses, ","),
				Source:           pkg.Source,
				Description:      pkg.Description,
				Homepage:         pkg.Homepage,
				Maintainer:       pkg.Maintainer,
			})
		}

		// Batch insert
		const batchSize = 500
		for i := 0; i < len(components); i += batchSize {
			end := i + batchSize
			if end > len(components) {
				end = len(components)
			}
			batch := components[i:end]
			if err := s.db.WithContext(ctx).CreateInBatches(&batch, batchSize).Error; err != nil {
				log.Printf("⚠️  Failed to insert component batch: %v", err)
			}
		}
	}

	// Link pod to SBOM
	if err := s.linkPodToSBOM(ctx, sbom, sbomModel.ID); err != nil {
		log.Printf("⚠️  Failed to link pod to SBOM: %v", err)
	}

	return sbomModel.ID, nil
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
			CVSS:           float32(match.CvssScore),
			FixedVersion:   match.FixedIn,
			MatchedBy:      match.MatchedBy,
		}

		// Upsert CVE match (unique by sbom_id + cve_id + package_name)
		err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sbom_id"}, {Name: "cve_id"}, {Name: "package_name"}},
			DoNothing: true, // Don't update if already exists
		}).Create(&cveMatch).Error

		if err != nil {
			log.Printf("⚠️  Failed to store CVE match %s: %v", match.CveId, err)
			continue
		}

		// Create insight with new schema
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
			CVSS:              match.CvssScore,
			DetectedAt:        time.Now(),
			Status:            "active",
		}

		// Upsert insight (unique by resource_uid + cve_id + affected_component)
		err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "resource_uid"},
				{Name: "cve_id"},
				{Name: "affected_component"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"detected_at": gorm.Expr("CURRENT_TIMESTAMP"),
				"updated_at":  gorm.Expr("CURRENT_TIMESTAMP"),
				"status":      "active",
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
	scan := models.PodImageScan{
		PodUID:         sbom.PodUid,
		PodName:        sbom.PodName,
		PodNamespace:   sbom.Namespace,
		ClusterID:      "default", // TODO: Get from config
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
	ecosystem := s.packageTypeToEcosystem(pkg.Type)
	name := pkg.Name
	version := pkg.Version

	switch pkg.Type {
	case pb.PackageType_PACKAGE_TYPE_DEB:
		distro := "debian"
		if osInfo != nil && osInfo.Name != "" {
			distro = osInfo.Name
		}
		return fmt.Sprintf("pkg:deb/%s/%s@%s", distro, name, version)
	case pb.PackageType_PACKAGE_TYPE_APK:
		return fmt.Sprintf("pkg:apk/alpine/%s@%s", name, version)
	case pb.PackageType_PACKAGE_TYPE_RPM:
		distro := "centos"
		if osInfo != nil && osInfo.Name != "" {
			distro = osInfo.Name
		}
		return fmt.Sprintf("pkg:rpm/%s/%s@%s", distro, name, version)
	case pb.PackageType_PACKAGE_TYPE_NPM:
		return fmt.Sprintf("pkg:npm/%s@%s", name, version)
	case pb.PackageType_PACKAGE_TYPE_PYPI:
		return fmt.Sprintf("pkg:pypi/%s@%s", name, version)
	case pb.PackageType_PACKAGE_TYPE_GEM:
		return fmt.Sprintf("pkg:gem/%s@%s", name, version)
	case pb.PackageType_PACKAGE_TYPE_GO_MOD:
		return fmt.Sprintf("pkg:golang/%s@%s", name, version)
	case pb.PackageType_PACKAGE_TYPE_MAVEN:
		return fmt.Sprintf("pkg:maven/%s@%s", name, version)
	case pb.PackageType_PACKAGE_TYPE_CARGO:
		return fmt.Sprintf("pkg:cargo/%s@%s", name, version)
	default:
		return fmt.Sprintf("pkg:generic/%s@%s", name, version)
	}
}

func (s *SBOMServiceServer) packageTypeToEcosystem(t pb.PackageType) string {
	switch t {
	case pb.PackageType_PACKAGE_TYPE_DEB:
		return "deb"
	case pb.PackageType_PACKAGE_TYPE_RPM:
		return "rpm"
	case pb.PackageType_PACKAGE_TYPE_APK:
		return "apk"
	case pb.PackageType_PACKAGE_TYPE_NPM:
		return "npm"
	case pb.PackageType_PACKAGE_TYPE_PYPI:
		return "pypi"
	case pb.PackageType_PACKAGE_TYPE_GEM:
		return "gem"
	case pb.PackageType_PACKAGE_TYPE_GO_MOD:
		return "go"
	case pb.PackageType_PACKAGE_TYPE_MAVEN:
		return "maven"
	case pb.PackageType_PACKAGE_TYPE_CARGO:
		return "cargo"
	default:
		return "unknown"
	}
}

func (s *SBOMServiceServer) mapComponentType(t pb.PackageType) string {
	switch t {
	case pb.PackageType_PACKAGE_TYPE_DEB, pb.PackageType_PACKAGE_TYPE_RPM, pb.PackageType_PACKAGE_TYPE_APK:
		return "library"
	case pb.PackageType_PACKAGE_TYPE_NPM, pb.PackageType_PACKAGE_TYPE_PYPI, pb.PackageType_PACKAGE_TYPE_GEM,
		pb.PackageType_PACKAGE_TYPE_GO_MOD, pb.PackageType_PACKAGE_TYPE_MAVEN, pb.PackageType_PACKAGE_TYPE_CARGO:
		return "library"
	default:
		return "library"
	}
}

func (s *Server) mapSeverityToInsight(severity string) string {
	switch severity {
	case "CRITICAL":
		return "critical"
	case "HIGH":
		return "high"
	case "MEDIUM":
		return "medium"
	case "LOW":
		return "low"
	default:
		return "info"
	}
}

func (s *SBOMServiceServer) getRecommendation(match *pb.CVEMatch) string {
	if match.FixedIn != "" {
		return fmt.Sprintf("Update %s to version %s or later to fix this vulnerability.",
			match.PackageName, match.FixedIn)
	}
	return fmt.Sprintf("No fix is currently available for %s. Monitor security advisories.",
		match.PackageName)
}

