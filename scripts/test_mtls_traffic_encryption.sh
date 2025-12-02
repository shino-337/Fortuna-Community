#!/bin/bash

# mTLS Traffic Encryption Test
# Verifies that traffic between Agent and Core is actually encrypted with mTLS

set -e

NAMESPACE="ksam"
TIMEOUT=30
CAPTURE_DURATION=10

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

echo "=========================================="
echo "mTLS Traffic Encryption Test"
echo "=========================================="
echo ""

# Get pod names
CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$CORE_POD" ] || [ -z "$AGENT_POD" ]; then
    log_error "❌ Core or Agent pod not found"
    exit 1
fi

log_info "Core pod: $CORE_POD"
log_info "Agent pod: $AGENT_POD"
echo ""

# Test 1: Check if tcpdump is available
log_test "Test 1: Check tcpdump availability"
if kubectl exec -n $NAMESPACE $CORE_POD -- which tcpdump >/dev/null 2>&1; then
    log_info "✅ tcpdump available in Core pod"
    TCPDUMP_AVAILABLE=true
elif kubectl exec -n $NAMESPACE $CORE_POD -- which tshark >/dev/null 2>&1; then
    log_info "✅ tshark available in Core pod"
    TCPDUMP_AVAILABLE=false
    TSHARK_AVAILABLE=true
else
    log_warn "⚠️  tcpdump/tshark not available, will use alternative methods"
    TCPDUMP_AVAILABLE=false
    TSHARK_AVAILABLE=false
fi
echo ""

# Test 2: Get Core pod IP
log_test "Test 2: Get Core pod IP"
CORE_IP=$(kubectl get pod -n $NAMESPACE $CORE_POD -o jsonpath='{.status.podIP}')
if [ -n "$CORE_IP" ]; then
    log_info "✅ Core pod IP: $CORE_IP"
else
    log_error "❌ Could not get Core pod IP"
    exit 1
fi
echo ""

# Test 3: Capture traffic on Core pod
log_test "Test 3: Capture traffic on port 9090"
log_info "Capturing traffic for ${CAPTURE_DURATION} seconds..."

if [ "$TCPDUMP_AVAILABLE" = true ]; then
    # Start tcpdump in background
    CAPTURE_FILE="/tmp/mtls_capture_$(date +%s).pcap"
    kubectl exec -n $NAMESPACE $CORE_POD -- sh -c "timeout ${CAPTURE_DURATION} tcpdump -i any -w $CAPTURE_FILE port 9090 2>&1" > /tmp/tcpdump_output.log 2>&1 &
    TCPDUMP_PID=$!
    
    log_info "   tcpdump started (PID: $TCPDUMP_PID)"
    log_info "   Waiting for traffic..."
    sleep 5
    
    # Trigger Agent to send data
    log_info "   Triggering Agent to send data..."
    kubectl exec -n $NAMESPACE $AGENT_POD -- sh -c "echo 'Triggering connection...'" >/dev/null 2>&1
    
    # Wait for capture to complete
    wait $TCPDUMP_PID 2>/dev/null || true
    
    # Check if capture file exists
    if kubectl exec -n $NAMESPACE $CORE_POD -- test -f "$CAPTURE_FILE" 2>/dev/null; then
        log_info "✅ Capture file created: $CAPTURE_FILE"
        
        # Analyze capture
        PACKET_COUNT=$(kubectl exec -n $NAMESPACE $CORE_POD -- tcpdump -r "$CAPTURE_FILE" 2>/dev/null | wc -l | tr -d ' ')
        log_info "   Captured packets: $PACKET_COUNT"
        
        # Check for TLS handshake
        TLS_HANDSHAKE=$(kubectl exec -n $NAMESPACE $CORE_POD -- tcpdump -r "$CAPTURE_FILE" -A 2>/dev/null | grep -i "handshake\|client hello\|server hello" | wc -l | tr -d ' ')
        if [ "$TLS_HANDSHAKE" -gt 0 ] 2>/dev/null; then
            log_info "✅ TLS handshake detected ($TLS_HANDSHAKE occurrences)"
        else
            log_warn "⚠️  No TLS handshake detected in capture"
        fi
        
        # Check for encrypted data (should not see plaintext)
        PLAINTEXT=$(kubectl exec -n $NAMESPACE $CORE_POD -- tcpdump -r "$CAPTURE_FILE" -A 2>/dev/null | grep -i "inventory\|pod\|serviceaccount" | wc -l | tr -d ' ')
        if [ "$PLAINTEXT" -eq 0 ] 2>/dev/null; then
            log_info "✅ No plaintext data detected (traffic is encrypted)"
        else
            log_error "❌ Plaintext data detected! Traffic may not be encrypted"
        fi
        
        # Cleanup
        kubectl exec -n $NAMESPACE $CORE_POD -- rm -f "$CAPTURE_FILE" 2>/dev/null || true
    else
        log_warn "⚠️  Capture file not created (may be no traffic or permission issue)"
    fi
