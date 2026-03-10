# Capability Specification – MITRE ATT&CK for Containers/Kubernetes

> **Scope**: This specification is split into **two strictly separated parts**:
>
> **PART A – Capability Definition (Design-Time)**
>
> * Used by: Agent/Core developers
> * Purpose: implement detection logic, promotion rules, enrichment, UI metadata
>
> **PART B – Capability Verification Testcases (Run-Time)**
>
> * Used by: QA, Red Team, Validation pipeline
> * Purpose: verify detection correctness after implementation
>
> ⚠️ **IMPORTANT RULE**:
>
> * PART A **MUST NOT** contain procedural attack steps
> * PART B **MUST NOT** redefine capability semantics

---

# PART A – CAPABILITY DEFINITIONS (FOR DEV & CORE LOGIC)

## A.1 Capability Design Principles

1. One capability represents **one attacker intent**, not a configuration flag.
2. Each capability MUST map to **MITRE ATT&CK for Containers/Kubernetes**.
3. Capability description MUST answer:

   * What the attacker is doing
   * Why it is dangerous
   * Under which conditions it is valid
4. Capability definitions are **static security knowledge**, not environment-specific findings.

---

## A.2 Capability Metadata Schema (Mandatory)

```yaml
capability_id: string
name: string
summary: string
full_description: string
mitre:
  tactic: string
  technique: string
  subtechnique: string
kill_chain_stage: discovery | credential_access | privilege_escalation | lateral_movement | defense_evasion | impact
severity: low | medium | high | critical
preconditions:
  - string
technical_indicators:
  - string
impact:
  - string
recommended_mitigations:
  - string
false_positive_considerations:
  - string
references:
  - url
```

---

## A.3 Capability Catalogue (Extended – Canonical)

---

### CAP-ESC-PRIV-POD – Privileged Container Execution

**Summary**
A container is running in privileged mode, granting unrestricted access to host resources.

**Full Description**
Privileged containers disable most isolation mechanisms enforced by the container runtime. An attacker controlling such a container can directly interact with host devices, kernel interfaces, and filesystem paths, effectively bypassing container boundaries and achieving host-level control.

**MITRE ATT&CK Mapping**

* Tactic: Privilege Escalation (TA0004)
* Technique: T1611 – Escape to Host

**Kill Chain Stage**
Privilege Escalation

**Preconditions**

* Pod specification sets `securityContext.privileged=true`

**Technical Indicators**

* Access to `/dev`, `/proc`, kernel interfaces

**Impact**

* Full host compromise
* Persistence outside Kubernetes control plane

**Recommended Mitigations**

* Remove privileged flag from workload definitions
* Enforce Pod Security Admission (Restricted profile)

**False Positive Considerations**

* Common for infrastructure components such as CNI or CSI drivers

**References**

