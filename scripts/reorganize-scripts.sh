#!/bin/sh
# Reorganize Scripts - Fortuna K8s Management Platform
# Usage: wsl sh scripts/reorganize-scripts.sh

echo "════════════════════════════════════════════════════════"
echo "   Fortuna Scripts Reorganization"
echo "════════════════════════════════════════════════════════"
echo ""

cd "$(dirname "$0")"

# Create new structure
echo "📁 Creating new structure..."

mkdir -p deployment
mkdir -p testing/e2e
mkdir -p testing/integration
mkdir -p testing/unit
mkdir -p testing/performance
mkdir -p testing/verification
mkdir -p database
mkdir -p monitoring
mkdir -p development
mkdir -p setup
mkdir -p utils
mkdir -p archive/old-tests
mkdir -p archive/deprecated

echo "  ✓ Folders created"
echo ""

# Move deployment scripts
echo "🚀 Organizing deployment scripts..."

for file in \
    deploy.sh \
    deploy_full.sh \
    deploy-and-test-webhook.sh \
    rebuild_and_deploy.sh \
    rebuild-deploy-v2-scorer.sh \
    fix-deployment-old-image.sh \
    clean-rebuild-deploy.sh
do
    [ -f "$file" ] && mv "$file" deployment/ && echo "  ✓ $file → deployment/"
done

# Move database scripts
echo ""
echo "💾 Organizing database scripts..."

for file in \
    setup_database.sh \
    clear_database.sh \
    clear_database_k8s.sh \
    compare_db_k8s.sh \
    verify_and_sync_database.sh \
    sync_and_verify_all.sh \
    cleanup_insights.sql \
    test_database_rename.sh
do
    [ -f "$file" ] && mv "$file" database/ && echo "  ✓ $file → database/"
done

# Move monitoring scripts
echo ""
echo "📊 Organizing monitoring scripts..."

for file in \
    monitor_fortuna.sh \
    monitor_admission_metrics.sh \
    monitor-pipeline.sh \
    monitor-sbom-performance.sh \
    list_insights.sh \
    query_insights.sh
do
    [ -f "$file" ] && mv "$file" monitoring/ && echo "  ✓ $file → monitoring/"
done

# Move setup scripts
echo ""
echo "⚙️  Organizing setup scripts..."

for file in \
    start_minikube.sh \
    quick_start_minikube.sh \
    setup_test_environment.sh \
    start_portforwards.sh \
    start_dashboard_portforward.sh \
    check_minikube_resources.sh \
    verify_deployment.sh \
    verify-dependencies.sh
do
    [ -f "$file" ] && mv "$file" setup/ && echo "  ✓ $file → setup/"
done

# Move development scripts
echo ""
echo "🔧 Organizing development scripts..."

for file in \
    debug_routes.sh \
    generate_certs.sh \
    generate_hash.py \
    generate_password_hash.go \
    generate_test_traffic.sh \
    generate-webhook-certs.sh \
    create_admin_and_test_api.sh \
    fix_admin_password.sh \
    fix_password_and_test_api.sh \
    create_test_template.sh
do
    [ -f "$file" ] && mv "$file" development/ && echo "  ✓ $file → development/"
done

# Move E2E test scripts
echo ""
echo "🧪 Organizing E2E test scripts..."

for file in \
    full_e2e_test.sh \
    test_e2e_comprehensive.sh \
    test_e2e_full.sh \
    test_e2e_mvp1_comprehensive.sh \
    test_e2e_mvp1_with_api_verification.sh \
    test_e2e_risk_detection.sh \
    test_end_to_end_event_flow.sh \
    test_end_to_end_flow.sh \
    test_end_to_end.sh \
    test_event_flow_validation.sh \
    test_complete_system.sh \
    test_comprehensive.sh \
    test_mvp2_phase1_comprehensive.sh \
    test_phase1_2_3_e2e.sh \
    test_pod_insights_e2e.sh \
    test-e2e-cve-pipeline.sh \
    test-sbom-end-to-end.sh \
    test-custom-sbom-pipeline.sh \
    retrigger-e2e-pod.sh
