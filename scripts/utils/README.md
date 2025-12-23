# Utility Scripts

**20 scripts** for utilities, cleanup, and helper functions.

---

## 📁 Categories

### NATS Publishing (10 scripts)
Publish test messages to NATS:
- `publish_messages_direct.sh`
- `publish_messages_fixed.sh`
- `publish_messages_via_testpod.sh`
- `publish_test_messages_natsbox.sh`
- `publish_test_messages_simple.sh`
- `publish_test_messages.go`
- `publish_vulnerable_pod_data.sh`
- `publish-test-pod-to-nats.sh`
- `publish_and_test_metrics.sh`
- `send_test_messages.sh`

### Database Queries (5 scripts)
Query and view database data:
- `query_policy_db.sh`
- `view_policy_instances.sh`
- `view_policy_templates.sh`
- `view_templates_k8s.sh`
- `view_templates_via_api.sh`

### Cleanup (4 scripts)
Cleanup and maintenance:
- `cleanup_mvp1.sh`
- `cleanup_duplicate_pods.sh`
- `cleanup_unused_code.sh`
- `clear_images.sh`

### Other (1 script)
- `evaluate_existing_resources.sh`

---

## 🚀 Quick Start

### Publish Test Messages
```bash
./publish_test_messages_natsbox.sh
```

### View Policy Templates
```bash
./view_policy_templates.sh
```

### Cleanup Duplicates
```bash
./cleanup_duplicate_pods.sh
```

---

## 📚 NATS Publishing Scripts

### publish_test_messages_natsbox.sh
Publish messages using nats-box pod:
```bash
./publish_test_messages_natsbox.sh
```

Most reliable method for publishing to NATS.

---

### publish_vulnerable_pod_data.sh
Publish vulnerable pod data for testing:
```bash
./publish_vulnerable_pod_data.sh
```

Publishes:
- Pod with known CVEs
- Test SBOM
- CVE data

---

### publish_messages_direct.sh
Direct NATS publishing:
```bash
./publish_messages_direct.sh
```

Requires: nats-cli installed

---

### publish_messages_via_testpod.sh
Publish via test pod:
```bash
./publish_messages_via_testpod.sh
```

Creates temporary pod to publish messages.

---

### publish_test_messages.go
Go program to publish test messages:
```bash
go run publish_test_messages.go
```

Requires: Go 1.21+, NATS Go client

---

### publish_and_test_metrics.sh
Publish messages and test metrics:
```bash
./publish_and_test_metrics.sh
```

Actions:
1. Publishes test messages
2. Waits for processing
3. Checks metrics

---

## 📊 Database Query Scripts

### query_policy_db.sh
Query policy database:
```bash
# All policies
./query_policy_db.sh

# Specific policy
POLICY_ID=123 ./query_policy_db.sh
```

---

### view_policy_templates.sh
View policy templates:
```bash
./view_policy_templates.sh
```

Shows:
- Template name
- Description
- CEL expression
- Parameters

---

### view_policy_instances.sh
View policy instances:
```bash
./view_policy_instances.sh
```

Shows:
- Instance ID
- Template used
- Namespace
- Status

---

### view_templates_k8s.sh
View templates from Kubernetes:
```bash
./view_templates_k8s.sh
```

Lists ConfigMaps with policy templates.

---

### view_templates_via_api.sh
View templates via API:
```bash
./view_templates_via_api.sh
```

Requires: Port forward to API (8080)

---

## 🧹 Cleanup Scripts

### cleanup_duplicate_pods.sh
Remove duplicate pods:
```bash
./cleanup_duplicate_pods.sh
```

⚠️ **Use with caution** - removes pods!

---

### cleanup_mvp1.sh
Cleanup MVP1 resources:
```bash
./cleanup_mvp1.sh
```

Removes:
- MVP1 deployments
- MVP1 services
- MVP1 ConfigMaps

---

### cleanup_unused_code.sh
Cleanup unused code (dry-run):
```bash
# Dry run (shows what would be removed)
./cleanup_unused_code.sh

# Actually remove
FORCE=yes ./cleanup_unused_code.sh
```

---

### clear_images.sh
Clear Docker images:
```bash
# Clear unused images
./clear_images.sh

# Force remove all
FORCE=yes ./clear_images.sh
```

⚠️ **Use with caution** - removes images!

---

## 🔧 Other Utilities

### evaluate_existing_resources.sh
Evaluate existing K8s resources:
```bash
./evaluate_existing_resources.sh
```

Shows:
- Resource counts
- Resource types
- Namespace distribution
- Resource status

---

## 🔄 Common Workflows

### Testing NATS Pipeline
```bash
# 1. Publish test messages
./publish_test_messages_natsbox.sh

# 2. Wait for processing
sleep 30

# 3. Check insights
../monitoring/list_insights.sh

# 4. Verify metrics
./publish_and_test_metrics.sh
```

### Policy Management
```bash
# 1. View templates
./view_policy_templates.sh

# 2. View instances
./view_policy_instances.sh

# 3. Query specific policy
POLICY_ID=123 ./query_policy_db.sh

# 4. View via API
./view_templates_via_api.sh
```

### Maintenance
```bash
# 1. Check resources
./evaluate_existing_resources.sh

# 2. Cleanup duplicates
./cleanup_duplicate_pods.sh

# 3. Clear old images
./clear_images.sh

# 4. Verify
kubectl get pods --all-namespaces
```

---

## 🚨 Troubleshooting

### NATS Publishing Fails
```bash
# Check NATS connectivity
kubectl exec -it -n fortuna nats-0 -- nats server check jetstream

# Try different publishing method
./publish_test_messages_natsbox.sh

# Check NATS logs
kubectl logs -n fortuna nats-0 --tail=50
```

### Database Query Fails
```bash
# Check database connection
kubectl exec -n fortuna postgres-xxx -- psql -U postgres -c "SELECT 1;"

# Verify database exists
kubectl exec -n fortuna postgres-xxx -- psql -U postgres -l

# Check table exists
kubectl exec -n fortuna postgres-xxx -- \
  psql -U postgres -d fortuna -c "\dt"
```

### Cleanup Issues
```bash
# Check what would be removed (dry-run)
kubectl get pods --all-namespaces | grep duplicate

# Manual cleanup
kubectl delete pod <pod-name> -n <namespace>

# Verify
kubectl get pods --all-namespaces
```

---

## ⚠️ Important Notes

### Destructive Scripts
These scripts modify or delete resources:
- `cleanup_*.sh` - Removes resources
- `clear_images.sh` - Deletes images

**Always dry-run first!**

### NATS Publishing
- Use `publish_test_messages_natsbox.sh` (most reliable)
- Avoid `publish_messages_direct.sh` (requires nats-cli)
- Check NATS logs if publishing fails

### Database Queries
- Requires port forward or exec into postgres pod
- Use `../database/compare_db_k8s.sh` for comprehensive queries
- Be careful with large result sets

---

## 📖 Related Documentation

- [NATS Architecture](../../docs/02-architecture/README.md#nats-jetstream)
- [Policy Engine](../../docs/03-components/policy-engine/README.md)
- [Database Scripts](../database/README.md)
- [Monitoring Scripts](../monitoring/README.md)

---

*Back to [Scripts README](../README.md)*

