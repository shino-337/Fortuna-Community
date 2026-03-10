# Verify after clean/rebuild/redeploy
**Time:** 2026-03-04T06:04:28+00:00

NAME                                 READY   STATUS    RESTARTS   AGE    IP            NODE           NOMINATED NODE   READINESS GATES
fortuna-agent-gpp7v                  1/1     Running   0          19h    10.244.0.75   k8s-master     <none>           <none>
fortuna-agent-pdpc5                  1/1     Running   0          19h    10.244.1.58   k8s-worker01   <none>           <none>
fortuna-core-c6fd6697b-mf6nn         1/1     Running   0          48m    10.244.0.80   k8s-master     <none>           <none>
fortuna-dashboard-6659f44859-v9qn2   1/1     Running   0          167m   10.244.0.79   k8s-master     <none>           <none>
nats-0                               1/1     Running   0          2d3h   10.244.1.38   k8s-worker01   <none>           <none>
nats-1                               1/1     Running   0          47h    10.244.0.65   k8s-master     <none>           <none>
nats-2                               1/1     Running   0          47h    10.244.1.41   k8s-worker01   <none>           <none>
postgres-5484f7745d-znslf            1/1     Running   0          2d4h   10.244.1.39   k8s-worker01   <none>           <none>

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

  pods: 48
  pod_runtime_metrics: 0
  pod_processes: 0
  pod_network_connections: 0
  k8s_events: 0
  runtime_events: 8
  runtime_signals: 8
  pod_capabilities: 162

Sample pods (id, name, uid, spec_hash):
 14 | kube-apiserver-k8s-master          | e69f29f5-1cba-4519-b566-993325f2e4e3 | 25389eaf456e
 17 | kube-proxy-nr8lb                   | 5bbc734d-f668-4d2a-a3b6-cb2ed2998244 | 65b8aa627790
  7 | nats-2                             | 77116437-5df0-4809-89ff-ad92d5111d66 | 9d14c466626f
 15 | kube-controller-manager-k8s-master | e86b05b2-a2fa-48cf-9a3e-4a2d190696f5 | 43847f98e1fd
 16 | kube-proxy-kt6sf                   | bd71cabf-2360-48af-8c60-164a7bd7728e | 441b2d6064aa


### Core last 30 lines
[GIN] 2026/03/04 - 06:02:55 | 200 |      40.346µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:02:55 | 200 |     2.82199ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:00 | 200 |    5.452011ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:05 | 200 |      80.762µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:03:05 | 200 |   25.213934ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:10 | 200 |     2.96607ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:15 | 200 |      53.441µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:03:15 | 200 |   26.754512ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:20 | 200 |   17.238632ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:25 | 200 |       74.63µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:03:25 | 200 |    3.594431ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:30 | 200 |    2.752989ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:35 | 200 |      56.737µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:03:35 | 200 |   54.423282ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:40 | 200 |    2.828149ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:45 | 200 |      67.787µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:03:45 | 200 |    3.076957ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:50 | 200 |   34.166872ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:03:55 | 200 |      68.369µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:03:55 | 200 |    3.283313ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:04:00 | 200 |    2.906426ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:04:05 | 200 |      67.607µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:04:05 | 200 |    3.332134ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:04:10 | 200 |    2.467241ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:04:15 | 200 |      60.744µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:04:15 | 200 |   19.855767ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:04:20 | 200 |   29.373696ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:04:25 | 200 |     184.466µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/04 - 06:04:25 | 200 |    2.736015ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/04 - 06:04:30 | 200 |    2.458794ms |      10.244.0.1 | GET      "/ready"

