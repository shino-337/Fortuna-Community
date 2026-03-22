package sbom

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/google/go-containerregistry/pkg/name"
	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/agent/internal/client"
	"github.com/fortuna/agent/pkg/sbom/extractor"
)

// Processor handles SBOM generation for pods on the local node
// NOTE: CVE matching is done in Core, NOT in Agent (see LOGIC_FLOW_REFACTOR_IMPLEMENTATION.md)
type Processor struct {
	extractor  *extractor.Extractor
	grpcClient client.GRPCClient
	agentID    string
	nodeID     string
	nodeName   string
	logger     *log.Logger
}

// NewProcessor creates a new SBOM processor
// Agent is Data Plane: Only extracts SBOM, does NOT match CVEs
func NewProcessor(grpcClient client.GRPCClient, agentID, nodeID, nodeName string) *Processor {
	return &Processor{
		extractor:  extractor.NewExtractor(),
		grpcClient: grpcClient,
		agentID:    agentID,
		nodeID:     nodeID,
		nodeName:   nodeName,
		logger:     log.New(log.Writer(), "[SBOMProcessor] ", log.LstdFlags),
	}
}

// ProcessPod extracts SBOM from a pod's container images and sends to Core
// Core will handle CVE matching asynchronously
func (p *Processor) ProcessPod(ctx context.Context, pod *corev1.Pod) error {
	p.logger.Printf("Processing pod %s/%s on node %s", pod.Namespace, pod.Name, pod.Spec.NodeName)

	// Verify pod is on our node
	if pod.Spec.NodeName != p.nodeName {
		return fmt.Errorf("pod %s/%s is not on our node (expected %s, got %s)",
			pod.Namespace, pod.Name, p.nodeName, pod.Spec.NodeName)
	}

	// Process each container
	for _, container := range pod.Spec.Containers {
		if err := p.processContainer(ctx, pod, container); err != nil {
			p.logger.Printf("⚠️  Failed to process container %s in pod %s/%s: %v",
				container.Name, pod.Namespace, pod.Name, err)
			// Continue with other containers
		}
	}

	return nil
}

// processContainer extracts SBOM for a single container
// Agent responsibility: Extract raw package inventory, send to Core
// Core responsibility: Match CVEs, generate insights, calculate risk
func (p *Processor) processContainer(ctx context.Context, pod *corev1.Pod, container corev1.Container) error {
	imageRef := container.Image
	p.logger.Printf("🔍 Extracting SBOM: pod=%s/%s container=%s image=%s",
		pod.Namespace, pod.Name, container.Name, imageRef)

	// Extract SBOM using local extractor
	rawSBOM, err := p.extractor.ExtractSBOM(ctx, imageRef)
	if err != nil {
		// C0.1: pull/extraction failure must still emit a SBOMFinding so Core can
		// create an SBOM row with sbom_status=failed (empty packages).
		p.logger.Printf("⚠️  SBOM extraction failed for %s: %v; emitting failed SBOMFinding", imageRef, err)

		imageName, imageTag := parseImageRef(imageRef)
		imageDigest := resolveImageDigest(imageRef)

		sbomFinding := &pb.SBOMFinding{
			SchemaVersion: 1,
			PodUid:        string(pod.UID),
			PodName:       pod.Name,
			Namespace:     pod.Namespace,
			ContainerName: container.Name,
			ImageName:     imageName,
			ImageDigest:   imageDigest,
			ImageTag:      imageTag,
			OsInfo:        nil,
			Packages:      []*pb.Package{},
			GeneratedAt:   timestamppb.New(time.Now()),
			AgentId:       p.agentID,
			NodeId:        p.nodeID,
			Labels:        pod.Labels,
			Annotations:   pod.Annotations,
			SbomSource:    pb.SBOMSource_SBOM_SOURCE_UNKNOWN,
			Confidence:    pb.Confidence_CONFIDENCE_LOW,
		}

		resp, sendErr := p.grpcClient.SendSBOMFinding(ctx, sbomFinding)
		if sendErr != nil {
			// Treat duplicate key as non-fatal (Core uses ON CONFLICT)
			if strings.Contains(sendErr.Error(), "duplicate key") {
				p.logger.Printf("⚠️  Core reported duplicate SBOMFinding (already stored); skipping: %v", sendErr)
				return nil
			}
			return fmt.Errorf("failed to send failed SBOMFinding to Core: %w", sendErr)
		}
		if !resp.Success {
			return fmt.Errorf("Core rejected failed SBOMFinding: %s", resp.Message)
		}

		return nil
	}

	p.logger.Printf("✅ Extracted %d packages from %s", len(rawSBOM.Packages), imageRef)

	// Convert to proto format
	sbomFinding := p.convertToProto(pod, container, rawSBOM)

	// Send SBOM to Core (Core will match CVEs)
	resp, err := p.grpcClient.SendSBOMFinding(ctx, sbomFinding)
	if err != nil {
		// Treat duplicate key as non-fatal (Core uses ON CONFLICT; old Core may still return this)
		if strings.Contains(err.Error(), "duplicate key") {
			p.logger.Printf("⚠️  Core reported duplicate component (already stored); skipping: %v", err)
			return nil
		}
		return fmt.Errorf("failed to send SBOM to Core: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("Core rejected SBOM: %s", resp.Message)
	}

	p.logger.Printf("✅ SBOM sent to Core: sbom_id=%s message=%s", 
		resp.SbomId, resp.Message)
	return nil
}

// convertToProto converts raw SBOM to proto format
func (p *Processor) convertToProto(pod *corev1.Pod, container corev1.Container, rawSBOM *extractor.RawSBOM) *pb.SBOMFinding {
	// Parse image reference
	imageName, imageTag := parseImageRef(container.Image)

	// Convert packages (Finding #8.2, 8.4: PURL, generic type)
	packages := make([]*pb.Package, 0, len(rawSBOM.Packages))
	for _, pkg := range rawSBOM.Packages {
		pp := &pb.Package{
			Name:         pkg.Name,
			Version:      pkg.Version,
			Type:         mapPackageType(pkg.Type),
			Architecture: pkg.Arch,
			Source:       pkg.Source,
		}
		if pkg.PURL != "" {
			pp.Purl = pkg.PURL
		}
		packages = append(packages, pp)
	}

	// Convert OS info
	var osInfo *pb.OSInfo
	if rawSBOM.OS.Name != "" {
		osInfo = &pb.OSInfo{
			Name:    rawSBOM.OS.Name,
			Version: rawSBOM.OS.Version,
			// Note: Variant and Architecture not available in current extractor OSInfo
		}
	}

	labels := make(map[string]string, len(pod.Labels)+1)
	for k, v := range pod.Labels {
		labels[k] = v
	}
	// Determinism metadata for core (Phase 1): signature DB version used by agent.
	if rawSBOM != nil && rawSBOM.SignatureVersion != "" {
		labels["fortuna_signature_db_version"] = rawSBOM.SignatureVersion
	}

	annotations := make(map[string]string, len(pod.Annotations))
	for k, v := range pod.Annotations {
		annotations[k] = v
	}

	out := &pb.SBOMFinding{
		SchemaVersion: 1,
		PodUid:        string(pod.UID),
		PodName:       pod.Name,
		Namespace:     pod.Namespace,
		ContainerName: container.Name,
		ImageName:     imageName,
		ImageDigest:   rawSBOM.ImageDigest,
		ImageTag:      imageTag,
		OsInfo:        osInfo,
		Packages:      packages,
		GeneratedAt:   timestamppb.New(time.Now()),
		AgentId:       p.agentID,
		NodeId:        p.nodeID,
		Labels:        labels,
		Annotations:   annotations,
	}
	// Finding #8.4: SBOM-level source and confidence for Core (Trivy vs NVD fallback)
	out.SbomSource = mapSBOMSource(rawSBOM.SBOMSource)
	out.Confidence = mapConfidence(rawSBOM.Confidence)
	out.GoVersion = strings.TrimSpace(rawSBOM.GoVersion)
	return out
}

// parseImageRef parses an image reference (registry:port/ns/img:tag or img@sha256:...) into
// repository name and tag/digest identifier using go-containerregistry so registry:5000 and
// digest refs are handled correctly (Finding #2).
func parseImageRef(imageRef string) (imageName, imageTag string) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return imageRef, "latest"
	}
	imageName = ref.Context().Name()
	imageTag = ref.Identifier()
	if imageTag == "" {
		imageTag = "latest"
	}
	return imageName, imageTag
}

