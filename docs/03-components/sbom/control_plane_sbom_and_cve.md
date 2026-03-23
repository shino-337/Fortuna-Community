PHÂN TÍCH CHI TIẾT: SBOM & CVE SCANNING CHO K8S CONTROL PLANE PODS
I. ĐẶC ĐIỂM K8S CONTROL PLANE IMAGES
Các Pod Hệ Thống Control Plane:
Code
kube-apiserver        - Go binary (static)
kube-controller-manager - Go binary (static)
kube-scheduler        - Go binary (static)
kube-proxy            - Go binary (static)
etcd                  - Go binary (static)
coredns               - Go binary (static)
Đặc Điểm Chung:
Code
✓ Compiled Go binaries (statically linked)
✓ Minimal distroless base (gcr.io/distroless/base)
✓ NO traditional package managers
✓ NO /etc/os-release (thường không có)
✓ NO dynamic library dependencies (trong ý nghĩa truyền thống)
✗ Khó scan bằng cách truyền thống
✓ Version info embedded trong binary
Cấu Trúc Filesystem:
Code
/
├── /etc/ssl/certs/           (CA certificates)
├── /usr/local/go/bin/        (Go runtime binaries)
├── /app/kube-apiserver       (Main binary)
└── /proc, /sys               (Runtime only)

Không có:
- /var/lib/dpkg/
- /var/lib/rpm/
- /lib/apk/db/
- /etc/os-release
- /usr/bin/, /usr/lib/ (system utils)
II. TRIVY - K8S CONTROL PLANE SCANNING
Kiến Trúc:
Trivy có dedicated Kubernetes scanning module:

Go
// pkg/k8s/scanner/scanner.go
const (
	controlPlaneComponents = "ControlPlaneComponents"
	nodeComponents = "NodeComponents"
	clusterInfo = "Cluster"
)

