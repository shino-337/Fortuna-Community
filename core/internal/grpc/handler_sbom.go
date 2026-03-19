package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/internal/contextkeys"
	"github.com/fortuna/core/internal/ingest"
	"github.com/fortuna/core/internal/repository"
	"github.com/fortuna/core/pkg/messaging"
	"github.com/fortuna/core/pkg/cve/matcher"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
)

var goVersionNoBuildMeta = regexp.MustCompile(`^\s*v?\d+\.\d+\.\d+([\-\.].*)?\s*$`)
var goModuleNameAllowed = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(/[a-z0-9._-]+)*$`)

func normalizeGoVersionForPURL(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// Drop build metadata (+...) to avoid logic injection / comparator mismatches.
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	// Ensure leading "v" for Go module versions when it looks like semver/pseudo-version.
	if strings.HasPrefix(v, "v") {
		return v
	}
	if goVersionNoBuildMeta.MatchString(v) {
		return "v" + v
	}
	return v
}

func canonicalizeGoModuleName(name string) (string, bool) {
	// Returns (canonical, ok). Normalize first, then validate on canonical form.
	// Reject non-ASCII to prevent homoglyph confusion (Go modules are effectively ASCII).
	for i := 0; i < len(name); i++ {
		if name[i] > 0x7f {
			return "", false
		}
	}
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return "", false
	}
	// Normalize harmless trailing slashes; do not clean other path elements.
	n = strings.TrimRight(n, "/")
	if n == "" {
		return "", false
	}
	// Reject traversal on canonical form (no mixed raw/normalized validation).
	if strings.Contains(n, "..") {
		return "", false
	}
	// Basic allowlist for go module path segments.
	if !goModuleNameAllowed.MatchString(n) {
		return "", false
	}
	// Must have a domain segment containing a dot (e.g., github.com, go.etcd.io, k8s.io).
	first := n
	if i := strings.IndexByte(n, '/'); i >= 0 {
		first = n[:i]
	}
	if !strings.Contains(first, ".") {
		return "", false
	}
	return n, true
}

type goVersionLevel string

const (
	goVerStrict  goVersionLevel = "strict"
	goVerLoose   goVersionLevel = "loose"
	goVerInvalid goVersionLevel = "invalid"
)

var goStrictSemver = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
var goPseudoVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+-(0\.)?\d{14}-[0-9a-f]{7,}$`)
var goLooseSemver = regexp.MustCompile(`^v?\d+\.\d+$`)

func classifyGoVersion(v string) goVersionLevel {
	v = strings.TrimSpace(v)
	if v == "" {
		return goVerInvalid
	}
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if goStrictSemver.MatchString(v) || goPseudoVersion.MatchString(v) {
		return goVerStrict
	}
	if goLooseSemver.MatchString(v) {
		return goVerLoose
	}
	return goVerInvalid
}

func expectedPurlEcosystemForType(t pb.PackageType) string {
	switch t {
	case pb.PackageType_PACKAGE_TYPE_GO_MOD:
		return "go"
	case pb.PackageType_PACKAGE_TYPE_NPM:
		return "npm"
	case pb.PackageType_PACKAGE_TYPE_PYPI:
		return "pypi"
	case pb.PackageType_PACKAGE_TYPE_GEM:
		return "gem"
	case pb.PackageType_PACKAGE_TYPE_MAVEN:
		return "maven"
	case pb.PackageType_PACKAGE_TYPE_CARGO:
		return "cargo"
	case pb.PackageType_PACKAGE_TYPE_DEB:
		return "deb"
	case pb.PackageType_PACKAGE_TYPE_APK:
		return "apk"
	case pb.PackageType_PACKAGE_TYPE_RPM:
		return "rpm"
	default:
		return ""
	}
}

// SBOMServiceServer implements the SBOM-related RPCs from AgentService
type SBOMServiceServer struct {
	db              *gorm.DB
	natsClient      *messaging.NATSClient
	clusterLimiter  *ingest.ClusterRateLimiter
	pb.UnimplementedAgentServiceServer
}

// NewSBOMServiceServer creates a new SBOM service server
func NewSBOMServiceServer(db *gorm.DB, natsClient *messaging.NATSClient, clusterLimiter *ingest.ClusterRateLimiter) *SBOMServiceServer {
	return &SBOMServiceServer{
		db:             db,
		natsClient:     natsClient,
		clusterLimiter: clusterLimiter,
	}
}

