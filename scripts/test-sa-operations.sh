#!/bin/bash

# Script to test ServiceAccount operations on Kubernetes
# This script helps test if KSAM system detects and syncs changes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
NAMESPACE="default"
ACTION=""
SA_NAME=""

# Function to print usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -a, --action ACTION     Action to perform: create, delete, edit, list"
    echo "  -n, --name NAME         ServiceAccount name"
    echo "  -s, --namespace NS      Namespace (default: default)"
    echo "  -h, --help             Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -a create -n test-sa-1 -s default"
    echo "  $0 -a delete -n test-sa-1 -s default"
    echo "  $0 -a edit -n test-sa-1 -s default"
    echo "  $0 -a list -s default"
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -a|--action)
            ACTION="$2"
            shift 2
            ;;
        -n|--name)
            SA_NAME="$2"
            shift 2
            ;;
        -s|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate action
if [[ -z "$ACTION" ]]; then
    echo -e "${RED}Error: Action is required${NC}"
    usage
fi

# Function to create ServiceAccount
create_sa() {
    if [[ -z "$SA_NAME" ]]; then
        echo -e "${RED}Error: ServiceAccount name is required for create action${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}Creating ServiceAccount: ${SA_NAME} in namespace: ${NAMESPACE}${NC}"
    
    # Create namespace if it doesn't exist
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1 || true
    
    # Create ServiceAccount
    kubectl create serviceaccount "$SA_NAME" -n "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1
    
    # Add annotations and labels
    kubectl annotate serviceaccount "$SA_NAME" -n "$NAMESPACE" \
        description="Test ServiceAccount created by KSAM test script" \
        created-by="ksam-test-script" \
        created-at="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --overwrite > /dev/null 2>&1
    
    kubectl label serviceaccount "$SA_NAME" -n "$NAMESPACE" \
        app=ksam-test \
        test=true \
        --overwrite > /dev/null 2>&1
    
    echo -e "${GREEN}✓ ServiceAccount '${SA_NAME}' created successfully in namespace '${NAMESPACE}'${NC}"
    
    # Show the created ServiceAccount
    echo -e "\n${BLUE}ServiceAccount details:${NC}"
    kubectl get serviceaccount "$SA_NAME" -n "$NAMESPACE" -o yaml | grep -A 10 "metadata:"
    
    echo -e "\n${YELLOW}Note: Wait a few seconds for KSAM agent to sync this change...${NC}"
}

# Function to delete ServiceAccount
delete_sa() {
    if [[ -z "$SA_NAME" ]]; then
        echo -e "${RED}Error: ServiceAccount name is required for delete action${NC}"
        exit 1
    fi
    
    echo -e "${YELLOW}Deleting ServiceAccount: ${SA_NAME} from namespace: ${NAMESPACE}${NC}"
    
    # Check if ServiceAccount exists
    if ! kubectl get serviceaccount "$SA_NAME" -n "$NAMESPACE" > /dev/null 2>&1; then
        echo -e "${RED}Error: ServiceAccount '${SA_NAME}' not found in namespace '${NAMESPACE}'${NC}"
        exit 1
    fi
    
    kubectl delete serviceaccount "$SA_NAME" -n "$NAMESPACE"
    
    echo -e "${GREEN}✓ ServiceAccount '${SA_NAME}' deleted successfully from namespace '${NAMESPACE}'${NC}"
    echo -e "\n${YELLOW}Note: Wait a few seconds for KSAM agent to sync this change...${NC}"
}

# Function to edit ServiceAccount
edit_sa() {
    if [[ -z "$SA_NAME" ]]; then
        echo -e "${RED}Error: ServiceAccount name is required for edit action${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}Editing ServiceAccount: ${SA_NAME} in namespace: ${NAMESPACE}${NC}"
    
    # Check if ServiceAccount exists
    if ! kubectl get serviceaccount "$SA_NAME" -n "$NAMESPACE" > /dev/null 2>&1; then
        echo -e "${RED}Error: ServiceAccount '${SA_NAME}' not found in namespace '${NAMESPACE}'${NC}"
        exit 1
    fi
    
    # Add/update annotations
    kubectl annotate serviceaccount "$SA_NAME" -n "$NAMESPACE" \
        description="Updated ServiceAccount - edited by KSAM test script" \
        updated-by="ksam-test-script" \
        updated-at="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --overwrite
    
    # Add/update labels
    kubectl label serviceaccount "$SA_NAME" -n "$NAMESPACE" \
        app=ksam-test \
        test=true \
        edited=true \
        --overwrite
    
    echo -e "${GREEN}✓ ServiceAccount '${SA_NAME}' updated successfully in namespace '${NAMESPACE}'${NC}"
    
    # Show the updated ServiceAccount
    echo -e "\n${BLUE}Updated ServiceAccount details:${NC}"
    kubectl get serviceaccount "$SA_NAME" -n "$NAMESPACE" -o yaml | grep -A 15 "metadata:"
    
    echo -e "\n${YELLOW}Note: Wait a few seconds for KSAM agent to sync this change...${NC}"
}