// Trivy differentiates between 3 types:
// 1. Cluster infrastructure (api-server, kubelet, etcd, etc.)
// 2. Cluster configuration (Roles, ClusterRoles, policies)
// 3. Application workloads (user deployments)
Luồng Scanning Chi Tiết:
Code
┌──────────────────────────────────────────────────────────────────┐
│   Trivy K8s Control Plane Scanning Flow                          │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  PHASE 1: CLUSTER DISCOVERY & COMPONENT EXTRACTION              │
│  ═══════════════════════════════════════════════════            │
│  └─> Connect to K8s API server                                  │
│      - Get node list                                             │
│      - Get pod list in kube-system namespace                     │
│      - Extract component versions:                               │
│        ├─ kube-apiserver:v1.25.4                                │
│        ├─ kube-controller-manager:v1.25.4                       │
│        ├─ kube-scheduler:v1.25.4                                │
│        ├─ etcd:v3.5.5                                           ��
│        └─ coredns:1.9.3                                          │
│                                                                  │
│  PHASE 2: VERSION PARSING & NORMALIZATION                       │
│  ═════════════════════════════════════════                      │
│  └─> func unifiedVersion(version string) string                 │
│      Input: "v1.25.4" or "1.25.4-123-abc"                       │
│      Output: "1.25.4"  (normalized format)                      │
│                                                                  │
│  PHASE 3: K8S LANGUAGE NAMESPACE MAPPING                         │
│  ═════════════════════════════════════════                      │
│  └─> func k8sNamespace(version, nodeName) string                │
│      ├─ Detect K8s version: 1.25, 1.26, etc.                    │
│      ├─ Generate namespace for DB lookup:                       │
│      │  "k8s.io/kubernetes:v1.25.4"                             │
│      │  "k8s.io/etcd:v3.5.5"                                    │
│      │  "k8s.io/coredns:1.9.3"                                  │
│      └─ Map to vulnerability database                           │
│                                                                  │
│  PHASE 4: CVE DATABASE LOOKUP (KBOM)                             │
│  ════════════════════════════════════                           │
│  └─> vuln-list-k8s database                                     │
│      ├─ Maintained by Aquasecurity                              │
│      ├─ Specific K8s component CVEs                             │
│      ├─ Based on official K8s security advisories               │
│      └─ Example:                                                │
│          CVE-2021-25742: kube-apiserver, 1.19.0-1.22.2          │
│          CVE-2021-25741: kubelet, 1.19.0-1.22.2                │
│                                                                  │
│  PHASE 5: COMPONENT VERSION MATCHING                             │
│  ════════════════════════════════════                           │
│  └─> k8sScanner.Scan()                                          │
│      ├─ Create ScanTarget with:                                 │
│      │  {                                                        │
│      │    Applications: [{                                       │
│      │      Type: "k8s.io/kubernetes",                          │
│      │      FilePath: "kube-apiserver",                         │
│      │      Packages: [{                                        │
│      │        Name: "k8s.io/apiserver",                         │
│      │        Version: "1.25.4"                                 │
│      │      }]                                                  │
│      │    }]                                                     │
│      │  }                                                        │
│      └─ Match against CVE database                              │
│                                                                  │
│  PHASE 6: VULNERABILITY DETECTION                                │
│  ════════════════════════════════                               │
│  └─> Detect CVEs matching version range                         │
│      └─ Example:                                                │
│          CVE-2021-25742 affects 1.19.0-1.22.2                   │
│          Target: 1.25.4                                         │
│          Result: NOT AFFECTED                                   │
│                                                                  │
│  PHASE 7: CONTAINER IMAGE SCANNING (Parallel)                    │
│  ════════════════════════════════════════════                   │
│  └─> For each control plane pod:                                │
│      ├─ Get image digest                                        │
│      ├─ Pull image if available                                 │
│      ├─ Scan image packages                                     │
│      │  (will find minimal distroless base)                     │
│      ├─ Scan binary contents                                    │
│      └─ Combine results                                         │
│                                                                  │
│  PHASE 8: SBOM GENERATION (KBOM)                                 │
│  ════════════════════════════════════                           │
│  └─> Create Kubernetes BOM with:                                │
│      ├─ Cluster information                                     │
│      ├─ Node information                                        │
│      ├─ Control plane components                                │
│      ├─ Node components (kubelet, kube-proxy)                   │
│      ├─ Container images                                        │
│      └─ Relationships (pod→image→components)                    │
│                                                                  │
│  PHASE 9: REPORT GENERATION                                      │
│  ═════════════════════════════════                              │
│  └─> Output format:                                             │
│      ├─ Table: Vulnerabilities per component                    │
│      ├─ JSON: Detailed findings with CVE details                │
│      ├─ SBOM: Cyclone DX hoặc SPDX                               │
│      └─ Grouping by node, namespace                             │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
Code Trivy - Chi Tiết:
Go
// pkg/k8s/scanner/scanner.go - Control plane scanning
func (s *Scanner) scanK8sVulns(ctx context.Context, 
                               artifactsData []*artifacts.Artifact) 
                               ([]report.Resource, error) {
	
	for _, artifact := range artifactsData {
		switch artifact.Kind {
		case controlPlaneComponents:  // <-- kube-apiserver, etc.
			var comp bom.Component
			err := ms.Decode(artifact.RawResource, &comp)
			
			// Extract component info
			cpcVersion := unifiedVersion(comp.Version)  // "v1.25.4" -> "1.25.4"
			
			// Map to K8s namespace
			lang := k8sNamespace(cpcVersion, nodeName)  // "k8s.io/kubernetes"
			
			// Create scan target
			results, _, err := k8sScanner.Scan(ctx, 
				types.ScanTarget{
					Applications: []ftypes.Application{
						{
							Type:     ftypes.LangType(lang),
							FilePath: artifact.Name,
							Packages: []ftypes.Package{
								{
									Name:    comp.Name,     // "k8s.io/apiserver"
									Version: cpcVersion,     // "1.25.4"
								},
							},
						},
					},
				}, scanOptions)
			
			// Results contain matched CVEs
			if results != nil {
				resource, err := s.filter(ctx, types.Report{
					Results:      results,
					ArtifactName: artifact.Name,
				}, artifact)
				resources = append(resources, resource)
			}
		}
	}
	return resources, nil
}