// correlationIDFromContext returns x-correlation-id from gRPC metadata or generates one (Finding #1.2).
func correlationIDFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md.Get("x-correlation-id"); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("core-%d", time.Now().UnixNano())
}

func newEventID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("ev-%d", time.Now().UnixNano())
}

// clusterIDFromContext returns x-cluster-id from gRPC metadata or empty (Finding #6).
func clusterIDFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md.Get("x-cluster-id"); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	return ""
}

// SendSBOMFinding handles a single SBOM finding from Agent
func (s *SBOMServiceServer) SendSBOMFinding(ctx context.Context, req *pb.SBOMFinding) (*pb.SBOMFindingResponse, error) {
	correlationID := correlationIDFromContext(ctx)
	eventID := newEventID()
	clusterID := clusterIDFromContext(ctx)
	if clusterID == "" && s.db != nil {
		var pod models.Pod
		if err := s.db.Where("uid = ? AND deleted_at IS NULL", req.PodUid).First(&pod).Error; err == nil {
			clusterID = pod.ClusterID
		}
	}
	if clusterID == "" {
		clusterID = "unknown"
	}
	if s.clusterLimiter != nil && !s.clusterLimiter.AllowSBOM(clusterID) {
		log.Printf("[SBOM] correlation_id=%s rate limit exceeded for cluster_id=%s", correlationID, clusterID)
		return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded for cluster %s", clusterID)
	}
	log.Printf("[SBOM] correlation_id=%s received SBOM from agent=%s, pod=%s, image=%s",
		correlationID, req.AgentId, req.PodName, req.ImageDigest)

	// Check if database is available
	if s.db == nil {
		log.Printf("[SBOM] Warning: Database not available, returning unavailable status")
		return nil, status.Errorf(codes.Unavailable, "database not available")
	}

	// Convert proto to internal model (Finding #8.4: sbom_source, confidence)
	sbomSource, confidence := protoSBOMSourceAndConfidence(req)
	sbomModel := &models.SBOM{
		PodUID:        req.PodUid,
		PodName:       req.PodName,
		Namespace:     req.Namespace,
		ContainerName: req.ContainerName,
		ImageName:     req.ImageName,
		ImageDigest:   req.ImageDigest,
		ImageTag:      req.ImageTag,
		GeneratedAt:   req.GeneratedAt.AsTime(),
		AgentID:       req.AgentId,
		NodeID:        req.NodeId,
		PackageCount:  len(req.Packages),
		SBOMFormat:    "fortuna-agent",
		SBOMContent:   "{}",
		Labels:        make(map[string]string),
		Annotations:   make(map[string]string),
		LastUsedAt:    time.Now(),
		UseCount:      1,
		SbomSource:    sbomSource,
		Confidence:    confidence,
	}

	// Insert SBOM components. Use agent-provided PURL when set (Finding #8.2 generic/distroless).
	seenPURL := make(map[string]bool)
	var components []*models.SBOMComponent
	for _, pkg := range req.Packages {
		trustLevel := "high"
		purlValidated := true
		originalPURL := ""

		// Trust boundary: prefer agent-provided PURL, but sanitize malformed input (best-effort).
		purl := pkg.GetPurl()
		if purl != "" {
			originalPURL = purl
			parsed, err := matcher.ParsePURL(purl)
			if err != nil {
				log.Printf("[SBOM] WARNING: invalid package PURL %q (name=%q type=%v): %v; regenerating", purl, pkg.Name, pkg.Type, err)
				purl = ""
				trustLevel = "low"
				purlValidated = false
			} else if parsed != nil {
				// Semantic validation: ecosystem must be consistent with package type (when applicable).
				if exp := expectedPurlEcosystemForType(pkg.Type); exp != "" {
					got := strings.ToLower(strings.TrimSpace(parsed.Ecosystem))
					// Backward compatible alias for Go.
					if got == "golang" {
						got = "go"
					}
					if got != exp {
						log.Printf("[SBOM] WARNING: PURL ecosystem mismatch purl=%q expected=%q got=%q (name=%q type=%v); regenerating",
							purl, exp, got, pkg.Name, pkg.Type)
						purl = ""
						trustLevel = "low"
						purlValidated = false
					}
				}
				// Go-specific: forbid build metadata injection in versions; normalize.
				if purl != "" && strings.EqualFold(parsed.Ecosystem, "go") {
					canonName, ok := canonicalizeGoModuleName(pkg.Name)
					if !ok {
						log.Printf("[SBOM] WARNING: invalid Go module name %q (purl=%q); dropping component", pkg.Name, originalPURL)
						continue
					}
					nv := normalizeGoVersionForPURL(pkg.Version)
					level := classifyGoVersion(nv)
					switch level {
					case goVerStrict:
						// ok
					case goVerLoose:
						trustLevel = "medium"
					default:
						log.Printf("[SBOM] WARNING: invalid Go module version %q (name=%q purl=%q); dropping component", pkg.Version, canonName, originalPURL)
						continue
					}
					// Canonicalize PURL if agent sent non-canonical name or build metadata.
					purl = fmt.Sprintf("pkg:go/%s@%s", canonName, nv)
					if purl != originalPURL {
						if trustLevel == "high" {
							trustLevel = "medium"
						}
					}
				}
			}
		}
		if purl == "" {
			ecosystem := purlEcosystem(pkg.Type)
			version := pkg.Version
			if ecosystem == "go" {
				canonName, ok := canonicalizeGoModuleName(pkg.Name)
				if !ok {
					log.Printf("[SBOM] WARNING: invalid Go module name %q (no PURL); dropping component", pkg.Name)
					continue
				}
				version = normalizeGoVersionForPURL(version)
				level := classifyGoVersion(version)
				if level == goVerInvalid {
					log.Printf("[SBOM] WARNING: invalid Go module version %q (name=%q no PURL); dropping component", pkg.Version, canonName)
					continue
				}
				if level == goVerLoose {
					trustLevel = "medium"
				} else {
					trustLevel = "low" // generated PURL from fields even though strict -> still LOW trust (edge-derived)
				}
				purl = fmt.Sprintf("pkg:go/%s@%s", canonName, version)
				purlValidated = false
				originalPURL = ""
				// skip default formatting below
				if seenPURL[purl] {
					continue
				}
				seenPURL[purl] = true
				components = append(components, &models.SBOMComponent{
					ComponentType:    mapComponentType(pkg.Type),
					ComponentName:    canonName,
					ComponentVersion: version,
					PURL:             purl,
					OriginalPURL:     originalPURL,
					PURLValidated:    purlValidated,
					TrustLevel:       trustLevel,
					Licenses:         models.ToJSONBString(pkg.Licenses),
					Source:           pkg.Source,
					Description:      pkg.Description,
					Homepage:         pkg.Homepage,
					Maintainer:       pkg.Maintainer,
				})
				continue
			}
			purl = fmt.Sprintf("pkg:%s/%s@%s", ecosystem, pkg.Name, version)
			trustLevel = "low"
			purlValidated = false
		}
		if seenPURL[purl] {
			continue
		}
		seenPURL[purl] = true
		components = append(components, &models.SBOMComponent{
			ComponentType:    mapComponentType(pkg.Type),
			ComponentName:    pkg.Name,
			ComponentVersion: pkg.Version,
			PURL:             purl,
			OriginalPURL:     originalPURL,
			PURLValidated:    purlValidated,
			TrustLevel:       trustLevel,
			Licenses:         models.ToJSONBString(pkg.Licenses),
			Source:           pkg.Source,
			Description:      pkg.Description,
			Homepage:         pkg.Homepage,
			Maintainer:       pkg.Maintainer,
		})
	}
	// Guarded SBOM write through repository with explicit mutation flag.
	repo := repository.NewSBOMRepository(s.db)
	ctxWithFlag := contextkeys.WithSBOMMutationAllowed(ctx)
	persistedSBOM, isNewSBOM, err := repo.UpsertSBOMWithComponents(ctxWithFlag, sbomModel, components)
	if err != nil {
		log.Printf("[SBOM] Failed to upsert SBOM: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to store SBOM: %v", err)
	}

	// Publish SBOM_CREATED event to NATS (for CVE matching worker)
	// Use subject 'fortuna.sbom.created' to match stream pattern 'fortuna.sbom.>' in 'fortuna-events' stream
	// Include all required fields for worker to create insights with new schema
	// IMPORTANT: Use PodUID from request (req.PodUid), not from SBOM record (sbom.PodUID)
	// This ensures insights are created for the CURRENT pod, not the pod that first created the SBOM
	if s.natsClient != nil {
		// Use PodUID from request to ensure insights are created for the current pod
		// even when SBOM is reused (same image_digest)
		podUID := req.PodUid
		podName := req.PodName
		podNamespace := req.Namespace
		containerName := req.ContainerName

		// Resolve cluster_id from pod in DB or env only (no hardcoded name)
		var clusterID string
		var pod models.Pod
		if err := s.db.Where("uid = ? AND deleted_at IS NULL", podUID).First(&pod).Error; err == nil {
			clusterID = pod.ClusterID
		}
		if clusterID == "" {
			if v := os.Getenv("DEFAULT_CLUSTER_ID"); v != "" {
				clusterID = v
			} else {
				clusterID = "unknown"
			}
		}

		// P1-5: component snapshot at publish time so CVE matcher can use it and avoid soft-delete race
		componentsSnapshot := make([]map[string]interface{}, 0, len(components))
		for _, c := range components {
			eco := ""
			ns := ""
			arch := ""
			normalizedName := ""
			versionClass := "UNKNOWN"

			if p, err := matcher.ParsePURL(c.PURL); err == nil && p != nil {
				eco = strings.ToLower(strings.TrimSpace(p.Ecosystem))
				if eco == "golang" {
					eco = "go"
				}
				ns = strings.TrimSpace(p.Namespace)
				if p.Qualifiers != nil {
					arch = strings.TrimSpace(p.Qualifiers["arch"])
				}
				normalizedName = strings.ToLower(strings.TrimSpace(p.Name))
				if eco == "go" {
					// Canonicalize based on component fields (already firewall-validated).
					normalizedName = c.ComponentName
					switch classifyGoVersion(normalizeGoVersionForPURL(c.ComponentVersion)) {
					case goVerStrict:
						versionClass = "STRICT"
					case goVerLoose:
						versionClass = "LOOSE"
					default:
						versionClass = "INVALID"
					}
				}
			}

			componentsSnapshot = append(componentsSnapshot, map[string]interface{}{
				"purl":            c.PURL,
				"name":            c.ComponentName,
				"version":         c.ComponentVersion,
				"normalized_name": normalizedName,
				"version_class":   versionClass,
				"ecosystem":       eco,
				"namespace":       ns,
				"arch":            arch,
				"source":          c.Source,
				"trust_level":     c.TrustLevel,
				"original_purl":   c.OriginalPURL,
				"purl_validated":  c.PURLValidated,
			})
		}
		// Create proper JSON event with all required fields using map to avoid import issues
		event := map[string]interface{}{
			"type":                "sbom.created",
			"timestamp":           time.Now().Unix(),
			"event_id":            eventID,
			"schema_version":      sbom.SBOMCreatedEventSchemaVersion,
			"correlation_id":      correlationID,
			"cluster_id":          clusterID,
			"pod_uid":             podUID,
			"pod_name":            podName,
			"pod_namespace":       podNamespace,
			"container_name":      containerName,
			"container_image":     fmt.Sprintf("%s:%s", persistedSBOM.ImageName, persistedSBOM.ImageTag),
			"sbom_id":             persistedSBOM.ID,
			"image_digest":        persistedSBOM.ImageDigest,
			"components_snapshot": componentsSnapshot,
		}
		eventJSON, err := json.Marshal(event)
		if err != nil {
			log.Printf("[SBOM] WARNING: Failed to marshal SBOM_CREATED event: %v", err)
		} else {
			if err := s.natsClient.Publish("fortuna.sbom.created", eventJSON); err != nil {
				log.Printf("[SBOM] WARNING: Failed to publish SBOM_CREATED event: %v", err)
				// Non-fatal, continue
			} else {
				log.Printf("[SBOM] correlation_id=%s published SBOM_CREATED event for sbom_id=%d (pod_uid=%s, reused=%v)",
					correlationID, persistedSBOM.ID, podUID, !isNewSBOM)
			}
		}
	}

	log.Printf("[SBOM] correlation_id=%s successfully stored SBOM id=%d with %d components", correlationID, persistedSBOM.ID, len(req.Packages))

	return &pb.SBOMFindingResponse{
		Success:    true,
		Message:    "SBOM received and stored",
		SbomId:     fmt.Sprintf("%d", persistedSBOM.ID),
		ReceivedAt: timestamppb.New(time.Now()),
	}, nil
}

