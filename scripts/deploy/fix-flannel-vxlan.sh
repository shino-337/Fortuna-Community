#!/bin/bash

# ============================================================================
# Flannel VXLAN Configuration Fix Script
# ============================================================================
# This script fixes Flannel VXLAN tunnel issues for multi-node Kubernetes
# clusters. If Flannel is not installed (no kube-flannel-cfg ConfigMap),
# the script exits successfully (cluster may use Calico, Cilium, or other CNI).
#
# Steps when Flannel is present:
# 1. Verifying Flannel ConfigMap configuration
# 2. Checking Node PodCIDR assignments
# 3. Restarting Flannel DaemonSet to reinitialize VXLAN
# 4. Verifying VXLAN interfaces and routes
#
# Env: CNI_WAIT_SECONDS – seconds to wait after Flannel restart (default 30).
#      SKIP_FLANNEL_RESTART_IF_HEALTHY=true (default) to avoid unnecessary restart-induced pod churn.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} ✅ $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} ❌ $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} ⚠️  $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Step 1: Verify Flannel ConfigMap
# Returns: 0 = OK, 1 = error (wrong config), 2 = Flannel not installed (caller may skip)
verify_flannel_configmap() {
    log_info "Step 1: Verifying Flannel ConfigMap..."
    
    if ! kubectl get configmap kube-flannel-cfg -n kube-flannel &> /dev/null; then
        log_warning "Flannel ConfigMap not found. Cluster may use another CNI (Calico, Cilium, etc.). Skipping."
        return 2
    fi
    
    local network=$(kubectl get configmap kube-flannel-cfg -n kube-flannel -o jsonpath='{.data.net-conf\.json}' | grep -o '"Network":\s*"[^"]*"' | cut -d'"' -f4)
    local backend=$(kubectl get configmap kube-flannel-cfg -n kube-flannel -o jsonpath='{.data.net-conf\.json}' | grep -o '"Type":\s*"[^"]*"' | cut -d'"' -f4)
    
    if [ -z "$network" ] || [ -z "$backend" ]; then
        log_error "Could not parse Flannel ConfigMap"
        return 1
    fi
    
    if [ "$network" != "10.244.0.0/16" ]; then
        log_warning "Flannel Network is $network (expected: 10.244.0.0/16)"
        log_warning "This may cause routing issues. Consider updating ConfigMap."
    else
        log_success "Flannel Network: $network"
    fi
    
    if [ "$backend" != "vxlan" ]; then
        log_error "Flannel Backend is $backend (expected: vxlan)"
        log_error "VXLAN backend is required for multi-node routing"
        return 1
    else
        log_success "Flannel Backend: $backend"
    fi
}

# Step 2: Verify Node PodCIDR
verify_node_podcidr() {
    log_info "Step 2: Verifying Node PodCIDR assignments..."
    
    local nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
    local all_ok=true
    
    for node in $nodes; do
        local podcidr=$(kubectl get node "$node" -o jsonpath='{.spec.podCIDR}')
        if [ -z "$podcidr" ]; then
            log_error "Node $node has no PodCIDR assigned"
            log_error "kube-controller-manager may not be running or configured correctly"
            all_ok=false
        else
            log_success "Node $node: PodCIDR = $podcidr"
        fi
    done
    
    if [ "$all_ok" = false ]; then
        log_error "Some nodes are missing PodCIDR assignments"
        return 1
    fi
}