# Function to list ServiceAccounts
list_sa() {
    echo -e "${BLUE}Listing ServiceAccounts in namespace: ${NAMESPACE}${NC}\n"
    
    # List all ServiceAccounts
    kubectl get serviceaccounts -n "$NAMESPACE" -o wide
    
    echo -e "\n${BLUE}ServiceAccounts with details:${NC}\n"
    kubectl get serviceaccounts -n "$NAMESPACE" -o custom-columns=\
NAME:.metadata.name,\
NAMESPACE:.metadata.namespace,\
AGE:.metadata.creationTimestamp,\
LABELS:.metadata.labels.app
}

# Function to create multiple test ServiceAccounts
create_multiple() {
    echo -e "${BLUE}Creating multiple test ServiceAccounts...${NC}\n"
    
    # Create namespace if it doesn't exist
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1 || true
    
    for i in {1..5}; do
        sa_name="test-sa-${i}"
        echo -e "${BLUE}Creating ${sa_name}...${NC}"
        kubectl create serviceaccount "$sa_name" -n "$NAMESPACE" \
            --dry-run=client -o yaml | \
            kubectl apply -f - > /dev/null 2>&1
        
        kubectl annotate serviceaccount "$sa_name" -n "$NAMESPACE" \
            description="Test ServiceAccount #${i}" \
            created-by="ksam-test-script" \
            --overwrite > /dev/null 2>&1
        
        kubectl label serviceaccount "$sa_name" -n "$NAMESPACE" \
            app=ksam-test \
            test=true \
            index="${i}" \
            --overwrite > /dev/null 2>&1
        
        echo -e "${GREEN}✓ Created ${sa_name}${NC}"
    done
    
    echo -e "\n${GREEN}✓ Created 5 test ServiceAccounts in namespace '${NAMESPACE}'${NC}"
    echo -e "${YELLOW}Note: Wait a few seconds for KSAM agent to sync these changes...${NC}"
}

# Function to clean up test ServiceAccounts
cleanup_test() {
    echo -e "${YELLOW}Cleaning up test ServiceAccounts in namespace: ${NAMESPACE}${NC}\n"
    
    # Get all ServiceAccounts with test label
    sas=$(kubectl get serviceaccounts -n "$NAMESPACE" -l app=ksam-test -o name 2>/dev/null || echo "")
    
    if [[ -z "$sas" ]]; then
        echo -e "${BLUE}No test ServiceAccounts found to clean up${NC}"
        return
    fi
    
    echo "$sas" | while read -r sa; do
        sa_name=$(echo "$sa" | cut -d'/' -f2)
        echo -e "${BLUE}Deleting ${sa_name}...${NC}"
        kubectl delete serviceaccount "$sa_name" -n "$NAMESPACE" > /dev/null 2>&1
        echo -e "${GREEN}✓ Deleted ${sa_name}${NC}"
    done
    
    echo -e "\n${GREEN}✓ Cleanup completed${NC}"
}

# Main execution
case "$ACTION" in
    create)
        create_sa
        ;;
    delete)
        delete_sa
        ;;
    edit)
        edit_sa
        ;;
    list)
        list_sa
        ;;
    create-multiple)
        create_multiple
        ;;
    cleanup)
        cleanup_test
        ;;
    *)
        echo -e "${RED}Error: Unknown action '${ACTION}'${NC}"
        echo "Valid actions: create, delete, edit, list, create-multiple, cleanup"
        exit 1
        ;;
esac

echo -e "\n${BLUE}To check if changes are synced to KSAM:${NC}"
echo "  1. Check the dashboard at http://localhost:3000/serviceaccounts"
echo "  2. Check audit logs at http://localhost:3000/audit"
echo "  3. Check the graph view at http://localhost:3000/graph"