do
    [ -f "$file" ] && mv "$file" testing/e2e/ && echo "  ✓ $file → testing/e2e/"
done

# Move verification scripts
echo ""
echo "✓ Organizing verification scripts..."

for file in \
    verify_insights_complete.sh \
    verify_dashboard_data.sh \
    verify_dashboard_counts.sh \
    verify_dashboard_api_vs_database.sh \
    verify_sbom_duplicate_fix.sh \
    verify-e2e-pipeline.sh \
    quick_verify_sbom_fix.sh
do
    [ -f "$file" ] && mv "$file" testing/verification/ && echo "  ✓ $file → testing/verification/"
done

# Move performance test scripts
echo ""
echo "⚡ Organizing performance test scripts..."

for file in \
    test_performance.sh \
    test_worker_backpressure.sh \
    test_backpressure.sh \
    test_sbom_duplicate_key_fix.sh
do
    [ -f "$file" ] && mv "$file" testing/performance/ && echo "  ✓ $file → testing/performance/"
done

# Move integration test scripts
echo ""
echo "🔗 Organizing integration test scripts..."

for file in \
    test_integration.sh \
    test_all_apis.sh \
    test_phase1_2_apis.sh \
    test_data_flow.sh \
    test_migration.sh \
    test_policy_engine_migrations.sh \
    test_runtime_environment.sh \
    test_runtime_full.sh
do
    [ -f "$file" ] && mv "$file" testing/integration/ && echo "  ✓ $file → testing/integration/"
done

# Move component test scripts (keep in testing root for now)
echo ""
echo "🧩 Organizing component test scripts..."

for file in \
    test_admission_webhook.sh \
    test_admission_metrics.sh \
    test_policy_engine_e2e.sh \
    test_policy_api_simple.sh \
    test_webhook_with_pod.sh \
    test-webhook-complete.sh \
    test-webhook-deployment.sh \
    deploy-and-test-webhook.sh \
    test_risk_engine.sh \
    test_mvp2_phase1_risk_scoring.sh \
    test-v2-scorer.sh \
    test-v2-scorer-complete.sh \
    test-v2-complete.sh \
    test-cve-detection.sh \
    test-sbom-generation.sh \
    trigger-sbom-processing.sh
do
    [ -f "$file" ] && mv "$file" testing/ && echo "  ✓ $file → testing/"
done

# Move dashboard test scripts
echo ""
echo "🎨 Organizing dashboard test scripts..."

for file in \
    test_dashboard.sh \
    test_dashboard_comprehensive.sh \
    test_dashboard_browser.sh \
    test_dashboard_screens.sh \
    test_dashboard_insight_display.sh \
    test_dashboard_insight_fix.sh \
    test_all_dashboard_apis.sh \
    test_agent_sync_dashboard.sh \
    test_cors_complete.sh \
    test_login_cors.sh
do
    [ -f "$file" ] && mv "$file" testing/ && echo "  ✓ $file → testing/"
done

# Move mTLS test scripts
echo ""
echo "🔒 Organizing mTLS test scripts..."

for file in \
    test_mtls_connection.sh \
    test_mtls_traffic_encryption.sh \
    test_mtls_traffic_from_agent.sh \
    test_mtls_traffic_with_debug_pod.sh \
    demo_mtls_data_transmission.sh \
    test_issue5_mtls_advanced.sh
do
    [ -f "$file" ] && mv "$file" testing/ && echo "  ✓ $file → testing/"
done

# Move test runner scripts
echo ""
echo "🏃 Organizing test runner scripts..."

for file in \
    run_all_tests.sh \
    run_tests.sh \
    run_failed_tests.sh \
    run_validation_tests.sh \
    run-pending-testcases.sh \
    test_cases.sh \
    test_all_issues.sh
do
    [ -f "$file" ] && mv "$file" testing/ && echo "  ✓ $file → testing/"
done

# Move utility scripts (publish, query, etc.)
echo ""
echo "🛠️  Organizing utility scripts..."