// BatchSendSBOMFindings handles multiple SBOM findings in a stream
// Note: Proto uses stream, not BatchSBOMRequest
func (s *SBOMServiceServer) BatchSendSBOMFindings(stream pb.AgentService_BatchSendSBOMFindingsServer) error {
	// Process stream of SBOM findings
	var successCount int32
	var totalCount int32

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		totalCount++

		// Process each SBOM finding
		_, err = s.SendSBOMFinding(stream.Context(), req)
		if err == nil {
			successCount++
		}
	}

	log.Printf("[SBOM] Batch processed: total=%d, success=%d", totalCount, successCount)

	resp := &pb.BatchSBOMFindingResponse{
		Success:        true,
		Message:        fmt.Sprintf("Processed %d/%d SBOM findings", successCount, totalCount),
		ReceivedCount:  totalCount,
		ProcessedCount: successCount,
	}

	return stream.SendAndClose(resp)
}

// Ping handles health check from agent; updates last_seen_at so dashboard shows agents in time.
// Upserts agent by agent_id so dashboard updates even if Register failed or ran after first Ping.
// Prefer agent_id; if empty (old agent image), fallback to node_name (update first matching agent).
func (s *SBOMServiceServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	agentID := ""
	nodeName := ""
	if req != nil {
		agentID = req.AgentId
		nodeName = req.NodeName
	}
	now := time.Now()
	if s.db == nil {
		return &pb.PingResponse{Status: "healthy", Version: "1.0.0"}, nil
	}

	updated := false
	if agentID != "" {
		// Use raw Exec so last_seen_at is always updated (avoids GORM scope/zero-value issues)
		res := s.db.Exec(
			"UPDATE agents SET last_seen_at = ?, node_name = ?, status = ?, updated_at = ?, deleted_at = NULL WHERE agent_id = ?",
			now, nodeName, "ready", now, agentID,
		)
		if res.Error != nil {
			log.Printf("[Agent] Ping: failed to update agent_id=%s: %v", agentID, res.Error)
		} else if res.RowsAffected > 0 {
			updated = true
		} else {
			// No row: create from Ping so dashboard shows agent without waiting for Register
			agent := models.Agent{
				AgentID:    agentID,
				NodeName:   nodeName,
				Version:    "",
				Status:     "ready",
				LastSeenAt: &now,
			}
			if err := s.db.Create(&agent).Error; err != nil {
				log.Printf("[Agent] Ping: failed to create agent from Ping agent_id=%s: %v", agentID, err)
			} else {
				log.Printf("[Agent] Ping: created agent from Ping agent_id=%s node=%s", agentID, nodeName)
				updated = true
			}
		}
	}
	if !updated && nodeName != "" {
		// Fallback: old agent may not send agent_id; update first matching row by node_name (PostgreSQL: subquery for LIMIT)
		res := s.db.Exec(
			"UPDATE agents SET last_seen_at = ?, updated_at = ? WHERE id = (SELECT id FROM agents WHERE node_name = ? AND (status = ? OR status IS NULL) AND deleted_at IS NULL LIMIT 1)",
			now, now, nodeName, "ready",
		)
		if res.Error != nil {
			log.Printf("[Agent] Ping: fallback update by node_name=%s failed: %v", nodeName, res.Error)
		}
	}

	return &pb.PingResponse{
		Status:  "healthy",
		Version: "1.0.0", // TODO: Get from build info
	}, nil
}