* [https://attack.mitre.org/techniques/T1611/](https://attack.mitre.org/techniques/T1611/)

---

### CAP-ESC-HOSTPATH-RW – Writable HostPath Mount

**Summary**
A container mounts a host filesystem path with write permissions.

**Full Description**
Writable hostPath volumes expose the host filesystem directly to the container. Attackers can modify host binaries, inject malicious files, or tamper with node-level configuration, enabling host takeover or persistence.

**MITRE ATT&CK Mapping**

* Tactic: Defense Evasion (TA0005)
* Technique: T1610 – Escape to Host

**Kill Chain Stage**
Defense Evasion

**Preconditions**

* hostPath volume mounted as read-write

**Technical Indicators**

* Write operations on host-mounted paths

**Impact**

* Host integrity compromise
* Long-term persistence

**Recommended Mitigations**

* Remove hostPath usage where possible
* Enforce read-only mounts if unavoidable

**False Positive Considerations**

* Some monitoring or storage agents may require hostPath access

**References**

* [https://attack.mitre.org/techniques/T1610/](https://attack.mitre.org/techniques/T1610/)

---

### CAP-ESC-HOSTPID – Host PID Namespace Sharing

**Summary**
A container shares the host PID namespace, allowing visibility into host processes.

**Full Description**
When hostPID is enabled, processes inside the container can observe and potentially interfere with processes running on the host, enabling reconnaissance or direct interaction with host-level services.

**MITRE ATT&CK Mapping**

* Tactic: Privilege Escalation (TA0004)
* Technique: T1611.001

**Kill Chain Stage**
Privilege Escalation

**Preconditions**

* Pod specification sets `hostPID=true`

**Technical Indicators**

* Visibility of host process IDs

**Impact**

* Host process manipulation
* Increased attack surface

**Recommended Mitigations**

* Disable hostPID unless strictly required

**False Positive Considerations**

* Used by certain debugging or monitoring workloads

**References**

* [https://attack.mitre.org/techniques/T1611/](https://attack.mitre.org/techniques/T1611/)

---

### CAP-ESC-HOSTIPC – Host IPC Namespace Sharing

**Summary**
A container shares the host IPC namespace, exposing inter-process communication channels.

**Full Description**
Sharing the IPC namespace allows containers to interact with host IPC mechanisms such as shared memory or semaphores, potentially enabling data leakage or process interference.

**MITRE ATT&CK Mapping**

* Tactic: Privilege Escalation
* Technique: T1611.002

**Kill Chain Stage**
Privilege Escalation

**Preconditions**

* Pod specification sets `hostIPC=true`

**Technical Indicators**

* Access to host IPC resources

**Impact**

* Data leakage
* Host process interference

**Recommended Mitigations**

* Disable hostIPC by default

**False Positive Considerations**

* Rare, mostly system-level workloads

**References**

* [https://attack.mitre.org/techniques/T1611/](https://attack.mitre.org/techniques/T1611/)

---

### CAP-ID-SA-TOKEN-ACCESS – ServiceAccount Token Access

**Summary**
A container accesses a mounted Kubernetes ServiceAccount token.

**Full Description**
Kubernetes mounts ServiceAccount tokens into pods by default. Attackers can extract these tokens and authenticate to the Kubernetes API, potentially enabling lateral movement or privilege escalation depending on RBAC configuration.

**MITRE ATT&CK Mapping**

* Tactic: Credential Access (TA0006)
* Technique: T1552.007 – Container API Credentials

**Kill Chain Stage**
Credential Access

**Preconditions**

* ServiceAccount token is auto-mounted

**Technical Indicators**

* Read access to SA token file

**Impact**

* Kubernetes API abuse
* Cross-namespace access

**Recommended Mitigations**

* Disable automountServiceAccountToken where unnecessary
* Apply least-privilege RBAC

**False Positive Considerations**

* Legitimate controllers may access tokens

**References**

* [https://attack.mitre.org/techniques/T1552/007/](https://attack.mitre.org/techniques/T1552/007/)

---

### CAP-RBAC-CLUSTER-WRITE – Excessive Cluster-Level RBAC

**Summary**
A ServiceAccount has cluster-wide write permissions.

**Full Description**
Excessive RBAC permissions allow attackers to create, modify, or bind high-privilege roles, enabling full cluster takeover.

**MITRE ATT&CK Mapping**

* Tactic: Lateral Movement (TA0008)
* Technique: T1612 – Kubernetes RBAC Abuse

**Kill Chain Stage**
Lateral Movement

**Preconditions**

* RBAC grants write access to cluster-scoped resources

**Technical Indicators**

* Verbs: create/update/delete on clusterroles or bindings

**Impact**

* Full cluster compromise

**Recommended Mitigations**

* Audit and reduce RBAC permissions

**False Positive Considerations**

* Cluster operators may require elevated access

**References**

* [https://attack.mitre.org/techniques/T1612/](https://attack.mitre.org/techniques/T1612/)

---

### CAP-NET-HOSTNETWORK – Host Network Usage

**Summary**
A container runs in the host network namespace.

**Full Description**
Using the host network namespace allows containers to bypass network isolation, enabling traffic sniffing, port scanning, or direct access to node-level services.

**MITRE ATT&CK Mapping**

* Tactic: Lateral Movement (TA0008)
* Technique: T1614 – Container and Cluster Networking Abuse

**Kill Chain Stage**
Lateral Movement

**Preconditions**

* Pod specification sets `hostNetwork=true`

**Technical Indicators**

* Access to node network interfaces

**Impact**

* Network reconnaissance
* Service exploitation

**Recommended Mitigations**

* Avoid hostNetwork usage
* Enforce network policies

**False Positive Considerations**

* Used by certain network plugins

**References**

* [https://attack.mitre.org/techniques/T1614/](https://attack.mitre.org/techniques/T1614/)

---

### CAP-DISC-ENV-ENUM – Environment and Secret Enumeration

**Summary**
A container enumerates environment variables and mounted secrets.

**Full Description**
Attackers commonly enumerate environment variables and secret mounts to discover credentials, API keys, or configuration details that enable further compromise.

**MITRE ATT&CK Mapping**

* Tactic: Discovery (TA0007)
* Technique: T1613 – Container and Host Discovery

**Kill Chain Stage**
Discovery

**Preconditions**

* Interactive shell or command execution in container

**Technical Indicators**

* Execution of commands such as `env`, `printenv`, or directory listing of secret mounts

**Impact**

* Credential exposure
* Expanded attack surface awareness

**Recommended Mitigations**

* Minimize sensitive data in environment variables
* Use external secret management

**False Positive Considerations**

* Common during debugging sessions

**References**

* [https://attack.mitre.org/techniques/T1613/](https://attack.mitre.org/techniques/T1613/)

---

### CAP-DISC-K8S-API-PROBE – Kubernetes API Probing

**Summary**
A container probes the Kubernetes API to enumerate cluster resources.

**Full Description**
By querying the Kubernetes API, attackers can enumerate namespaces, pods, nodes, and services to identify potential lateral movement or privilege escalation paths.

**MITRE ATT&CK Mapping**

* Tactic: Discovery (TA0007)
* Technique: T1613

**Kill Chain Stage**
Discovery

**Preconditions**

* Network access to kube-apiserver

**Technical Indicators**

* API requests for listing cluster resources

**Impact**

* Cluster topology disclosure

**Recommended Mitigations**

* Restrict API access via RBAC
* Monitor anomalous API usage

**False Positive Considerations**

* Legitimate controllers perform API enumeration

**References**

* [https://attack.mitre.org/techniques/T1613/](https://attack.mitre.org/techniques/T1613/)

---

### CAP-LM-SA-TOKEN-REUSE – ServiceAccount Token Reuse

**Summary**
A ServiceAccount token is reused from an unintended context.

**Full Description**
Attackers may reuse stolen ServiceAccount tokens from one pod or namespace to authenticate against the Kubernetes API from another context, enabling lateral movement.

**MITRE ATT&CK Mapping**

* Tactic: Lateral Movement (TA0008)
* Technique: T1612

**Kill Chain Stage**
Lateral Movement

**Preconditions**

* Valid ServiceAccount token available

**Technical Indicators**

* API usage from unexpected workload or namespace

**Impact**

* Cross-namespace compromise

**Recommended Mitigations**

* Scope ServiceAccounts to namespaces
* Rotate and audit tokens

**False Positive Considerations**

* Shared automation tokens

**References**

* [https://attack.mitre.org/techniques/T1612/](https://attack.mitre.org/techniques/T1612/)

---

# PART B – CAPABILITY VERIFICATION TESTCASES (FOR VALIDATION)

(FOR VALIDATION)

## B.1 Testcase Design Rules

1. Testcases validate **observable outcomes**, not implementation details.
2. Each testcase MUST be reproducible in a lab Kubernetes cluster.
3. Expected result MUST be deterministic.

---

## B.2 Testcase Catalogue

---

### Testcase: TC-ESC-PRIV-POD-01

**Related Capability**
CAP-ESC-PRIV-POD

**Objective**
Verify detection of privileged container execution.

**Environment Setup**

* Kubernetes cluster (kind/minikube acceptable)

**Steps**

1. Deploy a pod with `securityContext.privileged=true`
2. Wait for agent data collection cycle

**Expected Results**

* Capability CAP-ESC-PRIV-POD is reported
* Severity is HIGH or CRITICAL

---

### Testcase: TC-ESC-HOSTPATH-RW-01

**Related Capability**
CAP-ESC-HOSTPATH-RW

**Objective**
Verify detection of writable hostPath mounts.

**Steps**

1. Deploy a pod mounting `/etc` via hostPath (read-write)
2. Perform a write operation inside the container

**Expected Results**

* Capability CAP-ESC-HOSTPATH-RW is detected

---

### Testcase: TC-ID-SA-TOKEN-01

**Related Capability**
CAP-ID-SA-TOKEN-ACCESS

**Objective**
Verify ServiceAccount token access detection.

**Steps**

1. Exec into pod
2. Read ServiceAccount token file

**Expected Results**

* Capability CAP-ID-SA-TOKEN-ACCESS is detected

---

# Acceptance Criteria

* Capability definitions are language-consistent and implementation-ready
* Testcases can be executed independently
* Detection results match expected capabilities
* MITRE references are present and verifiable