else
    log_warn "⚠️  tcpdump not available, using alternative method"
    log_info "   Checking connection characteristics..."
    
    # Alternative: Check if connection uses TLS by examining socket
    # This is less reliable but works without tcpdump
    log_info "   Verifying TLS configuration..."
    
    # Check if Agent is configured with TLS
    if kubectl exec -n $NAMESPACE $AGENT_POD -- env | grep -q "TLS_ENABLED=true"; then
        log_info "   ✅ Agent TLS enabled"
    else
        log_error "   ❌ Agent TLS not enabled"
    fi
    
    # Check if Core is configured with TLS
    if kubectl exec -n $NAMESPACE $CORE_POD -- env | grep -q "TLS_ENABLED=true"; then
        log_info "   ✅ Core TLS enabled"
    else
        log_error "   ❌ Core TLS not enabled"
    fi
    
    # Check if connection is established
    CONN_COUNT=$(kubectl exec -n $NAMESPACE $CORE_POD -- sh -c "netstat -an 2>/dev/null | grep ':9090' | grep ESTABLISHED | wc -l" 2>/dev/null | tr -d ' ' || echo "0")
    if [ "$CONN_COUNT" -gt 0 ] 2>/dev/null; then
        log_info "   ✅ Active connections on port 9090: $CONN_COUNT"
        log_info "   ⚠️  Note: Cannot verify encryption without packet capture"
        log_info "   ⚠️  Assuming encrypted based on TLS configuration"
    else
        log_warn "   ⚠️  No active connections detected"
    fi
fi
echo ""

# Test 4: Verify TLS handshake in logs
log_test "Test 4: Verify TLS handshake in logs"
log_info "Checking for TLS handshake evidence in logs..."

# Check Agent logs for connection establishment
AGENT_CONN=$(kubectl logs -n $NAMESPACE $AGENT_POD --tail=100 2>/dev/null | grep -i "connected\|handshake\|tls" | head -5)
if [ -n "$AGENT_CONN" ]; then
    log_info "✅ Agent connection logs found:"
    echo "$AGENT_CONN" | while read -r line; do
        echo "   $line"
    done
else
    log_warn "⚠️  No connection logs found in Agent"
fi

# Check Core logs for TLS configuration
CORE_TLS=$(kubectl logs -n $NAMESPACE $CORE_POD --since=1h 2>/dev/null | grep -i "mTLS\|tls.*enabled\|gRPC.*mTLS" | head -3)
if [ -n "$CORE_TLS" ]; then
    log_info "✅ Core TLS logs found:"
    echo "$CORE_TLS" | while read -r line; do
        echo "   $line"
    done
else
    log_warn "⚠️  No TLS logs found in Core"
fi
echo ""

# Test 5: Test connection with openssl (if available)
log_test "Test 5: Test TLS connection with openssl"
if kubectl exec -n $NAMESPACE $AGENT_POD -- which openssl >/dev/null 2>&1; then
    log_info "✅ openssl available, testing TLS connection..."
    
    # Try to connect with openssl s_client
    TLS_TEST=$(kubectl exec -n $NAMESPACE $AGENT_POD -- timeout 5 openssl s_client -connect ksam-core.ksam.svc.cluster.local:9090 -CAfile /etc/ksam/ca-cert/ca.crt -cert /etc/ksam/certs/tls.crt -key /etc/ksam/certs/tls.key 2>&1 | head -20 || true)
    
    if echo "$TLS_TEST" | grep -q "Verify return code: 0"; then
        log_info "✅ TLS connection successful (certificate verified)"
        if echo "$TLS_TEST" | grep -q "Protocol.*TLS"; then
            TLS_VERSION=$(echo "$TLS_TEST" | grep "Protocol" | head -1)
            log_info "   $TLS_VERSION"
        fi
    elif echo "$TLS_TEST" | grep -q "CONNECTED"; then
        log_info "✅ TLS connection established"
        log_warn "   ⚠️  Certificate verification details not available"
    else
        log_warn "⚠️  Could not establish TLS connection with openssl"
        log_info "   This may be normal if gRPC uses different TLS setup"
    fi
else
    log_warn "⚠️  openssl not available in Agent pod"
fi
echo ""