// RegisterAgent handles agent registration
func (s *SBOMServiceServer) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	log.Printf("[Agent] Register: id=%s, node=%s, version=%s, capabilities=%v",
		req.AgentId, req.NodeName, req.Version, req.Capabilities)

	if s.db == nil {
		log.Printf("[Agent] Warning: database not available during registration")
		return nil, status.Errorf(codes.Unavailable, "database not available")
	}

	capJSON, _ := json.Marshal(req.Capabilities)
	now := time.Now()

	var existing models.Agent
	err := s.db.Where("agent_id = ?", req.AgentId).First(&existing).Error
	switch {
	case err == nil:
		if err := s.db.Model(&existing).Updates(map[string]interface{}{
			"node_name":    req.NodeName,
			"version":      req.Version,
			"status":       "ready",
			"capabilities": string(capJSON),
			"last_seen_at": now,
			"deleted_at":   nil,
		}).Error; err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update agent: %v", err)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		agent := models.Agent{
			AgentID:      req.AgentId,
			NodeName:     req.NodeName,
			Version:      req.Version,
			Status:       "ready",
			Capabilities: string(capJSON),
			LastSeenAt:   &now,
		}
		if err := s.db.Create(&agent).Error; err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create agent: %v", err)
		}
	default:
		return nil, status.Errorf(codes.Internal, "failed to query agent: %v", err)
	}

	clusterID := os.Getenv("DEFAULT_CLUSTER_ID")
	if clusterID == "" {
		clusterID = "unknown"
	}
	return &pb.RegisterAgentResponse{
		Success:   true,
		Message:   "Agent registered successfully",
		ClusterId: clusterID, // From env only; no hardcoded default
	}, nil
}

