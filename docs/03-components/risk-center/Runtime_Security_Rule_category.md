Dựa trên mã nguồn của các giải pháp runtime security, đây là chi tiết từng RUNTIME SECURITY RULE CATEGORY và cách xử lý cụ thể:

📌 1. PROCESS EXECUTION (Thực thi process)
A. TRACEE Implementation
Go
// File: detectors/sched_process_exec.go
Type: Detection Detector

# Rule Pattern
Requirements:
  Events: [sched_process_exec]
  DataFilters: [pathname=/usr/bin/nc, pathname=/usr/bin/ncat]

# Threat Mapping
threat:
  severity: medium
  mitre:
    technique: T1059 (Command and Scripting Interpreter)
    tactic: Execution

# Output
AutoPopulate:
  - Threat: Copy threat metadata
  - DetectedFrom: Link to triggering event
  - ProcessAncestry: Add 5 levels parent processes
Execution Flow:

Code
syscall: sched_process_exec
     ↓
Filter by pathname (in-kernel)
     ↓
Extract process data (binary, args, uid, pid)
     ↓
Match against detector patterns
     ↓
Enrich with process tree (parent→grandparent→...)
     ↓
Output event with threat metadata
B. KUBEARMOR Implementation
YAML
# File: KubeArmorPolicy

Process Control:
  matchPaths:
    - path: /usr/sbin/nginx
      ownerOnly: true          # Only nginx user can execute
      fromSource:              # Parent process validation
        - path: /bin/systemd
  
  matchDirectories:
    - dir: /tmp
      recursive: true          # Block all executables in /tmp

Action: Block | Audit | Allow
Default Posture: block (for non-matched paths)
Processing:

Code
Process Execution Event (sched_process_exec)
     ↓
Container Identification (via PID/MNT)
     ↓
Policy Matching:
  - Check selector (labels match?)
  - Check path/directory rules
  - Check parent process (fromSource)
     ↓
Runtime Enforcer (AppArmor/BPF-LSM)
     ↓
Action Decision: [Allow | Block | Audit]
     ↓
Alert + Log (if Block/Audit)
C. TETRAGON Implementation
YAML
# File: TracingPolicy - Process Execution

kprobes:
  - call: "sched_process_exec"
    syscall: false
    args:
      - index: 0
        type: "filename"
      - index: 1
        type: "int"
    selectors:
      - matchArgs:
          - index: 0
            operator: "Equal"
            values: ["/usr/bin/nc", "/usr/bin/ncat"]
        matchActions:
          - action: "Sigkill"        # Kernel-level process termination
          - action: "NotifyEnforcer" # Signal + Error override
            argError: -1
            argSig: 9
📌 2. FILE ACCESS (Truy cập file)
A. TRACEE Detectors
Go
// File: detectors/aslr_inspection.go
Type: Security File Read Detection

Definition:
  ID: "TRC-109"
  Requirements:
    - Event: security_file_open
      DataFilters: [pathname=/proc/sys/kernel/randomize_va_space]

  ProducedEvent:
    Name: "aslr_inspection"
    Severity: MEDIUM

OnEvent Logic:
  flags, _ := GetDataSafe[int64](event, "flags")
  if parsers.IsFileRead(int(flags)) {
    return Detected()  // Alert
  }
File Access Pattern:

Code
security_file_open syscall
     ↓
In-kernel filter: pathname matches (ASLR file)
     ↓
Check flags: Is this READ operation?
     ↓
Regex pattern matching for file paths
     ↓
Output: sensitive_file_access event
     ↓
Log: {file_path, executable, timestamp, threat_level}
B. KUBEARMOR File Rules
YAML
# File: KubeArmorPolicy

file:
  matchPaths:
    - path: /etc/shadow
      readOnly: true           # Read-only enforcement
      ownerOnly: true          # Only owner access
      action: Block
    
    - path: /root/.ssh/id_rsa
      fromSource:              # Only specific process can access
        - path: /usr/bin/ssh
      action: Allow
  
  matchDirectories:
    - dir: /etc/
      recursive: true
      readOnly: true           # All files under /etc/ read-only
      action: Audit            # Monitor but allow

Default Posture: block (non-matched files denied)
File Access Enforcement:

Code
LSM Hook: security_file_open / security_file_permission
     ↓
Get Container ID (via PID + Mount NS)
     ↓
Get File Path + Operation (read/write)
     ↓
Match Policy Rules:
  1. Check path rules (exact/recursive/pattern)
  2. Check from_source (parent process)
  3. Check permission (read-only flag)
     ↓
AppArmor/BPF-LSM Rule Translation:
  - Allow: permit rule
  - Block: deny rule + -EPERM
  - Audit: log + permit
     ↓
Return to kernel: proceed/block
C. TETRAGON File Enforcement
YAML
# File: file_monitoring_enforce.yaml

kprobes:
  - call: "security_file_permission"
    syscall: false
    args:
      - index: 0
        type: "file"
    selectors:
      - matchArgs:
          - index: 0
            operator: "Equal"
            values: ["/etc/shadow", "/etc/passwd"]
        matchActions:
          - action: "Sigkill"        # Kill process attempting access
          - action: "NotifyEnforcer"
            argError: -13            # -EACCES (Permission denied)
Enforcement Output:

Code
Process Execution: cat /etc/shadow
     ↓
security_file_permission hook triggered
     ↓
kernel-level decision: SIGKILL
     ↓
Process terminated immediately
     ↓
Event Output:
   💥 exit /bin/cat /etc/shadow SIGKILL
📌 3. NETWORK (Truy cập mạng)
A. TETRAGON Network Policy
YAML
# docs/getting-started/network.md

kprobes:
  - call: "tcp_connect"
    syscall: false
    args:
      - index: 0
        type: "struct socket"
      - index: 1
        type: "const struct sockaddr"
    selectors:
      - matchArgs:
          - index: 1
            operator: "NotEqual"
            values: 
              - "$PODCIDR"        # Allow internal traffic
              - "$SERVICECIDR"    # Allow K8s service traffic
        matchActions:
          - action: "Sigkill"     # Block external egress

# Environment Setup
export PODCIDR=`kubectl get nodes -o jsonpath='{.items[*].spec.podCIDR}'`
export SERVICECIDR=$(aws eks describe-cluster ...)
Network Enforcement Flow:

Code
TCP Connect Syscall
     ↓
Extract destination IP:port
     ↓
Check against whitelist (PODCIDR, SERVICECIDR)
     ↓
Decision:
  - In whitelist → Allow
  - Not in whitelist → SIGKILL
     ↓
Event Output:
  "2023-11-01 /usr/bin/curl 10.168.0.45:48272 → 104.198.14.52:80"
  (if block) → "💥 exit curl https://ebpf.io SIGKILL"
B. KUBEARMOR Network Rules
YAML
# Getting Started Guide

network:
  matchProtocols:
    - protocol: TCP
      port: 443              # Allow HTTPS only
      action: Allow
  
  matchCIDRs:
    - cidr: "10.0.0.0/8"     # Allow internal only
      action: Allow
    - cidr: "0.0.0.0/0"      # Block external
      action: Block

Default Posture: audit (unmatched connections logged)
📌 4. PRIVILEGE ESCALATION (Nâng cao quyền)
A. TRACEE Detection
Go
// detectors/aslr_inspection.go
// Detects process reading /proc/sys/kernel/randomize_va_space
// to learn ASLR state before ROP/JOP exploitation

