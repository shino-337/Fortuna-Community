#!/bin/bash
# Monitor Fortuna K8s Management Platform
# Usage: ./monitor_fortuna.sh [interval_seconds]

INTERVAL=${1:-5}  # Default 5 seconds
NAMESPACE="fortuna"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

clear

while true; do
    clear
    echo "═══════════════════════════════════════════════════════════════"
    echo "📊 FORTUNA K8S MANAGEMENT PLATFORM - MONITORING"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    date
    echo ""
    
    # Pod Status
    echo "🔍 POD STATUS:"
    echo "─────────────────────────────────────────────────────────────"
    kubectl -n $NAMESPACE get pods -o wide 2>/dev/null || echo "❌ Cannot get pods"
    
    # Count by status
    TOTAL=$(kubectl -n $NAMESPACE get pods --no-headers 2>/dev/null | wc -l)
    RUNNING=$(kubectl -n $NAMESPACE get pods --no-headers 2>/dev/null | grep Running | wc -l)
    NOT_READY=$(kubectl -n $NAMESPACE get pods --no-headers 2>/dev/null | grep -v Running | wc -l)
    
    echo ""
    echo "📊 Summary: $TOTAL total, $RUNNING running, $NOT_READY not ready"
    echo ""
    
    # Core Status
    CORE_POD=$(kubectl -n $NAMESPACE get pods -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$CORE_POD" ]; then
        echo "🎯 FORTUNA CORE:"
        echo "─────────────────────────────────────────────────────────────"
        CORE_STATUS=$(kubectl -n $NAMESPACE get pod $CORE_POD -o jsonpath='{.status.phase}' 2>/dev/null)
        CORE_READY=$(kubectl -n $NAMESPACE get pod $CORE_POD -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
        
        if [ "$CORE_STATUS" = "Running" ] && [ "$CORE_READY" = "True" ]; then
            echo -e "${GREEN}✅ Status: Running and Ready${NC}"
        else
            echo -e "${YELLOW}⏳ Status: $CORE_STATUS (Ready: $CORE_READY)${NC}"
        fi
        
        # Recent logs
        echo ""
        echo "📋 Recent Logs (last 5 lines):"
        kubectl -n $NAMESPACE logs $CORE_POD --tail=5 2>&1 | sed 's/^/  /'
    else
        echo -e "${RED}❌ FORTUNA CORE: Not found${NC}"
    fi
    
    echo ""
    
    # Database Status
    POSTGRES_POD=$(kubectl -n $NAMESPACE get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$POSTGRES_POD" ]; then
        echo "🗄️  DATABASE:"
        echo "─────────────────────────────────────────────────────────────"
        DB_STATUS=$(kubectl -n $NAMESPACE get pod $POSTGRES_POD -o jsonpath='{.status.phase}' 2>/dev/null)
        
        if [ "$DB_STATUS" = "Running" ]; then
            echo -e "${GREEN}✅ PostgreSQL: Running${NC}"
            
            # Quick DB check
            CVE_COUNT=$(kubectl -n $NAMESPACE exec $POSTGRES_POD -- psql -U postgres -d fortuna -t -c "SELECT COUNT(*) FROM cves;" 2>/dev/null | tr -d ' ')
            INSIGHT_COUNT=$(kubectl -n $NAMESPACE exec $POSTGRES_POD -- psql -U postgres -d fortuna -t -c "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL;" 2>/dev/null | tr -d ' ')
            
            if [ -n "$CVE_COUNT" ]; then
                echo "  📊 CVEs: $CVE_COUNT"
                echo "  📊 Insights: $INSIGHT_COUNT"
            fi
        else
            echo -e "${YELLOW}⏳ PostgreSQL: $DB_STATUS${NC}"
        fi
    else
        echo -e "${RED}❌ DATABASE: Not found${NC}"
    fi
    
    echo ""
    
    # Services
    echo "🌐 SERVICES:"
    echo "─────────────────────────────────────────────────────────────"
    kubectl -n $NAMESPACE get svc 2>/dev/null || echo "❌ Cannot get services"
    
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo "🔄 Refreshing in $INTERVAL seconds... (Ctrl+C to stop)"
    echo "═══════════════════════════════════════════════════════════════"
    
    sleep $INTERVAL
done

