#!/bin/bash

# Policy Engine End-to-End Test Script
# Tests API endpoints and verifies database consistency

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
API_BASE_URL="${API_BASE_URL:-http://localhost:8080/api/v1}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"
DB_PASSWORD="${DB_PASSWORD:-ksam}"

# Test results
TEST_RESULTS_DIR="./test_results/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$TEST_RESULTS_DIR"

# Test counters
PASSED=0
FAILED=0
TOTAL=0

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_test() {
    echo -e "\n${YELLOW}[TEST]${NC} $1"
}

# Test assertion
assert() {
    TOTAL=$((TOTAL + 1))
    if [ $1 -eq 0 ]; then
        PASSED=$((PASSED + 1))
        log_info "✓ $2"
        return 0
    else
        FAILED=$((FAILED + 1))
        log_error "✗ $2"
        return 1
    fi
}

# Save test result
save_result() {
    local test_name=$1
    local status=$2
    local details=$3
    echo "{\"test\": \"$test_name\", \"status\": \"$status\", \"details\": \"$details\", \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" >> "$TEST_RESULTS_DIR/results.jsonl"
}

# Database query function
db_query() {
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -A -c "$1" 2>/dev/null
}

# API request function
api_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    local token=$4
    
    local curl_cmd="curl -s -w '\n%{http_code}' -X $method"
    
    if [ -n "$token" ]; then
        curl_cmd="$curl_cmd -H 'Authorization: Bearer $token'"
    fi
    
    if [ -n "$data" ]; then
        curl_cmd="$curl_cmd -H 'Content-Type: application/json' -d '$data'"
    fi
    
    curl_cmd="$curl_cmd '$API_BASE_URL$endpoint'"
    
    eval "$curl_cmd"
}

# Get initial database state
get_db_state() {
    local table=$1
    local output_file="$TEST_RESULTS_DIR/db_state_before_${table}.json"
    
    log_info "Capturing database state for $table (BEFORE)"
    
    case $table in
        "templates")
            PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
                -c "SELECT json_agg(row_to_json(t)) FROM (SELECT * FROM policy_templates WHERE deleted_at IS NULL) t;" \
                -t > "$output_file" 2>/dev/null || echo "[]" > "$output_file"
            ;;
        "instances")
            PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
                -c "SELECT json_agg(row_to_json(t)) FROM (SELECT * FROM policy_instances WHERE deleted_at IS NULL) t;" \
                -t > "$output_file" 2>/dev/null || echo "[]" > "$output_file"
            ;;
    esac
    
    log_info "Database state saved to $output_file"
}

# Compare API response with database
compare_api_db() {
    local api_response=$1
    local db_query=$2
    local test_name=$3
    
    local db_result=$(db_query "$db_query")
    local api_json=$(echo "$api_response" | head -n -1)
    local api_count=$(echo "$api_json" | jq -r '.count // .templates | length // 0' 2>/dev/null || echo "0")
    local db_count=$(echo "$db_result" | jq 'length' 2>/dev/null || echo "0")
    
    if [ "$api_count" = "$db_count" ]; then
        log_info "Count matches: API=$api_count, DB=$db_count"
        return 0
    else
        log_error "Count mismatch: API=$api_count, DB=$db_count"
        return 1
    fi
}

# Test 1: Check API availability
test_api_availability() {
    log_test "TC-0.1: API Availability Check"
    
    local response=$(api_request "GET" "/policies/templates" "" "")
    local http_code=$(echo "$response" | tail -n 1)
    
    if [ "$http_code" = "200" ] || [ "$http_code" = "401" ]; then
        assert 0 "API is accessible (HTTP $http_code)"
        save_result "TC-0.1" "PASS" "API returned HTTP $http_code"
    else
        assert 1 "API is not accessible (HTTP $http_code)"
        save_result "TC-0.1" "FAIL" "API returned HTTP $http_code"
    fi
}

# Test 2: Check database tables
test_database_tables() {
    log_test "TC-0.2: Database Tables Check"
    
    local tables=("policy_templates" "policy_instances" "policy_violations")
    local all_exist=0
    
    for table in "${tables[@]}"; do
        local exists=$(db_query "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = '$table');")
        if [ "$exists" = "t" ]; then
            log_info "Table $table exists"
        else
            log_error "Table $table does not exist"
            all_exist=1
        fi
    done
    
    assert $all_exist "All required tables exist"
    save_result "TC-0.2" "$([ $all_exist -eq 0 ] && echo "PASS" || echo "FAIL")" "Tables check"
}

