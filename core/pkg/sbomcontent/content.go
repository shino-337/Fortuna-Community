// Package sbomcontent persists immutable image snapshots independently of workload
// observations. Sharing content never grants access to another workload's SBOM ID.
package sbomcontent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Attach snapshots the actual persisted components, including their trust and
// extraction provenance. Call inside the observation's write transaction.
func Attach(tx *gorm.DB, observationID uint) (uint, error) {
	var observation models.SBOM
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&observation, observationID).Error; err != nil {
		return 0, err
	}
	var components []models.SBOMComponent
	if err := tx.Where("sbom_id = ?", observationID).Find(&components).Error; err != nil {
		return 0, err
	}
	packages := make([]string, 0, len(components))
	for _, c := range components {
		// Explicit projection prevents future workload fields leaking into shared content.
		projection := map[string]interface{}{
			"type": c.ComponentType, "name": c.ComponentName, "version": c.ComponentVersion,
			"purl": c.PURL, "originalPurl": c.OriginalPURL, "purlValidated": c.PURLValidated,
			"trustLevel": c.TrustLevel, "sourceDetail": c.SourceDetail, "source": c.Source,
			"licenses": c.Licenses, "description": c.Description, "homepage": c.Homepage, "maintainer": c.Maintainer,
		}
		raw, err := json.Marshal(projection)
		if err != nil {
			return 0, err
		}
		packages = append(packages, string(raw))
	}
	sort.Strings(packages)
	rawPackages := make([]json.RawMessage, 0, len(packages))
	for _, p := range packages {
		rawPackages = append(rawPackages, json.RawMessage(p))
	}
	payload, err := json.Marshal(map[string]interface{}{
		"schemaVersion": 1, "imageDigest": observation.ImageDigest,
		"osName": observation.OSName, "osVersion": observation.OSVersion, "architecture": observation.OSArchitecture, "goVersion": observation.GoVersion,
		"source": models.NormalizeSBOMSource(observation.SbomSource), "confidence": models.NormalizeSBOMConfidence(observation.Confidence),
		"resolverVersion": observation.ResolverVersion, "signatureDBVersion": observation.SignatureDBVersion,
		"components": rawPackages,
	})
	if err != nil {
		return 0, err
	}
	hash := sha256.Sum256(payload)
	content := models.SBOMImageContent{ContentHash: hex.EncodeToString(hash[:]), Payload: string(payload)}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "content_hash"}}, DoNothing: true}).Create(&content).Error; err != nil {
		return 0, err
	}
	var stored models.SBOMImageContent
	if err := tx.Where("content_hash = ?", content.ContentHash).First(&stored).Error; err != nil {
		return 0, err
	}
	if stored.Payload != string(payload) {
		return 0, fmt.Errorf("SBOM content hash collision or corrupt payload")
	}
	if err := tx.Model(&models.SBOM{}).Where("id = ?", observationID).UpdateColumn("content_id", stored.ID).Error; err != nil {
		return 0, err
	}
	return stored.ID, nil
}
