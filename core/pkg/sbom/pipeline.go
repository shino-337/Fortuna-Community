package sbom

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ksam/core/pkg/cve/database"
	"github.com/ksam/core/pkg/cve/matcher"
	"github.com/ksam/core/pkg/models"
	"github.com/ksam/core/pkg/riskengine"
	"github.com/ksam/core/pkg/sbom/extractor"
	"github.com/ksam/core/pkg/sbom/normalizer"
	"gorm.io/gorm"
)

// parseImageRef parses image reference into name and tag
func parseImageRef(imageRef string) (string, string) {
	parts := strings.SplitN(imageRef, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return imageRef, "latest" // Default tag
}

// Pipeline orchestrates the SBOM-based CVE detection flow
type Pipeline struct {
	extractor      *extractor.Extractor
	normalizer     *normalizer.Normalizer
	cveMatcher     *matcher.Matcher
	insightManager *riskengine.InsightManager
	db             *gorm.DB
	logger         *log.Logger
	useCustom      bool // Use custom pipeline instead of Syft/Grype
}

// NewPipeline creates a new SBOM pipeline
func NewPipeline(
	insightManager *riskengine.InsightManager,
	db *gorm.DB,
) *Pipeline {
	useCustom := os.Getenv("KSAM_SBOM_USE_CUSTOM") != "false" // Default to custom

	var ext *extractor.Extractor
	var norm *normalizer.Normalizer
	var cveMatch *matcher.Matcher

	if useCustom {
		// Use custom components
		ext = extractor.NewExtractor()
		norm = normalizer.NewNormalizer()

		// Initialize CVE database manager
		// Default: query PostgreSQL tables loaded from /cve-data (OSV JSON) via cve-loader.
		source := os.Getenv("KSAM_CVE_SOURCE")
		if source == "" || strings.EqualFold(source, "postgres") {
			dbManager := database.NewPostgresManager(db)
			cveMatch = matcher.NewMatcher(dbManager, db)
		} else {
			trivyDBPath := os.Getenv("KSAM_TRIVY_DB_PATH")
			if trivyDBPath == "" {
				trivyDBPath = "/var/lib/ksam/trivy.db" // Default path
			}

			dbManager, err := database.NewManager(trivyDBPath)
			if err != nil {
				log.Printf("⚠️  Failed to initialize CVE database manager: %v", err)
				log.Printf("⚠️  CVE matching will use NVD API only")
			}

			cveMatch = matcher.NewMatcher(dbManager, db)
		}
		log.Printf("[SBOMPipeline] ✅ Using custom SBOM pipeline (zero external dependencies)")
	} else {
		// Legacy: Use Syft/Grype (deprecated)
		log.Printf("[SBOMPipeline] ⚠️  Using legacy Syft/Grype pipeline (deprecated)")
		// This will be removed in future versions
	}

	return &Pipeline{
		extractor:      ext,
		normalizer:     norm,
		cveMatcher:     cveMatch,
		insightManager: insightManager,
		db:             db,
		logger:         log.New(log.Writer(), "[SBOMPipeline] ", log.LstdFlags),
		useCustom:      useCustom,
	}
}

// ProcessImage processes an image through the SBOM pipeline
func (p *Pipeline) ProcessImage(
	ctx context.Context,
	imageRef string,
	podUID string,
	podName string,
	podNamespace string,
	containerName string,
) error {
	p.logger.Printf("🔄 Processing image: %s (pod: %s/%s, container: %s)",
		imageRef, podNamespace, podName, containerName)

	if !p.useCustom {
		return fmt.Errorf("custom pipeline not enabled (KSAM_SBOM_USE_CUSTOM=false)")
	}

	// Step 1: Extract SBOM using custom extractor (includes image digest)
	rawSBOM, err := p.extractor.ExtractSBOM(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("SBOM extraction failed: %w", err)
	}

	p.logger.Printf("✅ Extracted %d packages from %s (digest: %s)", len(rawSBOM.Packages), imageRef, rawSBOM.ImageDigest)

	// Step 2: Normalize SBOM to CycloneDX format
	normalizedSBOM, err := p.normalizer.Normalize(rawSBOM)
	if err != nil {
		return fmt.Errorf("SBOM normalization failed: %w", err)
	}

	// Extract components from raw SBOM for saving
	components := make([]models.SBOMComponent, 0)
	for _, pkg := range rawSBOM.Packages {
		// Generate PURL
		purl := p.generatePURL(pkg, rawSBOM.OS)
		component := models.SBOMComponent{
			ComponentType:    p.mapComponentType(pkg.Type),
			ComponentName:    pkg.Name,
			ComponentVersion: pkg.Version,
			PURL:            purl,
		}
		components = append(components, component)
	}

	// Get image digest for caching
	imageName, imageTag := parseImageRef(imageRef)
	digest := rawSBOM.ImageDigest // Use digest from extractor

	// If digest is still empty, use image name:tag as fallback (but this shouldn't happen)
	if digest == "" {
		digest = fmt.Sprintf("%s:%s", imageName, imageTag)
		p.logger.Printf("⚠️  Image digest not available, using %s as fallback", digest)
	}

	// Use database-level upsert to handle concurrent inserts safely
	// This prevents duplicate key errors when multiple pods use the same image
	normalizedSBOM.ImageDigest = digest

	// Try to find existing SBOM first (to get ID for proper update)
	var existingSBOM models.SBOM
	err = p.db.WithContext(ctx).
		Where("image_digest = ? AND deleted_at IS NULL", digest).
		First(&existingSBOM).Error

	if err == nil {
		// SBOM exists - update it
		p.logger.Printf("🔄 Found existing SBOM (ID: %d), updating...", existingSBOM.ID)
		normalizedSBOM.ID = existingSBOM.ID
		normalizedSBOM.UseCount = existingSBOM.UseCount + 1
		normalizedSBOM.CreatedAt = existingSBOM.CreatedAt // Preserve creation time

		// Update using GORM Save (which does UPDATE based on ID)
		if err := p.db.WithContext(ctx).Save(normalizedSBOM).Error; err != nil {
			return fmt.Errorf("failed to update existing SBOM: %w", err)
		}
		p.logger.Printf("✅ Updated existing SBOM (ID: %d, digest: %s, use_count: %d)",
			normalizedSBOM.ID, normalizedSBOM.ImageDigest, normalizedSBOM.UseCount)
	} else {
		// SBOM doesn't exist - try to create using ON CONFLICT for race condition safety
		p.logger.Printf("🆕 Creating new SBOM for digest: %s", digest)

		// Use raw SQL with ON CONFLICT to handle race conditions at database level
		// This ensures atomicity even when multiple pods process the same image simultaneously
		createSQL := `
			INSERT INTO sboms (
				image_name, image_tag, image_digest, sbom_format, sbom_content,
				component_count, os_packages, language_packages, generator, generator_version,
				generated_at, last_used_at, use_count, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (image_digest) DO UPDATE SET
				last_used_at = CURRENT_TIMESTAMP,
				use_count = sboms.use_count + 1,
				updated_at = CURRENT_TIMESTAMP
			RETURNING id, use_count
		`

		var returnedID uint
		var returnedUseCount int
		err = p.db.WithContext(ctx).Raw(createSQL,
			normalizedSBOM.ImageName,
			normalizedSBOM.ImageTag,
			normalizedSBOM.ImageDigest,
			normalizedSBOM.SBOMFormat,
			normalizedSBOM.SBOMContent,
			normalizedSBOM.ComponentCount,
			normalizedSBOM.OSPackages,
			normalizedSBOM.LanguagePackages,
			normalizedSBOM.Generator,
			normalizedSBOM.GeneratorVersion,
			normalizedSBOM.GeneratedAt,
			normalizedSBOM.LastUsedAt,
			normalizedSBOM.UseCount,
			normalizedSBOM.CreatedAt,
			normalizedSBOM.UpdatedAt,
		).Row().Scan(&returnedID, &returnedUseCount)

		if err != nil {
			return fmt.Errorf("failed to upsert SBOM: %w", err)
		}

		normalizedSBOM.ID = returnedID
		normalizedSBOM.UseCount = returnedUseCount

		if returnedUseCount == 1 {
			p.logger.Printf("✅ Created new SBOM (ID: %d, digest: %s)", normalizedSBOM.ID, normalizedSBOM.ImageDigest)
		} else {
			p.logger.Printf("✅ Updated existing SBOM via ON CONFLICT (ID: %d, digest: %s, use_count: %d)",
				normalizedSBOM.ID, normalizedSBOM.ImageDigest, returnedUseCount)
		}
	}

	// Save components
	savedCount := 0
	for i := range components {
		components[i].SBOMID = normalizedSBOM.ID
		if err := p.db.WithContext(ctx).Create(&components[i]).Error; err != nil {
			p.logger.Printf("⚠️  Failed to save component %s: %v", components[i].ComponentName, err)
		} else {
			savedCount++
		}
	}

	p.logger.Printf("✅ SBOM ready: %d components (OS: %d, Language: %d), saved %d components to database",
		normalizedSBOM.ComponentCount, normalizedSBOM.OSPackages, normalizedSBOM.LanguagePackages, savedCount)

	// Verify components were saved before matching
	var componentCount int64
	if err := p.db.WithContext(ctx).Model(&models.SBOMComponent{}).
		Where("sbom_id = ? AND deleted_at IS NULL", normalizedSBOM.ID).
		Count(&componentCount).Error; err == nil {
		p.logger.Printf("🔍 Verified: %d components in database for SBOM ID %d", componentCount, normalizedSBOM.ID)
	} else {
		p.logger.Printf("⚠️  Failed to verify components: %v", err)
	}

	// Step 3: Match CVEs
	matches, err := p.cveMatcher.MatchSBOM(ctx, normalizedSBOM)
	if err != nil {
		return fmt.Errorf("CVE matching failed: %w", err)
	}

	p.logger.Printf("✅ CVE matching complete: %d matches found", len(matches))

	// Step 4: Filter by severity (CRITICAL + HIGH only)
	filtered := p.cveMatcher.FilterBySeverity(matches, []string{"CRITICAL", "HIGH"})

	p.logger.Printf("✅ After filtering: %d critical/high vulnerabilities", len(filtered))
	
	// Step 4: Create insights for each CVE
	createdCount := 0
	for _, match := range filtered {
		// Load component details
		var component models.SBOMComponent
		if err := p.db.WithContext(ctx).
			Where("id = ?", match.ComponentID).
			First(&component).Error; err != nil {
			p.logger.Printf("⚠️  Failed to load component %d: %v", match.ComponentID, err)
			continue
		}
		
		// Create insight
		description := fmt.Sprintf(
			"Pod '%s' in namespace '%s' is running container '%s' with image '%s' which contains vulnerability %s (%s) in package %s@%s. Fixed version: %s",
			podName,
			podNamespace,
			containerName,
			imageRef,
			match.CVEID,
			match.Severity,
			component.ComponentName,
			component.ComponentVersion,
			match.FixedVersion,
		)
		
		insight := &models.Insight{
			Type:              "vulnerability",
			Description:       description,
			AffectedResources: models.ToJSONBString([]map[string]interface{}{
				{
					"type":      "Pod",
					"uid":       podUID,
					"name":      podName,
					"namespace": podNamespace,
					"container": containerName,
					"image":     imageRef,
				},
			}),
			Severity:          strings.ToLower(match.Severity),
			RecommendedAction: fmt.Sprintf("Upgrade package '%s' to version '%s' or update image '%s' to a fixed version.", component.ComponentName, match.FixedVersion, imageRef),
			Source:            "custom-sbom-scanner", // Indicate custom SBOM source
			CVEID:             match.CVEID,
			CVSSScore:         match.CVSSScore,
			PackageName:       component.ComponentName,
			InstalledVersion:  component.ComponentVersion,
			FixedVersion:      match.FixedVersion,
			SBOMID:            &normalizedSBOM.ID,
			CVEMatchID:        &match.ID,
		}
		
		if err := p.insightManager.CreateOrUpdateInsight(insight); err != nil {
			p.logger.Printf("⚠️  Failed to create/update insight for CVE %s: %v", match.CVEID, err)
			continue
		}
		
		createdCount++
	}
	
	// Step 5: Link pod to SBOM
	if err := p.linkPodToSBOM(ctx, podUID, podName, podNamespace, imageRef, normalizedSBOM.ID); err != nil {
		p.logger.Printf("⚠️  Failed to link pod to SBOM: %v", err)
	}
	
	p.logger.Printf("✅ Pipeline completed: %d insights created for %s", createdCount, imageRef)
	return nil
}

// generatePURL generates Package URL
func (p *Pipeline) generatePURL(pkg extractor.Package, os extractor.OSInfo) string {
	// PURL spec: pkg:<type>/<namespace>/<name>@<version>
	switch pkg.Type {
	case "deb":
		return fmt.Sprintf("pkg:deb/%s/%s@%s", os.Name, pkg.Name, pkg.Version)
	case "apk":
		return fmt.Sprintf("pkg:apk/alpine/%s@%s", pkg.Name, pkg.Version)
	case "rpm":
		return fmt.Sprintf("pkg:rpm/%s/%s@%s", os.Name, pkg.Name, pkg.Version)
	case "npm":
		return fmt.Sprintf("pkg:npm/%s@%s", pkg.Name, pkg.Version)
	case "pypi":
		return fmt.Sprintf("pkg:pypi/%s@%s", pkg.Name, pkg.Version)
	case "go":
		return fmt.Sprintf("pkg:golang/%s@%s", pkg.Name, pkg.Version)
	default:
		return fmt.Sprintf("pkg:generic/%s@%s", pkg.Name, pkg.Version)
	}
}

// mapComponentType maps package type to component type
func (p *Pipeline) mapComponentType(pkgType string) string {
	switch pkgType {
	case "deb", "rpm", "apk":
		return "library" // OS packages
	case "npm", "pypi", "go":
		return "library" // Language packages
	default:
		return "library"
	}
}

// filterBySeverity filters matches by severity
func (p *Pipeline) filterBySeverity(
	matches []*models.CVEMatch,
	severities []string,
) []*models.CVEMatch {
	if len(severities) == 0 {
		return matches // No filter
	}
	
	filtered := make([]*models.CVEMatch, 0)
	severityMap := make(map[string]bool)
	for _, sev := range severities {
		severityMap[strings.ToUpper(sev)] = true
	}
	
	for _, match := range matches {
		if severityMap[strings.ToUpper(match.Severity)] {
			filtered = append(filtered, match)
		}
	}
	
	return filtered
}

// linkPodToSBOM links a pod to an SBOM in pod_image_scans table
func (p *Pipeline) linkPodToSBOM(
	ctx context.Context,
	podUID string,
	podName string,
	podNamespace string,
	imageRef string,
	sbomID uint,
) error {
	// Parse image reference
	imageName, imageTag := parseImageRef(imageRef)
	
	// Find or create pod_image_scan entry
	var scan models.PodImageScan
	err := p.db.WithContext(ctx).
		Where("pod_uid = ? AND container_image = ? AND deleted_at IS NULL", podUID, imageRef).
		First(&scan).Error
	
	if err == nil {
		// Update existing entry
		scan.SBOMID = &sbomID
		return p.db.WithContext(ctx).Save(&scan).Error
	}
	
	// Create new entry
	scan = models.PodImageScan{
		PodUID:         podUID,
		PodName:        podName,
		PodNamespace:   podNamespace,
		ContainerImage: imageRef,
		ImageName:      imageName,
		ImageTag:       imageTag,
		SBOMID:         &sbomID,
	}
	
	return p.db.WithContext(ctx).Create(&scan).Error
}