# Test 3: List Templates (Empty State)
test_list_templates_empty() {
    log_test "TC-1.1: List Templates (Empty State)"
    
    get_db_state "templates"
    
    local response=$(api_request "GET" "/policies/templates" "" "")
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" > "$TEST_RESULTS_DIR/tc_1_1_response.json"
    
    local count=$(echo "$body" | jq -r '.count // 0' 2>/dev/null || echo "0")
    local db_count=$(db_query "SELECT COUNT(*) FROM policy_templates WHERE deleted_at IS NULL;")
    
    if [ "$http_code" = "200" ] && [ "$count" = "$db_count" ]; then
        assert 0 "List templates returns correct count (API: $count, DB: $db_count)"
        save_result "TC-1.1" "PASS" "Count: $count"
    else
        assert 1 "List templates failed (HTTP: $http_code, API: $count, DB: $db_count)"
        save_result "TC-1.1" "FAIL" "HTTP: $http_code, Count mismatch"
    fi
}

# Test 4: Create Template
test_create_template() {
    log_test "TC-1.2: Create Template"
    
    local template_json='{
        "templateId": "test-template-e2e",
        "version": "v1.0.0",
        "name": "E2E Test Template",
        "description": "Template for E2E testing",
        "category": "security",
        "defaultSeverity": "high",
        "celExpression": "resource.labels.env == '\''prod'\''",
        "defaultScope": "{}",
        "defaultAction": "alert",
        "supportsRemediation": false,
        "isSystem": false
    }'
    
    local response=$(api_request "POST" "/policies/templates" "$template_json" "")
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" > "$TEST_RESULTS_DIR/tc_1_2_response.json"
    
    if [ "$http_code" = "201" ] || [ "$http_code" = "200" ]; then
        local template_id=$(echo "$body" | jq -r '.templateId // empty' 2>/dev/null)
        local db_exists=$(db_query "SELECT EXISTS (SELECT 1 FROM policy_templates WHERE template_id = 'test-template-e2e' AND version = 'v1.0.0' AND deleted_at IS NULL);")
        
        if [ "$db_exists" = "t" ]; then
            assert 0 "Template created successfully (ID: $template_id)"
            save_result "TC-1.2" "PASS" "Template ID: $template_id"
        else
            assert 1 "Template not found in database"
            save_result "TC-1.2" "FAIL" "Template not in DB"
        fi
    else
        assert 1 "Create template failed (HTTP: $http_code)"
        save_result "TC-1.2" "FAIL" "HTTP: $http_code"
    fi
}

# Test 5: Get Template
test_get_template() {
    log_test "TC-1.3: Get Template"
    
    local response=$(api_request "GET" "/policies/templates/test-template-e2e" "" "")
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" > "$TEST_RESULTS_DIR/tc_1_3_response.json"
    
    if [ "$http_code" = "200" ]; then
        local api_id=$(echo "$body" | jq -r '.templateId // empty' 2>/dev/null)
        local db_id=$(db_query "SELECT template_id FROM policy_templates WHERE template_id = 'test-template-e2e' AND deleted_at IS NULL LIMIT 1;")
        
        if [ "$api_id" = "$db_id" ] && [ -n "$api_id" ]; then
            assert 0 "Get template returns correct data (ID: $api_id)"
            save_result "TC-1.3" "PASS" "Template ID: $api_id"
        else
            assert 1 "Template ID mismatch (API: $api_id, DB: $db_id)"
            save_result "TC-1.3" "FAIL" "ID mismatch"
        fi
    else
        assert 1 "Get template failed (HTTP: $http_code)"
        save_result "TC-1.3" "FAIL" "HTTP: $http_code"
    fi
}

# Test 6: List Templates (With Data)
test_list_templates_with_data() {
    log_test "TC-1.4: List Templates (With Data)"
    
    local response=$(api_request "GET" "/policies/templates" "" "")
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" > "$TEST_RESULTS_DIR/tc_1_4_response.json"
    
    local api_count=$(echo "$body" | jq -r '.count // (.templates | length) // 0' 2>/dev/null || echo "0")
    local db_count=$(db_query "SELECT COUNT(*) FROM policy_templates WHERE deleted_at IS NULL;")
    
    if [ "$http_code" = "200" ] && [ "$api_count" = "$db_count" ] && [ "$api_count" -gt 0 ]; then
        assert 0 "List templates returns correct count (API: $api_count, DB: $db_count)"
        save_result "TC-1.4" "PASS" "Count: $api_count"
    else
        assert 1 "List templates count mismatch (HTTP: $http_code, API: $api_count, DB: $db_count)"
        save_result "TC-1.4" "FAIL" "Count mismatch"
    fi
}

