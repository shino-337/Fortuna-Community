# Verify after clean/rebuild/redeploy
**Time:** 2026-03-04T05:16:41+00:00

NAME                                 READY   STATUS    RESTARTS   AGE    IP            NODE           NOMINATED NODE   READINESS GATES
fortuna-agent-gpp7v                  1/1     Running   0          19h    10.244.0.75   k8s-master     <none>           <none>
fortuna-agent-pdpc5                  1/1     Running   0          19h    10.244.1.58   k8s-worker01   <none>           <none>
fortuna-core-c6fd6697b-mf6nn         1/1     Running   0          27s    10.244.0.80   k8s-master     <none>           <none>
fortuna-dashboard-6659f44859-v9qn2   1/1     Running   0          120m   10.244.0.79   k8s-master     <none>           <none>
nats-0                               1/1     Running   0          2d3h   10.244.1.38   k8s-worker01   <none>           <none>
nats-1                               1/1     Running   0          46h    10.244.0.65   k8s-master     <none>           <none>
nats-2                               1/1     Running   0          46h    10.244.1.41   k8s-worker01   <none>           <none>
postgres-5484f7745d-znslf            1/1     Running   0          2d3h   10.244.1.39   k8s-worker01   <none>           <none>

Core running: 1 | Agent running: 2

### Tables (including 071)
 public | agents                  | table | postgres
 public | audit_logs              | table | postgres
 public | capability_metadata     | table | postgres
 public | cluster_role_bindings   | table | postgres
 public | cluster_roles           | table | postgres
 public | clusters                | table | postgres
 public | cve_file_metadata       | table | postgres
 public | cve_matches             | table | postgres
 public | cves                    | table | postgres
 public | deployments             | table | postgres
 public | error_logs              | table | postgres
 public | events_index            | table | postgres
 public | insights                | table | postgres
 public | k8s_events              | table | postgres
 public | namespaces              | table | postgres
 public | nodes                   | table | postgres
 public | notifications           | table | postgres
 public | package_vulnerabilities | table | postgres
 public | pod_attack_steps        | table | postgres
 public | pod_capabilities        | table | postgres
 public | pod_instances           | table | postgres
 public | pod_network_connections | table | postgres
 public | pod_processes           | table | postgres
 public | pod_risk_profiles       | table | postgres
 public | pod_runtime_metrics     | table | postgres
 public | pods                    | table | postgres
 public | policies                | table | postgres
 public | policy_templates        | table | postgres
 public | promotion_rules         | table | postgres
 public | replicasets             | table | postgres
 public | risk_scores             | table | postgres
 public | role_bindings           | table | postgres
 public | roles                   | table | postgres
 public | runtime_events          | table | postgres
 public | runtime_signals         | table | postgres
 public | sbom_components         | table | postgres
 public | sboms                   | table | postgres
 public | service_accounts        | table | postgres
 public | users                   | table | postgres

  pods: 45
  pod_runtime_metrics: 0
  pod_processes: 0
  pod_network_connections: 0
  k8s_events: 0
  runtime_events: 8
  runtime_signals: 8
  pod_capabilities: 149

Sample pods (id, name, uid, spec_hash):
 16 | kube-proxy-kt6sf                        | bd71cabf-2360-48af-8c60-164a7bd7728e | 441b2d6064aa
 18 | kube-scheduler-k8s-master               | c88e5f07-fd1d-4688-8d22-411e62228024 | 8d47f1c24969
 19 | local-path-provisioner-844bd8758f-cvps6 | 7d6a0d23-c724-41ff-b604-97ef860ab43a | 0dc54c020f5e
 17 | kube-proxy-nr8lb                        | 5bbc734d-f668-4d2a-a3b6-cb2ed2998244 | 65b8aa627790
 44 | coredns-76f75df574-mhfpc                | 65a53485-f39b-4880-ab55-cbc6c13a86e4 | ee6a56d66bf4


### Core last 30 lines

