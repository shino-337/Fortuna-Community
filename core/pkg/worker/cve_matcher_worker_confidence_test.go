package worker

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/stretchr/testify/require"
)

func TestBuildVulnInsightFromEvent_UnknownVersionCapsConfidence(t *testing.T) {
	ev := sbom.SBOMCreatedEvent{
		PodUID:         "pod-uid-1",
		PodName:        "pod-1",
		PodNamespace:   "ns",
		ContainerName:  "c1",
		ContainerImage: "img:tag",
	}

	sbomStatus := "complete"
	component := &models.SBOMComponent{
		ComponentName:    "openssl",
		ComponentVersion: "unknown",
		TrustLevel:       "high",
		PURLValidated:    true,
	}
	match := &models.CVEMatch{
		CVEID:          "CVE-TEST",
		Severity:       "HIGH",
		FixedVersion:   "2.0",
		MatchedBy:      "fortuna-core-cve-matcher",
		CVSS:           7.5,
		PackageName:    "openssl",
		PackageVersion: "unknown",
	}

	insight := buildVulnInsightFromEvent(ev, sbomStatus, component, match)
	require.Equal(t, "LOW", insight.MatchConfidence)
	require.Equal(t, "LOW", insight.FinalRiskConfidence)
	require.False(t, insight.Degraded)
}

func TestBuildVulnInsightFromEvent_PartialSBOMCapsFinalConfidence(t *testing.T) {
	ev := sbom.SBOMCreatedEvent{
		PodUID:         "pod-uid-2",
		PodName:        "pod-2",
		PodNamespace:   "ns",
		ContainerName:  "c1",
		ContainerImage: "img:tag",
	}

	sbomStatus := "partial"
	component := &models.SBOMComponent{
		ComponentName:    "openssl",
		ComponentVersion: "1.0",
		TrustLevel:       "high",
		PURLValidated:    true,
	}
	match := &models.CVEMatch{
		CVEID:          "CVE-TEST",
		Severity:       "HIGH",
		FixedVersion:   "2.0",
		MatchedBy:      "fortuna-core-cve-matcher",
		CVSS:           7.5,
		PackageName:    "openssl",
		PackageVersion: "1.0",
		MatchedAt:      time.Now(),
	}

	insight := buildVulnInsightFromEvent(ev, sbomStatus, component, match)
	require.True(t, insight.Degraded)
	require.NotEqual(t, "HIGH", insight.FinalRiskConfidence)
	require.Equal(t, "MEDIUM", insight.SBOMConfidence)
}

func TestBuildVulnInsightFromEvent_FallbackMatchedByIsLowConfidence(t *testing.T) {
	ev := sbom.SBOMCreatedEvent{
		PodUID:         "pod-uid-3",
		PodName:        "pod-3",
		PodNamespace:   "ns",
		ContainerName:  "c1",
		ContainerImage: "img:tag",
	}

	sbomStatus := "complete"
	component := &models.SBOMComponent{
		ComponentName:    "openssl",
		ComponentVersion: "1.0",
		TrustLevel:       "high",
		PURLValidated:    true,
	}
	match := &models.CVEMatch{
		CVEID:          "CVE-TEST",
		Severity:       "HIGH",
		FixedVersion:   "2.0",
		MatchedBy:      "fortuna-external-fallback",
		CVSS:           7.5,
		PackageName:    "openssl",
		PackageVersion: "1.0",
		MatchedAt:      time.Now(),
	}

	insight := buildVulnInsightFromEvent(ev, sbomStatus, component, match)
	require.Equal(t, "LOW", insight.MatchConfidence)
	require.Equal(t, "LOW", insight.FinalRiskConfidence)
}

func TestBuildVulnInsightFromEvent_DeterminismConfidenceOnly(t *testing.T) {
	ev := sbom.SBOMCreatedEvent{
		PodUID:         "pod-uid-4",
		PodName:        "pod-4",
		PodNamespace:   "ns",
		ContainerName:  "c1",
		ContainerImage: "img:tag",
	}

	sbomStatus := "complete"
	component := &models.SBOMComponent{
		ComponentName:    "openssl",
		ComponentVersion: "1.0",
		TrustLevel:       "high",
		PURLValidated:    true,
	}
	match := &models.CVEMatch{
		CVEID:          "CVE-TEST",
		Severity:       "HIGH",
		FixedVersion:   "2.0",
		MatchedBy:      "fortuna-core-cve-matcher",
		CVSS:           7.5,
		PackageName:    "openssl",
		PackageVersion: "1.0",
	}

	ins1 := buildVulnInsightFromEvent(ev, sbomStatus, component, match)
	ins2 := buildVulnInsightFromEvent(ev, sbomStatus, component, match)

	require.Equal(t, ins1.MatchConfidence, ins2.MatchConfidence)
	require.Equal(t, ins1.ComponentConfidence, ins2.ComponentConfidence)
	require.Equal(t, ins1.SBOMConfidence, ins2.SBOMConfidence)
	require.Equal(t, ins1.FinalRiskConfidence, ins2.FinalRiskConfidence)
	require.Equal(t, ins1.Degraded, ins2.Degraded)
}

func TestBuildVulnInsightFromEvent_MatchConstraintSatisfiedRaisesConfidence(t *testing.T) {
	ev := sbom.SBOMCreatedEvent{
		PodUID:         "pod-uid-5",
		PodName:        "pod-5",
		PodNamespace:   "ns",
		ContainerName:  "c1",
		ContainerImage: "img:tag",
	}

	sbomStatus := "complete"
	component := &models.SBOMComponent{
		ComponentName:    "openssl",
		ComponentVersion: "1.0",
		TrustLevel:       "high",
		PURLValidated:    true,
		SourceDetail:     "agent-fields",
	}
	match := &models.CVEMatch{
		CVEID:               "CVE-TEST",
		Severity:            "HIGH",
		FixedVersion:        "2.0",
		MatchedBy:           "fortuna-core-cve-matcher",
		CVSS:                7.5,
		PackageName:         "openssl",
		PackageVersion:      "1.0",
		HasConstraint:       true,
		ConstraintSatisfied: true,
	}

	insight := buildVulnInsightFromEvent(ev, sbomStatus, component, match)
	require.Equal(t, "HIGH", insight.MatchConfidence)
	require.Equal(t, "HIGH", insight.ComponentConfidence)
	require.Equal(t, "HIGH", insight.SBOMConfidence)
	require.Equal(t, "HIGH", insight.FinalRiskConfidence)
}

