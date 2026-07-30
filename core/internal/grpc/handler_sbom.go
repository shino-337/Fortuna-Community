package grpc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
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
	"github.com/fortuna/core/pkg/cve/matcher"
	"github.com/fortuna/core/pkg/messaging"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/fortuna/core/pkg/worker"
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
	// Go buildinfo uses "command-line-arguments" for binaries built without an explicit
	// module path (common for kubernetes). Accept it as a special case.
	if n == "command-line-arguments" {
		return n, true
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
	// Go buildinfo main modules often report "(devel)" when built outside module context
	// (e.g., kubernetes monorepo). Accept as loose to avoid silently dropping components.
	if v == "(devel)" {
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

	// G1: SBOM ingestion contract validation — reject structurally invalid payloads early.
	{
		components := make([]sbom.SBOMComponentInput, 0, len(req.Packages))
		for _, pkg := range req.Packages {
			purl := strings.TrimSpace(pkg.GetPurl())
			// ValidateSBOM only checks shape for non-empty PURLs. Agents sometimes send
			// placeholders (e.g. "invalid"); treat those as "unset" for this gate so ingestion
			// can regenerate pkg:… from name/version/type in the component loop below.
			if purl != "" && !strings.HasPrefix(purl, "pkg:") {
				purl = ""
			}
			components = append(components, sbom.SBOMComponentInput{PURL: purl})
		}
		imageRef := req.ImageName
		if req.ImageTag != "" {
			imageRef = req.ImageName + ":" + req.ImageTag
		}
		vr := sbom.ValidateSBOM(sbom.SBOMInput{
			ImageRef:    imageRef,
			ImageDigest: req.ImageDigest,
			Components:  components,
		})
		if !vr.Valid {
			metrics.SBOMValidationTotal.WithLabelValues("rejected").Inc()
			log.Printf("[SBOM] correlation_id=%s SBOM ingestion validation failed (pod=%s): %v",
				correlationID, req.PodName, vr.Errors)
			return nil, status.Errorf(codes.InvalidArgument, "SBOM validation failed: %v", vr.Errors)
		}
		metrics.SBOMValidationTotal.WithLabelValues("accepted").Inc()
	}

	// Convert proto to internal model (Finding #8.4: sbom_source, confidence)
	sbomSource, confidence := protoSBOMSourceAndConfidence(req)

	// Determinism metadata (Phase 1): resolver + agent signature DB versions.
	signatureDBVersion := ""
	if req.Labels != nil {
		signatureDBVersion = strings.TrimSpace(req.Labels["fortuna_signature_db_version"])
	}
	resolverVersion := matcher.ResolverVersion

	// Validate agent-provided signature DB version (agent boundary is untrusted).
	// If invalid, mark SBOM as failed instead of trusting nondeterministic metadata.
	signatureDBVersionInvalid := !isValidSignatureDBVersion(signatureDBVersion)
	if signatureDBVersionInvalid {
		signatureDBVersion = ""
	}

	// C0.7: SBOM monotonic state transitions.
	// If an SBOM already exists for this pod, never downgrade its sbom_status.
	// This prevents races/retries from jumping state backwards (e.g. complete -> failed).
	var existingSBOM models.SBOM
	oldSBOMStatus := ""
	oldSBOMStatusReason := ""
	oldSBOMFound := false
	if err := s.db.Where("pod_uid = ? AND image_digest = ? AND deleted_at IS NULL", req.PodUid, req.ImageDigest).First(&existingSBOM).Error; err == nil {
		oldSBOMFound = true
		oldSBOMStatus = existingSBOM.Status
		oldSBOMStatusReason = existingSBOM.StatusReason
	}

	// SBOM status model (C0): propagate ground-truth trust for downstream risk decisions.
	// - failed: SBOM was not extracted (pull failure / extraction failure) => empty packages
	// - partial: distroless-heuristic and/or low/unknown confidence
	// - complete: parsers + high confidence
	sbomStatus := "complete"
	sbomStatusReason := "ok"
	if len(req.Packages) == 0 {
		sbomStatus = "failed"
		// Failed on agent side commonly comes from pull/extract failure path (C0.1).
		sbomStatusReason = "pull_error"
	} else if sbomSource == "distroless-heuristic" || strings.ToLower(strings.TrimSpace(confidence)) != "high" || sbomSource == "" {
		sbomStatus = "partial"
		sbomStatusReason = "parse_error"
	}
	// Signature DB version comes from agent labels (untrusted boundary).
	// If it's invalid, do not proceed as a trustworthy SBOM.
	if signatureDBVersionInvalid && sbomStatus != "failed" {
		sbomStatus = "failed"
		sbomStatusReason = "validation_failed"
	}

	// C0.7 monotonic downgrade protection (and keep components stable):
	// If the pod already has a COMPLETE/PARTIAL SBOM and the new request contains
	// no packages (agent pull/extract failure), do not overwrite existing components.
	if oldSBOMFound && len(req.Packages) == 0 {
		oldNorm := strings.ToLower(strings.TrimSpace(oldSBOMStatus))
		if oldNorm == "finalized" {
			oldNorm = "complete"
		}
		if oldNorm == "complete" || oldNorm == "partial" {
			return &pb.SBOMFindingResponse{
				Success:  true,
				Message:  "SBOM reused (monotonic state preserved; no new components)",
				SbomId:    fmt.Sprintf("%d", existingSBOM.ID),
				ReceivedAt: timestamppb.New(time.Now()),
			}, nil
		}
	}

	sbomModel := &models.SBOM{
		PodUID:         req.PodUid,
		PodName:        req.PodName,
		Namespace:      req.Namespace,
		ContainerName:  req.ContainerName,
		ImageName:      req.ImageName,
		ImageDigest:    req.ImageDigest,
		ImageTag:       req.ImageTag,
		OSName:         req.GetOsInfo().GetName(),
		OSVersion:      req.GetOsInfo().GetVersion(),
		OSArchitecture: req.GetOsInfo().GetArchitecture(),
		GoVersion:      strings.TrimSpace(req.GetGoVersion()),
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
		Status:        sbomStatus,
		StatusReason:  sbomStatusReason,
		ResolverVersion:     resolverVersion,
		SignatureDBVersion: signatureDBVersion,
	}

	// Insert SBOM components. Use agent-provided PURL when set (Finding #8.2 generic/distroless).
	seenPURL := make(map[string]bool)
	var components []*models.SBOMComponent
	for _, pkg := range req.Packages {
		trustLevel := "high"
		purlValidated := true
		originalPURL := ""
		sourceDetail := "agent-fields"

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
				sourceDetail = "core-regenerated-purl"
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
						sourceDetail = "core-regenerated-purl"
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
			sourceDetail = "core-generated-purl"
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
					SourceDetail:     sourceDetail,
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
			SourceDetail:     sourceDetail,
			Licenses:         models.ToJSONBString(pkg.Licenses),
			Source:           pkg.Source,
			Description:      pkg.Description,
			Homepage:         pkg.Homepage,
			Maintainer:       pkg.Maintainer,
		})
	}

	// C0.7: SBOM status model should reflect sanitized components count
	// (after PURL parsing/sanitization). This avoids marking SBOM as "complete"
	// when all requested components were dropped.
	sbomModel.PackageCount = len(components)
	if len(components) == 0 {
		if len(req.Packages) == 0 {
			sbomModel.Status = "failed"
			sbomModel.StatusReason = "pull_error"
		} else {
			// Components were requested but sanitized down to zero (best-effort partial).
			sbomModel.Status = "partial"
			sbomModel.StatusReason = "validation_failed"
		}
	} else if sbomSource == "distroless-heuristic" || strings.ToLower(strings.TrimSpace(confidence)) != "high" || sbomSource == "" {
		sbomModel.Status = "partial"
		sbomModel.StatusReason = "parse_error"
	} else {
		sbomModel.Status = "complete"
		sbomModel.StatusReason = "ok"
	}

	// Determinism fingerprint (Phase 1):
	// hash(sorted(component.ecosystem + name + version)) with dedup.
	// Ignore empty/failed SBOMs to keep drift metrics meaningful.
	if sbomModel.Status != "failed" && len(components) > 0 {
		sbomModel.NormalizedFingerprint = computeNormalizedSBOMFingerprint(components)
	} else {
		sbomModel.NormalizedFingerprint = ""
	}

	// Enforce monotonic status boundaries (downgrade protection).
	if oldSBOMFound {
		old := strings.ToLower(strings.TrimSpace(oldSBOMStatus))
		if old == "" || old == "pending" {
			// pending -> anything is allowed (no override)
		} else if old == "finalized" {
			old = "complete"
		}

		switch old {
		case "complete":
			sbomModel.Status = "complete"
			sbomModel.StatusReason = "ok"
		case "failed":
			// Allow recovery from transient failed state when new ingest has valid components.
			// This prevents pods from being permanently stuck without complete SBOM after one bad run.
			if sbomModel.PackageCount > 0 && (sbomModel.Status == "complete" || sbomModel.Status == "partial") {
				// Keep newly computed status/reason.
			} else {
				sbomModel.Status = "failed"
				if strings.TrimSpace(oldSBOMStatusReason) != "" {
					sbomModel.StatusReason = oldSBOMStatusReason
				}
			}
		case "partial":
			// partial -> complete is allowed; partial -> failed is not.
			if sbomModel.Status != "complete" {
				sbomModel.Status = "partial"
				// Use the NEW status_reason when the current scan has more data
				// (e.g., old was validation_failed with 0 packages, new has packages).
				// Otherwise keep old reason for continuity.
				if sbomModel.PackageCount <= 0 && strings.TrimSpace(oldSBOMStatusReason) != "" {
					sbomModel.StatusReason = oldSBOMStatusReason
				}
			}
		}
	}

	// Guarded SBOM write through repository with explicit mutation flag.
	repo := repository.NewSBOMRepository(s.db)
	ctxWithFlag := contextkeys.WithSBOMMutationAllowed(ctx)
	persistedSBOM, isNewSBOM, err := repo.UpsertSBOMWithComponents(ctxWithFlag, sbomModel, components)
	if err != nil {
		log.Printf("[SBOM] Failed to upsert SBOM: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to store SBOM: %v", err)
	}

	// SBOM_CREATED payload for CVE pipeline: JetStream (preferred) or in-process when NATS client is nil.
	// Use PodUID from request (req.PodUid), not from SBOM record, so insights target the current pod.
	podUID := req.PodUid
	podName := req.PodName
	podNamespace := req.Namespace
	containerName := req.ContainerName
	// clusterID already resolved above (metadata → pod → unknown) for rate limiting and event payload.

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
	eventJSON, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		log.Printf("[SBOM] WARNING: Failed to marshal SBOM_CREATED event: %v", marshalErr)
	} else if s.natsClient != nil {
		priErr, dlqErr := publishSBOMCreatedWithRetry(ctx, s.natsClient.Publish, eventJSON, time.Sleep)
		if priErr == nil {
			log.Printf("[SBOM] correlation_id=%s published SBOM_CREATED event for sbom_id=%d (pod_uid=%s, reused=%v)",
				correlationID, persistedSBOM.ID, podUID, !isNewSBOM)
		} else {
			if dlqErr != nil {
				log.Printf("[SBOM] WARNING: Failed to publish SBOM_CREATED to DLQ: primary_err=%v dlq_err=%v", priErr, dlqErr)
			} else {
				log.Printf("[SBOM] WARNING: Failed to publish SBOM_CREATED after %d attempts; sent to DLQ (%s): %v",
					sbomPublishMaxAttempts, sbomCreatedDLQSubject, priErr)
			}
		}
	} else {
		log.Printf("[SBOM] correlation_id=%s NATS unavailable — running CVE matcher in-process for sbom_id=%d (pod_uid=%s)",
			correlationID, persistedSBOM.ID, podUID)
		cveW := worker.NewCVEMatcherWorker(nil, s.db, nil)
		if err := cveW.ProcessSBOMCreatedEventJSON(ctx, eventJSON); err != nil {
			log.Printf("[SBOM] WARNING: in-process CVE pipeline failed sbom_id=%d: %v", persistedSBOM.ID, err)
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

	// Quick DB connectivity check: if PostgreSQL is unreachable, skip DB writes
	// to avoid noisy GORM error logs on every Ping cycle.
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
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

// computeNormalizedSBOMFingerprint hashes a canonical representation of component identity:
// fingerprint = sha256(sorted( ecosystem + name + version )) with deterministic dedup.
// (Phase 1 determinism v1)
func computeNormalizedSBOMFingerprint(components []*models.SBOMComponent) string {
	normalizeVersionForFingerprint := func(v string) string {
		// Determinism fingerprint should be robust to common distro revision suffixes.
		// Example: APK-style "...-r0" => treat as base version.
		v = strings.TrimSpace(v)
		if v == "" {
			return ""
		}
		// Go build metadata (+...) can also lead to string drift.
		if i := strings.IndexByte(v, '+'); i >= 0 {
			v = v[:i]
		}
		// APK revision: 1.2.3-r0 => 1.2.3
		// Keep conservative: only strip when it matches "-r<digits>" suffix.
		if idx := strings.LastIndex(v, "-r"); idx >= 0 && idx+2 < len(v) {
			suffix := v[idx+2:]
			allDigits := true
			for i := 0; i < len(suffix); i++ {
				if suffix[i] < '0' || suffix[i] > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				v = v[:idx]
			}
		}
		return v
	}

	type tuple struct {
		Ecosystem string
		Name      string
		Version   string
	}

	dedup := make(map[string]tuple, len(components))
	for _, c := range components {
		if c == nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(c.ComponentName))
		version := normalizeVersionForFingerprint(c.ComponentVersion)
		ecosystem := "unknown"

		// Prefer PURL-derived ecosystem/name/version to keep identity aligned with matcher queries.
		if p, err := matcher.ParsePURL(c.PURL); err == nil && p != nil {
			if strings.TrimSpace(p.Ecosystem) != "" {
				ecosystem = strings.ToLower(strings.TrimSpace(p.Ecosystem))
			}
			if strings.TrimSpace(p.Name) != "" {
				name = strings.ToLower(strings.TrimSpace(p.Name))
			}
			if strings.TrimSpace(p.Version) != "" {
				version = normalizeVersionForFingerprint(p.Version)
			}
		}

		key := ecosystem + "|" + name + "|" + version
		dedup[key] = tuple{Ecosystem: ecosystem, Name: name, Version: version}
	}

	if len(dedup) == 0 {
		return ""
	}

	tuples := make([]tuple, 0, len(dedup))
	for _, v := range dedup {
		tuples = append(tuples, v)
	}
	sort.Slice(tuples, func(i, j int) bool {
		if tuples[i].Ecosystem != tuples[j].Ecosystem {
			return tuples[i].Ecosystem < tuples[j].Ecosystem
		}
		if tuples[i].Name != tuples[j].Name {
			return tuples[i].Name < tuples[j].Name
		}
		return tuples[i].Version < tuples[j].Version
	})

	var b strings.Builder
	for i := range tuples {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(tuples[i].Ecosystem)
		b.WriteByte('/')
		b.WriteString(tuples[i].Name)
		b.WriteByte('@')
		b.WriteString(tuples[i].Version)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func isValidSignatureDBVersion(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true // empty means "unknown/unchecked" and should be allowed
	}
	if len(v) > 64 {
		return false
	}
	// Allow common embedded-version shapes: "1", "2026-03", "v1.1", "1.0.0".
	// Avoid path separators / whitespace injection.
	for i := 0; i < len(v); i++ {
		ch := v[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		switch ch {
		case '.', '-', '_':
			continue
		default:
			return false
		}
	}
	return true
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