# Test 7: Update Template
test_update_template() {
    log_test "TC-1.7: Update Template"
    
    local update_json='{
        "templateId": "test-template-e2e",
        "version": "v1.0.0",
        "name": "E2E Test Template Updated",
        "description": "Updated description",
        "category": "security",
        "defaultSeverity": "critical",
        "celExpression": "resource.labels.env == '\''prod'\''",
        "defaultScope": "{}",
        "defaultAction": "block",
        "supportsRemediation": false,
        "isSystem": false
    }'
    
    local response=$(api_request "PUT" "/policies/templates/test-template-e2e/v1.0.0" "$update_json" "")
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" > "$TEST_RESULTS_DIR/tc_1_7_response.json"
    
    if [ "$http_code" = "200" ]; then
        local api_name=$(echo "$body" | jq -r '.name // empty' 2>/dev/null)
        local db_name=$(db_query "SELECT name FROM policy_templates WHERE template_id = 'test-template-e2e' AND version = 'v1.0.0' AND deleted_at IS NULL;")
        
        if [ "$api_name" = "$db_name" ] && [ "$api_name" = "E2E Test Template Updated" ]; then
            assert 0 "Template updated successfully (Name: $api_name)"
            save_result "TC-1.7" "PASS" "Name: $api_name"
        else
            assert 1 "Template update mismatch (API: $api_name, DB: $db_name)"
            save_result "TC-1.7" "FAIL" "Name mismatch"
        fi
    else
        assert 1 "Update template failed (HTTP: $http_code)"
        save_result "TC-1.7" "FAIL" "HTTP: $http_code"
    fi
}

# Test 8: Create Instance
test_create_instance() {
    log_test "TC-2.2: Create Instance"
    
    local instance_json='{
        "templateId": "test-template-e2e",
        "templateVersion": "v1.0.0",
        "instanceName": "test-instance-e2e",
        "description": "E2E Test Instance",
        "enabled": true,
        "clusters": ["cluster-1"],
        "namespaces": ["default"],
        "resourceTypes": ["Pod"],
        "action": "block",
        "severity": "critical"
    }'
    
    local response=$(api_request "POST" "/policies/instances" "$instance_json" "")
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" > "$TEST_RESULTS_DIR/tc_2_2_response.json"
    
    if [ "$http_code" = "201" ] || [ "$http_code" = "200" ]; then
        local instance_name=$(echo "$body" | jq -r '.instanceName // empty' 2>/dev/null)
        local db_exists=$(db_query "SELECT EXISTS (SELECT 1 FROM policy_instances WHERE instance_name = 'test-instance-e2e' AND deleted_at IS NULL);")
        
        if [ "$db_exists" = "t" ]; then
            assert 0 "Instance created successfully (Name: $instance_name)"
            save_result "TC-2.2" "PASS" "Instance: $instance_name"
        else
            assert 1 "Instance not found in database"
            save_result "TC-2.2" "FAIL" "Instance not in DB"
        fi
    else
        assert 1 "Create instance failed (HTTP: $http_code)"
        save_result "TC-2.2" "FAIL" "HTTP: $http_code"
    fi
}

# Test 9: Get Instance
test_get_instance() {
    log_test "TC-2.3: Get Instance"
    
    local response=$(api_request "GET" "/policies/instances/test-instance-e2e" "" "")
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" > "$TEST_RESULTS_DIR/tc_2_3_response.json"
    
    if [ "$http_code" = "200" ]; then
        local api_name=$(echo "$body" | jq -r '.instanceName // empty' 2>/dev/null)
        local db_name=$(db_query "SELECT instance_name FROM policy_instances WHERE instance_name = 'test-instance-e2e' AND deleted_at IS NULL LIMIT 1;")
        
        if [ "$api_name" = "$db_name" ] && [ -n "$api_name" ]; then
            assert 0 "Get instance returns correct data (Name: $api_name)"
            save_result "TC-2.3" "PASS" "Instance: $api_name"
        else
            assert 1 "Instance name mismatch (API: $api_name, DB: $db_name)"
            save_result "TC-2.3" "FAIL" "Name mismatch"
        fi
    else
        assert 1 "Get instance failed (HTTP: $http_code)"
        save_result "TC-2.3" "FAIL" "HTTP: $http_code"
    fi
}

# Test 10: List Instances
test_list_instances() {
    log_test "TC-2.4: List Instances"
    
    get_db_state "instances"
    
    local response=$(api_request "GET" "/policies/instances" "" "")
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" > "$TEST_RESULTS_DIR/tc_2_4_response.json"
    
    local api_count=$(echo "$body" | jq -r '.count // (.instances | length) // 0' 2>/dev/null || echo "0")
    local db_count=$(db_query "SELECT COUNT(*) FROM policy_instances WHERE deleted_at IS NULL;")
    
    if [ "$http_code" = "200" ] && [ "$api_count" = "$db_count" ]; then
        assert 0 "List instances returns correct count (API: $api_count, DB: $db_count)"
        save_result "TC-2.4" "PASS" "Count: $api_count"
    else
        assert 1 "List instances count mismatch (HTTP: $http_code, API: $api_count, DB: $db_count)"
        save_result "TC-2.4" "FAIL" "Count mismatch"
    fi
}

