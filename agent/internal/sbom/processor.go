package sbom

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

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
		return fmt.Errorf("SBOM extraction failed for %s: %w", imageRef, err)
	}

	p.logger.Printf("✅ Extracted %d packages from %s", len(rawSBOM.Packages), imageRef)

	// Convert to proto format
	sbomFinding := p.convertToProto(pod, container, rawSBOM)

	// Send SBOM to Core (Core will match CVEs)
	resp, err := p.grpcClient.SendSBOMFinding(ctx, sbomFinding)
	if err != nil {
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

	// Convert packages
	packages := make([]*pb.Package, 0, len(rawSBOM.Packages))
	for _, pkg := range rawSBOM.Packages {
		packages = append(packages, &pb.Package{
			Name:         pkg.Name,
			Version:      pkg.Version,
			Type:         mapPackageType(pkg.Type),
			Architecture: pkg.Arch,
			// Note: Licenses, Source, Description, Homepage, Maintainer not available in current extractor
		})
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

	return &pb.SBOMFinding{
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
		Labels:        pod.Labels,
		Annotations:   pod.Annotations,
	}
}

// Helper functions
func parseImageRef(imageRef string) (string, string) {
	// Simple parser: split by ':'
	// TODO: Handle registry URLs properly (e.g., registry.io/namespace/image:tag)
	lastColon := -1
	for i := len(imageRef) - 1; i >= 0; i-- {
		if imageRef[i] == ':' {
			lastColon = i
			break
		}
		if imageRef[i] == '/' {
			break // Hit a slash before colon, no tag
		}
	}

	if lastColon > 0 {
		return imageRef[:lastColon], imageRef[lastColon+1:]
	}
	return imageRef, "latest"
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
	case "go":
		return pb.PackageType_PACKAGE_TYPE_GO_MOD
	case "maven":
		return pb.PackageType_PACKAGE_TYPE_MAVEN
	case "cargo":
		return pb.PackageType_PACKAGE_TYPE_CARGO
	default:
		return pb.PackageType_PACKAGE_TYPE_UNKNOWN
	}
}
