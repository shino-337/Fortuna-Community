package sbom

// SBOMCreatedEvent is emitted after an SBOM is ensured/persisted for a pod container.
// This enables an event-driven pipeline: SBOM_CREATED -> CVE matching -> insights.
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
}