# Test 11: Delete Instance (Soft Delete)
test_delete_instance() {
    log_test "TC-2.8: Delete Instance (Soft Delete)"
    
    local response=$(api_request "DELETE" "/policies/instances/test-instance-e2e" "" "")
    local http_code=$(echo "$response" | tail -n 1)
    
    if [ "$http_code" = "200" ]; then
        local db_deleted=$(db_query "SELECT deleted_at IS NOT NULL FROM policy_instances WHERE instance_name = 'test-instance-e2e';")
        local db_exists=$(db_query "SELECT EXISTS (SELECT 1 FROM policy_instances WHERE instance_name = 'test-instance-e2e');")
        
        if [ "$db_deleted" = "t" ] && [ "$db_exists" = "t" ]; then
            assert 0 "Instance soft deleted successfully (deleted_at is set, record exists)"
            save_result "TC-2.8" "PASS" "Soft delete successful"
        else
            assert 1 "Soft delete failed (deleted_at: $db_deleted, exists: $db_exists)"
            save_result "TC-2.8" "FAIL" "Soft delete failed"
        fi
    else
        assert 1 "Delete instance failed (HTTP: $http_code)"
        save_result "TC-2.8" "FAIL" "HTTP: $http_code"
    fi
}

# Test 12: Delete Template (Soft Delete)
test_delete_template() {
    log_test "TC-1.9: Delete Template (Soft Delete)"
    
    local response=$(api_request "DELETE" "/policies/templates/test-template-e2e/v1.0.0" "" "")
    local http_code=$(echo "$response" | tail -n 1)
    
    if [ "$http_code" = "200" ]; then
        local db_deleted=$(db_query "SELECT deleted_at IS NOT NULL FROM policy_templates WHERE template_id = 'test-template-e2e' AND version = 'v1.0.0';")
        local db_exists=$(db_query "SELECT EXISTS (SELECT 1 FROM policy_templates WHERE template_id = 'test-template-e2e' AND version = 'v1.0.0');")
        
        if [ "$db_deleted" = "t" ] && [ "$db_exists" = "t" ]; then
            assert 0 "Template soft deleted successfully (deleted_at is set, record exists)"
            save_result "TC-1.9" "PASS" "Soft delete successful"
        else
            assert 1 "Soft delete failed (deleted_at: $db_deleted, exists: $db_exists)"
            save_result "TC-1.9" "FAIL" "Soft delete failed"
        fi
    else
        assert 1 "Delete template failed (HTTP: $http_code)"
        save_result "TC-1.9" "FAIL" "HTTP: $http_code"
    fi
}

# Final database state
get_final_db_state() {
    log_info "Capturing final database state"
    get_db_state "templates"
    get_db_state "instances"
}

# Generate summary report
generate_summary() {
    log_info "Generating test summary..."
    
    local summary_file="$TEST_RESULTS_DIR/summary.txt"
    {
        echo "Policy Engine E2E Test Summary"
        echo "=============================="
        echo "Date: $(date)"
        echo ""
        echo "Test Results:"
        echo "  Total: $TOTAL"
        echo "  Passed: $PASSED"
        echo "  Failed: $FAILED"
        echo ""
        echo "Success Rate: $(( PASSED * 100 / TOTAL ))%"
        echo ""
        echo "Results saved to: $TEST_RESULTS_DIR"
    } > "$summary_file"
    
    cat "$summary_file"
}

# Main execution
main() {
    log_info "Starting Policy Engine E2E Tests"
    log_info "Results will be saved to: $TEST_RESULTS_DIR"
    
    # Initialize results file
    echo "[]" > "$TEST_RESULTS_DIR/results.jsonl"
    
    # Run tests
    test_api_availability
    test_database_tables
    test_list_templates_empty
    test_create_template
    test_get_template
    test_list_templates_with_data
    test_update_template
    test_create_instance
    test_get_instance
    test_list_instances
    test_delete_instance
    test_delete_template
    
    get_final_db_state
    
    # Generate summary
    generate_summary
    
    log_info "Tests completed!"
    log_info "Results: $PASSED/$TOTAL passed, $FAILED/$TOTAL failed"
    
    if [ $FAILED -eq 0 ]; then
        exit 0
    else
        exit 1
    fi
}

# Run main
main