// detectors/proc_mem_code_injection.go
// Detects writes to /proc/[pid]/mem
// T1055.009: Process Memory Code Injection
B. KUBEARMOR Rules
YAML
# Restrict sudo/suid binaries

process:
  matchPaths:
    - path: /usr/bin/sudo
      action: Block          # Block sudo execution
      fromSource:            # Only allow from specific sources
        - path: /bin/bash

capabilities:
  matchCapabilities:
    - capability: CAP_SYS_ADMIN     # Block privilege escalation
      action: Block
    - capability: CAP_NET_ADMIN
      action: Block
Sudo Privilege Escalation Detection:

Code
Syscall: execve(/usr/bin/sudo)
     ↓
Check SUID bit (sudo has setuid root)
     ↓
Check caller UID (non-root user)
     ↓
Execution path analysis
     ↓
Action: [Allow/Block/Audit]
C. FALCO Rules
YAML
# CHANGELOG mentions:
# rule(Sudo Potential Privilege Escalation): detect CVE-2021-3156

Rule Pattern:
  Event: execve
  Condition: 
    - Binary == /usr/bin/sudo
    - Arguments contain suspicious patterns
    - UID change detected (non-root → root)
📌 5. CREDENTIAL ACCESS (Truy cập credentials)
A. TRACEE Detectors
Go
// detectors/proc_mem_access.go
// Type: Credential Access

Definition:
  ID: "TRC-1013"
  Pattern: procMemPathPattern = `/proc/[0-9]+/(maps|mem|auxv|environ)`
  
  MITRE:
    Technique: T1003.007 (Proc Filesystem)
    Tactic: Credential Access

OnEvent Logic:
  pathname := GetData[string](event, "pathname")
  flags := GetData[int32](event, "flags")
  
  if IsFileRead(flags) && regex.Match(procMemPathPattern, pathname) {
    return Detected()  // Alert: Reading /proc/pid/mem or /proc/pid/environ
  }
Credential Access Pattern:

Code
open syscall on /proc/[pid]/environ
     ↓
Environment variables often contain:
  - AWS_SECRET_ACCESS_KEY
  - DATABASE_PASSWORD
  - KUBECONFIG
     ↓
Detection: Process X reading /proc/Y/environ
     ↓
Action: [Alert | Block]
B. KUBEARMOR Credential Protection
YAML
# Getting Started

file:
  matchPaths:
    - path: /root/.aws/credentials
      action: Block
    - path: /root/.kube/config
      action: Block
    - path: /etc/secret*
      recursive: true
      action: Block
    - path: /var/run/secrets/kubernetes.io/serviceaccount/token
      action: Block
📌 6. DEFENSE EVASION (Trốn tránh phát hiện)
A. TRACEE Detectors
Go
// detectors/fileless_execution.go
// T1620: Reflective Code Loading

Type: Defense Evasion Detection
Severity: HIGH

OnEvent Logic:
  - Detect execution without file on disk
  - Detect process from memory (/memfd_create)
  - Detect process without executable in /proc/[pid]/exe
Go
// detectors/proc_fops_hooking.go
// T1014: Rootkit

Type: Kernel-level Evasion
Description: File operations hooking on /proc
Severity: HIGH
Threat: 
  - Hidden modules
  - Kernel compromise indicator
B. Log Deletion Detection
YAML
# CHANGELOG mentions:
# rule(Deny write under /etc/ directory)