# Step 3: Restart Flannel DaemonSet
# Returns: 0 = restarted OK, 1 = rollout error, 2 = no Flannel DS (other CNI or incomplete install — not fatal for deploy)
restart_flannel() {
    local skip_if_healthy="${SKIP_FLANNEL_RESTART_IF_HEALTHY:-true}"
    log_info "Step 3: Restarting Flannel DaemonSet..."

    local ds_name=""
    if kubectl get daemonset kube-flannel-ds -n kube-flannel &> /dev/null; then
        ds_name="kube-flannel-ds"
    else
        # Helm / arch-specific manifests may use a different DaemonSet name
        ds_name=$(kubectl get ds -n kube-flannel -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        if [ -n "$ds_name" ]; then
            log_info "Using DaemonSet $ds_name (kube-flannel-ds not present)"
        fi
    fi

    if [ -z "$ds_name" ]; then
        log_warning "Flannel DaemonSet not found in namespace kube-flannel (expected kube-flannel-ds)."
        log_warning "Common: cluster uses Calico, Cilium, Canal, or Weave — Flannel fix does not apply."
        log_info "Diagnostics: kubectl get ds -A | grep -iE 'flannel|calico|cilium|weave'"
        return 2
    fi

    local desired ready
    desired=$(kubectl get ds "$ds_name" -n kube-flannel -o jsonpath='{.status.desiredNumberScheduled}' 2>/dev/null || echo "0")
    ready=$(kubectl get ds "$ds_name" -n kube-flannel -o jsonpath='{.status.numberReady}' 2>/dev/null || echo "0")

    if [ "$skip_if_healthy" = "true" ] && [ "${desired:-0}" -gt 0 ] && [ "${ready:-0}" -ge "${desired:-0}" ]; then
        log_success "Flannel DaemonSet already healthy ($ready/$desired). Skipping restart to avoid pod sandbox churn."
        return 0
    fi

    if kubectl rollout restart daemonset "$ds_name" -n kube-flannel; then
        log_success "Flannel DaemonSet restart initiated ($ds_name)"
        CNI_WAIT="${CNI_WAIT_SECONDS:-30}"
        log_info "Waiting for Flannel pods to restart (${CNI_WAIT} seconds)..."
        sleep "$CNI_WAIT"

        ready=$(kubectl get ds "$ds_name" -n kube-flannel -o jsonpath='{.status.numberReady}' 2>/dev/null || echo "0")
        desired=$(kubectl get ds "$ds_name" -n kube-flannel -o jsonpath='{.status.desiredNumberScheduled}' 2>/dev/null || echo "0")

        if [ "$ready" -eq "$desired" ] && [ "$desired" -gt 0 ]; then
            log_success "Flannel pods restarted and ready ($ready/$desired)"
        else
            log_warning "Flannel pods may not be fully ready yet ($ready/$desired)"
        fi
    else
        log_error "Failed to restart Flannel DaemonSet ($ds_name)"
        return 1
    fi
}


# Step 4: Verify VXLAN Interfaces (via Flannel pods with hostNetwork)
verify_vxlan_interfaces() {
    log_info "Step 4: Verifying VXLAN interfaces (via Flannel pods on each node)..."
    
    local nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
    local all_ok=true
    
    for node in $nodes; do
        local flannel_pod=$(kubectl get pods -n kube-flannel -l app=flannel --field-selector=spec.nodeName="$node",status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -z "$flannel_pod" ]; then
            log_warning "No Flannel pod on node $node, skipping VXLAN check"
            continue
        fi
        
        local out
        if out=$(kubectl exec -n kube-flannel "$flannel_pod" -- ip addr show flannel.1 2>/dev/null); then
            if echo "$out" | grep -q 'inet 10\.244\.'; then
                local inet=$(echo "$out" | grep 'inet 10\.244\.' | head -1 | sed -n 's/.*inet \(10\.244\.[^/]*\).*/\1/p')
                log_success "Node $node: flannel.1 has IPv4 $inet"
            else
                log_warning "Node $node: flannel.1 has no IPv4 10.244.x.x (may be initializing)"
                all_ok=false
            fi
        else
            log_warning "Node $node: could not run 'ip addr show flannel.1' in Flannel pod (container may lack ip command)"
            log_info "Manual check: kubectl exec -n kube-flannel -it <flannel-pod> -- ip addr show flannel.1"
            all_ok=false
        fi
    done
    
    if [ "$all_ok" = true ]; then
        log_success "VXLAN interfaces verified on all nodes"
    else
        log_info "Expected: inet 10.244.x.0/32 on flannel.1 per node"
    fi
}

# Step 5: Verify Routes (via Flannel pods with hostNetwork)
verify_routes() {
    log_info "Step 5: Verifying pod-to-pod routes (via Flannel pods on each node)..."
    
    local nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
    local all_ok=true
    
    for node in $nodes; do
        local flannel_pod=$(kubectl get pods -n kube-flannel -l app=flannel --field-selector=spec.nodeName="$node",status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -z "$flannel_pod" ]; then
            log_warning "No Flannel pod on node $node, skipping route check"
            continue
        fi
        
        local routes
        routes=$(kubectl exec -n kube-flannel "$flannel_pod" -- ip route 2>/dev/null) || true
        if [ -n "$routes" ]; then
            local count=$(echo "$routes" | grep -c '10\.244\.' 2>/dev/null || echo "0")
            if [ "$count" -ge 1 ]; then
                log_success "Node $node: $count route(s) for 10.244.x.x via flannel.1"
            else
                log_warning "Node $node: no 10.244 routes yet (cross-node may fail until Flannel stabilizes)"
                all_ok=false
            fi
        else
            log_warning "Node $node: could not run 'ip route' in Flannel pod (container may lack ip command)"
            log_info "Expected: 10.244.x.0/24 via 10.244.x.0 dev flannel.1 for other subnets"
            all_ok=false
        fi
    done
    
    if [ "$all_ok" = true ]; then
        log_success "Pod-to-pod routes verified on all nodes"
    fi
}

# Step 6: Test Pod-to-Pod Connectivity
test_connectivity() {
    log_info "Step 6: Testing pod-to-pod connectivity..."
    
    # Get Core service IP (may not exist yet if run before Core is deployed)
    local core_svc_ip=$(kubectl get svc fortuna-core -n fortuna -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
    
    if [ -n "$core_svc_ip" ]; then
        # Get Agent pod on worker node (if exists)
        local agent_pod=""
        while IFS= read -r line; do
            local name node
            name="${line%%	*}"
            node="${line##*	}"
            node=$(echo "$node" | tr -d ' ')
            if [ "$node" != "k8s-master" ] && [ -n "$node" ]; then
                agent_pod="$name"
                break
            fi
        done < <(kubectl get pods -n fortuna -l app.kubernetes.io/component=agent --field-selector=status.phase=Running -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.nodeName}{"\n"}{end}' 2>/dev/null)
        if [ -z "$agent_pod" ]; then
            agent_pod=$(kubectl get pods -n fortuna -l app.kubernetes.io/name=fortuna -l app.kubernetes.io/component=agent --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        fi
        
        if [ -n "$agent_pod" ]; then
            log_info "Testing connection from Agent pod ($agent_pod) to Core ($core_svc_ip:8080)..."
            local connected=false
            # Prefer curl (included in Agent image); fallback wget/nc
            if kubectl exec -n fortuna "$agent_pod" -- sh -c "command -v curl >/dev/null 2>&1 && curl -sf --max-time 3 http://${core_svc_ip}:8080/healthz" 2>/dev/null | grep -q ok; then
                connected=true
            elif kubectl exec -n fortuna "$agent_pod" -- wget -q -O- --timeout=3 "http://${core_svc_ip}:8080/healthz" 2>/dev/null | grep -q ok; then
                connected=true
            elif kubectl exec -n fortuna "$agent_pod" -- sh -c "command -v wget >/dev/null && wget -q -O- --timeout=3 http://${core_svc_ip}:8080/healthz 2>/dev/null" | grep -q ok; then
                connected=true
            elif kubectl exec -n fortuna "$agent_pod" -- sh -c "command -v nc >/dev/null && nc -z -w3 ${core_svc_ip} 8080 2>/dev/null"; then
                connected=true
            fi
            if [ "$connected" = true ]; then
                log_success "Pod-to-pod connectivity test passed (Agent -> Core HTTP)"
            else
                log_warning "Pod-to-pod connectivity test failed or Agent image has no curl/wget/nc. Rebuild agent after Dockerfile adds curl, or verify: kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=20"
            fi
        else
            log_warning "No Agent pod found. Deploy Core and Agent first, then re-run or check: kubectl logs -n fortuna -l app.kubernetes.io/component=agent"
        fi
    else
        log_info "Core service not deployed yet (normal when Step 4b runs before Step 8)."
        log_info "After full deploy, verify connectivity: kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=10 | grep -E 'Connected|Heartbeat'"
        # Optional: test cross-node DNS from any pod to prove pod network
        local dns_ip=$(kubectl get svc -n kube-system kube-dns -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
        if [ -n "$dns_ip" ]; then
            local test_pod=$(kubectl get pods -n kube-flannel -l app=flannel --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
            if [ -n "$test_pod" ]; then
                if kubectl exec -n kube-flannel "$test_pod" -- nslookup kubernetes.default.svc.cluster.local "$dns_ip" >/dev/null 2>&1; then
                    log_success "DNS reachable from Flannel pod (pod network OK)"
                fi
            fi
        fi
    fi
}

# Main
main() {
    echo "=========================================="
    echo "Fortuna Flannel VXLAN Configuration Fix"
    echo "=========================================="
    echo ""
    
    check_prerequisites
    echo ""
    
    log_info "Starting Flannel VXLAN configuration fixes..."
    echo ""
    
    # Step 1: Verify ConfigMap (skip entire script if Flannel not installed)
    verify_flannel_configmap
    r=$?
    if [ $r -eq 2 ]; then
        log_info "Flannel not installed; skipping (optional for non-Flannel clusters)"
        exit 0
    fi
    if [ $r -ne 0 ]; then
        log_error "Flannel ConfigMap verification failed"
        exit 1
    fi
    echo ""
    
    # Step 2: Verify PodCIDR
    if ! verify_node_podcidr; then
        log_error "Node PodCIDR verification failed"
        exit 1
    fi
    echo ""
    
    # Step 3: Restart Flannel
    restart_flannel
    fr=$?
    if [ "$fr" -eq 2 ]; then
        log_warning "Skipping Flannel restart (no Flannel DaemonSet). OK if your CNI is not Flannel."
        echo ""
        test_connectivity
        echo ""
        log_success "Flannel VXLAN script finished (restart skipped — use Calico/Cilium/… or install Flannel if needed)"
        exit 0
    fi
    if [ "$fr" -ne 0 ]; then
        log_error "Flannel restart failed"
        exit 1
    fi
    echo ""
    
    # Step 4: Verify VXLAN (informational)
    verify_vxlan_interfaces
    echo ""
    
    # Step 5: Verify Routes (informational)
    verify_routes
    echo ""
    
    # Step 6: Test Connectivity
    test_connectivity
    echo ""
    
    log_success "Flannel VXLAN configuration fix completed"
    echo ""
    echo "Next steps:"
    echo "  1. Wait 30-60 seconds for VXLAN to fully initialize"
    echo "  2. Verify VXLAN interfaces: ip addr show flannel.1 (on each node)"
    echo "  3. Verify routes: ip route | grep 10.244 (on each node)"
    echo "  4. Monitor Agent logs: kubectl logs -n fortuna -l app=fortuna-agent --tail=20"
    echo "  5. Look for 'Heartbeat successful' messages in Agent logs"
    echo ""
    echo "If Agent still cannot connect:"
    echo "  - Check Flannel pod logs: kubectl logs -n kube-flannel -l app=flannel"
    echo "  - Verify network routes on nodes"
    echo "  - Check firewall rules"
    echo ""
}

main "$@"