2026/03/04 05:16:33 [32mgithub.com/fortuna/core/pkg/capability/evaluator.go:400
[0m[33m[2.877ms] [34;1m[rows:1][0m INSERT INTO "pod_risk_profiles" ("pod_uid","namespace","static_risk","runtime_score","capabilities","last_event_at","created_at","updated_at") VALUES ('63015d8c-ecdf-4ea5-a0b0-ba68c06b7e92','fortuna',0,0,'{"ID_TOKEN_POD"}',NULL,'2026-03-04 05:16:33.006','2026-03-04 05:16:33.006') ON CONFLICT ("pod_uid") DO UPDATE SET "namespace"="excluded"."namespace","static_risk"="excluded"."static_risk","capabilities"="excluded"."capabilities","updated_at"="excluded"."updated_at" RETURNING "id"

2026/03/04 05:16:33 [32mgithub.com/fortuna/core/pkg/capability/evaluator.go:168
[0m[33m[1.030ms] [34;1m[rows:1][0m SELECT * FROM "capability_metadata" WHERE capability_id IN ('ID_TOKEN_POD')

2026/03/04 05:16:33 [32mgithub.com/fortuna/core/pkg/riskengine/insight_manager.go:103
[0m[33m[0.970ms] [34;1m[rows:1][0m SELECT * FROM "insights" WHERE (insight_type = 'capability' AND resource_uid = '63015d8c-ecdf-4ea5-a0b0-ba68c06b7e92' AND cve_id = 'ID_TOKEN_POD' AND (status = 'active' OR status IS NULL) AND deleted_at IS NULL) AND "insights"."deleted_at" IS NULL ORDER BY "insights"."id" LIMIT 1
2026/03/04 05:16:33.012356 [InsightManager] Capability insight already exists (ID=958), no update needed

2026/03/04 05:16:33 [32mgithub.com/fortuna/core/pkg/capability/evaluator.go:227
[0m[33m[2.662ms] [34;1m[rows:0][0m UPDATE "insights" SET "status"='resolved',"updated_at"='2026-03-04 05:16:33.013' WHERE (resource_uid = '63015d8c-ecdf-4ea5-a0b0-ba68c06b7e92' AND insight_type = 'capability' AND deleted_at IS NULL AND (status = 'active' OR status IS NULL)) AND title NOT IN ('ServiceAccount Token Access detected') AND "insights"."deleted_at" IS NULL
2026/03/04 05:16:33.015927 [PCEScheduler] PCE evaluation completed in 1.786608709s
[GIN] 2026/03/04 - 05:16:35 | 200 |    4.316085ms |      10.244.0.1 | GET      "/ready"
2026/03/04 05:16:37.065445 [Agent] Register: id=k8s-worker01-agent, node=k8s-worker01, version=dev, capabilities=[sbom pod-watcher]

2026/03/04 05:16:37 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:345
[0m[33m[4.959ms] [34;1m[rows:1][0m SELECT * FROM "agents" WHERE agent_id = 'k8s-worker01-agent' AND "agents"."deleted_at" IS NULL ORDER BY "agents"."id" LIMIT 1

2026/03/04 05:16:37 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:348
[0m[33m[3.717ms] [34;1m[rows:1][0m UPDATE "agents" SET "capabilities"='["sbom","pod-watcher"]',"deleted_at"=NULL,"last_seen_at"='2026-03-04 05:16:37.065',"node_name"='k8s-worker01',"status"='ready',"version"='dev',"updated_at"='2026-03-04 05:16:37.071' WHERE "agents"."deleted_at" IS NULL AND "id" = 1
2026/03/04 05:16:37.176492 [Agent] Register: id=k8s-master-agent, node=k8s-master, version=dev, capabilities=[sbom pod-watcher]

2026/03/04 05:16:37 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:345
[0m[33m[3.211ms] [34;1m[rows:1][0m SELECT * FROM "agents" WHERE agent_id = 'k8s-master-agent' AND "agents"."deleted_at" IS NULL ORDER BY "agents"."id" LIMIT 1

2026/03/04 05:16:37 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:348
[0m[33m[2.877ms] [34;1m[rows:1][0m UPDATE "agents" SET "capabilities"='["sbom","pod-watcher"]',"deleted_at"=NULL,"last_seen_at"='2026-03-04 05:16:37.176',"node_name"='k8s-master',"status"='ready',"version"='dev',"updated_at"='2026-03-04 05:16:37.18' WHERE "agents"."deleted_at" IS NULL AND "id" = 2
[GIN] 2026/03/04 - 05:16:40 | 200 |    2.571983ms |      10.244.0.1 | GET      "/ready"

### Agent last 25 lines
[SBOMExtractor] 2026/03/04 05:16:24 Detected OS: debian 12.13
[SBOMExtractor] 2026/03/04 05:16:24    Selected parsers for OS 'debian': [dpkg npm pip gomod]
[SBOMExtractor] 2026/03/04 05:16:24 ✅ Parser dpkg found 106 packages
2026/03/04 05:16:24.662035 [SBOMExtractor] npm fallback: found 0 package-lock.json, 0 node_modules/.../package.json
[SBOMExtractor] 2026/03/04 05:16:24    Parser npm: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/04 05:16:24    Parser pip: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/04 05:16:24    Parser gomod: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/04 05:16:24 ✅ Extracted 106 unique packages in 8.71678467s
[SBOMProcessor] 2026/03/04 05:16:24 ✅ Extracted 106 packages from fortuna-core:latest
[SBOMProcessor] 2026/03/04 05:16:24 ⚠️  Failed to process container core in pod fortuna/fortuna-core-c6fd6697b-mf6nn: failed to send SBOM to Core: client not connected
[SBOMQueue] 2026/03/04 05:16:24 [Worker 0] ✅ Completed pod fortuna/fortuna-core-c6fd6697b-mf6nn in 8.718503674s
2026/03/04 05:16:37.149240 ⚠️  Reconnect failed: dial Core: context deadline exceeded (hint: check Core pod Ready, Service endpoints, DNS, mTLS certs)
2026/03/04 05:16:37.149307 ⚠️  Heartbeat failed: client not connected
2026/03/04 05:16:37.149314 🔄 Reconnecting to Core after 11 consecutive failures...
[gRPCClient] 2026/03/04 05:16:37 Connecting to Core at fortuna-core.fortuna.svc.cluster.local:9090 (TLS: true)
[gRPCClient] 2026/03/04 05:16:37 Loading mTLS certificates...
[gRPCClient] 2026/03/04 05:16:37   Cert: /etc/fortuna/certs/tls.crt
[gRPCClient] 2026/03/04 05:16:37   Key:  /etc/fortuna/certs/tls.key
[gRPCClient] 2026/03/04 05:16:37   CA:   /etc/fortuna/ca-cert/ca.crt
[gRPCClient] 2026/03/04 05:16:37 ✅ mTLS configured
[gRPCClient] 2026/03/04 05:16:37 ✅ Connected to Core at fortuna-core.fortuna.svc.cluster.local:9090
[gRPCClient] 2026/03/04 05:16:37 Registering agent: id=k8s-master-agent node=k8s-master
[gRPCClient] 2026/03/04 05:16:37 ✅ Agent registered: Agent registered successfully
2026/03/04 05:16:37.183403 📋 Agent registered with Core (Cluster ID: unknown)
2026/03/04 05:16:37.183449 ✅ Reconnected and re-registered


[0;34m========== 1. Unit tests (Spec Hash, PCE, Pod Detail) ==========[0m

=== RUN   TestEqualTimePtr
--- PASS: TestEqualTimePtr (0.00s)
=== RUN   TestProcessSyncedPods_PodDetailFields
[AgentService] 2026/03/04 05:16:48.682038 📦 Processing 1 pods
[AgentService] 2026/03/04 05:16:48.682071 📦 Pod phase from agent: default/nginx phase=Running

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1052 [35;1mrecord not found
[0m[33m[0.117ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-1" AND uid = "pod-uid-123") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1126 [35;1mrecord not found
[0m[33m[0.198ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-1" AND uid = "pod-uid-123" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 05:16:48.683105 ✨ Created Pod default/nginx (SA: default)

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.046ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-123" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
--- PASS: TestProcessSyncedPods_PodDetailFields (0.01s)
=== RUN   TestProcessSyncedPods_SpecHashStoredAndConditionalPCE

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:543 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.041ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-1" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL
[AgentService] 2026/03/04 05:16:48.687834 ❌ PCE evaluation failed for pod default/nginx: SQL logic error: no such table: role_bindings (1)
[AgentService] 2026/03/04 05:16:48.705600 📦 Processing 1 pods
[AgentService] 2026/03/04 05:16:48.705769 📦 Pod phase from agent: default/p1 phase=Running

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1052 [35;1mrecord not found
[0m[33m[0.128ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1126 [35;1mrecord not found
[0m[33m[0.114ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-spec" AND uid = "uid-spec-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 05:16:48.706606 ✨ Created Pod default/p1 (SA: default)

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.040ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-spec-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/04 05:16:48.708134 📦 Processing 1 pods
[AgentService] 2026/03/04 05:16:48.708198 📦 Pod phase from agent: default/p1 phase=Pending
[AgentService] 2026/03/04 05:16:48.708618 🔄 Updated Pod default/p1 (SA: default)

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1241 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[2.013ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 05:16:48.709593 ⚠️  PCE skipped: pod not found (cluster=test-cluster-spec uid=uid-spec-1): SQL logic error: no such table: pods (1)
[AgentService] 2026/03/04 05:16:48.710645 📦 Processing 1 pods
[AgentService] 2026/03/04 05:16:48.710671 📦 Pod phase from agent: default/p1 phase=Running
[AgentService] 2026/03/04 05:16:48.711749 🔄 Updated Pod default/p1 (SA: default)

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1241 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.046ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 05:16:48.714072 ⚠️  PCE skipped: pod not found (cluster=test-cluster-spec uid=uid-spec-1): SQL logic error: no such table: pods (1)
--- PASS: TestProcessSyncedPods_SpecHashStoredAndConditionalPCE (0.03s)
=== RUN   TestProcessSyncedPods_NullToNonNullSpecHash
[AgentService] 2026/03/04 05:16:48.724374 📦 Processing 1 pods
[AgentService] 2026/03/04 05:16:48.724498 📦 Pod phase from agent: default/p2 phase=Running

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1052 [35;1mrecord not found
[0m[33m[0.118ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1126 [35;1mrecord not found
[0m[33m[0.105ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-null" AND uid = "uid-null-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 05:16:48.725176 ✨ Created Pod default/p2 (SA: default)

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.039ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-null-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/04 05:16:48.726872 📦 Processing 1 pods
[AgentService] 2026/03/04 05:16:48.727058 📦 Pod phase from agent: default/p2 phase=Running
[AgentService] 2026/03/04 05:16:48.727362 🔄 Updated Pod default/p2 (SA: default)
--- PASS: TestProcessSyncedPods_NullToNonNullSpecHash (0.01s)
=== RUN   TestProcessSyncedPods_UpdatePodDetailFields

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1241 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[2.634ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 05:16:48.730120 ⚠️  PCE skipped: pod not found (cluster=test-cluster-null uid=uid-null-1): SQL logic error: no such table: pods (1)

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1241 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.028ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 05:16:48.730133 ⚠️  PCE skipped: pod not found (cluster=test-cluster-null uid=uid-null-1): SQL logic error: no such table: pods (1)
[AgentService] 2026/03/04 05:16:48.734924 📦 Processing 1 pods
[AgentService] 2026/03/04 05:16:48.735048 📦 Pod phase from agent: default/app phase=Running
[AgentService] 2026/03/04 05:16:48.735340 🔄 Updated Pod default/app (SA: default)

2026/03/04 05:16:48 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.041ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-456" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
--- PASS: TestProcessSyncedPods_UpdatePodDetailFields (0.01s)
PASS
ok  	github.com/fortuna/core/internal/service	0.080s
[0;32mAgent service tests: PASS[0m
=== RUN   TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch
2026/03/04 05:16:50 [PCE] Skip stale evaluation for pod default/p (spec_hash changed)
--- PASS: TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch (0.00s)
=== RUN   TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount
--- PASS: TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount (0.01s)
PASS
ok  	github.com/fortuna/core/pkg/capability	0.020s
[0;32mCapability evaluator tests: PASS[0m
=== RUN   TestGetPod_ReturnsPodDetailFields
--- PASS: TestGetPod_ReturnsPodDetailFields (0.00s)
=== RUN   TestGetPodByUID_ReturnsPodDetailFields
--- PASS: TestGetPodByUID_ReturnsPodDetailFields (0.02s)
PASS
ok  	github.com/fortuna/core/internal/api	0.078s
?   	github.com/fortuna/core/internal/api/policy	[no test files]
testing: warning: no tests to run
PASS
ok  	github.com/fortuna/core/internal/api/risk	0.018s [no tests to run]
[0;32mAPI pod handlers tests: PASS[0m

[0;34m========== 2. Database verification ==========[0m

[0;32mUsing Postgres pod: postgres-5484f7745d-znslf[0m
[0;32m  pods.spec_hash: present[0m
[0;32m  pods.last_evaluated_hash: present[0m
[0;32m  EXPLAIN uses index (or seq scan on small table).[0m

[0;34m========== 3. Process / environment info ==========[0m

[0;32mCore pod: fortuna-core-c6fd6697b-mf6nn[0m
[0;32mAgent pod: fortuna-agent-gpp7v[0m

[0;34m========== 4. Test suite mapping (testSuite.md) ==========[0m


[0;32mReport written to: /home/k8s/KSAM/docs/test-results/pod-detail-test-suite-report-20260304-051644.md[0m


### API checks
- GET /pods: pod id=45
podIP: 10.244.0.79 | startTime: 2026-03-04T03:16:36Z | riskCount: 54
- GET /pods/45/runtime-metrics: 200
- GET /pods/45/processes: 200
- GET /pods/45/network-connections: 200
- GET /pods/45/events: 200