Detection Pattern:
  matchPaths:
    - path: /root/*_history
      fromSource:
        - path: /usr/bin/shred
        - path: /usr/bin/rm
        - path: /bin/mv
      action: Block  # Block evidence removal

# Monitoring sensitive files
file:
  matchPaths:
    - path: /var/log/*
      ownerOnly: true
      action: Audit  # Alert on log modifications
🔄 COMPLETE RULE EXECUTION FLOW
Code
┌─────────────────────────────────────────────────────────────────┐
│ 1. SYSCALL/LSM HOOK TRIGGER                                      │
│    - Process execution: sched_process_exec                      │
│    - File access: security_file_open, security_file_permission  │
│    - Network: tcp_connect, udp_send                             │
│    - Capabilities: security_capable                             │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 2. IN-KERNEL FILTERING (eBPF Programs)                           │
│    - Extract syscall arguments                                  │
│    - Apply DataFilters: pathname, UID, PID, flags              │
│    - Skip non-matching events (early drop)                      │
│    - Extract metadata: binary, args, file, IP:port             │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 3. CONTAINER/WORKLOAD IDENTIFICATION                             │
│    - Get PID + Mount Namespace → Container ID                   │
│    - Get K8s metadata (labels, namespace, pod name)             │
│    - Lookup in policy cache                                     │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 4. POLICY MATCHING (Detection Rules)                             │
│    - Load applicable policies for container                     │
│    - Match process path / file path / network CIDR              │
│    - Check parent process (fromSource)                          │
│    - Check resource type (path vs directory)                    │
│    - Match severity + tags                                      │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 5. THREAT ENRICHMENT (MITRE ATT&CK Mapping)                      │
│    - Map to MITRE technique (T1055, T1059, etc.)                │
│    - Set severity (low/medium/high/critical)                    │
│    - Add tactic (Execution, Defense Evasion, etc.)             │
│    - Add process ancestry (parent→grandparent)                  │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 6. ENFORCEMENT ACTION (Kernel-level or Userspace)               │
│    ├─ Allow: Permit operation (continue)                        │
│    ├─ Audit: Log event + Permit                                │
│    └─ Block:                                                    │
│        ├─ Return -EPERM / -EACCES                              │
│        ├─ Send SIGKILL to process                              │
│        └─ Prevent syscall from completing                      │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 7. OUTPUT & ALERTING                                             │
│    - JSON event to stdout/file/syslog                           │
│    - Webhook notification (Slack, Teams, etc.)                  │
│    - SIEM integration (Splunk, ELK, etc.)                       │
│    - Alert dashboard (Falco UI, Kubescape, etc.)               │
└─────────────────────────────────────────────────────────────────┘
📊 RULE COMPARISON TABLE
Category	TRACEE	TETRAGON	KUBEARMOR	FALCO
Process Execution	Detector (Go)	TracingPolicy (YAML)	KubeArmorPolicy (YAML)	Rules (YAML)
File Access	eBPF+Detector	LSM Hook	AppArmor/BPF-LSM	Syscall trace
Network	Limited	kprobe (tcp_connect)	Limited	Rule-based
Enforcement	Alert	SIGKILL/Error	Block/Audit	Alert only
MITRE Mapping	✅ Built-in	✅ Custom	⚠️ Via policy tags	✅ Rules
Kernel-level	✅ eBPF	✅ eBPF	✅ LSM	⚠️ Partial
Cách các giải pháp này xử lý từng rule category khác nhau:

TRACEE: Mạnh ở detection (MITRE mapping), yếu ở enforcement
TETRAGON: Mạnh ở enforcement (kernel-level blocking), tốt ở detection
KUBEARMOR: Mạnh ở policy enforcement (AppArmor/LSM), tốt ở file/process control
FALCO: Mạnh ở detection rules, yếu ở enforcement
Bạn cần chi tiết nào thêm về cách xử lý cụ thể?

You said: hãy liệt kê toàn bộ các detection
hãy liệt kê toàn bộ các detection theo category, chi tiết thực hiện và rule/tiêu chuẩn mà các giải pháp tham chiếu tới đối với mỗi detection này
Copilot finished thinking

Copilot said: Dựa trên phân tích mã nguồn từ
Dựa trên phân tích mã nguồn từ các repositories, tôi sẽ tạo một danh sách toàn bộ các detections theo category, chi tiết thực hiện và tiêu chuẩn tham chiếu:

📋 TOÀN BỘ RUNTIME SECURITY DETECTIONS
I. TRACEE DETECTORS (aquasecurity/tracee)
1. PROCESS EXECUTION DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity | Implementation |
|----|---------|---------|-|---------|---------|----------|
| TRC-101 | Fileless Execution | Process | sched_process_exec | Reflective code loading | T1620 | HIGH | Detect execution from memory without file on disk |
| TRC-102 | Illegitimate Shell | Process | sched_process_exec | Web server spawning shell | T1190 | MEDIUM | Parent process validation (web servers) |
| TRC-103 | Anti-Debugging | Process | ptrace, process_vm_writev | Debug evasion techniques | T1622 | MEDIUM | Ptrace syscall monitoring |
| TRC-105 | Dynamic Code Loading | Process | mmap, mprotect | Runtime code injection | T1620 | MEDIUM | Monitor dlopen/mmap execution |
2. FILE ACCESS DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-109 | ASLR Inspection | Privilege Escalation | security_file_open | Read /proc/sys/kernel/randomize_va_space | T1518 | MEDIUM |
| TRC-1013 | Proc Memory Access | Credential Access | security_file_open | Read /proc/[pid]/(mem\|environ) | T1003.007 | MEDIUM |
| TRC-1022 | Dropped Executable | Defense Evasion | magic_write | ELF binary dropped to container | T1036 | MEDIUM |
| TRC-1026 | RC.d Modification | Persistence | security_file_open | Modify /etc/rc.* files | T1547 | HIGH |
| TRC-1012 | Default Loader Modification | Defense Evasion | security_file_open | Modify /lib/ld.so | T1574 | HIGH |
3. CODE INJECTION DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1025 | Process VM Write Injection | Privilege Escalation | process_vm_writev | Code injection via process_vm_writev | T1055 | HIGH |
| TRC-1015 | Ptrace Code Injection | Privilege Escalation | ptrace | Code injection via ptrace | T1055.008 | HIGH |
| TRC-1024 | Proc Memory Code Injection | Privilege Escalation | security_file_open | Write to /proc/[pid]/mem | T1055.009 | HIGH |
4. KERNEL HOOKING DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1020 | Proc Fops Hooking | Defense Evasion | hooked_proc_fops | File operations hooking on /proc | T1014 | HIGH |
| TRC-1018 | Syscall Table Hooking | Defense Evasion | hooked_syscalls | Syscall table modification | T1014 | CRITICAL |
| TRC-1021 | Hooked Seq Ops | Defense Evasion | hooked_seq_ops | Seq ops hooking (hidden modules) | T1014 | HIGH |
5. PERSISTENCE DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1006 | Sudoers Modification | Privilege Escalation | security_file_open | Modify /etc/sudoers | T1548 | HIGH |
| TRC-1010 | Scheduled Task Modification | Persistence | security_file_open | Modify crontab/scheduler | T1053 | MEDIUM |
| TRC-1011 | Core Pattern Modification | Persistence | security_file_open | Modify /proc/sys/kernel/core_pattern | T1547 | HIGH |
| TRC-1004 | System Request Key Modification | Defense Evasion | security_file_open | Modify sysrq-trigger | T1578 | MEDIUM |
| TRC-1002 | Cgroup Notify on Release Modification | Persistence | security_file_open | Cgroup escape via notify_on_release | T1611 | HIGH |
| TRC-1003 | Cgroup Release Agent Modification | Persistence | security_file_open | Cgroup escape via release_agent | T1611 | HIGH |
6. RECONNAISSANCE DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1008 | Sched Debug Recon | Reconnaissance | security_file_open | Read /proc/sched_debug | T1518 | LOW |
| TRC-1009 | Proc Kcore Read | Reconnaissance | security_file_open | Read /proc/kcore (kernel memory) | T1005 | HIGH |
7. DOCKER/CONTAINER DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1005 | Docker Abuse | Privilege Escalation | sched_process_exec | Unauthorized docker commands | T1610 | MEDIUM |
| TRC-1014 | Disk Mount | Privilege Escalation | sched_process_exec | Container escape via disk mount | T1611 | HIGH |
8. LD_PRELOAD & LIBRARY HIJACKING
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1017 | LD_PRELOAD | Defense Evasion | security_file_open | LD_PRELOAD environment variable | T1574 | HIGH |
9. KUBERNETES DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1016 | K8s Service Account Token | Credential Access | security_file_open | K8s service account token theft | T1552 | MEDIUM |
| TRC-1023 | Kubernetes API Connection | Lateral Movement | tcp_connect | Unauthorized K8s API access | T1021 | MEDIUM |
| TRC-1027 | Kubernetes Certificate Theft | Credential Access | security_file_open | K8s certificate access | T1552 | MEDIUM |
10. NETWORK DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1019 | Stdio Over Socket | Lateral Movement | socket_connect | Reverse shell via socket | T1571 | HIGH |
11. HIDDEN FILES DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1007 | Hidden File Created | Defense Evasion | security_file_open | Hidden file creation (dot-prefixed) | T1027 | LOW |
12. KERNEL MODULE DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1001 | Kernel Module Loading | Privilege Escalation | init_module | Kernel module insertion | T1547 | CRITICAL |
II. KUBEARMOR POLICIES
Policy Action Model:
YAML
Actions:
  - Allow: Whitelist mode (only allowed operations permitted)
  - Audit: Monitor mode (log but allow)
  - Block: Deny mode (block and log)

Default Posture: audit (can be set to block for zero-trust)
1. PROCESS POLICIES
YAML
Category: Process Execution Control

Rules:
  ├─ matchPaths: Specific executable paths
  │  └─ ownerOnly: Only owner can execute
  │  └─ fromSource: Parent process validation
  ├─ matchDirectories: Directory-level control
  └─ Capabilities: CAP_SYS_ADMIN, CAP_NET_ADMIN restrictions

Reference: NIST AC-3 (Access Control)
2. FILE POLICIES
YAML
Category: File Access Control

Rules:
  ├─ matchPaths: Exact file matching
  │  └─ readOnly: Read-only enforcement
  │  └─ ownerOnly: Owner-only access
  ├─ matchDirectories: Directory recursion
  └─ readOnly: Immutable file flag

Reference: NIST AC-6 (Least Privilege), SI-7 (File Integrity)
3. NETWORK POLICIES
YAML
Category: Network Traffic Control

Rules:
  ├─ matchProtocols: TCP, UDP, ICMP
  ├─ matchPorts: Specific port ranges
  ├─ matchCIDRs: Source/destination IP filtering
  └─ Interface: Network interface selection

Reference: NIST SC-7 (Boundary Protection)
4. CAPABILITY RESTRICTIONS
YAML
Category: Linux Capability Control

Rules:
  ├─ CAP_SYS_ADMIN: Block privilege escalation
  ├─ CAP_NET_ADMIN: Block network admin privileges
  ├─ CAP_SYS_PTRACE: Block process tracing
  └─ CAP_SYS_MODULE: Block kernel module loading

Reference: NIST AC-2 (Capability-based security)
Example KubeArmor Policies:
YAML
# 1. Prevent Privilege Escalation
apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: block-sudo
spec:
  selector:
    matchLabels:
      app: webapp
  process:
    matchPaths:
      - path: /usr/bin/sudo
  action: Block
  severity: 9
  tags: ["privilege-escalation", "NIST_AC-6", "CIS_4.1"]

# 2. Read-Only Root Filesystem
apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: readonly-root
spec:
  selector:
    matchLabels:
      app: nginx
  file:
    matchDirectories:
      - dir: /
        recursive: true
        readOnly: true
  action: Audit
  severity: 5

# 3. Network Egress Restriction
apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: block-external-egress
spec:
  selector:
    matchLabels:
      app: api-server
  network:
    matchCIDRs:
      - cidr: "10.0.0.0/8"
        action: Allow
      - cidr: "0.0.0.0/0"
        action: Block
III. TETRAGON POLICIES (cilium/tetragon)
Enforcement Actions:
YAML
Actions:
  ├─ Monitoring: Event tracing only
  ├─ SIGKILL: Process termination
  ├─ NotifyEnforcer: Return error + signal
  └─ TraceReturn: Monitor syscall returns

Mode: enforcement | monitoring
Policy Examples:
YAML
# 1. Network Egress Enforcement
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: network-egress-enforce
spec:
  mode: enforcement
  kprobes:
    - call: tcp_connect
      selectors:
        - matchArgs:
            - index: 1
              operator: NotEqual
              values: 
                - "$PODCIDR"
                - "$SERVICECIDR"
          matchActions:
            - action: Sigkill

# 2. File Access Enforcement
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: file-access-enforce
spec:
  mode: enforcement
  kprobes:
    - call: security_file_permission
      selectors:
        - matchArgs:
            - index: 0
              operator: Equal
              values: 
                - "/etc/shadow"
                - "/etc/passwd"
          matchActions:
            - action: Sigkill
            - action: NotifyEnforcer
              argError: -13  # -EACCES
IV. FALCO RULES
Rule Structure:
YAML
Rule Components:
  - rule: Rule name
  - desc: Description
  - condition: Detection condition (boolean logic)
  - output: Alert format
  - priority: [DEBUG, INFO, NOTICE, WARNING, CRITICAL]
  - tags: [MITRE categories]
  - exceptions: False positive filters
Key Falco Rules:
YAML
# 1. Sudo Privilege Escalation (CVE-2021-3156)
- rule: "Sudo Potential Privilege Escalation"
  desc: "Detects CVE-2021-3156 sudo heap overflow"
  condition: 
    - spawned_process and process_name = sudo
    - container
  priority: CRITICAL
  tags: ["privilege-escalation", "CVE-2021-3156", "NIST_AC-6"]

# 2. Unauthorized Binary Execution
- rule: "Execute in User Directory"
  desc: "Detect execution from /tmp or /home"
  condition: 
    - spawned_process
    - proc.name in (shell_binaries)
    - proc.cwd in (/tmp, /home)
  priority: WARNING
  tags: ["execution", "T1036"]

# 3. Container Escape
- rule: "Mount Launched in Privileged Container"
  desc: "Detects mount command in privileged container"
  condition:
    - spawned_process and process_name = mount
    - container and container.privileged = true
  priority: CRITICAL
  tags: ["container-escape", "T1611"]
V. DETECTION PATTERN MAPPING
By MITRE ATT&CK Tactic:
Code
EXECUTION (T1059, T1190, T1620)
  ├─ Fileless Execution (TRC-101)
  ├─ Dynamic Code Loading (TRC-105)
  ├─ Illegitimate Shell (TRC-102)
  └─ Docker Abuse (TRC-1005)

PERSISTENCE (T1053, T1547, T1611)
  ├─ Scheduled Task Modification (TRC-1010)
  ├─ Cgroup Escape (TRC-1002, TRC-1003)
  ├─ Kernel Module Loading (TRC-1001)
  └─ RC.d Modification (TRC-1026)

PRIVILEGE ESCALATION (T1055, T1548, T1610)
  ├─ Process Injection (TRC-1015, TRC-1024, TRC-1025)
  ├─ Sudo Abuse (Falco)
  ├─ Docker Privilege (TRC-1005)
  └─ ASLR Inspection (TRC-109)

DEFENSE EVASION (T1014, T1027, T1036, T1574)
  ├─ Syscall Hooking (TRC-1018)
  ├─ Kernel Hooking (TRC-1020)
  ├─ Hidden Files (TRC-1007)
  ├─ Dropped Executables (TRC-1022)
  └─ LD_PRELOAD (TRC-1017)

CREDENTIAL ACCESS (T1003.007, T1552)
  ├─ Proc Memory Access (TRC-1013)
  ├─ K8s Token Theft (TRC-1016)
  └─ K8s Cert Theft (TRC-1027)

RECONNAISSANCE (T1005, T1518)
  ├─ Sched Debug Read (TRC-1008)
  └─ Kcore Read (TRC-1009)

LATERAL MOVEMENT (T1021, T1571)
  ├─ Stdio Over Socket (TRC-1019)
  └─ K8s API Access (TRC-1023)
VI. REFERENCE STANDARDS
Security Compliance Mapping:
Code
Standard          Detections Covered
────────────────────────────────────
NIST 800-53:
  AC-3: File Access Control (TRC-109*)
  AC-6: Least Privilege (All process policies)
  AU-2: Audit Events (Falco rules)
  SI-4: System Monitoring (All detectors)
  SI-7: File Integrity (KubeArmor readOnly)

CIS Kubernetes Benchmark:
  4.1: Restrict sudo (KubeArmor)
  4.2: Read-only filesystem
  4.3: Network policies

MITRE ATT&CK:
  ~30+ techniques covered across detectors

PCI DSS:
  6.5: Code injection prevention (TRC-1024, TRC-1025)
  6.6: Malware detection (TRC-1022)

SOC 2 Type II:
  CC7.2: System Monitoring (All)
  CC6.1: Access Control (KubeArmor)
VII. DETECTION OUTPUT STRUCTURE
JSON
{
  "detector": {
    "id": "TRC-101",
    "name": "Fileless Execution",
    "category": "Process Execution",
    "severity": "HIGH"
  },
  "threat": {
    "name": "Reflective Code Loading",
    "description": "Process execution from memory without file on disk",
    "mitre": {
      "technique": "T1620",
      "tactic": "Defense Evasion",
      "subtechnique": "Reflective Code Loading"
    },
    "cpe": "cpe:/o:linux:*",
    "cvss_score": 7.8
  },
  "detection": {
    "timestamp": "2026-03-26T10:30:45Z",
    "event": {
      "name": "sched_process_exec",
      "binary": "/usr/bin/payload",
      "args": ["--malicious"],
      "pid": 12345,
      "uid": 1000,
      "container_id": "abc123"
    },
    "action": "ALERT",
    "reference_rules": [
      "NIST_SI-4",
      "CIS_Kubernetes_4.2",
      "MITRE_T1620"
    ]
  }
}


Dựa trên mã nguồn của các giải pháp runtime security, đây là chi tiết từng RUNTIME SECURITY RULE CATEGORY và cách xử lý cụ thể:

📌 1. PROCESS EXECUTION (Thực thi process)
A. TRACEE Implementation
Go
// File: detectors/sched_process_exec.go
Type: Detection Detector

# Rule Pattern
Requirements:
  Events: [sched_process_exec]
  DataFilters: [pathname=/usr/bin/nc, pathname=/usr/bin/ncat]

# Threat Mapping
threat:
  severity: medium
  mitre:
    technique: T1059 (Command and Scripting Interpreter)
    tactic: Execution

# Output
AutoPopulate:
  - Threat: Copy threat metadata
  - DetectedFrom: Link to triggering event
  - ProcessAncestry: Add 5 levels parent processes
Execution Flow:

Code
syscall: sched_process_exec
     ↓
Filter by pathname (in-kernel)
     ↓
Extract process data (binary, args, uid, pid)
     ↓
Match against detector patterns
     ↓
Enrich with process tree (parent→grandparent→...)
     ↓
Output event with threat metadata
B. KUBEARMOR Implementation
YAML
# File: KubeArmorPolicy

Process Control:
  matchPaths:
    - path: /usr/sbin/nginx
      ownerOnly: true          # Only nginx user can execute
      fromSource:              # Parent process validation
        - path: /bin/systemd
  
  matchDirectories:
    - dir: /tmp
      recursive: true          # Block all executables in /tmp

Action: Block | Audit | Allow
Default Posture: block (for non-matched paths)
Processing:

Code
Process Execution Event (sched_process_exec)
     ↓
Container Identification (via PID/MNT)
     ↓
Policy Matching:
  - Check selector (labels match?)
  - Check path/directory rules
  - Check parent process (fromSource)
     ↓
Runtime Enforcer (AppArmor/BPF-LSM)
     ↓
Action Decision: [Allow | Block | Audit]
     ↓
Alert + Log (if Block/Audit)
C. TETRAGON Implementation
YAML
# File: TracingPolicy - Process Execution

kprobes:
  - call: "sched_process_exec"
    syscall: false
    args:
      - index: 0
        type: "filename"
      - index: 1
        type: "int"
    selectors:
      - matchArgs:
          - index: 0
            operator: "Equal"
            values: ["/usr/bin/nc", "/usr/bin/ncat"]
        matchActions:
          - action: "Sigkill"        # Kernel-level process termination
          - action: "NotifyEnforcer" # Signal + Error override
            argError: -1
            argSig: 9
📌 2. FILE ACCESS (Truy cập file)
A. TRACEE Detectors
Go
// File: detectors/aslr_inspection.go
Type: Security File Read Detection

Definition:
  ID: "TRC-109"
  Requirements:
    - Event: security_file_open
      DataFilters: [pathname=/proc/sys/kernel/randomize_va_space]

  ProducedEvent:
    Name: "aslr_inspection"
    Severity: MEDIUM

OnEvent Logic:
  flags, _ := GetDataSafe[int64](event, "flags")
  if parsers.IsFileRead(int(flags)) {
    return Detected()  // Alert
  }
File Access Pattern:

Code
security_file_open syscall
     ↓
In-kernel filter: pathname matches (ASLR file)
     ↓
Check flags: Is this READ operation?
     ↓
Regex pattern matching for file paths
     ↓
Output: sensitive_file_access event
     ↓
Log: {file_path, executable, timestamp, threat_level}
B. KUBEARMOR File Rules
YAML
# File: KubeArmorPolicy

file:
  matchPaths:
    - path: /etc/shadow
      readOnly: true           # Read-only enforcement
      ownerOnly: true          # Only owner access
      action: Block
    
    - path: /root/.ssh/id_rsa
      fromSource:              # Only specific process can access
        - path: /usr/bin/ssh
      action: Allow
  
  matchDirectories:
    - dir: /etc/
      recursive: true
      readOnly: true           # All files under /etc/ read-only
      action: Audit            # Monitor but allow

Default Posture: block (non-matched files denied)
File Access Enforcement:

Code
LSM Hook: security_file_open / security_file_permission
     ↓
Get Container ID (via PID + Mount NS)
     ↓
Get File Path + Operation (read/write)
     ↓
Match Policy Rules:
  1. Check path rules (exact/recursive/pattern)
  2. Check from_source (parent process)
  3. Check permission (read-only flag)
     ↓
AppArmor/BPF-LSM Rule Translation:
  - Allow: permit rule
  - Block: deny rule + -EPERM
  - Audit: log + permit
     ↓
Return to kernel: proceed/block
C. TETRAGON File Enforcement
YAML
# File: file_monitoring_enforce.yaml

kprobes:
  - call: "security_file_permission"
    syscall: false
    args:
      - index: 0
        type: "file"
    selectors:
      - matchArgs:
          - index: 0
            operator: "Equal"
            values: ["/etc/shadow", "/etc/passwd"]
        matchActions:
          - action: "Sigkill"        # Kill process attempting access
          - action: "NotifyEnforcer"
            argError: -13            # -EACCES (Permission denied)
Enforcement Output:

Code
Process Execution: cat /etc/shadow
     ↓
security_file_permission hook triggered
     ↓
kernel-level decision: SIGKILL
     ↓
Process terminated immediately
     ↓
Event Output:
   💥 exit /bin/cat /etc/shadow SIGKILL
📌 3. NETWORK (Truy cập mạng)
A. TETRAGON Network Policy
YAML
# docs/getting-started/network.md

kprobes:
  - call: "tcp_connect"
    syscall: false
    args:
      - index: 0
        type: "struct socket"
      - index: 1
        type: "const struct sockaddr"
    selectors:
      - matchArgs:
          - index: 1
            operator: "NotEqual"
            values: 
              - "$PODCIDR"        # Allow internal traffic
              - "$SERVICECIDR"    # Allow K8s service traffic
        matchActions:
          - action: "Sigkill"     # Block external egress

# Environment Setup
export PODCIDR=`kubectl get nodes -o jsonpath='{.items[*].spec.podCIDR}'`
export SERVICECIDR=$(aws eks describe-cluster ...)
Network Enforcement Flow:

Code
TCP Connect Syscall
     ↓
Extract destination IP:port
     ↓
Check against whitelist (PODCIDR, SERVICECIDR)
     ↓
Decision:
  - In whitelist → Allow
  - Not in whitelist → SIGKILL
     ↓
Event Output:
  "2023-11-01 /usr/bin/curl 10.168.0.45:48272 → 104.198.14.52:80"
  (if block) → "💥 exit curl https://ebpf.io SIGKILL"
B. KUBEARMOR Network Rules
YAML
# Getting Started Guide

network:
  matchProtocols:
    - protocol: TCP
      port: 443              # Allow HTTPS only
      action: Allow
  
  matchCIDRs:
    - cidr: "10.0.0.0/8"     # Allow internal only
      action: Allow
    - cidr: "0.0.0.0/0"      # Block external
      action: Block

Default Posture: audit (unmatched connections logged)
📌 4. PRIVILEGE ESCALATION (Nâng cao quyền)
A. TRACEE Detection
Go
// detectors/aslr_inspection.go
// Detects process reading /proc/sys/kernel/randomize_va_space
// to learn ASLR state before ROP/JOP exploitation

// detectors/proc_mem_code_injection.go
// Detects writes to /proc/[pid]/mem
// T1055.009: Process Memory Code Injection
B. KUBEARMOR Rules
YAML
# Restrict sudo/suid binaries

process:
  matchPaths:
    - path: /usr/bin/sudo
      action: Block          # Block sudo execution
      fromSource:            # Only allow from specific sources
        - path: /bin/bash

capabilities:
  matchCapabilities:
    - capability: CAP_SYS_ADMIN     # Block privilege escalation
      action: Block
    - capability: CAP_NET_ADMIN
      action: Block
Sudo Privilege Escalation Detection:

Code
Syscall: execve(/usr/bin/sudo)
     ↓
Check SUID bit (sudo has setuid root)
     ↓
Check caller UID (non-root user)
     ↓
Execution path analysis
     ↓
Action: [Allow/Block/Audit]
C. FALCO Rules
YAML
# CHANGELOG mentions:
# rule(Sudo Potential Privilege Escalation): detect CVE-2021-3156

Rule Pattern:
  Event: execve
  Condition: 
    - Binary == /usr/bin/sudo
    - Arguments contain suspicious patterns
    - UID change detected (non-root → root)
📌 5. CREDENTIAL ACCESS (Truy cập credentials)
A. TRACEE Detectors
Go
// detectors/proc_mem_access.go
// Type: Credential Access

Definition:
  ID: "TRC-1013"
  Pattern: procMemPathPattern = `/proc/[0-9]+/(maps|mem|auxv|environ)`
  
  MITRE:
    Technique: T1003.007 (Proc Filesystem)
    Tactic: Credential Access

OnEvent Logic:
  pathname := GetData[string](event, "pathname")
  flags := GetData[int32](event, "flags")
  
  if IsFileRead(flags) && regex.Match(procMemPathPattern, pathname) {
    return Detected()  // Alert: Reading /proc/pid/mem or /proc/pid/environ
  }
Credential Access Pattern:

Code
open syscall on /proc/[pid]/environ
     ↓
Environment variables often contain:
  - AWS_SECRET_ACCESS_KEY
  - DATABASE_PASSWORD
  - KUBECONFIG
     ↓
Detection: Process X reading /proc/Y/environ
     ↓
Action: [Alert | Block]
B. KUBEARMOR Credential Protection
YAML
# Getting Started

file:
  matchPaths:
    - path: /root/.aws/credentials
      action: Block
    - path: /root/.kube/config
      action: Block
    - path: /etc/secret*
      recursive: true
      action: Block
    - path: /var/run/secrets/kubernetes.io/serviceaccount/token
      action: Block
📌 6. DEFENSE EVASION (Trốn tránh phát hiện)
A. TRACEE Detectors
Go
// detectors/fileless_execution.go
// T1620: Reflective Code Loading

Type: Defense Evasion Detection
Severity: HIGH

OnEvent Logic:
  - Detect execution without file on disk
  - Detect process from memory (/memfd_create)
  - Detect process without executable in /proc/[pid]/exe
Go
// detectors/proc_fops_hooking.go
// T1014: Rootkit

Type: Kernel-level Evasion
Description: File operations hooking on /proc
Severity: HIGH
Threat: 
  - Hidden modules
  - Kernel compromise indicator
B. Log Deletion Detection
YAML
# CHANGELOG mentions:
# rule(Deny write under /etc/ directory)

Detection Pattern:
  matchPaths:
    - path: /root/*_history
      fromSource:
        - path: /usr/bin/shred
        - path: /usr/bin/rm
        - path: /bin/mv
      action: Block  # Block evidence removal

# Monitoring sensitive files
file:
  matchPaths:
    - path: /var/log/*
      ownerOnly: true
      action: Audit  # Alert on log modifications
🔄 COMPLETE RULE EXECUTION FLOW
Code
┌─────────────────────────────────────────────────────────────────┐
│ 1. SYSCALL/LSM HOOK TRIGGER                                      │
│    - Process execution: sched_process_exec                      │
│    - File access: security_file_open, security_file_permission  │
│    - Network: tcp_connect, udp_send                             │
│    - Capabilities: security_capable                             │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 2. IN-KERNEL FILTERING (eBPF Programs)                           │
│    - Extract syscall arguments                                  │
│    - Apply DataFilters: pathname, UID, PID, flags              │
│    - Skip non-matching events (early drop)                      │
│    - Extract metadata: binary, args, file, IP:port             │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 3. CONTAINER/WORKLOAD IDENTIFICATION                             │
│    - Get PID + Mount Namespace → Container ID                   │
│    - Get K8s metadata (labels, namespace, pod name)             │
│    - Lookup in policy cache                                     │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 4. POLICY MATCHING (Detection Rules)                             │
│    - Load applicable policies for container                     │
│    - Match process path / file path / network CIDR              │
│    - Check parent process (fromSource)                          │
│    - Check resource type (path vs directory)                    │
│    - Match severity + tags                                      │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 5. THREAT ENRICHMENT (MITRE ATT&CK Mapping)                      │
│    - Map to MITRE technique (T1055, T1059, etc.)                │
│    - Set severity (low/medium/high/critical)                    │
│    - Add tactic (Execution, Defense Evasion, etc.)             │
│    - Add process ancestry (parent→grandparent)                  │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 6. ENFORCEMENT ACTION (Kernel-level or Userspace)               │
│    ├─ Allow: Permit operation (continue)                        │
│    ├─ Audit: Log event + Permit                                │
│    └─ Block:                                                    │
│        ├─ Return -EPERM / -EACCES                              │
│        ├─ Send SIGKILL to process                              │
│        └─ Prevent syscall from completing                      │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│ 7. OUTPUT & ALERTING                                             │
│    - JSON event to stdout/file/syslog                           │
│    - Webhook notification (Slack, Teams, etc.)                  │
│    - SIEM integration (Splunk, ELK, etc.)                       │
│    - Alert dashboard (Falco UI, Kubescape, etc.)               │
└─────────────────────────────────────────────────────────────────┘
📊 RULE COMPARISON TABLE
Category	TRACEE	TETRAGON	KUBEARMOR	FALCO
Process Execution	Detector (Go)	TracingPolicy (YAML)	KubeArmorPolicy (YAML)	Rules (YAML)
File Access	eBPF+Detector	LSM Hook	AppArmor/BPF-LSM	Syscall trace
Network	Limited	kprobe (tcp_connect)	Limited	Rule-based
Enforcement	Alert	SIGKILL/Error	Block/Audit	Alert only
MITRE Mapping	✅ Built-in	✅ Custom	⚠️ Via policy tags	✅ Rules
Kernel-level	✅ eBPF	✅ eBPF	✅ LSM	⚠️ Partial
Cách các giải pháp này xử lý từng rule category khác nhau:

TRACEE: Mạnh ở detection (MITRE mapping), yếu ở enforcement
TETRAGON: Mạnh ở enforcement (kernel-level blocking), tốt ở detection
KUBEARMOR: Mạnh ở policy enforcement (AppArmor/LSM), tốt ở file/process control
FALCO: Mạnh ở detection rules, yếu ở enforcement
Bạn cần chi tiết nào thêm về cách xử lý cụ thể?

You said: hãy liệt kê toàn bộ các detection
hãy liệt kê toàn bộ các detection theo category, chi tiết thực hiện và rule/tiêu chuẩn mà các giải pháp tham chiếu tới đối với mỗi detection này
Copilot finished thinking

Copilot said: Dựa trên phân tích mã nguồn từ
Dựa trên phân tích mã nguồn từ các repositories, tôi sẽ tạo một danh sách toàn bộ các detections theo category, chi tiết thực hiện và tiêu chuẩn tham chiếu:

📋 TOÀN BỘ RUNTIME SECURITY DETECTIONS
I. TRACEE DETECTORS (aquasecurity/tracee)
1. PROCESS EXECUTION DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity | Implementation |
|----|---------|---------|-|---------|---------|----------|
| TRC-101 | Fileless Execution | Process | sched_process_exec | Reflective code loading | T1620 | HIGH | Detect execution from memory without file on disk |
| TRC-102 | Illegitimate Shell | Process | sched_process_exec | Web server spawning shell | T1190 | MEDIUM | Parent process validation (web servers) |
| TRC-103 | Anti-Debugging | Process | ptrace, process_vm_writev | Debug evasion techniques | T1622 | MEDIUM | Ptrace syscall monitoring |
| TRC-105 | Dynamic Code Loading | Process | mmap, mprotect | Runtime code injection | T1620 | MEDIUM | Monitor dlopen/mmap execution |
2. FILE ACCESS DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-109 | ASLR Inspection | Privilege Escalation | security_file_open | Read /proc/sys/kernel/randomize_va_space | T1518 | MEDIUM |
| TRC-1013 | Proc Memory Access | Credential Access | security_file_open | Read /proc/[pid]/(mem\|environ) | T1003.007 | MEDIUM |
| TRC-1022 | Dropped Executable | Defense Evasion | magic_write | ELF binary dropped to container | T1036 | MEDIUM |
| TRC-1026 | RC.d Modification | Persistence | security_file_open | Modify /etc/rc.* files | T1547 | HIGH |
| TRC-1012 | Default Loader Modification | Defense Evasion | security_file_open | Modify /lib/ld.so | T1574 | HIGH |
3. CODE INJECTION DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1025 | Process VM Write Injection | Privilege Escalation | process_vm_writev | Code injection via process_vm_writev | T1055 | HIGH |
| TRC-1015 | Ptrace Code Injection | Privilege Escalation | ptrace | Code injection via ptrace | T1055.008 | HIGH |
| TRC-1024 | Proc Memory Code Injection | Privilege Escalation | security_file_open | Write to /proc/[pid]/mem | T1055.009 | HIGH |
4. KERNEL HOOKING DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1020 | Proc Fops Hooking | Defense Evasion | hooked_proc_fops | File operations hooking on /proc | T1014 | HIGH |
| TRC-1018 | Syscall Table Hooking | Defense Evasion | hooked_syscalls | Syscall table modification | T1014 | CRITICAL |
| TRC-1021 | Hooked Seq Ops | Defense Evasion | hooked_seq_ops | Seq ops hooking (hidden modules) | T1014 | HIGH |
5. PERSISTENCE DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1006 | Sudoers Modification | Privilege Escalation | security_file_open | Modify /etc/sudoers | T1548 | HIGH |
| TRC-1010 | Scheduled Task Modification | Persistence | security_file_open | Modify crontab/scheduler | T1053 | MEDIUM |
| TRC-1011 | Core Pattern Modification | Persistence | security_file_open | Modify /proc/sys/kernel/core_pattern | T1547 | HIGH |
| TRC-1004 | System Request Key Modification | Defense Evasion | security_file_open | Modify sysrq-trigger | T1578 | MEDIUM |
| TRC-1002 | Cgroup Notify on Release Modification | Persistence | security_file_open | Cgroup escape via notify_on_release | T1611 | HIGH |
| TRC-1003 | Cgroup Release Agent Modification | Persistence | security_file_open | Cgroup escape via release_agent | T1611 | HIGH |
6. RECONNAISSANCE DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1008 | Sched Debug Recon | Reconnaissance | security_file_open | Read /proc/sched_debug | T1518 | LOW |
| TRC-1009 | Proc Kcore Read | Reconnaissance | security_file_open | Read /proc/kcore (kernel memory) | T1005 | HIGH |
7. DOCKER/CONTAINER DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1005 | Docker Abuse | Privilege Escalation | sched_process_exec | Unauthorized docker commands | T1610 | MEDIUM |
| TRC-1014 | Disk Mount | Privilege Escalation | sched_process_exec | Container escape via disk mount | T1611 | HIGH |
8. LD_PRELOAD & LIBRARY HIJACKING
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1017 | LD_PRELOAD | Defense Evasion | security_file_open | LD_PRELOAD environment variable | T1574 | HIGH |
9. KUBERNETES DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1016 | K8s Service Account Token | Credential Access | security_file_open | K8s service account token theft | T1552 | MEDIUM |
| TRC-1023 | Kubernetes API Connection | Lateral Movement | tcp_connect | Unauthorized K8s API access | T1021 | MEDIUM |
| TRC-1027 | Kubernetes Certificate Theft | Credential Access | security_file_open | K8s certificate access | T1552 | MEDIUM |
10. NETWORK DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1019 | Stdio Over Socket | Lateral Movement | socket_connect | Reverse shell via socket | T1571 | HIGH |
11. HIDDEN FILES DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1007 | Hidden File Created | Defense Evasion | security_file_open | Hidden file creation (dot-prefixed) | T1027 | LOW |
12. KERNEL MODULE DETECTIONS
table
| ID | Detector | Category | Event | Threat | MITRE | Severity |
|----|----------|----------|-------|--------|-------|----------|
| TRC-1001 | Kernel Module Loading | Privilege Escalation | init_module | Kernel module insertion | T1547 | CRITICAL |
II. KUBEARMOR POLICIES
Policy Action Model:
YAML
Actions:
  - Allow: Whitelist mode (only allowed operations permitted)
  - Audit: Monitor mode (log but allow)
  - Block: Deny mode (block and log)

Default Posture: audit (can be set to block for zero-trust)
1. PROCESS POLICIES
YAML
Category: Process Execution Control

Rules:
  ├─ matchPaths: Specific executable paths
  │  └─ ownerOnly: Only owner can execute
  │  └─ fromSource: Parent process validation
  ├─ matchDirectories: Directory-level control
  └─ Capabilities: CAP_SYS_ADMIN, CAP_NET_ADMIN restrictions

Reference: NIST AC-3 (Access Control)
2. FILE POLICIES
YAML
Category: File Access Control

Rules:
  ├─ matchPaths: Exact file matching
  │  └─ readOnly: Read-only enforcement
  │  └─ ownerOnly: Owner-only access
  ├─ matchDirectories: Directory recursion
  └─ readOnly: Immutable file flag

Reference: NIST AC-6 (Least Privilege), SI-7 (File Integrity)
3. NETWORK POLICIES
YAML
Category: Network Traffic Control

Rules:
  ├─ matchProtocols: TCP, UDP, ICMP
  ├─ matchPorts: Specific port ranges
  ├─ matchCIDRs: Source/destination IP filtering
  └─ Interface: Network interface selection

Reference: NIST SC-7 (Boundary Protection)
4. CAPABILITY RESTRICTIONS
YAML
Category: Linux Capability Control

Rules:
  ├─ CAP_SYS_ADMIN: Block privilege escalation
  ├─ CAP_NET_ADMIN: Block network admin privileges
  ├─ CAP_SYS_PTRACE: Block process tracing
  └─ CAP_SYS_MODULE: Block kernel module loading

Reference: NIST AC-2 (Capability-based security)
Example KubeArmor Policies:
YAML
# 1. Prevent Privilege Escalation
apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: block-sudo
spec:
  selector:
    matchLabels:
      app: webapp
  process:
    matchPaths:
      - path: /usr/bin/sudo
  action: Block
  severity: 9
  tags: ["privilege-escalation", "NIST_AC-6", "CIS_4.1"]

# 2. Read-Only Root Filesystem
apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: readonly-root
spec:
  selector:
    matchLabels:
      app: nginx
  file:
    matchDirectories:
      - dir: /
        recursive: true
        readOnly: true
  action: Audit
  severity: 5

# 3. Network Egress Restriction
apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: block-external-egress
spec:
  selector:
    matchLabels:
      app: api-server
  network:
    matchCIDRs:
      - cidr: "10.0.0.0/8"
        action: Allow
      - cidr: "0.0.0.0/0"
        action: Block
III. TETRAGON POLICIES (cilium/tetragon)
Enforcement Actions:
YAML
Actions:
  ├─ Monitoring: Event tracing only
  ├─ SIGKILL: Process termination
  ├─ NotifyEnforcer: Return error + signal
  └─ TraceReturn: Monitor syscall returns

Mode: enforcement | monitoring
Policy Examples:
YAML
# 1. Network Egress Enforcement
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: network-egress-enforce
spec:
  mode: enforcement
  kprobes:
    - call: tcp_connect
      selectors:
        - matchArgs:
            - index: 1
              operator: NotEqual
              values: 
                - "$PODCIDR"
                - "$SERVICECIDR"
          matchActions:
            - action: Sigkill

# 2. File Access Enforcement
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: file-access-enforce
spec:
  mode: enforcement
  kprobes:
    - call: security_file_permission
      selectors:
        - matchArgs:
            - index: 0
              operator: Equal
              values: 
                - "/etc/shadow"
                - "/etc/passwd"
          matchActions:
            - action: Sigkill
            - action: NotifyEnforcer
              argError: -13  # -EACCES
IV. FALCO RULES
Rule Structure:
YAML
Rule Components:
  - rule: Rule name
  - desc: Description
  - condition: Detection condition (boolean logic)
  - output: Alert format
  - priority: [DEBUG, INFO, NOTICE, WARNING, CRITICAL]
  - tags: [MITRE categories]
  - exceptions: False positive filters
Key Falco Rules:
YAML
# 1. Sudo Privilege Escalation (CVE-2021-3156)
- rule: "Sudo Potential Privilege Escalation"
  desc: "Detects CVE-2021-3156 sudo heap overflow"
  condition: 
    - spawned_process and process_name = sudo
    - container
  priority: CRITICAL
  tags: ["privilege-escalation", "CVE-2021-3156", "NIST_AC-6"]

# 2. Unauthorized Binary Execution
- rule: "Execute in User Directory"
  desc: "Detect execution from /tmp or /home"
  condition: 
    - spawned_process
    - proc.name in (shell_binaries)
    - proc.cwd in (/tmp, /home)
  priority: WARNING
  tags: ["execution", "T1036"]

# 3. Container Escape
- rule: "Mount Launched in Privileged Container"
  desc: "Detects mount command in privileged container"
  condition:
    - spawned_process and process_name = mount
    - container and container.privileged = true
  priority: CRITICAL
  tags: ["container-escape", "T1611"]
V. DETECTION PATTERN MAPPING
By MITRE ATT&CK Tactic:
Code
EXECUTION (T1059, T1190, T1620)
  ├─ Fileless Execution (TRC-101)
  ├─ Dynamic Code Loading (TRC-105)
  ├─ Illegitimate Shell (TRC-102)
  └─ Docker Abuse (TRC-1005)

PERSISTENCE (T1053, T1547, T1611)
  ├─ Scheduled Task Modification (TRC-1010)
  ├─ Cgroup Escape (TRC-1002, TRC-1003)
  ├─ Kernel Module Loading (TRC-1001)
  └─ RC.d Modification (TRC-1026)

PRIVILEGE ESCALATION (T1055, T1548, T1610)
  ├─ Process Injection (TRC-1015, TRC-1024, TRC-1025)
  ├─ Sudo Abuse (Falco)
  ├─ Docker Privilege (TRC-1005)
  └─ ASLR Inspection (TRC-109)

DEFENSE EVASION (T1014, T1027, T1036, T1574)
  ├─ Syscall Hooking (TRC-1018)
  ├─ Kernel Hooking (TRC-1020)
  ├─ Hidden Files (TRC-1007)
  ├─ Dropped Executables (TRC-1022)
  └─ LD_PRELOAD (TRC-1017)

CREDENTIAL ACCESS (T1003.007, T1552)
  ├─ Proc Memory Access (TRC-1013)
  ├─ K8s Token Theft (TRC-1016)
  └─ K8s Cert Theft (TRC-1027)

RECONNAISSANCE (T1005, T1518)
  ├─ Sched Debug Read (TRC-1008)
  └─ Kcore Read (TRC-1009)

LATERAL MOVEMENT (T1021, T1571)
  ├─ Stdio Over Socket (TRC-1019)
  └─ K8s API Access (TRC-1023)
VI. REFERENCE STANDARDS
Security Compliance Mapping:
Code
Standard          Detections Covered
────────────────────────────────────
NIST 800-53:
  AC-3: File Access Control (TRC-109*)
  AC-6: Least Privilege (All process policies)
  AU-2: Audit Events (Falco rules)
  SI-4: System Monitoring (All detectors)
  SI-7: File Integrity (KubeArmor readOnly)

CIS Kubernetes Benchmark:
  4.1: Restrict sudo (KubeArmor)
  4.2: Read-only filesystem
  4.3: Network policies

MITRE ATT&CK:
  ~30+ techniques covered across detectors

PCI DSS:
  6.5: Code injection prevention (TRC-1024, TRC-1025)
  6.6: Malware detection (TRC-1022)

SOC 2 Type II:
  CC7.2: System Monitoring (All)
  CC6.1: Access Control (KubeArmor)
VII. DETECTION OUTPUT STRUCTURE
JSON
{
  "detector": {
    "id": "TRC-101",
    "name": "Fileless Execution",
    "category": "Process Execution",
    "severity": "HIGH"
  },
  "threat": {
    "name": "Reflective Code Loading",
    "description": "Process execution from memory without file on disk",
    "mitre": {
      "technique": "T1620",
      "tactic": "Defense Evasion",
      "subtechnique": "Reflective Code Loading"
    },
    "cpe": "cpe:/o:linux:*",
    "cvss_score": 7.8
  },
  "detection": {
    "timestamp": "2026-03-26T10:30:45Z",
    "event": {
      "name": "sched_process_exec",
      "binary": "/usr/bin/payload",
      "args": ["--malicious"],
      "pid": 12345,
      "uid": 1000,
      "container_id": "abc123"
    },
    "action": "ALERT",
    "reference_rules": [
      "NIST_SI-4",
      "CIS_Kubernetes_4.2",
      "MITRE_T1620"
    ]
  }
}
