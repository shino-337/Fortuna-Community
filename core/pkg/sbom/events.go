package sbom

// SBOMCreatedEventSchemaVersion is bumped when SBOMCreatedEvent / ComponentSnapshot wire semantics change.
const SBOMCreatedEventSchemaVersion = "v1"

// ComponentSnapshot is a component descriptor for CVE matcher (P1-5: avoid race with DB).
type ComponentSnapshot struct {
	PURL    string `json:"purl"`
	Name    string `json:"name"`
	Version string `json:"version"`

	NormalizedName string `json:"normalized_name,omitempty"`
	VersionClass   string `json:"version_class,omitempty"` // STRICT | LOOSE | INVALID | UNKNOWN

	Ecosystem  string `json:"ecosystem,omitempty"`
	Namespace  string `json:"namespace,omitempty"` // distro
	Arch       string `json:"arch,omitempty"`

	Source        string `json:"source,omitempty"`
	TrustLevel    string `json:"trust_level,omitempty"`
	OriginalPURL  string `json:"original_purl,omitempty"`
	PURLValidated bool   `json:"purl_validated,omitempty"`
}

// SBOMCreatedEvent is emitted after an SBOM is ensured/persisted for a pod container.
// This enables an event-driven pipeline: SBOM_CREATED -> CVE matching -> insights.
// ComponentsSnapshot (P1-5): when set, CVE matcher uses it instead of loading from DB to avoid soft-delete race.
type SBOMCreatedEvent struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	EventID   string `json:"event_id,omitempty"`
	// SchemaVersion lets workers detect snapshot format drift (forward compatibility).
	SchemaVersion string `json:"schema_version,omitempty"`
	// CorrelationID propagates request tracing from ingest to workers.
	CorrelationID string `json:"correlation_id,omitempty"`

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