### Agent last 25 lines
[gRPCClient] 2026/03/04 05:16:37   CA:   /etc/fortuna/ca-cert/ca.crt
[gRPCClient] 2026/03/04 05:16:37 ✅ mTLS configured
[gRPCClient] 2026/03/04 05:16:37 ✅ Connected to Core at fortuna-core.fortuna.svc.cluster.local:9090
[gRPCClient] 2026/03/04 05:16:37 Registering agent: id=k8s-master-agent node=k8s-master
[gRPCClient] 2026/03/04 05:16:37 ✅ Agent registered: Agent registered successfully
2026/03/04 05:16:37.183403 📋 Agent registered with Core (Cluster ID: unknown)
2026/03/04 05:16:37.183449 ✅ Reconnected and re-registered
2026/03/04 05:20:30.086558 [Syncer] ✅ Full sync completed
2026/03/04 05:21:22.109143 ✅ Heartbeat OK (interval=15s)
2026/03/04 05:25:30.108871 [Syncer] ✅ Full sync completed
2026/03/04 05:26:22.108613 ✅ Heartbeat OK (interval=15s)
2026/03/04 05:30:30.217050 [Syncer] ✅ Full sync completed
2026/03/04 05:31:22.125736 ✅ Heartbeat OK (interval=15s)
2026/03/04 05:35:30.130475 [Syncer] ✅ Full sync completed
2026/03/04 05:36:22.109683 ✅ Heartbeat OK (interval=15s)
2026/03/04 05:40:30.025365 [Syncer] ✅ Full sync completed
2026/03/04 05:41:22.108255 ✅ Heartbeat OK (interval=15s)
2026/03/04 05:45:30.061744 [Syncer] ✅ Full sync completed
2026/03/04 05:46:22.108255 ✅ Heartbeat OK (interval=15s)
2026/03/04 05:50:30.321535 [Syncer] ✅ Full sync completed
2026/03/04 05:51:22.108407 ✅ Heartbeat OK (interval=15s)
2026/03/04 05:55:29.932997 [Syncer] ✅ Full sync completed
2026/03/04 05:56:22.110473 ✅ Heartbeat OK (interval=15s)
2026/03/04 06:00:30.041711 [Syncer] ✅ Full sync completed
2026/03/04 06:01:22.112734 ✅ Heartbeat OK (interval=15s)


