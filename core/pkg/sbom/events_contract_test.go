package sbom

import (
	"encoding/json"
	"testing"
)

func TestSBOMCreatedEvent_ContractRoundTrip(t *testing.T) {
	in := SBOMCreatedEvent{
		Type:          "sbom.created",
		Timestamp:     1710000000,
		EventID:       "ev-abc",
		SchemaVersion: SBOMCreatedEventSchemaVersion,
		SBOMID:        123,
		ImageDigest:   "sha256:test",
		ComponentsSnapshot: []ComponentSnapshot{
			{
				PURL:           "pkg:deb/debian/openssl@1.1.1?arch=amd64",
				Name:           "openssl",
				Version:        "1.1.1",
				NormalizedName: "openssl",
				VersionClass:   "STRICT",
				Ecosystem:      "deb",
				Namespace:      "debian",
				Arch:           "amd64",
				Source:         "os",
				TrustLevel:     "high",
				OriginalPURL:   "pkg:deb/debian/openssl@1.1.1?arch=amd64",
				PURLValidated:  true,
			},
		},
	}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var out SBOMCreatedEvent
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if out.SchemaVersion != SBOMCreatedEventSchemaVersion {
		t.Fatalf("schema_version mismatch: got=%q want=%q", out.SchemaVersion, SBOMCreatedEventSchemaVersion)
	}
	if out.EventID != in.EventID {
		t.Fatalf("event_id mismatch: got=%q want=%q", out.EventID, in.EventID)
	}
	if len(out.ComponentsSnapshot) != 1 {
		t.Fatalf("expected 1 component snapshot, got=%d", len(out.ComponentsSnapshot))
	}
	c := out.ComponentsSnapshot[0]
	if c.NormalizedName == "" || c.Ecosystem == "" || c.VersionClass == "" {
		t.Fatalf("canonical snapshot fields must be present, got normalized=%q eco=%q class=%q",
			c.NormalizedName, c.Ecosystem, c.VersionClass)
	}
}

