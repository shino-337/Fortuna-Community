#!/bin/bash

# Script to create test policy template via API or direct SQL

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-ksam}"
POSTGRES_POD="${POSTGRES_POD:-postgres-6d84b5b778-n2cvk}"
API_URL="${API_URL:-http://localhost:8080/api/v1}"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Create template via API
create_via_api() {
    local template_file=$1
    
    log_info "Creating template via API from $template_file"
    
    # Convert YAML to JSON (simplified)
    template_json=$(cat <<EOF
{
  "templateId": "test-privileged-container",
  "version": "v1.0.0",
  "name": "Test Privileged Container Policy",
  "description": "Test policy to detect and block privileged containers",
  "category": "security",
  "defaultSeverity": "critical",
  "celExpression": "resource.spec.containers.exists(c, c.securityContext.privileged == true) || resource.spec.securityContext.privileged == true",
  "defaultScope": "{\"resourceTypes\":[\"Pod\",\"Deployment\"],\"clusters\":[],\"namespaces\":[]}",
  "defaultAction": "block",
  "supportsRemediation": true,
  "remediationTemplate": "{\"type\":\"patch\",\"operations\":[{\"op\":\"replace\",\"path\":\"/spec/containers/0/securityContext/privileged\",\"value\":false}]}",
  "rationale": "Privileged containers have access to all host devices",
  "references": ["https://kubernetes.io/docs/concepts/security/pod-security-standards/"],
  "examples": "[]",
  "isSystem": false
}
EOF
)
    
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$template_json" \
        "$API_URL/policies/templates" 2>/dev/null)
    
    http_code=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" = "201" ] || [ "$http_code" = "200" ]; then
        log_info "Template created successfully (HTTP $http_code)"
        echo "$body" | jq . 2>/dev/null || echo "$body"
        return 0
    else
        log_error "Failed to create template (HTTP $http_code)"
        echo "$body"
        return 1
    fi
}

# Create template via SQL
create_via_sql() {
    log_info "Creating template via SQL in Kubernetes database"
    
    kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam <<'SQL'
INSERT INTO policy_templates (
    template_id,
    version,
    name,
    description,
    category,
    default_severity,
    cel_expression,
    default_scope,
    default_action,
    supports_remediation,
    remediation_template,
    rationale,
    references,
    examples,
    is_system,
    created_by
) VALUES (
    'test-privileged-container',
    'v1.0.0',
    'Test Privileged Container Policy',
    'Test policy to detect and block privileged containers',
    'security',
    'critical',
    'resource.spec.containers.exists(c, c.securityContext.privileged == true) || resource.spec.securityContext.privileged == true',
    '{"resourceTypes": ["Pod", "Deployment"], "clusters": [], "namespaces": []}'::jsonb,
    'block',
    true,
    '{"type": "patch", "operations": [{"op": "replace", "path": "/spec/containers/0/securityContext/privileged", "value": false}]}'::jsonb,
    'Privileged containers have access to all host devices and capabilities, which poses a significant security risk.',
    ARRAY['https://kubernetes.io/docs/concepts/security/pod-security-standards/'],
    '[]'::jsonb,
    false,
    'test-script'
) ON CONFLICT (template_id, version) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    category = EXCLUDED.category,
    default_severity = EXCLUDED.default_severity,
    cel_expression = EXCLUDED.cel_expression,
    default_scope = EXCLUDED.default_scope,
    default_action = EXCLUDED.default_action,
    supports_remediation = EXCLUDED.supports_remediation,
    remediation_template = EXCLUDED.remediation_template,
    rationale = EXCLUDED.rationale,
    references = EXCLUDED.references,
    examples = EXCLUDED.examples,
    updated_at = now();
SQL

    if [ $? -eq 0 ]; then
        log_info "Template created successfully via SQL"
        return 0
    else
        log_error "Failed to create template via SQL"
        return 1
    fi
}

# Main
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Create Test Policy Template${NC}"
    echo -e "${BLUE}========================================${NC}\n"
    
    # Try API first, fallback to SQL
    if create_via_api "test-privileged-container"; then
        log_info "Template created via API"
    else
        log_info "API failed, trying SQL..."
        create_via_sql
    fi
    
    # Verify creation
    log_info "Verifying template creation..."
    kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -c \
        "SELECT template_id, version, name, category, default_severity, default_action FROM policy_templates WHERE template_id = 'test-privileged-container';" 2>&1
}

main