// purlEcosystem returns PURL ecosystem name for package type (Finding #8.2).
func purlEcosystem(t pb.PackageType) string {
	switch t {
	case pb.PackageType_PACKAGE_TYPE_DEB:
		return "deb"
	case pb.PackageType_PACKAGE_TYPE_RPM:
		return "rpm"
	case pb.PackageType_PACKAGE_TYPE_APK:
		return "apk"
	case pb.PackageType_PACKAGE_TYPE_NPM:
		return "npm"
	case pb.PackageType_PACKAGE_TYPE_PYPI:
		return "pypi"
	case pb.PackageType_PACKAGE_TYPE_GEM:
		return "gem"
	case pb.PackageType_PACKAGE_TYPE_GO_MOD:
		return "go"
	case pb.PackageType_PACKAGE_TYPE_MAVEN:
		return "maven"
	case pb.PackageType_PACKAGE_TYPE_CARGO:
		return "cargo"
	case pb.PackageType_PACKAGE_TYPE_GENERIC:
		return "generic"
	default:
		return "generic"
	}
}

// protoSBOMSourceAndConfidence maps proto enums to stored strings (Finding #8.4).
func protoSBOMSourceAndConfidence(req *pb.SBOMFinding) (sbomSource, confidence string) {
	switch req.GetSbomSource() {
	case pb.SBOMSource_SBOM_SOURCE_PARSERS:
		sbomSource = "parsers"
	case pb.SBOMSource_SBOM_SOURCE_DISTROLLESS_HEURISTIC:
		sbomSource = "distroless-heuristic"
	case pb.SBOMSource_SBOM_SOURCE_LABEL_METADATA:
		sbomSource = "label-metadata"
	default:
		sbomSource = ""
	}
	switch req.GetConfidence() {
	case pb.Confidence_CONFIDENCE_HIGH:
		confidence = "high"
	case pb.Confidence_CONFIDENCE_MEDIUM:
		confidence = "medium"
	case pb.Confidence_CONFIDENCE_LOW:
		confidence = "low"
	default:
		confidence = ""
	}
	return sbomSource, confidence
}

func mapComponentType(t pb.PackageType) string {
	switch t {
	case pb.PackageType_PACKAGE_TYPE_DEB, pb.PackageType_PACKAGE_TYPE_RPM, pb.PackageType_PACKAGE_TYPE_APK:
		return "os-package"
	case pb.PackageType_PACKAGE_TYPE_NPM, pb.PackageType_PACKAGE_TYPE_PYPI, pb.PackageType_PACKAGE_TYPE_GEM,
		pb.PackageType_PACKAGE_TYPE_GO_MOD, pb.PackageType_PACKAGE_TYPE_MAVEN, pb.PackageType_PACKAGE_TYPE_CARGO:
		return "language-package"
	case pb.PackageType_PACKAGE_TYPE_GENERIC:
		return "application" // distroless/system heuristic component
	default:
		return "unknown"
	}
}
