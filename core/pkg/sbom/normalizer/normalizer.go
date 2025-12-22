package normalizer

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ksam/core/pkg/models"
	"github.com/ksam/core/pkg/sbom/extractor"
)

// Normalizer normalizes raw SBOM to CycloneDX format
type Normalizer struct {
	logger *log.Logger
}

// NewNormalizer creates a new SBOM normalizer
func NewNormalizer() *Normalizer {
	return &Normalizer{
		logger: log.New(log.Writer(), "[SBOMNormalizer] ", log.LstdFlags),
	}
}

// Normalize converts raw SBOM to normalized CycloneDX-like format
func (n *Normalizer) Normalize(raw *extractor.RawSBOM) (*models.SBOM, error) {
	n.logger.Printf("Normalizing SBOM for image: %s", raw.ImageName)

	// Convert packages to components
	components := make([]models.SBOMComponent, 0)
	osPackages := 0
	languagePackages := 0

	for _, pkg := range raw.Packages {
		component := models.SBOMComponent{
			ComponentType:    n.mapComponentType(pkg.Type),
			ComponentName:    pkg.Name,
			ComponentVersion: pkg.Version,
			PURL:            n.generatePURL(pkg, raw.OS),
		}

		// Count by type
		if pkg.Type == "deb" || pkg.Type == "rpm" || pkg.Type == "apk" {
			osPackages++
		} else {
			languagePackages++
		}

		components = append(components, component)
	}

	// Create CycloneDX JSON
	cyclonedx := n.createCycloneDX(raw, components)

	// Create SBOM model
	sbom := &models.SBOM{
		ImageName:        n.parseImageName(raw.ImageName),
		ImageTag:         n.parseImageTag(raw.ImageName),
		ImageDigest:      "", // Will be set by caller
		SBOMFormat:       "cyclonedx-json",
		SBOMContent:       cyclonedx,
		ComponentCount:    len(components),
		OSPackages:        osPackages,
		LanguagePackages:  languagePackages,
		Generator:         "custom",
		GeneratorVersion:  "v1.0.0",
		GeneratedAt:      time.Now(),
		LastUsedAt:        time.Now(),
		UseCount:          1,
	}

	n.logger.Printf("Normalized %d components (%d OS, %d language)", 
		len(components), osPackages, languagePackages)

	return sbom, nil
}

// createCycloneDX creates CycloneDX JSON format
func (n *Normalizer) createCycloneDX(raw *extractor.RawSBOM, components []models.SBOMComponent) string {
	cyclonedx := map[string]interface{}{
		"bomFormat":   "CycloneDX",
		"specVersion": "1.4",
		"version":     1,
		"metadata": map[string]interface{}{
			"timestamp": raw.ExtractedAt.Format(time.RFC3339),
			"component": map[string]interface{}{
				"type":    "container",
				"name":    raw.ImageName,
				"version": "latest",
			},
		},
		"components": n.convertComponents(components),
	}

	jsonBytes, _ := json.Marshal(cyclonedx)
	return string(jsonBytes)
}

// convertComponents converts SBOMComponents to CycloneDX format
func (n *Normalizer) convertComponents(components []models.SBOMComponent) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)

	for _, comp := range components {
		compMap := map[string]interface{}{
			"type":    comp.ComponentType,
			"name":    comp.ComponentName,
			"version": comp.ComponentVersion,
			"purl":    comp.PURL,
		}
		result = append(result, compMap)
	}

	return result
}

// generatePURL generates Package URL (PURL)
func (n *Normalizer) generatePURL(pkg extractor.Package, os extractor.OSInfo) string {
	// PURL spec: pkg:<type>/<namespace>/<name>@<version>
	switch pkg.Type {
	case "deb":
		// pkg:deb/debian/openssl@1.1.1d
		return fmt.Sprintf("pkg:deb/%s/%s@%s", os.Name, pkg.Name, pkg.Version)
	case "apk":
		// pkg:apk/alpine/openssl@1.1.1g-r0
		return fmt.Sprintf("pkg:apk/alpine/%s@%s", pkg.Name, pkg.Version)
	case "rpm":
		// pkg:rpm/centos/openssl@1.1.1k-5.el8
		return fmt.Sprintf("pkg:rpm/%s/%s@%s", os.Name, pkg.Name, pkg.Version)
	case "npm":
		// pkg:npm/lodash@4.17.21
		return fmt.Sprintf("pkg:npm/%s@%s", pkg.Name, pkg.Version)
	case "pypi":
		// pkg:pypi/django@3.2.5
		return fmt.Sprintf("pkg:pypi/%s@%s", pkg.Name, pkg.Version)
	case "go":
		// pkg:golang/github.com/gin-gonic/gin@v1.7.0
		return fmt.Sprintf("pkg:golang/%s@%s", pkg.Name, pkg.Version)
	default:
		// Generic format
		return fmt.Sprintf("pkg:generic/%s@%s", pkg.Name, pkg.Version)
	}
}

// mapComponentType maps package type to CycloneDX component type
func (n *Normalizer) mapComponentType(pkgType string) string {
	switch pkgType {
	case "deb", "rpm", "apk":
		return "library" // OS packages are libraries
	case "npm", "pypi", "go":
		return "library"
	default:
		return "library"
	}
}

// parseImageName extracts image name from image reference
func (n *Normalizer) parseImageName(imageRef string) string {
	// Remove tag/digest: image:tag -> image
	parts := strings.Split(imageRef, ":")
	if len(parts) > 1 && !strings.Contains(parts[1], "/") {
		return parts[0]
	}
	return imageRef
}

// parseImageTag extracts image tag from image reference
func (n *Normalizer) parseImageTag(imageRef string) string {
	// Extract tag: image:tag -> tag
	parts := strings.Split(imageRef, ":")
	if len(parts) > 1 && !strings.Contains(parts[1], "/") {
		return parts[1]
	}
	return "latest"
}