[0;34m========== 1. Unit tests (Spec Hash, PCE, Pod Detail) ==========[0m

=== RUN   TestEqualTimePtr
--- PASS: TestEqualTimePtr (0.00s)
=== RUN   TestProcessSyncedPods_PodDetailFields
[AgentService] 2026/03/04 06:04:31.376517 📦 Processing 1 pods
[AgentService] 2026/03/04 06:04:31.376721 📦 Pod phase from agent: default/nginx phase=Running

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1052 [35;1mrecord not found
[0m[33m[0.821ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-1" AND uid = "pod-uid-123") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1126 [35;1mrecord not found
[0m[33m[0.117ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-1" AND uid = "pod-uid-123" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 06:04:31.379051 ✨ Created Pod default/nginx (SA: default)

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.045ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-123" ORDER BY `pod_instances`.`pod_uid` LIMIT 1

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1241 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.993ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-1" AND uid = "pod-uid-123") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 06:04:31.381324 ⚠️  PCE skipped: pod not found (cluster=test-cluster-1 uid=pod-uid-123): SQL logic error: no such table: pods (1)
--- PASS: TestProcessSyncedPods_PodDetailFields (0.01s)
=== RUN   TestProcessSyncedPods_SpecHashStoredAndConditionalPCE
[AgentService] 2026/03/04 06:04:31.390876 📦 Processing 1 pods
[AgentService] 2026/03/04 06:04:31.391377 📦 Pod phase from agent: default/p1 phase=Running

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1052 [35;1mrecord not found
[0m[33m[0.584ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1126 [35;1mrecord not found
[0m[33m[0.099ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-spec" AND uid = "uid-spec-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 06:04:31.393575 ✨ Created Pod default/p1 (SA: default)

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.060ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-spec-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/04 06:04:31.401852 📦 Processing 1 pods
[AgentService] 2026/03/04 06:04:31.402948 📦 Pod phase from agent: default/p1 phase=Pending

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:543 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.182ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-spec" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL
[AgentService] 2026/03/04 06:04:31.403382 ❌ PCE evaluation failed for pod default/p1: SQL logic error: no such table: role_bindings (1)
[AgentService] 2026/03/04 06:04:31.404079 🔄 Updated Pod default/p1 (SA: default)
[AgentService] 2026/03/04 06:04:31.404901 📦 Processing 1 pods
[AgentService] 2026/03/04 06:04:31.405038 📦 Pod phase from agent: default/p1 phase=Running
[AgentService] 2026/03/04 06:04:31.405593 🔄 Updated Pod default/p1 (SA: default)
--- PASS: TestProcessSyncedPods_SpecHashStoredAndConditionalPCE (0.02s)
=== RUN   TestProcessSyncedPods_NullToNonNullSpecHash

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1241 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[1.043ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 06:04:31.407560 ⚠️  PCE skipped: pod not found (cluster=test-cluster-spec uid=uid-spec-1): SQL logic error: no such table: pods (1)
[AgentService] 2026/03/04 06:04:31.410205 📦 Processing 1 pods
[AgentService] 2026/03/04 06:04:31.410564 📦 Pod phase from agent: default/p2 phase=Running

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1052 [35;1mrecord not found
[0m[33m[0.121ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1126 [35;1mrecord not found
[0m[33m[0.091ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-null" AND uid = "uid-null-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 06:04:31.411995 ✨ Created Pod default/p2 (SA: default)

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.039ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-null-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/04 06:04:31.413500 📦 Processing 1 pods
[AgentService] 2026/03/04 06:04:31.413506 📦 Pod phase from agent: default/p2 phase=Running
[AgentService] 2026/03/04 06:04:31.414029 🔄 Updated Pod default/p2 (SA: default)
--- PASS: TestProcessSyncedPods_NullToNonNullSpecHash (0.01s)
=== RUN   TestProcessSyncedPods_UpdatePodDetailFields

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1241 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[2.340ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 06:04:31.415188 ⚠️  PCE skipped: pod not found (cluster=test-cluster-null uid=uid-null-1): SQL logic error: no such table: pods (1)

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1241 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.018ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/04 06:04:31.415236 ⚠️  PCE skipped: pod not found (cluster=test-cluster-null uid=uid-null-1): SQL logic error: no such table: pods (1)
[AgentService] 2026/03/04 06:04:31.422076 📦 Processing 1 pods
[AgentService] 2026/03/04 06:04:31.422130 📦 Pod phase from agent: default/app phase=Running
[AgentService] 2026/03/04 06:04:31.422848 🔄 Updated Pod default/app (SA: default)

2026/03/04 06:04:31 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.044ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-456" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
--- PASS: TestProcessSyncedPods_UpdatePodDetailFields (0.01s)
PASS
ok  	github.com/fortuna/core/internal/service	0.073s
[0;32mAgent service tests: PASS[0m
=== RUN   TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch
2026/03/04 06:04:32 [PCE] Skip stale evaluation for pod default/p (spec_hash changed)
--- PASS: TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch (0.00s)
=== RUN   TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount
--- PASS: TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount (0.00s)
PASS
ok  	github.com/fortuna/core/pkg/capability	0.018s
[0;32mCapability evaluator tests: PASS[0m
=== RUN   TestGetPod_ReturnsPodDetailFields
--- PASS: TestGetPod_ReturnsPodDetailFields (0.00s)
=== RUN   TestGetPodByUID_ReturnsPodDetailFields
--- PASS: TestGetPodByUID_ReturnsPodDetailFields (0.00s)
PASS
ok  	github.com/fortuna/core/internal/api	0.036s
?   	github.com/fortuna/core/internal/api/policy	[no test files]
testing: warning: no tests to run
PASS
ok  	github.com/fortuna/core/internal/api/risk	0.009s [no tests to run]
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


[0;32mReport written to: /home/k8s/KSAM/docs/test-results/pod-detail-test-suite-report-20260304-060430.md[0m


### API checks
- GET /pods: pod id=48
podIP: 192.168.56.100 | startTime: 2026-03-04T05:14:30Z | riskCount: 5
- GET /pods/48/runtime-metrics: 200
- GET /pods/48/processes: 200
- GET /pods/48/network-connections: 200
- GET /pods/48/events: 200