for file in \
    publish_messages_direct.sh \
    publish_messages_fixed.sh \
    publish_messages_via_testpod.sh \
    publish_test_messages_natsbox.sh \
    publish_test_messages_simple.sh \
    publish_test_messages.go \
    publish_vulnerable_pod_data.sh \
    publish-test-pod-to-nats.sh \
    publish_and_test_metrics.sh \
    send_test_messages.sh \
    query_policy_db.sh \
    view_policy_instances.sh \
    view_policy_templates.sh \
    view_templates_k8s.sh \
    view_templates_via_api.sh \
    evaluate_existing_resources.sh
do
    [ -f "$file" ] && mv "$file" utils/ && echo "  ✓ $file → utils/"
done

# Move cleanup scripts
echo ""
echo "🧹 Organizing cleanup scripts..."

for file in \
    cleanup_mvp1.sh \
    cleanup_duplicate_pods.sh \
    cleanup_unused_code.sh \
    clear_images.sh
do
    [ -f "$file" ] && mv "$file" utils/ && echo "  ✓ $file → utils/"
done

# Move migration scripts
echo ""
echo "🔄 Organizing migration scripts..."

for file in \
    manual-run-migration018.sh \
    run-migration-020.sh \
    run-migration018-direct.sh
do
    [ -f "$file" ] && mv "$file" database/ && echo "  ✓ $file → database/"
done

# Move use case scripts
echo ""
echo "📋 Organizing use case scripts..."

for file in \
    usecase_vulnerable_pod.sh \
    usecase_vulnerable_pod_complete.sh
do
    [ -f "$file" ] && mv "$file" testing/ && echo "  ✓ $file → testing/"
done

# Move specific test scripts
echo ""
echo "🎯 Organizing specific test scripts..."

for file in \
    test_issue1_cel_hotreload.sh \
    test_yaml_rules.sh \
    test_yaml_rules_integration.sh \
    test_phase1_components.sh \
    test_insight_status_flow.sh \
    test_metrics_with_traffic.sh \
    test_all_metrics.sh
do
    [ -f "$file" ] && mv "$file" testing/ && echo "  ✓ $file → testing/"
done

# Archive old/deprecated scripts
echo ""
echo "📦 Archiving deprecated scripts..."

# Check for KSAM folder (old structure)
if [ -d "KSAM" ]; then
    mv KSAM archive/deprecated/
    echo "  ✓ KSAM/ → archive/deprecated/"
fi

# Check for test_results folder
if [ -d "test_results" ]; then
    mv test_results archive/
    echo "  ✓ test_results/ → archive/"
fi

# Move documentation scripts (keep in root)
echo ""
echo "📚 Documentation scripts (keeping in scripts/)..."
echo "  ✓ commit-docs-restructure.sh (current location)"
echo "  ✓ show-docs-tree.sh (current location)"
echo "  ✓ Restructure-Docs-Phase1.ps1 (current location)"
echo "  ✓ restructure-docs-phase2.sh (current location)"
echo "  ✓ reorganize_docs.sh → archive/deprecated/"
[ -f "reorganize_docs.sh" ] && mv reorganize_docs.sh archive/deprecated/

echo ""
echo "════════════════════════════════════════════════════════"
echo "✅ Script reorganization complete!"
echo "════════════════════════════════════════════════════════"
echo ""

# Count files
deployment_count=$(find deployment -type f 2>/dev/null | wc -l)
testing_count=$(find testing -type f 2>/dev/null | wc -l)
database_count=$(find database -type f 2>/dev/null | wc -l)
monitoring_count=$(find monitoring -type f 2>/dev/null | wc -l)
setup_count=$(find setup -type f 2>/dev/null | wc -l)
development_count=$(find development -type f 2>/dev/null | wc -l)
utils_count=$(find utils -type f 2>/dev/null | wc -l)

echo "📊 Summary:"
echo "  deployment/   : $deployment_count scripts"
echo "  testing/      : $testing_count scripts"
echo "  database/     : $database_count scripts"
echo "  monitoring/   : $monitoring_count scripts"
echo "  setup/        : $setup_count scripts"
echo "  development/  : $development_count scripts"
echo "  utils/        : $utils_count scripts"
echo ""
echo "📝 Next: Create README files for each category"
echo ""