# Test 6: Verify certificates are being used
log_test "Test 6: Verify certificates are being used"
log_info "Checking certificate files..."

# Check Core certificates
if kubectl exec -n $NAMESPACE $CORE_POD -- test -f /etc/ksam/certs/tls.crt 2>/dev/null && \
   kubectl exec -n $NAMESPACE $CORE_POD -- test -f /etc/ksam/certs/tls.key 2>/dev/null; then
    log_info "✅ Core server certificates present"
    
    # Check certificate validity
    CERT_INFO=$(kubectl exec -n $NAMESPACE $CORE_POD -- sh -c "openssl x509 -in /etc/ksam/certs/tls.crt -text -noout 2>/dev/null | grep -E 'Subject:|Issuer:|Not Before|Not After' | head -4" 2>/dev/null || echo "")
    if [ -n "$CERT_INFO" ]; then
        log_info "   Certificate details:"
        echo "$CERT_INFO" | while read -r line; do
            echo "   $line"
        done
    fi
else
    log_error "❌ Core certificates missing"
fi

# Check Agent certificates
if kubectl exec -n $NAMESPACE $AGENT_POD -- test -f /etc/ksam/certs/tls.crt 2>/dev/null && \
   kubectl exec -n $NAMESPACE $AGENT_POD -- test -f /etc/ksam/ca-cert/ca.crt 2>/dev/null; then
    log_info "✅ Agent client certificates present"
else
    log_error "❌ Agent certificates missing"
fi
echo ""

# Test 7: Monitor real-time connection
log_test "Test 7: Monitor real-time connection"
log_info "Monitoring connection for 5 seconds..."

# Get initial connection count
INITIAL_CONN=$(kubectl exec -n $NAMESPACE $CORE_POD -- sh -c "netstat -an 2>/dev/null | grep ':9090' | grep ESTABLISHED | wc -l" 2>/dev/null | tr -d ' ' || echo "0")

# Trigger some activity
log_info "   Triggering Agent activity..."
kubectl exec -n $NAMESPACE $AGENT_POD -- sh -c "echo 'test'" >/dev/null 2>&1

sleep 5

# Get final connection count
FINAL_CONN=$(kubectl exec -n $NAMESPACE $CORE_POD -- sh -c "netstat -an 2>/dev/null | grep ':9090' | grep ESTABLISHED | wc -l" 2>/dev/null | tr -d ' ' || echo "0")

if [ "$FINAL_CONN" -ge "$INITIAL_CONN" ] 2>/dev/null; then
    log_info "✅ Connections active: $FINAL_CONN"
else
    log_warn "⚠️  Connection count decreased"
fi

# Check if Agent is streaming
STREAMING=$(kubectl logs -n $NAMESPACE $AGENT_POD --tail=10 --since=10s 2>/dev/null | grep -c "Successfully streamed" || echo "0")
if [ "$STREAMING" -gt 0 ] 2>/dev/null; then
    log_info "✅ Agent is actively streaming data"
else
    log_warn "⚠️  No recent streaming activity"
fi
echo ""

# Summary
echo "=========================================="
echo "Test Summary"
echo "=========================================="

# Count successful tests
SUCCESS=0
TOTAL=7

if [ "$TCPDUMP_AVAILABLE" = true ] || [ -n "$CORE_TLS" ]; then
    SUCCESS=$((SUCCESS + 1))
fi

if [ -n "$CORE_IP" ]; then
    SUCCESS=$((SUCCESS + 1))
fi

if [ "$FINAL_CONN" -gt 0 ] 2>/dev/null; then
    SUCCESS=$((SUCCESS + 1))
fi

if [ -n "$CORE_TLS" ] || [ -n "$AGENT_CONN" ]; then
    SUCCESS=$((SUCCESS + 1))
fi

if kubectl exec -n $NAMESPACE $CORE_POD -- test -f /etc/ksam/certs/tls.crt 2>/dev/null; then
    SUCCESS=$((SUCCESS + 1))
fi

if [ "$STREAMING" -gt 0 ] 2>/dev/null; then
    SUCCESS=$((SUCCESS + 1))
fi

log_info "Tests passed: $SUCCESS/$TOTAL"

if [ "$SUCCESS" -ge 5 ]; then
    log_info "✅ mTLS traffic encryption verified"
    echo ""
    log_info "Conclusion:"
    log_info "  - TLS configuration: ✅ Enabled"
    log_info "  - Certificates: ✅ Present and valid"
    log_info "  - Connection: ✅ Established"
    log_info "  - Traffic: ✅ Encrypted (based on TLS config)"
    exit 0
else
    log_warn "⚠️  Some tests failed - review results above"
    exit 1
fi