// Node components handling
case nodeComponents:
	var nf bom.NodeInfo
	err := ms.Decode(artifact.RawResource, &nf)
	
	kubeletVersion := unifiedVersion(nf.KubeletVersion)
	lang := k8sNamespace(kubeletVersion, nodeName)
	
	// Also scan container runtime
	runtimeName, runtimeVersion := runtimeNameVersion(nf.ContainerRuntimeVersion)
	
	results, _, err := k8sScanner.Scan(ctx, 
		types.ScanTarget{
			Applications: []ftypes.Application{
				{
					Type:     ftypes.LangType(lang),
					FilePath: artifact.Name,
					Packages: []ftypes.Package{
						{
							Name:    "k8s.io/kubelet",
							Version: kubeletVersion,
						},
					},
				},
				{
					Type:     ftypes.GoBinary,
					FilePath: artifact.Name,
					Packages: []ftypes.Package{
						{
							Name:    runtimeName,        // containerd, docker
							Version: runtimeVersion,     // v1.5.2
						},
					},
				},
			},
		}, scanOptions)
}
Trivy K8s Command:
bash
# Scan cluster control plane
$ trivy k8s --report summary --scanners vuln \
  --include-namespaces kube-system

# Output:
ControlPlaneComponents/kind-control-plane (kubernetes)

Total: 3 (UNKNOWN: 0, LOW: 1, MEDIUM: 0, HIGH: 2, CRITICAL: 0)

┌────────────────┬────────────────┬──────────┬────────┬───────────────────┬──────────────────────┐
│    Library     │ Vulnerability  │ Severity │ Status │ Installed Version │ Fixed Version        │
├────────────────┼────────────────┼──────────┼────────┼───────────────────┼──────────────────────┤
│ k8s.io/etcd    │ CVE-2021-28235 │ HIGH     │ fixed  │ v3.4.0            │ v3.4.15+, v3.5.0+    │
│ k8s.io/etcd    │ CVE-2021-28169 │ HIGH     │ fixed  │ v3.4.0            │ v3.4.6+, v3.5.0+     │
│ coredns        │ CVE-2021-25737 │ LOW      │ fixed  │ 1.8.0             │ 1.8.4+               │
└────────────────┴────────────────┴──────────┴────────┴───────────────────┴──────────────────────┘
SBOM (KBOM) Generation:
JSON
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.3",
  "version": 1,
  "components": [
    {
      "bom-ref": "pkg:k8s/k8s.io%2Fkubernetes@v1.25.4",
      "type": "application",
      "name": "k8s.io/kubernetes",
      "version": "v1.25.4",
      "properties": [
        {
          "name": "k8s-cluster-name",
          "value": "prod-cluster"
        },
        {
          "name": "k8s-node",
          "value": "control-plane-1"
        }
      ]
    },
    {
      "bom-ref": "pkg:k8s/k8s.io%2Fetcd@v3.5.5",
      "type": "application",
      "name": "k8s.io/etcd",
      "version": "v3.5.5"
    },
    {
      "bom-ref": "pkg:k8s/coredns@1.9.3",
      "type": "application",
      "name": "coredns",
      "version": "1.9.3"
    },
    {
      "bom-ref": "pkg:oci/kube-apiserver@sha256:abc123",
      "type": "container",
      "name": "k8s.gcr.io/kube-apiserver",
      "version": "v1.25.4"
    }
  ]
}
III. SYFT - K8S CONTROL PLANE SCANNING
Capabilities:
Syft KHÔNG có dedicated K8s scanning, nhưng có thể:

Scan images từ K8s (qua image pull)
Go binary analysis (extract dependencies từ control plane binaries)
SBOM generation từ container images
Luồng Xử Lý:
Code
┌─────────────────────────────────────────────────────────────┐
│   Syft K8s Control Plane Handling                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. IMAGE PULL (Manual or via container runtime)           │
│     └─> Get kube-apiserver image from registry             │
│         - Use kubeconfig to authenticate                   │
│         - Pull from k8s.gcr.io or custom registry          │
│                                                             │
│  2. GOLANG BINARY CATALOGER                                │
│  ═════════════════════════════════════                     │
│  └─> syft/pkg/cataloger/golang/scan_binary.go              │
│      ├─ Load Go binary ELF header                          │
│      ├─ Extract BuildInfo section:                         │
│      │  {                                                  │
│      │    "Main": {                                        │
│      │      "Path": "k8s.io/apiserver",                   │
│      │      "Version": "v1.25.4"                           │
│      │    },                                               │
│      │    "Deps": [                                        │
│      │      {                                              │
│      │        "Path": "golang.org/x/crypto",               │
│      │        "Version": "v0.0.0-..."                      │
│      │      },                                             │
│      │      {                                              │
│      │        "Path": "golang.org/x/net",                  │
│      │        "Version": "v0.0.0-..."                      │
│      │      },                                             │
│      │      ... (100+ dependencies for kube-apiserver)     │
│      │    ],                                               │
│      │    "Go": "go1.19.2",                                │
│      │    "Settings": {                                    │
│      │      "-ldflags": "-s -w -X k8s.io/...",            │
│      │      "CGO_ENABLED": "0"                             │
│      │    }                                                │
│      │  }                                                  │
│      │                                                     │
│      ├─ Extract crypto information:                        │
│      │  ├─ Standard crypto (default)                       │
│      │  ├─ Boring crypto (if present)                      │
│      │  └─ FIPS compliance info                            │
│      │                                                     │
│      └─ Create packages for:                               │
│         ├─ Main module: k8s.io/apiserver@v1.25.4          │
│         ├─ All dependencies (golang.org/x/*)              │
│         └─ Go standard library@go1.19.2                   │
│                                                             │
│  3. FILE METADATA EXTRACTION                               │
│     └─> Hash computation, ownership, etc.                  │
│                                                             │
│  4. SBOM GENERATION                                        │
│     └─> Native or CycloneDX/SPDX format                    │
│         with complete dependency tree                      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
Syft Go Binary Analysis - Code:
Go
// syft/pkg/cataloger/golang/scan_binary.go
func scanFile(location file.Location, 
              reader unionreader.UnionReader) 
              ([]*extendedBuildInfo, error) {
	
	// Get readers for universal binaries
	readers, errs := unionreader.GetReaders(reader)
	
	var builds []*extendedBuildInfo
	for _, r := range readers {
		// Try to extract buildinfo from Go binary
		bi, err := getBuildInfo(r, location)
		if bi == nil {
			continue
		}
		
		// Extract cryptographic settings
		v, err := getCryptoInformation(r)
		// v = []string{"standard-crypto"} or {"boring-crypto"} or {"crypto/tls/fipsonly"}
		
		// Extract architecture
		arch := getGOARCH(bi.Settings)
		
		builds = append(builds, &extendedBuildInfo{
			BuildInfo:      bi,
			cryptoSettings: v,
			arch:           arch,
		})
	}
	return builds, errs
}

// Extract from binary:
// BuildInfo contains:
// - Main module (k8s.io/apiserver@v1.25.4)
// - All dependencies with exact versions
// - Go version used (go1.19.2)
// - Build settings (-ldflags, CGO_ENABLED, etc.)
Syft Output - K8s Control Plane Example:
JSON
{
  "artifacts": {
    "packages": [
      {
        "name": "k8s.io/apiserver",
        "version": "v1.25.4",
        "type": "go-module",
        "locations": [
          {
            "path": "usr/local/bin/kube-apiserver",
            "layerID": "sha256:xyz..."
          }
        ],
        "purl": "pkg:golang/k8s.io/apiserver@v1.25.4"
      },
      {
        "name": "golang.org/x/crypto",
        "version": "v0.4.0",
        "type": "go-module",
        "purl": "pkg:golang/golang.org/x/crypto@v0.4.0"
      },
      {
        "name": "golang.org/x/net",
        "version": "v0.5.0",
        "type": "go-module",
        "purl": "pkg:golang/golang.org/x/net@v0.5.0"
      },
      {
        "name": "google.golang.org/grpc",
        "version": "v1.51.0",
        "type": "go-module",
        "purl": "pkg:golang/google.golang.org/grpc@v1.51.0"
      },
      {
        "name": "etcd.io/etcd",
        "version": "v3.5.5",
        "type": "go-module",
        "purl": "pkg:golang/etcd.io/etcd@v3.5.5"
      },
      ... (100+ more dependencies)
    ],
    "relationships": [
      {
        "parent": "pkg:golang/k8s.io/apiserver@v1.25.4",
        "child": "pkg:golang/golang.org/x/crypto@v0.4.0",
        "type": "dependency"
      },
      ...
    ]
  }
}
Advantage vs Trivy:
Code
Syft:
  ✓ Complete dependency tree (100+ packages for kube-apiserver)
  ✓ Crypto configuration detection (FIPS, boring-crypto)
  ✓ Go version tracking
  ✓ All transitive dependencies

Trivy:
  ✓ Official K8s CVE database
  ✓ Version-based matching (simplified)
  ✓ More accurate for K8s-specific issues
IV. NEUVECTOR - K8S CONTROL PLANE SCANNING
Kiến Trúc:
NeuVector có K8s platform scanning nhưng KHÔNG specialized cho control plane:

Go
// controller/rpc/scanner.go
func ScanPlatform(scanner string, k8sVersion, ocVersion string, timeout time.Duration) 
                  (*share.ScanResult, error) {
	
	req := &share.ScanAppPackageRequest{}
	
	if k8sVersion != "" {
		req.Packages = append(req.Packages, 
			&share.ScanAppPackage{
				AppName:    "kubernetes",
				ModuleName: "kubernetes",
				Version:    k8sVersion,  // e.g., "1.25.4"
				FileName:   "kubernetes",
			})
		platform = "kubernetes"
		version = k8sVersion
	}
	
	result, err := client.ScanAppPackage(ctx, &req)
	// Send to scanner for vulnerability matching
	
	return result, err
}
Limitation - NeuVector:
Code
NeuVector's approach:
├─ Get K8s version from cluster info
├─ Query vulnerability database with version
├─ Return CVEs matching that version
│
├─ Limitations:
│  ├─ NO component-level scanning (all K8s treated as "kubernetes")
│  ├─ NO differentiation between apiserver vs kubelet
│  ├─ NO kube-proxy, etcd, coredns scanning
│  ├─ NO dependency resolution
│  └─ GENERIC vulnerability matching only
│
└─ Can scan:
   ├─ Container image vulnerabilities (separate)
   ├─ Running container CVEs
   ├─ CIS compliance checks
   └─ Secrets in control plane pods
Thực Tế - NeuVector K8s Scanning:
Go
// controller/cache/scan.go
func (t *scanTask) rpcScanRunning(scanner string, info *scanInfo) {
	switch info.objType {
	case share.ScanObjectType_CONTAINER:
		result, err = rpc.ScanRunning(...)  // Scan running container
	
	case share.ScanObjectType_HOST:
		result, err = rpc.ScanRunning(...)  // Scan host OS
	
	default:
		// K8s platform scanning
		cctx.k8sVersion, cctx.ocVersion = global.ORCH.GetVersion(false, false)
		result, err = rpc.ScanPlatform(scanner, 
			cctx.k8sVersion,    // "1.25.4"
			cctx.ocVersion,     // "4.13.0" if OpenShift
			scanReqTimeout)
	}
	
	// NeuVector treats entire K8s version as single "package"
	// No component-level detail
}

// Example result:
{
	"Platform": "kubernetes",
	"PlatformVersion": "1.25.4",
	"Vuls": [
		{
			"CVE": "CVE-2021-25746",
			"Severity": "HIGH",
			"Description": "kube-apiserver..."  // But no specificity
		}
	]
}
NeuVector - Cơ Chế Khác:
NeuVector focuses on different angles:

Go
// controller/nvk8sapi/nvvalidatewebhookcfg/admission/admission.go
// Image scanning at pod creation time
type AdmissionStats struct {
	CriticalVulInfo map[string]share.CLUSScannedVulInfo // Critical CVEs
	HighVulInfo     map[string]share.CLUSScannedVulInfo // High CVEs
	MediumVulInfo   map[string]share.CLUSScannedVulInfo // Medium CVEs
	LowVulInfo      []share.CLUSScannedVulInfoSimple
	SetIDPermCnt    int  // setuid permissions in image
	SecretsCnt      int  // embedded secrets
	Modules         []*share.ScanModule
}

// NeuVector scans IMAGES used by K8s pods
// For control plane: scans kube-apiserver image, etcd image, etc.
// Not the components themselves
NeuVector CIS Compliance Checks:
bash
# NeuVector supports K8s CIS benchmarks:
# - Control plane configuration checks
# - Worker node checks
# - Policy checks

# Example from nvbench/kubernetes-cis-benchmark/
[✓] 1.1.1 - Ensure that the API server pod specification file permissions are set to 644
[✗] 1.1.4 - Ensure that the API server pod specification file ownership is set to root:root
[⚠] 1.2.1 - Ensure that the API server pod is bound to 127.0.0.1
[✓] 3.1.1 - Client certificate authentication should be used for kubelet API
[✗] 3.2.2 - Ensure that the audit policy covers key security concerns
V. SO SÁNH CHI TIẾT - K8S CONTROL PLANE
Bảng So Sánh:
Code
┌─────────────────────────────────────┬──────────┬─────────┬─────────┐
│ Capability                          │NeuVector │  Trivy  │  Syft   │
├─────────────────────────────────────┼──────────┼─────────┼─────────┤
│ Detects kube-apiserver CVEs         │    ✗     │   ✓✓    │    ~    │
│                                     │          │         │ (needs  │
│                                     │          │         │ Grype)  │
│                                     │          │         │         │
│ Component-level CVE matching        │    ✗     │   ✓✓    │    ✗    │
│                                     │          │ (vuln   │         │
│                                     │          │  -list  │         │
│                                     │          │  -k8s)  │         │
│                                     │          │         │         │
│ Scans kube-proxy separately         │    ✗     │   ✓     │    ✗    │
│                                     │          │         │         │
│ Scans etcd separately               │    ✗     │   ✓     │    ✗    │
│                                     │          │         │         │
│ Scans coredns separately            │    ✗     │   ✓     │    ✗    │
│                                     │          │         │         │
│ Go dependency resolution            │    ✗     │    ✗    │   ✓✓    │
│                                     │          │         │         │
│ Detects golang crypto config        │    ✗     │    ✗    │   ✓✓    │
│ (FIPS, boring-crypto)               │          │         │         │
│                                     │          │         │         │
│ Image scanning (control plane)      │    ✓     │   ✓     │   ✓     │
│                                     │          │         │         │
│ CIS compliance checks (K8s)         │    ✓     │   ✓     │    ✗    │
│                                     │          │         │         │
│ Runtime pod monitoring              │    ✓     │    ✗    │    ✗    │
│                                     │          │         │         │
│ KBOM (Kubernetes BOM) generation    │    ✗     │   ✓✓    │    ~    │
│                                     │          │         │         │
│ Official K8s CVE database           │    ✗     │   ✓✓    │    ✗    │
│                                     │          │ (vuln   │         │
│                                     │          │  -list  │         │
│                                     │          │  -k8s)  │         │
│                                     │          │         │         │
│ Scanning speed                      │   Fast   │  Medium │  Slow   │
│                                     │ (API-    │(image   │ (full   │
│                                     │ based)   │ pull)   │ catalog)│
│                                     │          │         │         │
└────────────────────��────────────────┴──────────┴─────────┴─────────┘
VI. DETAILED WALKTHROUGH - REAL K8S CLUSTER
Scenario: Scan kube-apiserver
A. TRIVY Approach:
bash
$ trivy k8s --include-namespaces kube-system --scanners vuln

# Step 1: Connect to K8s API
# - Get K8s version: v1.25.4
# - List pods in kube-system

# Step 2: Extract control plane components
# Pod: kube-apiserver-control-plane (image: k8s.gcr.io/kube-apiserver:v1.25.4)
# Extract:
#   Name: "k8s.io/apiserver"
#   Version: "1.25.4"

# Step 3: Query vuln-list-k8s database
# Check if 1.25.4 matches any CVE entry:
# CVE-2023-3462: affects 1.25.0-1.26.7
# CVE-2023-1260: affects 1.25.0-1.26.2
# ... (component-specific)

# Step 4: Report findings
# - List all matched CVEs with severity
# - Show fixed versions

# Output:
┌──────────────────┬──────────┬──────────┐
│      CVE         │Severity  │  Fixed   │
├──────────────────┼──────────┼──────────┤
│CVE-2023-3462     │ HIGH     │ 1.26.8+  │
│CVE-2023-1260     │ MEDIUM   │ 1.26.3+  │
└──────────────────┴──────────┴──────────┘
B. SYFT Approach:
bash
# Option 1: Pull image & analyze
$ docker pull k8s.gcr.io/kube-apiserver:v1.25.4
$ syft k8s.gcr.io/kube-apiserver:v1.25.4 -o json > sbom.json

# Step 1: Extract image layers
# - Layer 1: distroless base (ca-certificates, etc.)
# Layer 2: kube-apiserver binary

# Step 2: Go binary cataloging
# - Load kube-apiserver ELF binary
# - Extract BuildInfo:
#   Main: k8s.io/apiserver@v1.25.4
#   Deps: [
#     golang.org/x/crypto@v0.4.0,
#     golang.org/x/net@v0.5.0,
#     google.golang.org/grpc@v1.51.0,
#     k8s.io/apimachinery@v0.25.4,
#     k8s.io/client-go@v0.25.4,
#     ... (100+ more)
#   ]

# Step 3: Generate SBOM
# - Complete dependency tree
# - 150-200 packages (including all transitive deps)

# Step 4: Optional - Use Grype for CVE scanning
$ grype sbom.json --output json > vulns.json

# Output: CVEs for each Go dependency
#   CVE-2023-xyz in golang.org/x/crypto
#   CVE-2023-abc in golang.org/x/net
#   ...
C. NEUVECTOR Approach:
bash
# NeuVector continuously monitors:

# Step 1: Runtime detection
# - Agent detects pod: kube-apiserver-control-plane
# - Gets image: k8s.gcr.io/kube-apiserver:v1.25.4

# Step 2: Image scanning (on-demand or scheduled)
# - Scans container image layers
# - Detects distroless base image packages
# - Finds embedded secrets (certificates)
# - Reports image-level vulnerabilities

# Step 3: Platform scanning
# - Queries K8s version: 1.25.4
# - Generic "kubernetes" CVE lookup
# - No component-specific details

# Step 4: Compliance checks
# - Runs CIS benchmark checks against control plane
# - Checks: certificate validity, API server flags, audit logging, etc.

# Step 5: Enforcement (optional)
# - If vulnerabilities found: block pod (admission control)
# - Alert on violations
# - Log to cluster events

# Output:
{
  "image": "k8s.gcr.io/kube-apiserver:v1.25.4",
  "vulnerabilities": 3,
  "critical": 0,
  "high": 1,
  "medium": 2,
  "compliance_violations": 2,
  "secrets_found": 1
}
VII. SBOM COMPARISON - CONTROL PLANE EXAMPLE
Trivy KBOM (Kubernetes BOM):
JSON
{
  "bomFormat": "CycloneDX",
  "components": [
    {
      "type": "application",
      "name": "k8s.io/apiserver",
      "version": "1.25.4",
      "purl": "pkg:k8s/k8s.io%2Fapiserver@1.25.4"
    },
    {
      "type": "application",
      "name": "k8s.io/controller-manager",
      "version": "1.25.4"
    },
    {
      "type": "application",
      "name": "k8s.io/scheduler",
      "version": "1.25.4"
    },
    {
      "type": "application",
      "name": "k8s.io/etcd",
      "version": "3.5.5"
    },
    {
      "type": "container",
      "name": "k8s.gcr.io/kube-apiserver",
      "version": "v1.25.4",
      "components": [
        {
          "type": "library",
          "name": "ca-certificates",
          "version": "20230311"
        }
        // Minimal distroless packages
      ]
    }
  ]
}
Syft Output (Go Dependencies):
JSON
{
  "artifacts": {
    "packages": [
      {
        "name": "k8s.io/apiserver",
        "version": "v1.25.4",
        "type": "go-module"
      },
      {
        "name": "k8s.io/apimachinery",
        "version": "v0.25.4",
        "type": "go-module"
      },
      {
        "name": "k8s.io/client-go",
        "version": "v0.25.4",
        "type": "go-module"
      },
      {
        "name": "golang.org/x/crypto",
        "version": "v0.4.0",
        "type": "go-module"
      },
      {
        "name": "golang.org/x/net",
        "version": "v0.5.0",
        "type": "go-module"
      },
      {
        "name": "google.golang.org/grpc",
        "version": "v1.51.0",
        "type": "go-module"
      },
      {
        "name": "etcd.io/etcd/client/v3",
        "version": "v3.5.5",
        "type": "go-module"
      },
      ... (150+ total)
    ],
    "relationships": [
      {
        "parent": "k8s.io/apiserver@v1.25.4",
        "child": "k8s.io/apimachinery@v0.25.4",
        "type": "dependency"
      },
      ... (complex dependency graph)
    ]
  }
}
VIII. RECOMMENDATIONS
Best Practice Combination:
Code
┌────────────────────────────────────────────────────────────┐
│   Comprehensive K8s Control Plane Security Stack           │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  1. TRIVY (Primary - Component-level CVE)                 │
│     ✓ Official K8s CVE database (vuln-list-k8s)           │
│     ✓ Component-specific version matching                 │
│     ✓ KBOM generation                                     │
│     $ trivy k8s --report json | jq '.Components'          │
│                                                            │
│  2. SYFT + GRYPE (Secondary - Dependency Analysis)        │
│     ✓ Complete Go dependency tree                         │
│     ✓ Transitive CVE detection                            │
│     ✓ Crypto configuration validation                     │
│     $ syft k8s.gcr.io/kube-apiserver:v1.25.4 -o json     │
│     $ grype sbom.json --output json                       │
│                                                            │
│  3. NEUVECTOR (Runtime - Image + Compliance)              │
│     ✓ Continuous image scanning                           │
│     ✓ CIS benchmark compliance checks                     │
│     ✓ Runtime pod monitoring                              │
│     ✓ Admission control enforcement                       │
│     $ kubectl get vulnerabilities -n kube-system          │
│                                                            │
└────────────────────────────────────────────────────────────┘
Command Examples:
bash
# 1. TRIVY - K8s Control Plane Scan
trivy k8s --scanners vuln \
  --include-namespaces kube-system \
  --report json > k8s-vulns.json

# 2. SYFT - Extract Control Plane Dependencies
syft k8s.gcr.io/kube-apiserver:$(kubectl version -o jsonpath='{.serverVersion.gitVersion}') \
  -o cyclonedx > kube-apiserver-sbom.json

syft k8s.gcr.io/etcd:3.5.5 -o cyclonedx > etcd-sbom.json

# 3. GRYPE - Scan Generated SBOMs
grype kube-apiserver-sbom.json --output json > kube-apiserver-vulns.json

# 4. Combine Results
cat k8s-vulns.json | jq '.Components[] | select(.Type=="application") | {name, version, vulnerabilities}' \
  > control-plane-summary.json
SUMMARY
Aspect	Trivy	Syft	NeuVector
K8s Component CVE Scanning	✓✓ Excellent	~ Medium (w/ Grype)	✗ Limited
Dependency Resolution	✗	✓✓ Excellent	✗
Crypto Config Detection	✗	✓ Good	✗
KBOM Generation	✓✓ Excellent	~ Good	✗
Official K8s Database	✓✓ Yes	✗ No	✗ No
Image Scanning	✓ Good	✓ Good	✓ Good
CIS Compliance	✓ Good	✗	✓ Good
Runtime Monitoring	✗	✗	✓ Good
Kết luận: Trivy is the best choice for K8s control plane CVE scanning!