// resolveImageDigest returns a best-effort stable digest string for sbom cache keys.
// If the reference is already a digest ref (e.g. @sha256:...), we use it.
// Otherwise we fall back to the reference name (same behavior as the extractor).
func resolveImageDigest(imageRef string) string {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return imageRef
	}
	if digestRef, ok := ref.(name.Digest); ok {
		return digestRef.DigestStr()
	}
	return ref.Name()
}

func mapPackageType(pkgType string) pb.PackageType {
	switch pkgType {
	case "deb":
		return pb.PackageType_PACKAGE_TYPE_DEB
	case "rpm":
		return pb.PackageType_PACKAGE_TYPE_RPM
	case "apk":
		return pb.PackageType_PACKAGE_TYPE_APK
	case "npm":
		return pb.PackageType_PACKAGE_TYPE_NPM
	case "pypi":
		return pb.PackageType_PACKAGE_TYPE_PYPI
	case "gem":
		return pb.PackageType_PACKAGE_TYPE_GEM
	case "go", "go-binary":
		// go-binary: main module from gobinary parser — same proto enum as module deps (Finding: avoid UNKNOWN).
		return pb.PackageType_PACKAGE_TYPE_GO_MOD
	case "maven":
		return pb.PackageType_PACKAGE_TYPE_MAVEN
	case "cargo":
		return pb.PackageType_PACKAGE_TYPE_CARGO
	case "generic":
		return pb.PackageType_PACKAGE_TYPE_GENERIC
	default:
		return pb.PackageType_PACKAGE_TYPE_UNKNOWN
	}
}

func mapSBOMSource(s string) pb.SBOMSource {
	switch s {
	case "distroless-heuristic":
		return pb.SBOMSource_SBOM_SOURCE_DISTROLLESS_HEURISTIC
	case "label-metadata":
		return pb.SBOMSource_SBOM_SOURCE_LABEL_METADATA
	case "parsers":
		return pb.SBOMSource_SBOM_SOURCE_PARSERS
	default:
		return pb.SBOMSource_SBOM_SOURCE_UNKNOWN
	}
}

func mapConfidence(s string) pb.Confidence {
	switch s {
	case "high":
		return pb.Confidence_CONFIDENCE_HIGH
	case "medium":
		return pb.Confidence_CONFIDENCE_MEDIUM
	case "low":
		return pb.Confidence_CONFIDENCE_LOW
	default:
		return pb.Confidence_CONFIDENCE_UNKNOWN
	}
}
