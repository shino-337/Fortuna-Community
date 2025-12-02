# CIS Kubernetes Benchmark v1.8 - Complete Implementation

**Document Version**: 1.0  
**CIS Benchmark Version**: 1.8.0 / 1.9.0  
**Last Updated**: 2025-11-29  
**Scope**: Section 5 - Policies (RBAC, Pod Security, Network, Secrets)

---

## Table of Contents

- [Overview](#overview)
- [Section 5.1: RBAC and Service Accounts](#section-51-rbac-and-service-accounts)
- [Section 5.2: Pod Security Standards](#section-52-pod-security-standards)
- [Section 5.3: Network Policies](#section-53-network-policies)
- [Section 5.4: Secrets Management](#section-54-secrets-management)
- [Section 5.5: Extensible Admission Control](#section-55-extensible-admission-control)
- [Section 5.6: General Policies](#section-56-general-policies)
- [Implementation Guide](#implementation-guide)
- [CEL Expressions](#cel-expressions)
- [Test Cases](#test-cases)

---

## Overview

### About CIS Kubernetes Benchmark

The CIS Kubernetes Benchmark provides prescriptive guidance for establishing a secure configuration posture for Kubernetes. This implementation focuses on **Section 5 - Policies**, which covers:

- **5.1**: RBAC and Service Accounts (6 rules)
- **5.2**: Pod Security Standards (13 rules)
- **5.3**: Network Policies (2 rules)
- **5.4**: Secrets Management (2 rules)
- **5.5**: Extensible Admission Control (1 rule)
- **5.6**: General Policies (2 rules)

**Total**: 26 rules for comprehensive policy enforcement

### Scoring Levels

- **Level 1 (L1)**: Basic security, minimal operational impact
- **Level 2 (L2)**: Enhanced security, may impact operations

### Assessment Status

- **Automated**: Can be checked programmatically
- **Manual**: Requires human review

---

## Section 5.1: RBAC and Service Accounts

### CIS 5.1.1 - Ensure cluster-admin role is only used where required

**CIS ID**: 5.1.1  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
The cluster-admin role provides wide-ranging powers over the environment and should only be used where absolutely necessary. Minimize the use of this role to reduce the risk of privilege escalation.

**Rationale**:  
Kubernetes provides a set of default roles where RBAC is used. The cluster-admin role has unrestricted access to the Kubernetes cluster and should only be used in specific circumstances.

**Audit**:
```bash
# List all ClusterRoleBindings with cluster-admin
kubectl get clusterrolebindings -o json | jq -r '
  .items[] | 
  select(.roleRef.name == "cluster-admin") | 
  {name: .metadata.name, subjects: .subjects}'
```

**Remediation**:  
Identify all ClusterRoleBindings with cluster-admin and remove those that are not required. Create specific Roles/ClusterRoles with minimum necessary permissions.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.1.1
title: "Minimize cluster-admin role usage"
severity: high
category: rbac
cis_section: "5.1.1"
cis_level: 1

detection:
  type: rbac_binding
  conditions:
    - role_ref.name == "cluster-admin"
    - role_ref.kind == "ClusterRole"
  
risk_score_calculation:
  base: 8.0
  multipliers:
    - if: subject.kind == "ServiceAccount"
      multiply: 1.2  # 9.6
    - if: subject.kind == "User" && subject.name == "system:anonymous"
      multiply: 1.5  # 12.0 (critical)
    - if: subject.kind == "Group" && subject.name == "system:authenticated"
      multiply: 1.3  # 10.4 (critical)

evidence:
  - binding_name: "{{ .metadata.name }}"
  - subjects: "{{ .subjects }}"
  - namespace: "{{ .metadata.namespace | default 'cluster-wide' }}"

remediation:
  - "Review the necessity of cluster-admin binding: {{ .metadata.name }}"
  - "Create a custom ClusterRole with minimum required permissions"
  - "Delete the cluster-admin binding if not required"
  - "kubectl delete clusterrolebinding {{ .metadata.name }}"
```

**CEL Expression**:
```cel
// Check if ClusterRoleBinding uses cluster-admin
object.roleRef.name == "cluster-admin" && object.roleRef.kind == "ClusterRole"
```

---

### CIS 5.1.2 - Minimize access to secrets

**CIS ID**: 5.1.2  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
The Kubernetes API stores secrets, which may be service account tokens or credentials used by Pods to access services. Access to these secrets should be restricted to the smallest possible group of users.

**Rationale**:  
Inappropriate access to secrets could allow an attacker to gain access to sensitive information or escalate privileges.

**Audit**:
```bash
# Find Roles/ClusterRoles that can access secrets
kubectl get roles,clusterroles -A -o json | jq -r '
  .items[] | 
  select(.rules[]? | 
    select(.resources[]? == "secrets" and 
           (.verbs[]? == "get" or .verbs[]? == "list" or .verbs[]? == "*"))) | 
  {name: .metadata.name, namespace: .metadata.namespace, rules: .rules}'
```

**Remediation**:  
Where possible, remove get, list, and watch access to secret objects. Create specific roles with minimal permissions only for components that need secret access.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.1.2
title: "Excessive secret access permissions"
severity: high
category: rbac
cis_section: "5.1.2"
cis_level: 1

detection:
  type: rbac_role
  conditions:
    - resources contains "secrets" OR resources contains "*"
    - verbs contains "get" OR verbs contains "list" OR verbs contains "watch" OR verbs contains "*"
    - NOT (name matches "^system:")  # Exclude system roles
  
risk_score_calculation:
  base: 7.5
  multipliers:
    - if: verbs contains "*"
      multiply: 1.3  # 9.75
    - if: resources contains "*"
      multiply: 1.2  # 9.0
    - if: api_groups contains "*"
      multiply: 1.1  # 8.25
    - if: role_bindings_count > 5
      multiply: 1.2  # Many users with access

evidence:
  - role_name: "{{ .metadata.name }}"
  - role_type: "{{ .kind }}"
  - rules: "{{ .rules }}"
  - bound_subjects_count: "{{ .bindings_count }}"

remediation:
  - "Review role {{ .metadata.name }} for secret access"
  - "Restrict secret access to only required ServiceAccounts"
  - "Use specific secret names instead of wildcard access"
  - "Consider using external secret management (Vault, AWS Secrets Manager)"
```

**CEL Expression**:
```cel
// Check if role grants broad secret access
object.rules.exists(r, 
  (r.resources.exists(res, res == "secrets" || res == "*")) &&
  (r.verbs.exists(v, v in ["get", "list", "watch", "*"])) &&
  !object.metadata.name.startsWith("system:")
)
```

---

### CIS 5.1.3 - Minimize wildcard use in Roles and ClusterRoles

**CIS ID**: 5.1.3  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
Kubernetes Roles and ClusterRoles provide access to resources based on sets of objects and actions. The use of wildcards in resource or verb fields should be avoided.

**Rationale**:  
The principle of least privilege recommends that users and service accounts are provided only the access required to perform their tasks. Wildcard use grants excessive permissions and violates this principle.

**Audit**:
```bash
# Find Roles/ClusterRoles with wildcards
kubectl get roles,clusterroles -A -o json | jq -r '
  .items[] | 
  select(.rules[]? | 
    select(.resources[]? == "*" or .verbs[]? == "*" or .apiGroups[]? == "*")) | 
  {name: .metadata.name, namespace: .metadata.namespace, rules: .rules}'
```

**Remediation**:  
Where possible, remove wildcards from Roles and ClusterRoles. Create specific roles with explicit resources and verbs.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.1.3
title: "Wildcard usage in Roles/ClusterRoles"
severity: high
category: rbac
cis_section: "5.1.3"
cis_level: 1

detection:
  type: rbac_role
  conditions:
    - resources contains "*" OR verbs contains "*" OR api_groups contains "*"
    - NOT (name matches "^system:")  # Exclude system roles
    - NOT (name matches "^cluster-admin$")  # Exclude known admin role
  
risk_score_calculation:
  base: 7.0
  multipliers:
    - if: resources contains "*" AND verbs contains "*"
      multiply: 1.4  # 9.8 - Double wildcard
    - if: api_groups contains "*" AND resources contains "*"
      multiply: 1.3  # 9.1
    - if: verbs contains "*"
      multiply: 1.2  # 8.4
    - if: resources contains "*"
      multiply: 1.1  # 7.7
    - if: role_type == "ClusterRole"
      multiply: 1.2  # Cluster-wide is higher risk

evidence:
  - role_name: "{{ .metadata.name }}"
  - role_type: "{{ .kind }}"
  - wildcard_fields: |
      {{- if .rules | selectattr('resources', 'contains', '*') }}Resources: *{{- end }}
      {{- if .rules | selectattr('verbs', 'contains', '*') }}Verbs: *{{- end }}
      {{- if .rules | selectattr('apiGroups', 'contains', '*') }}API Groups: *{{- end }}
  - affected_rules: "{{ .rules | selectattr('resources', 'contains', '*') | list }}"

remediation:
  - "Replace wildcards in role {{ .metadata.name }} with specific values"
  - "Example: Instead of resources: ['*'], use resources: ['pods', 'services']"
  - "Example: Instead of verbs: ['*'], use verbs: ['get', 'list']"
  - "Review and update role bindings after modification"
```

**CEL Expression**:
```cel
// Check for wildcard usage in roles
object.rules.exists(r, 
  r.resources.exists(res, res == "*") ||
  r.verbs.exists(v, v == "*") ||
  (has(r.apiGroups) && r.apiGroups.exists(g, g == "*"))
) && !object.metadata.name.startsWith("system:")
```

---

### CIS 5.1.4 - Minimize access to create pods

**CIS ID**: 5.1.4  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
The ability to create pods in a cluster opens up possibilities for privilege escalation and should be restricted to the smallest possible group of users.

**Rationale**:  
The ability to create pods allows a user to run arbitrary code in the cluster. Users with pod creation rights can mount host paths, use privileged containers, and access secrets.

**Audit**:
```bash
# Find roles that can create pods
kubectl get roles,clusterroles -A -o json | jq -r '
  .items[] | 
  select(.rules[]? | 
    select(.resources[]? == "pods" or .resources[]? == "*") and
    select(.verbs[]? == "create" or .verbs[]? == "*")) | 
  {name: .metadata.name, namespace: .metadata.namespace}'
```

**Remediation**:  
Review roles and remove pod creation permissions where not required. Consider using Pod Security admission controllers.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.1.4
title: "Excessive pod creation permissions"
severity: medium
category: rbac
cis_section: "5.1.4"
cis_level: 1

detection:
  type: rbac_role
  conditions:
    - (resources contains "pods" OR resources contains "*")
    - (verbs contains "create" OR verbs contains "*")
    - NOT (name matches "^system:(controller|node|kube-scheduler)")
  
risk_score_calculation:
  base: 6.5
  multipliers:
    - if: resources contains "*" AND verbs contains "*"
      multiply: 1.3  # 8.45
    - if: role_type == "ClusterRole"
      multiply: 1.2  # 7.8
    - if: bound_service_accounts_count > 10
      multiply: 1.15

evidence:
  - role_name: "{{ .metadata.name }}"
  - can_create_pods: true
  - bound_subjects: "{{ .bindings }}"

remediation:
  - "Review necessity of pod creation permission in {{ .metadata.name }}"
  - "Consider using a pod controller (Deployment, StatefulSet) instead"
  - "Implement Pod Security Standards to restrict pod capabilities"
```

---

### CIS 5.1.5 - Ensure default service accounts are not actively used

**CIS ID**: 5.1.5  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
The default service account should not be used to ensure that rights granted to applications can be more easily audited and reviewed.

**Rationale**:  
Kubernetes provides a default service account which is used by cluster workloads where no specific service account is assigned. By default, this service account has no permissions.

**Audit**:
```bash
# Find pods using default service account
kubectl get pods -A -o json | jq -r '
  .items[] | 
  select(.spec.serviceAccountName == "default" or .spec.serviceAccountName == null) | 
  {name: .metadata.name, namespace: .metadata.namespace}'
```

**Remediation**:  
Create specific service accounts for each application and ensure pods use these instead of default.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.1.5
title: "Pod using default ServiceAccount"
severity: medium
category: rbac
cis_section: "5.1.5"
cis_level: 1

detection:
  type: pod
  conditions:
    - service_account_name == "default" OR service_account_name is null
    - namespace != "kube-system"  # Exclude system namespace
  
risk_score_calculation:
  base: 5.5
  multipliers:
    - if: default_sa_has_permissions == true
      multiply: 1.5  # 8.25
    - if: pod_has_sensitive_volumes == true
      multiply: 1.3  # 7.15

evidence:
  - pod_name: "{{ .metadata.name }}"
  - namespace: "{{ .metadata.namespace }}"
  - service_account: "{{ .spec.serviceAccountName | default 'default' }}"

remediation:
  - "Create a dedicated ServiceAccount for pod {{ .metadata.name }}"
  - "kubectl create serviceaccount <app-name> -n {{ .metadata.namespace }}"
  - "Update pod spec with: spec.serviceAccountName: <app-name>"
  - "Assign minimal RBAC permissions to the new ServiceAccount"
```

---

### CIS 5.1.6 - Ensure Service Account tokens are only mounted where necessary

**CIS ID**: 5.1.6  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
Service account tokens should only be mounted into pods where the workload requires access to the Kubernetes API.

**Rationale**:  
Mounting service account tokens increases the attack surface. If a pod doesn't need to access the API, it shouldn't have a token mounted.

**Audit**:
```bash
# Find pods with automountServiceAccountToken not set to false
kubectl get pods -A -o json | jq -r '
  .items[] | 
  select(.spec.automountServiceAccountToken != false) | 
  {name: .metadata.name, namespace: .metadata.namespace}'
```

**Remediation**:  
Set `automountServiceAccountToken: false` in pod specs or service account definitions where API access is not needed.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.1.6
title: "ServiceAccount token unnecessarily mounted"
severity: medium
category: rbac
cis_section: "5.1.6"
cis_level: 1

detection:
  type: pod
  conditions:
    - automount_service_account_token != false
    - NOT (pod_accesses_kubernetes_api == true)  # Heuristic check
  
risk_score_calculation:
  base: 5.0
  multipliers:
    - if: service_account_has_permissions == true
      multiply: 1.4  # 7.0
    - if: pod_is_privileged == true
      multiply: 1.5  # 7.5

evidence:
  - pod_name: "{{ .metadata.name }}"
  - namespace: "{{ .metadata.namespace }}"
  - automount_token: "{{ .spec.automountServiceAccountToken | default true }}"

remediation:
  - "Set automountServiceAccountToken: false in pod spec"
  - "Example: spec.automountServiceAccountToken: false"
  - "Or set it on ServiceAccount for all pods using it"
```

---

## Section 5.2: Pod Security Standards

### CIS 5.2.1 - Ensure privileged containers are not admitted

**CIS ID**: 5.2.1  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
Privileged containers have access to all Linux Kernel capabilities and devices. They should not be admitted.

**Rationale**:  
Privileged containers can access the host's resources and can be used to gain access to the host system.

**Audit**:
```bash
# Find privileged containers
kubectl get pods -A -o json | jq -r '
  .items[] | 
  select(.spec.containers[]?.securityContext?.privileged == true) | 
  {name: .metadata.name, namespace: .metadata.namespace}'
```

**Remediation**:  
Implement Pod Security Standards to deny privileged containers. Remove `privileged: true` from container security contexts.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.1
title: "Privileged container detected"
severity: critical
category: pod_security
cis_section: "5.2.1"
cis_level: 1

detection:
  type: pod
  conditions:
    - containers[*].security_context.privileged == true OR
      init_containers[*].security_context.privileged == true
  
risk_score_calculation:
  base: 9.0
  multipliers:
    - if: host_pid == true OR host_network == true
      multiply: 1.1  # 9.9
    - if: host_ipc == true
      multiply: 1.05  # 9.45

evidence:
  - pod_name: "{{ .metadata.name }}"
  - namespace: "{{ .metadata.namespace }}"
  - privileged_containers: "{{ .spec.containers | selectattr('securityContext.privileged', 'equalto', true) | map(attribute='name') | list }}"

remediation:
  - "Remove privileged: true from container security context"
  - "Identify required capabilities and add them specifically"
  - "Use securityContext.capabilities.add: [SPECIFIC_CAP] instead"
  - "Enable Pod Security Standards (restricted profile)"
```

---

### CIS 5.2.2 - Ensure containers do not use hostPID

**CIS ID**: 5.2.2  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
Do not allow containers to share the host process ID namespace.

**Rationale**:  
A container running with hostPID can inspect processes running outside its container and send signals to those processes.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.2
title: "Container using hostPID"
severity: high
category: pod_security
cis_section: "5.2.2"
cis_level: 1

detection:
  type: pod
  conditions:
    - host_pid == true
  
risk_score_calculation:
  base: 8.0
  multipliers:
    - if: privileged == true
      multiply: 1.2  # 9.6
    - if: host_network == true
      multiply: 1.1  # 8.8

evidence:
  - pod_name: "{{ .metadata.name }}"
  - host_pid_enabled: true

remediation:
  - "Remove hostPID: true from pod spec"
  - "If required for debugging, use dedicated debug pods"
```

---

### CIS 5.2.3 - Ensure containers do not use hostIPC

**CIS ID**: 5.2.3  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.3
title: "Container using hostIPC"
severity: high
category: pod_security
cis_section: "5.2.3"
cis_level: 1

detection:
  type: pod
  conditions:
    - host_ipc == true
  
risk_score_calculation:
  base: 7.5

remediation:
  - "Remove hostIPC: true from pod spec"
```

---

### CIS 5.2.4 - Ensure containers do not use hostNetwork

**CIS ID**: 5.2.4  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.4
title: "Container using hostNetwork"
severity: high
category: pod_security
cis_section: "5.2.4"
cis_level: 1

detection:
  type: pod
  conditions:
    - host_network == true
  
risk_score_calculation:
  base: 8.0
  multipliers:
    - if: host_pid == true
      multiply: 1.2

remediation:
  - "Remove hostNetwork: true from pod spec"
  - "Use Kubernetes Services for network communication"
```

---

### CIS 5.2.5 - Ensure allowPrivilegeEscalation is set to false

**CIS ID**: 5.2.5  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.5
title: "allowPrivilegeEscalation not disabled"
severity: high
category: pod_security
cis_section: "5.2.5"
cis_level: 1

detection:
  type: pod
  conditions:
    - containers[*].security_context.allow_privilege_escalation != false
  
risk_score_calculation:
  base: 7.5
  multipliers:
    - if: runs_as_root == true
      multiply: 1.3  # 9.75

remediation:
  - "Set allowPrivilegeEscalation: false in container security context"
  - "spec.containers[].securityContext.allowPrivilegeEscalation: false"
```

---

### CIS 5.2.6 - Ensure containers run as non-root user

**CIS ID**: 5.2.6  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.6
title: "Container running as root"
severity: medium
category: pod_security
cis_section: "5.2.6"
cis_level: 1

detection:
  type: pod
  conditions:
    - (security_context.run_as_non_root != true) AND
      (security_context.run_as_user == 0 OR security_context.run_as_user is null)
  
risk_score_calculation:
  base: 6.5
  multipliers:
    - if: privileged == true
      multiply: 1.5  # 9.75
    - if: allow_privilege_escalation != false
      multiply: 1.3

remediation:
  - "Set runAsNonRoot: true in pod or container security context"
  - "Set runAsUser: <non-zero-uid> in security context"
```

---

### CIS 5.2.7 - Ensure NET_RAW capability is not granted

**CIS ID**: 5.2.7  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.7
title: "NET_RAW capability granted"
severity: medium
category: pod_security
cis_section: "5.2.7"
cis_level: 1

detection:
  type: pod
  conditions:
    - containers[*].security_context.capabilities.add contains "NET_RAW" OR
      containers[*].security_context.capabilities.add contains "ALL"
  
risk_score_calculation:
  base: 6.0
  multipliers:
    - if: host_network == true
      multiply: 1.4

remediation:
  - "Remove NET_RAW from capabilities.add"
  - "Add NET_RAW to capabilities.drop explicitly"
```

---

### CIS 5.2.8 - Ensure containers with added capabilities are limited

**CIS ID**: 5.2.8  
**Level**: L2  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.8
title: "Excessive Linux capabilities added"
severity: medium
category: pod_security
cis_section: "5.2.8"
cis_level: 2

detection:
  type: pod
  conditions:
    - containers[*].security_context.capabilities.add is not empty
    - NOT (capabilities.add only contains allowed_caps)
  
allowed_capabilities:
    # Commonly safe capabilities
    - NET_BIND_SERVICE
    - CHOWN
    - DAC_OVERRIDE
    - SETUID
    - SETGID
  
dangerous_capabilities:
    - SYS_ADMIN
    - SYS_PTRACE
    - SYS_MODULE
    - DAC_READ_SEARCH
    - NET_ADMIN
  
risk_score_calculation:
  base: 6.5
  multipliers:
    - if: capabilities.add contains dangerous_capabilities
      multiply: 1.5  # 9.75

remediation:
  - "Review and minimize added capabilities"
  - "Drop all capabilities first: capabilities.drop: [ALL]"
  - "Add only required capabilities"
```

---

### CIS 5.2.9 - Ensure containers are not assigned all capabilities

**CIS ID**: 5.2.9  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.9
title: "Container assigned ALL capabilities"
severity: critical
category: pod_security
cis_section: "5.2.9"
cis_level: 1

detection:
  type: pod
  conditions:
    - containers[*].security_context.capabilities.add contains "ALL" OR
      containers[*].security_context.capabilities.add contains "*"
  
risk_score_calculation:
  base: 9.5

remediation:
  - "Remove ALL from capabilities.add"
  - "Add only specific required capabilities"
```

---

### CIS 5.2.10 - Ensure seccomp profile is set to docker/default or runtime/default

**CIS ID**: 5.2.10  
**Level**: L2  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.10
title: "Seccomp profile not set"
severity: low
category: pod_security
cis_section: "5.2.10"
cis_level: 2

detection:
  type: pod
  conditions:
    - security_context.seccomp_profile is null OR
      security_context.seccomp_profile.type not in ["RuntimeDefault", "Localhost"]
  
risk_score_calculation:
  base: 4.0

remediation:
  - "Set seccompProfile.type: RuntimeDefault in security context"
  - "Or use a custom profile: seccompProfile.type: Localhost"
```

---

### CIS 5.2.11 - Ensure AppArmor profile is configured

**CIS ID**: 5.2.11  
**Level**: L2  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.11
title: "AppArmor profile not configured"
severity: low
category: pod_security
cis_section: "5.2.11"
cis_level: 2

detection:
  type: pod
  conditions:
    - annotations["container.apparmor.security.beta.kubernetes.io/*"] is null
  
risk_score_calculation:
  base: 3.5

remediation:
  - "Add AppArmor annotation to pod metadata"
  - "container.apparmor.security.beta.kubernetes.io/<container>: runtime/default"
```

---

### CIS 5.2.12 - Ensure SELinux options are set appropriately

**CIS ID**: 5.2.12  
**Level**: L2  
**Automated**: Manual  
**Scoring**: Not Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.12
title: "SELinux options not configured"
severity: info
category: pod_security
cis_section: "5.2.12"
cis_level: 2

detection:
  type: pod
  conditions:
    - security_context.se_linux_options is null
  
risk_score_calculation:
  base: 2.0

remediation:
  - "Configure SELinux options in security context"
  - "seLinuxOptions: {level: 's0:c123,c456'}"
```

---

### CIS 5.2.13 - Ensure containers do not mount sensitive host paths

**CIS ID**: 5.2.13  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.2.13
title: "Sensitive host path mounted"
severity: high
category: pod_security
cis_section: "5.2.13"
cis_level: 1

detection:
  type: pod
  conditions:
    - volumes[*].host_path.path in sensitive_paths
  
sensitive_paths:
    - /
    - /boot
    - /dev
    - /etc
    - /lib
    - /proc
    - /sys
    - /usr
    - /var/run/docker.sock
    - /var/run/containerd/containerd.sock
  
risk_score_calculation:
  base: 8.5
  multipliers:
    - if: path == "/" or path == "/var/run/docker.sock"
      multiply: 1.2  # 10.2 (critical)

evidence:
  - mounted_paths: "{{ .spec.volumes | selectattr('hostPath') | map(attribute='hostPath.path') | list }}"

remediation:
  - "Remove sensitive host path mounts"
  - "Use ConfigMaps, Secrets, or PersistentVolumes instead"
  - "If required, mount read-only: readOnly: true"
```

---

## Section 5.3: Network Policies

### CIS 5.3.1 - Ensure NetworkPolicies are applied

**CIS ID**: 5.3.1  
**Level**: L2  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
Use NetworkPolicies to segment traffic in your cluster and prevent lateral movement.

**Rationale**:  
By default, all pod-to-pod traffic is allowed. NetworkPolicies should be used to restrict traffic.

**Audit**:
```bash
# Check for namespaces without NetworkPolicies
kubectl get namespaces -o json | jq -r '
  .items[] | 
  select(.metadata.name != "kube-system") | 
  .metadata.name' | while read ns; do
  policies=$(kubectl get networkpolicies -n $ns -o json | jq '.items | length')
  if [ "$policies" -eq 0 ]; then
    echo "Namespace $ns has no NetworkPolicies"
  fi
done
```

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.3.1
title: "Namespace without NetworkPolicy"
severity: high
category: network
cis_section: "5.3.1"
cis_level: 2

detection:
  type: namespace
  conditions:
    - network_policies_count == 0
    - name not in ["kube-system", "kube-public", "kube-node-lease", "default"]
    - pods_count > 0  # Only flag if namespace has pods
  
risk_score_calculation:
  base: 7.5
  multipliers:
    - if: pods_count > 10
      multiply: 1.2  # 9.0
    - if: has_services_with_external_ip == true
      multiply: 1.3  # 9.75

evidence:
  - namespace: "{{ .metadata.name }}"
  - network_policies_count: 0
  - pods_count: "{{ .pods_count }}"

remediation:
  - "Create a default deny-all NetworkPolicy"
  - "kubectl apply -f - <<EOF"
  - "apiVersion: networking.k8s.io/v1"
  - "kind: NetworkPolicy"
  - "metadata:"
  - "  name: default-deny-all"
  - "  namespace: {{ .metadata.name }}"
  - "spec:"
  - "  podSelector: {}"
  - "  policyTypes:"
  - "  - Ingress"
  - "  - Egress"
  - "EOF"
  - "Then create specific allow policies as needed"
```

---

### CIS 5.3.2 - Ensure default deny NetworkPolicy is configured

**CIS ID**: 5.3.2  
**Level**: L2  
**Automated**: Yes  
**Scoring**: Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.3.2
title: "Default deny NetworkPolicy missing"
severity: medium
category: network
cis_section: "5.3.2"
cis_level: 2

detection:
  type: namespace
  conditions:
    - NOT has_default_deny_network_policy == true
    - pods_count > 0
  
default_deny_detection:
    # A policy is considered "default deny" if:
    - pod_selector is empty (matches all pods)
    - policy_types includes both "Ingress" and "Egress"
    - ingress rules are empty OR not specified
    - egress rules are empty OR not specified
  
risk_score_calculation:
  base: 6.5

remediation:
  - "Create default deny NetworkPolicy"
  - "See CIS 5.3.1 remediation for example"
```

---

## Section 5.4: Secrets Management

### CIS 5.4.1 - Prefer using secrets as files over environment variables

**CIS ID**: 5.4.1  
**Level**: L1  
**Automated**: Yes  
**Scoring**: Scored

**Description**:  
Secrets should be mounted as files rather than exposed as environment variables.

**Rationale**:  
Environment variables can be inadvertently exposed through logs or crash dumps.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.4.1
title: "Secret exposed in environment variable"
severity: medium
category: secrets
cis_section: "5.4.1"
cis_level: 1

detection:
  type: pod
  conditions:
    - containers[*].env[*].value_from.secret_key_ref is not null OR
      containers[*].env_from[*].secret_ref is not null
  
risk_score_calculation:
  base: 6.0
  multipliers:
    - if: secret_count > 3
      multiply: 1.2
    - if: container_logs_enabled == true
      multiply: 1.1

evidence:
  - pod_name: "{{ .metadata.name }}"
  - secrets_in_env: "{{ .spec.containers | selectattr('env') | map(attribute='env') | selectattr('valueFrom.secretKeyRef') | list }}"

remediation:
  - "Mount secrets as volumes instead of environment variables"
  - "Example:"
  - "volumes:"
  - "- name: secret-volume"
  - "  secret:"
  - "    secretName: my-secret"
  - "volumeMounts:"
  - "- name: secret-volume"
  - "  mountPath: /etc/secrets"
  - "  readOnly: true"
```

---

### CIS 5.4.2 - Consider external secret storage

**CIS ID**: 5.4.2  
**Level**: L2  
**Automated**: Manual  
**Scoring**: Not Scored

**Description**:  
Consider using external secret management systems (Vault, AWS Secrets Manager, etc.) instead of Kubernetes Secrets.

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.4.2
title: "External secret management not used"
severity: info
category: secrets
cis_section: "5.4.2"
cis_level: 2

detection:
  type: cluster
  conditions:
    - external_secrets_operator_installed == false
    - secrets_count > 10
  
risk_score_calculation:
  base: 3.0

remediation:
  - "Consider implementing External Secrets Operator"
  - "Install: kubectl apply -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml"
  - "Integrate with Vault, AWS Secrets Manager, or Azure Key Vault"
```

---

## Section 5.5: Extensible Admission Control

### CIS 5.5.1 - Configure Image Provenance using ImagePolicyWebhook

**CIS ID**: 5.5.1  
**Level**: L2  
**Automated**: Manual  
**Scoring**: Not Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.5.1
title: "Image provenance validation not configured"
severity: low
category: admission_control
cis_section: "5.5.1"
cis_level: 2

detection:
  type: cluster
  conditions:
    - admission_controllers not contains "ImagePolicyWebhook"
    - binary_authorization_not_configured == true
  
risk_score_calculation:
  base: 4.5

remediation:
  - "Configure ImagePolicyWebhook admission controller"
  - "Or use Binary Authorization (GKE)"
  - "Or use Kyverno/OPA for image verification"
```

---

## Section 5.6: General Policies

### CIS 5.6.1 - Create administrative boundaries using namespaces

**CIS ID**: 5.6.1  
**Level**: L1  
**Automated**: Manual  
**Scoring**: Not Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.6.1
title: "Excessive use of default namespace"
severity: low
category: general
cis_section: "5.6.1"
cis_level: 1

detection:
  type: namespace
  conditions:
    - name == "default"
    - pods_count > 10
  
risk_score_calculation:
  base: 4.0
  multipliers:
    - if: pods_count > 50
      multiply: 1.3

remediation:
  - "Create dedicated namespaces for applications"
  - "kubectl create namespace <app-name>"
  - "Avoid using the default namespace for applications"
```

---

### CIS 5.6.2 - Apply Security Context to Your Pods and Containers

**CIS ID**: 5.6.2  
**Level**: L2  
**Automated**: Manual  
**Scoring**: Not Scored

**KSAM Detection Logic**:
```yaml
rule_id: cis-5.6.2
title: "Security context not configured"
severity: medium
category: general
cis_section: "5.6.2"
cis_level: 2

detection:
  type: pod
  conditions:
    - security_context is null OR
      (security_context.run_as_non_root is null AND
       security_context.run_as_user is null AND
       security_context.seccomp_profile is null)
  
risk_score_calculation:
  base: 5.5

remediation:
  - "Configure security context for pod/container"
  - "Recommended settings:"
  - "  runAsNonRoot: true"
  - "  runAsUser: 1000"
  - "  fsGroup: 2000"
  - "  seccompProfile:"
  - "    type: RuntimeDefault"
  - "  capabilities:"
  - "    drop: [ALL]"
```

---

## Implementation Guide

### Database Schema

**Table: `cis_rules`**
```sql
CREATE TABLE cis_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id VARCHAR(20) UNIQUE NOT NULL,  -- e.g., "cis-5.1.3"
    title TEXT NOT NULL,
    description TEXT,
    severity VARCHAR(20) NOT NULL,  -- critical, high, medium, low, info
    category VARCHAR(50) NOT NULL,  -- rbac, pod_security, network, secrets, etc.
    cis_section VARCHAR(10) NOT NULL,  -- e.g., "5.1.3"
    cis_level INTEGER NOT NULL,  -- 1 or 2
    automated BOOLEAN NOT NULL DEFAULT true,
    scored BOOLEAN NOT NULL DEFAULT true,
    resource_type VARCHAR(50) NOT NULL,  -- pod, role, namespace, etc.
    detection_logic JSONB NOT NULL,
    risk_calculation JSONB NOT NULL,
    remediation JSONB NOT NULL,
    cel_expression TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cis_rules_rule_id ON cis_rules(rule_id);
CREATE INDEX idx_cis_rules_category ON cis_rules(category);
CREATE INDEX idx_cis_rules_severity ON cis_rules(severity);
CREATE INDEX idx_cis_rules_resource_type ON cis_rules(resource_type);
```

### Rule Evaluation Engine

**File: `pkg/risk/cis_evaluator.go`**
```go
package risk

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/google/cel-go/cel"
    "github.com/your-org/ksam/pkg/models"
)

type CISEvaluator struct {
    rules   []CISRule
    celEnv  *cel.Env
}

type CISRule struct {
    RuleID          string                 `json:"rule_id"`
    Title           string                 `json:"title"`
    Severity        string                 `json:"severity"`
    Category        string                 `json:"category"`
    CISSection      string                 `json:"cis_section"`
    CISLevel        int                    `json:"cis_level"`
    ResourceType    string                 `json:"resource_type"`
    DetectionLogic  map[string]interface{} `json:"detection_logic"`
    RiskCalculation map[string]interface{} `json:"risk_calculation"`
    Remediation     []string               `json:"remediation"`
    CELExpression   string                 `json:"cel_expression,omitempty"`
    Enabled         bool                   `json:"enabled"`
}

func NewCISEvaluator() (*CISEvaluator, error) {
    // Load CIS rules from database
    rules, err := loadCISRules()
    if err != nil {
        return nil, fmt.Errorf("failed to load CIS rules: %w", err)
    }
    
    // Initialize CEL environment
    celEnv, err := cel.NewEnv(
        cel.Types(&models.Pod{}, &models.Role{}, &models.Namespace{}),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create CEL environment: %w", err)
    }
    
    return &CISEvaluator{
        rules:  rules,
        celEnv: celEnv,
    }, nil
}

func (e *CISEvaluator) EvaluatePod(ctx context.Context, pod *models.Pod) ([]*models.Insight, error) {
    var insights []*models.Insight
    
    for _, rule := range e.rules {
        if !rule.Enabled || rule.ResourceType != "pod" {
            continue
        }
        
        // Evaluate using CEL if expression is provided
        if rule.CELExpression != "" {
            match, err := e.evaluateCEL(rule.CELExpression, pod)
            if err != nil {
                return nil, fmt.Errorf("CEL evaluation failed for %s: %w", rule.RuleID, err)
            }
            
            if !match {
                continue
            }
        } else {
            // Fallback to condition-based evaluation
            match, err := e.evaluateConditions(rule.DetectionLogic, pod)
            if err != nil {
                return nil, err
            }
            
            if !match {
                continue
            }
        }
        
        // Calculate risk score
        riskScore := e.calculateRiskScore(rule, pod)
        
        // Create insight
        insight := &models.Insight{
            RuleID:       rule.RuleID,
            Title:        rule.Title,
            Severity:     rule.Severity,
            Category:     rule.Category,
            CISSection:   rule.CISSection,
            CISLevel:     rule.CISLevel,
            ResourceType: "Pod",
            ResourceID:   pod.ID,
            RiskScore:    riskScore,
            Status:       "active",
            Evidence:     e.extractEvidence(rule, pod),
            Remediation:  rule.Remediation,
        }
        
        insights = append(insights, insight)
    }
    
    return insights, nil
}

func (e *CISEvaluator) evaluateCEL(expression string, obj interface{}) (bool, error) {
    ast, issues := e.celEnv.Compile(expression)
    if issues != nil && issues.Err() != nil {
        return false, issues.Err()
    }
    
    prg, err := e.celEnv.Program(ast)
    if err != nil {
        return false, err
    }
    
    out, _, err := prg.Eval(map[string]interface{}{
        "object": obj,
    })
    if err != nil {
        return false, err
    }
    
    result, ok := out.Value().(bool)
    if !ok {
        return false, fmt.Errorf("CEL expression did not return boolean")
    }
    
    return result, nil
}

func (e *CISEvaluator) calculateRiskScore(rule CISRule, obj interface{}) float64 {
    riskCalc, ok := rule.RiskCalculation["base"]
    if !ok {
        return 5.0 // Default
    }
    
    baseScore, ok := riskCalc.(float64)
    if !ok {
        return 5.0
    }
    
    score := baseScore
    
    // Apply multipliers
    if multipliers, ok := rule.RiskCalculation["multipliers"].([]interface{}); ok {
        for _, m := range multipliers {
            mult, ok := m.(map[string]interface{})
            if !ok {
                continue
            }
            
            // Check condition
            if condition, ok := mult["if"].(string); ok {
                match, _ := e.evaluateCEL(condition, obj)
                if match {
                    if factor, ok := mult["multiply"].(float64); ok {
                        score *= factor
                    }
                }
            }
        }
    }
    
    // Cap at 10.0
    if score > 10.0 {
        score = 10.0
    }
    
    return score
}

func loadCISRules() ([]CISRule, error) {
    // Load from database or config file
    // This is a placeholder
    return []CISRule{}, nil
}
```

---

## CEL Expressions

### Common CEL Patterns

**1. Check for wildcard in RBAC**:
```cel
object.rules.exists(r, 
  r.resources.exists(res, res == "*") ||
  r.verbs.exists(v, v == "*") ||
  (has(r.apiGroups) && r.apiGroups.exists(g, g == "*"))
)
```

**2. Check for privileged container**:
```cel
object.spec.containers.exists(c, 
  has(c.securityContext) && 
  has(c.securityContext.privileged) && 
  c.securityContext.privileged == true
)
```

**3. Check for host path mount**:
```cel
object.spec.volumes.exists(v, 
  has(v.hostPath) && 
  v.hostPath.path in ["/", "/etc", "/var/run/docker.sock"]
)
```

**4. Check for default service account**:
```cel
!has(object.spec.serviceAccountName) || 
object.spec.serviceAccountName == "default"
```

---

## Test Cases

### Test Data Generator

```bash
#!/bin/bash
# generate-cis-test-data.sh

kubectl create namespace cis-test

# CIS 5.1.3: Wildcard in role
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: wildcard-role
  namespace: cis-test
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
EOF

# CIS 5.2.1: Privileged pod
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: privileged-pod
  namespace: cis-test
spec:
  containers:
  - name: nginx
    image: nginx
    securityContext:
      privileged: true
EOF

# CIS 5.2.4: hostNetwork
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: hostnetwork-pod
  namespace: cis-test
spec:
  hostNetwork: true
  containers:
  - name: nginx
    image: nginx
EOF

# CIS 5.3.1: Namespace without NetworkPolicy
# (already created with no policies)

# CIS 5.4.1: Secret in env var
kubectl create secret generic test-secret --from-literal=key=value -n cis-test
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: secret-env-pod
  namespace: cis-test
spec:
  containers:
  - name: nginx
    image: nginx
    env:
    - name: SECRET_KEY
      valueFrom:
        secretKeyRef:
          name: test-secret
          key: key
EOF

echo "CIS test data created in namespace cis-test"
```

### Validation Script

```bash
#!/bin/bash
# validate-cis-detection.sh

echo "=== CIS Detection Validation ==="

# Test CIS 5.1.3
echo "Testing CIS 5.1.3 (wildcard in role)..."
INSIGHTS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?rule_id=cis-5.1.3" | jq '.data | length')
if [ "$INSIGHTS" -gt 0 ]; then
  echo "✅ CIS 5.1.3 detected: $INSIGHTS insights"
else
  echo "❌ CIS 5.1.3 NOT detected"
fi

# Test CIS 5.2.1
echo "Testing CIS 5.2.1 (privileged container)..."
INSIGHTS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?rule_id=cis-5.2.1" | jq '.data | length')
if [ "$INSIGHTS" -gt 0 ]; then
  echo "✅ CIS 5.2.1 detected: $INSIGHTS insights"
else
  echo "❌ CIS 5.2.1 NOT detected"
fi

# Test CIS 5.3.1
echo "Testing CIS 5.3.1 (missing NetworkPolicy)..."
INSIGHTS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?rule_id=cis-5.3.1" | jq '.data | length')
if [ "$INSIGHTS" -gt 0 ]; then
  echo "✅ CIS 5.3.1 detected: $INSIGHTS insights"
else
  echo "❌ CIS 5.3.1 NOT detected"
fi

# Test CIS 5.4.1
echo "Testing CIS 5.4.1 (secret in env var)..."
INSIGHTS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?rule_id=cis-5.4.1" | jq '.data | length')
if [ "$INSIGHTS" -gt 0 ]; then
  echo "✅ CIS 5.4.1 detected: $INSIGHTS insights"
else
  echo "❌ CIS 5.4.1 NOT detected"
fi

echo ""
echo "=== Validation Complete ==="
```

---

## Summary

This implementation provides:

✅ **26 CIS Benchmark rules** across Section 5 (Policies)  
✅ **Complete detection logic** for each rule  
✅ **Risk scoring calculations** with multipliers  
✅ **CEL expressions** for evaluation  
✅ **Remediation steps** for each violation  
✅ **Database schema** for storing rules  
✅ **Go implementation** example  
✅ **Test data generator** and validation  

**Next Steps**:
1. Load rules into database
2. Implement CISEvaluator in Core
3. Integrate with Risk Engine
4. Add to API endpoints
5. Create dashboard visualizations
6. Generate compliance reports

---

**End of CIS Benchmark Implementation**
