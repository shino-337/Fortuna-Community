package sbom

// ComponentSnapshot is a minimal component descriptor for CVE matcher (P1-5: avoid race with DB).
type ComponentSnapshot struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl"`
}

// SBOMCreatedEvent is emitted after an SBOM is ensured/persisted for a pod container.
// This enables an event-driven pipeline: SBOM_CREATED -> CVE matching -> insights.
// ComponentsSnapshot (P1-5): when set, CVE matcher uses it instead of loading from DB to avoid soft-delete race.
type SBOMCreatedEvent struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`

	// Pod context (for insights)
	ClusterID      string `json:"cluster_id"`
	PodUID         string `json:"pod_uid"`
	PodName        string `json:"pod_name"`
	PodNamespace   string `json:"pod_namespace"`
	ContainerName  string `json:"container_name"`
	ContainerImage string `json:"container_image"`

	// SBOM identity
	SBOMID      uint   `json:"sbom_id"`
	ImageDigest string `json:"image_digest"`

	// P1-5: component snapshot at publish time (avoids race: matcher uses this instead of DB when present)
	ComponentsSnapshot []ComponentSnapshot `json:"components_snapshot,omitempty"`
}


