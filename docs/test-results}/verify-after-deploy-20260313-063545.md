# Verify after clean/rebuild/redeploy
**Time:** 2026-03-13T06:35:45+00:00

NAME                                 READY   STATUS    RESTARTS      AGE     IP             NODE           NOMINATED NODE   READINESS GATES
fortuna-agent-8cxst                  1/1     Running   0             3m16s   10.244.1.221   k8s-worker01   <none>           <none>
fortuna-agent-d2dtd                  1/1     Running   0             3m18s   10.244.0.252   k8s-master     <none>           <none>
fortuna-core-5577f44c8b-hk4fp        1/1     Running   0             3m8s    10.244.0.254   k8s-master     <none>           <none>
fortuna-dashboard-5d4d84c46b-5cnxb   1/1     Running   0             3m14s   10.244.0.253   k8s-master     <none>           <none>
nats-0                               1/1     Running   1 (27h ago)   36h     10.244.1.206   k8s-worker01   <none>           <none>
nats-1                               1/1     Running   1 (27h ago)   36h     10.244.0.234   k8s-master     <none>           <none>
nats-2                               1/1     Running   1 (27h ago)   36h     10.244.1.204   k8s-worker01   <none>           <none>
postgres-5484f7745d-998x9            1/1     Running   1 (27h ago)   36h     10.244.1.205   k8s-worker01   <none>           <none>

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
 public | image_scan_results      | table | postgres
 public | insights                | table | postgres
 public | k8s_events              | table | postgres
 public | namespaces              | table | postgres
 public | nodes                   | table | postgres
 public | notifications           | table | postgres
 public | package_vulnerabilities | table | postgres
 public | pod_attack_steps        | table | postgres
 public | pod_capabilities        | table | postgres
 public | pod_image_scans         | table | postgres
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
 public | risk_rules              | table | postgres
 public | risk_rules_history      | table | postgres
 public | risk_scores             | table | postgres
 public | role_bindings           | table | postgres
 public | roles                   | table | postgres
 public | runtime_events          | table | postgres
 public | runtime_signals         | table | postgres
 public | sbom_components         | table | postgres
 public | sboms                   | table | postgres
 public | service_accounts        | table | postgres
 public | users                   | table | postgres

  pods: 20
  pod_runtime_metrics: 46
  pod_processes: 50
  pod_network_connections: 5506
  k8s_events: 216
  runtime_events: 0
  runtime_signals: 0
  pod_capabilities: 69

Sample pods (id, name, uid, spec_hash):
 19 | local-path-provisioner-844bd8758f-4njgq | 749db13b-b936-427c-bcd1-73be5ff552be | 
 18 | kube-scheduler-k8s-master               | c88e5f07-fd1d-4688-8d22-411e62228024 | 
 17 | kube-proxy-nr8lb                        | 5bbc734d-f668-4d2a-a3b6-cb2ed2998244 | 
 16 | kube-proxy-kt6sf                        | bd71cabf-2360-48af-8c60-164a7bd7728e | 
 15 | kube-controller-manager-k8s-master      | e86b05b2-a2fa-48cf-9a3e-4a2d190696f5 | 


### Core last 30 lines
				GROUP BY resource_uid
			
[GIN] 2026/03/13 - 06:35:31 | 200 |    20.44781ms |       127.0.0.1 | GET      "/api/v1/inventory/pods?cluster=sha256-8b72a957dd83b5ea&page=1&pageSize=20"

2026/03/13 06:35:31 [32mgithub.com/fortuna/core/internal/middleware/auth.go:45
[0m[33m[1.830ms] [34;1m[rows:1][0m SELECT * FROM "users" WHERE "users"."id" = 1 AND "users"."deleted_at" IS NULL ORDER BY "users"."id" LIMIT 1

2026/03/13 06:35:31 [32mgithub.com/fortuna/core/internal/api/handlers.go:1114
[0m[33m[2.384ms] [34;1m[rows:1][0m SELECT count(*) FROM "pods" WHERE cluster_id = 'sha256-8b72a957dd83b5ea' AND "pods"."deleted_at" IS NULL

2026/03/13 06:35:31 [32mgithub.com/fortuna/core/internal/api/handlers.go:1117
[0m[33m[3.579ms] [34;1m[rows:18][0m SELECT * FROM "pods" WHERE cluster_id = 'sha256-8b72a957dd83b5ea' AND "pods"."deleted_at" IS NULL ORDER BY created_at DESC LIMIT 20

2026/03/13 06:35:31 [32mgithub.com/fortuna/core/internal/api/handlers.go:1124
[0m[33m[2.757ms] [34;1m[rows:1][0m SELECT count(*) FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'insights' AND table_type = 'BASE TABLE'

2026/03/13 06:35:31 [32mgithub.com/fortuna/core/internal/api/handlers.go:1137
[0m[33m[1.515ms] [34;1m[rows:18][0m 
				SELECT resource_uid, COUNT(*) as count FROM insights
				WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND resource_uid IN ('b59c2f18-6128-4fd8-8a85-b2c5f7a52b23','749db13b-b936-427c-bcd1-73be5ff552be','c88e5f07-fd1d-4688-8d22-411e62228024','5bbc734d-f668-4d2a-a3b6-cb2ed2998244','bd71cabf-2360-48af-8c60-164a7bd7728e','e86b05b2-a2fa-48cf-9a3e-4a2d190696f5','e69f29f5-1cba-4519-b566-993325f2e4e3','0e535fb4-dd6c-4b6d-9a18-f2b848400ac4','cbfae0bb-013e-4716-812f-4b4ef030bf2f','0f2e43e9-9f91-48ee-b98e-40a0def36902','29c4f314-6e3f-4d4e-a797-83aa377a82bb','32c696cd-f805-4f48-8df8-d6c9e72c08cf','e0e5054b-1d2f-4de3-ab61-277ca6cea511','14944354-ba8a-4e29-bae1-6de1bcadcbcb','eb24d3ce-b5cc-43cb-b766-7ad7ec4a5af4','ce1f7139-b690-41a4-a639-e14509dcd7e3','ad0e56dc-bf52-4fde-87ce-b7310fe7a879','2baf0843-7d90-4908-be92-b815cbfecba3')
				GROUP BY resource_uid
			
[GIN] 2026/03/13 - 06:35:31 | 200 |   15.345456ms |       127.0.0.1 | GET      "/api/v1/inventory/pods?cluster=sha256-8b72a957dd83b5ea&page=1&pageSize=20"
[GIN] 2026/03/13 - 06:35:33 | 200 |    3.438628ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/13 - 06:35:38 | 200 |      54.572µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/13 - 06:35:38 | 200 |    4.678532ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/13 - 06:35:43 | 200 |    4.571424ms |      10.244.0.1 | GET      "/ready"

2026/03/13 06:35:44 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:289
[0m[33m[5.080ms] [34;1m[rows:1][0m UPDATE agents SET last_seen_at = '2026-03-13 06:35:44.261', node_name = 'k8s-master', status = 'ready', updated_at = '2026-03-13 06:35:44.261', deleted_at = NULL WHERE agent_id = 'k8s-master-agent'

### Agent last 25 lines
2026/03/13 06:32:54.547770 [SBOMExtractor] npm fallback: found 0 package-lock.json, 0 node_modules/.../package.json
[SBOMExtractor] 2026/03/13 06:32:54    Parser npm: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 06:32:54    Parser pip: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 06:32:54    Parser gomod: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 06:32:54 ✅ Extracted 0 unique packages in 6.693237656s
[SBOMProcessor] 2026/03/13 06:32:54 ✅ Extracted 0 packages from registry.k8s.io/kube-proxy:v1.29.15
[gRPCClient] 2026/03/13 06:32:54 Sending SBOM: pod=kube-system/kube-proxy-kt6sf image=sha256:f6074f465fb3700456dccc5915340df4b59a7960c591693182fbd297cbe72b53
[SBOMProcessor] 2026/03/13 06:32:54 ✅ SBOM sent to Core: sbom_id=9 message=SBOM received and stored
[SBOMQueue] 2026/03/13 06:32:54 [Worker 0] ✅ Completed pod kube-system/kube-proxy-kt6sf in 6.739511438s
[SBOMExtractor] 2026/03/13 06:33:18 ✅ Found image in containerd: docker.io/library/fortuna-agent:latest (materialized 5 layers)
[SBOMExtractor] 2026/03/13 06:33:18 🔍 Attempting to get digest from image manifest...
[SBOMExtractor] 2026/03/13 06:33:18 ✅ Extracted digest from image manifest: sha256:4ecf2244712ee08a56cc3b48c47e8e2ca2ce5ad281e4554977e303141a8fe9ac
[SBOMExtractor] 2026/03/13 06:33:18    Virtual FS: 3719 files total, 0 paths containing package.json
[SBOMExtractor] 2026/03/13 06:33:18 Detected OS: debian 12.13
[SBOMExtractor] 2026/03/13 06:33:18    Selected parsers for OS 'debian': [dpkg npm pip gomod]
[SBOMExtractor] 2026/03/13 06:33:18 ✅ Parser dpkg found 106 packages
2026/03/13 06:33:18.467567 [SBOMExtractor] npm fallback: found 0 package-lock.json, 0 node_modules/.../package.json
[SBOMExtractor] 2026/03/13 06:33:18    Parser npm: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 06:33:18    Parser pip: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 06:33:18    Parser gomod: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 06:33:18 ✅ Extracted 106 unique packages in 24.53116378s
[SBOMProcessor] 2026/03/13 06:33:18 ✅ Extracted 106 packages from fortuna-agent:latest
[gRPCClient] 2026/03/13 06:33:18 Sending SBOM: pod=fortuna/fortuna-agent-8cxst image=sha256:4ecf2244712ee08a56cc3b48c47e8e2ca2ce5ad281e4554977e303141a8fe9ac
[SBOMProcessor] 2026/03/13 06:33:18 ✅ SBOM sent to Core: sbom_id=13 message=SBOM received and stored
[SBOMQueue] 2026/03/13 06:33:18 [Worker 1] ✅ Completed pod fortuna/fortuna-agent-8cxst in 24.585647522s


[0;34m========== 1. Unit tests (Spec Hash, PCE, Pod Detail) ==========[0m

=== RUN   TestEqualTimePtr
--- PASS: TestEqualTimePtr (0.00s)
=== RUN   TestProcessSyncedPods_PodDetailFields
[AgentService] 2026/03/13 06:35:51.481713 📦 Processing 1 pods
[AgentService] 2026/03/13 06:35:51.481759 📦 Pod phase from agent: default/nginx phase=Running

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1073 [35;1mrecord not found
[0m[33m[0.443ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-1" AND uid = "pod-uid-123") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1168 [35;1mrecord not found
[0m[33m[0.507ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-1" AND uid = "pod-uid-123" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 06:35:51.483380 ✨ Created Pod default/nginx (SA: default)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.037ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-123" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
--- PASS: TestProcessSyncedPods_PodDetailFields (0.01s)
=== RUN   TestProcessSyncedPods_PodDetailFields_SnakeCaseKeys
[AgentService] 2026/03/13 06:35:51.495361 📦 Processing 1 pods
[AgentService] 2026/03/13 06:35:51.495411 📦 Pod phase from agent: default/p2 phase=Running

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1073 [35;1mrecord not found
[0m[33m[0.328ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-snake" AND uid = "pod-uid-snake") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1168 [35;1mrecord not found
[0m[33m[0.110ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-snake" AND uid = "pod-uid-snake" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 06:35:51.496253 ✨ Created Pod default/p2 (SA: default)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.122ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-snake" ORDER BY `pod_instances`.`pod_uid` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:553 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.030ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-1" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:579 [35;1mSQL logic error: no such table: cluster_role_bindings (1)
[0m[33m[0.023ms] [34;1m[rows:0][0m SELECT * FROM `cluster_role_bindings` WHERE (cluster_id = "test-cluster-1" AND deleted_at IS NULL) AND `cluster_role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:183 [35;1mSQL logic error: no such table: capability_metadata (1)
[0m[33m[0.312ms] [34;1m[rows:0][0m SELECT * FROM `capability_metadata` WHERE capability_id = "ID_TOKEN_POD" ORDER BY `capability_metadata`.`capability_id` LIMIT 1
--- PASS: TestProcessSyncedPods_PodDetailFields_SnakeCaseKeys (0.01s)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:215 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[1.140ms] [34;1m[rows:0][0m INSERT INTO `pod_capabilities` (`pod_uid`,`namespace`,`capability_id`,`capability_group`,`severity`,`state`,`confidence`,`first_seen_at`,`last_seen_at`,`evidence`,`mitre`,`created_at`,`updated_at`) VALUES ("pod-uid-123","default","ID_TOKEN_POD","ID","MEDIUM","detected",0.5,"2026-03-13 06:35:51.502","2026-03-13 06:35:51.502","{""automountServiceAccountToken"":true}","{}","2026-03-13 06:35:51.502","2026-03-13 06:35:51.502") ON CONFLICT (`pod_uid`,`capability_id`) DO UPDATE SET `capability_group`=`excluded`.`capability_group`,`severity`=`excluded`.`severity`,`evidence`=`excluded`.`evidence`,`last_seen_at`=`excluded`.`last_seen_at`,`updated_at`=`excluded`.`updated_at` RETURNING `id`
2026/03/13 06:35:51 [PCE] Failed to initialize capability ID_TOKEN_POD for pod default/nginx: SQL logic error: no such table: pod_capabilities (1)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:151 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.038ms] [34;1m[rows:0][0m DELETE FROM `pod_capabilities` WHERE pod_uid = "pod-uid-123" AND capability_id NOT IN ("ID_TOKEN_POD")

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:553 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.022ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-snake" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:579 [35;1mSQL logic error: no such table: cluster_role_bindings (1)
[0m[33m[0.029ms] [34;1m[rows:0][0m SELECT * FROM `cluster_role_bindings` WHERE (cluster_id = "test-cluster-snake" AND deleted_at IS NULL) AND `cluster_role_bindings`.`deleted_at` IS NULL
=== RUN   TestProcessSyncedPods_SpecHashStoredAndConditionalPCE

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:183 [35;1mSQL logic error: no such table: capability_metadata (1)
[0m[33m[0.826ms] [34;1m[rows:0][0m SELECT * FROM `capability_metadata` WHERE capability_id = "ID_TOKEN_POD" ORDER BY `capability_metadata`.`capability_id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:215 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.457ms] [34;1m[rows:0][0m INSERT INTO `pod_capabilities` (`pod_uid`,`namespace`,`capability_id`,`capability_group`,`severity`,`state`,`confidence`,`first_seen_at`,`last_seen_at`,`evidence`,`mitre`,`created_at`,`updated_at`) VALUES ("pod-uid-snake","default","ID_TOKEN_POD","ID","MEDIUM","detected",0.5,"2026-03-13 06:35:51.508","2026-03-13 06:35:51.508","{""automountServiceAccountToken"":true}","{}","2026-03-13 06:35:51.508","2026-03-13 06:35:51.508") ON CONFLICT (`pod_uid`,`capability_id`) DO UPDATE SET `capability_group`=`excluded`.`capability_group`,`severity`=`excluded`.`severity`,`evidence`=`excluded`.`evidence`,`last_seen_at`=`excluded`.`last_seen_at`,`updated_at`=`excluded`.`updated_at` RETURNING `id`
2026/03/13 06:35:51 [PCE] Failed to initialize capability ID_TOKEN_POD for pod default/p2: SQL logic error: no such table: pod_capabilities (1)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:151 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.031ms] [34;1m[rows:0][0m DELETE FROM `pod_capabilities` WHERE pod_uid = "pod-uid-snake" AND capability_id NOT IN ("ID_TOKEN_POD")
[AgentService] 2026/03/13 06:35:51.523900 📦 Processing 1 pods
[AgentService] 2026/03/13 06:35:51.524199 📦 Pod phase from agent: default/p1 phase=Running

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1073 [35;1mrecord not found
[0m[33m[0.167ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1168 [35;1mrecord not found
[0m[33m[0.190ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-spec" AND uid = "uid-spec-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 06:35:51.525464 ✨ Created Pod default/p1 (SA: default)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.054ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-spec-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/13 06:35:51.527253 📦 Processing 1 pods
[AgentService] 2026/03/13 06:35:51.527652 📦 Pod phase from agent: default/p1 phase=Pending
[AgentService] 2026/03/13 06:35:51.528534 🔄 Updated Pod default/p1 (SA: default)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:553 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.055ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-spec" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:579 [35;1mSQL logic error: no such table: cluster_role_bindings (1)
[0m[33m[0.046ms] [34;1m[rows:0][0m SELECT * FROM `cluster_role_bindings` WHERE (cluster_id = "test-cluster-spec" AND deleted_at IS NULL) AND `cluster_role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:183 [35;1mSQL logic error: no such table: capability_metadata (1)
[0m[33m[1.297ms] [34;1m[rows:0][0m SELECT * FROM `capability_metadata` WHERE capability_id = "ID_TOKEN_POD" ORDER BY `capability_metadata`.`capability_id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:215 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.515ms] [34;1m[rows:0][0m INSERT INTO `pod_capabilities` (`pod_uid`,`namespace`,`capability_id`,`capability_group`,`severity`,`state`,`confidence`,`first_seen_at`,`last_seen_at`,`evidence`,`mitre`,`created_at`,`updated_at`) VALUES ("uid-spec-1","default","ID_TOKEN_POD","ID","MEDIUM","detected",0.5,"2026-03-13 06:35:51.534","2026-03-13 06:35:51.534","{""automountServiceAccountToken"":true}","{}","2026-03-13 06:35:51.534","2026-03-13 06:35:51.534") ON CONFLICT (`pod_uid`,`capability_id`) DO UPDATE SET `capability_group`=`excluded`.`capability_group`,`severity`=`excluded`.`severity`,`evidence`=`excluded`.`evidence`,`last_seen_at`=`excluded`.`last_seen_at`,`updated_at`=`excluded`.`updated_at` RETURNING `id`
2026/03/13 06:35:51 [PCE] Failed to initialize capability ID_TOKEN_POD for pod default/p1: SQL logic error: no such table: pod_capabilities (1)
[AgentService] 2026/03/13 06:35:51.535091 📦 Processing 1 pods
[AgentService] 2026/03/13 06:35:51.535125 📦 Pod phase from agent: default/p1 phase=Running

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:151 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.297ms] [34;1m[rows:0][0m DELETE FROM `pod_capabilities` WHERE pod_uid = "uid-spec-1" AND capability_id NOT IN ("ID_TOKEN_POD")
[AgentService] 2026/03/13 06:35:51.535557 🔄 Updated Pod default/p1 (SA: default)
--- PASS: TestProcessSyncedPods_SpecHashStoredAndConditionalPCE (0.03s)
=== RUN   TestProcessSyncedPods_NullToNonNullSpecHash

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:553 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.846ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-spec" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:579 [35;1mSQL logic error: no such table: cluster_role_bindings (1)
[0m[33m[0.018ms] [34;1m[rows:0][0m SELECT * FROM `cluster_role_bindings` WHERE (cluster_id = "test-cluster-spec" AND deleted_at IS NULL) AND `cluster_role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:183 [35;1mSQL logic error: no such table: capability_metadata (1)
[0m[33m[0.112ms] [34;1m[rows:0][0m SELECT * FROM `capability_metadata` WHERE capability_id = "ID_TOKEN_POD" ORDER BY `capability_metadata`.`capability_id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:215 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.062ms] [34;1m[rows:0][0m INSERT INTO `pod_capabilities` (`pod_uid`,`namespace`,`capability_id`,`capability_group`,`severity`,`state`,`confidence`,`first_seen_at`,`last_seen_at`,`evidence`,`mitre`,`created_at`,`updated_at`) VALUES ("uid-spec-1","default","ID_TOKEN_POD","ID","MEDIUM","detected",0.5,"2026-03-13 06:35:51.54","2026-03-13 06:35:51.54","{""automountServiceAccountToken"":true}","{}","2026-03-13 06:35:51.54","2026-03-13 06:35:51.54") ON CONFLICT (`pod_uid`,`capability_id`) DO UPDATE SET `capability_group`=`excluded`.`capability_group`,`severity`=`excluded`.`severity`,`evidence`=`excluded`.`evidence`,`last_seen_at`=`excluded`.`last_seen_at`,`updated_at`=`excluded`.`updated_at` RETURNING `id`
2026/03/13 06:35:51 [PCE] Failed to initialize capability ID_TOKEN_POD for pod default/p1: SQL logic error: no such table: pod_capabilities (1)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:151 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.016ms] [34;1m[rows:0][0m DELETE FROM `pod_capabilities` WHERE pod_uid = "uid-spec-1" AND capability_id NOT IN ("ID_TOKEN_POD")
[AgentService] 2026/03/13 06:35:51.542285 📦 Processing 1 pods
[AgentService] 2026/03/13 06:35:51.542314 📦 Pod phase from agent: default/p2 phase=Running

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1073 [35;1mrecord not found
[0m[33m[0.538ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1168 [35;1mrecord not found
[0m[33m[0.090ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-null" AND uid = "uid-null-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 06:35:51.543810 ✨ Created Pod default/p2 (SA: default)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.446ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-null-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/13 06:35:51.545728 📦 Processing 1 pods
[AgentService] 2026/03/13 06:35:51.545757 📦 Pod phase from agent: default/p2 phase=Running
[AgentService] 2026/03/13 06:35:51.547698 🔄 Updated Pod default/p2 (SA: default)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:553 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.261ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-null" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:579 [35;1mSQL logic error: no such table: cluster_role_bindings (1)
[0m[33m[1.047ms] [34;1m[rows:0][0m SELECT * FROM `cluster_role_bindings` WHERE (cluster_id = "test-cluster-null" AND deleted_at IS NULL) AND `cluster_role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:183 [35;1mSQL logic error: no such table: capability_metadata (1)
[0m[33m[1.001ms] [34;1m[rows:0][0m SELECT * FROM `capability_metadata` WHERE capability_id = "ID_TOKEN_POD" ORDER BY `capability_metadata`.`capability_id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:215 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.874ms] [34;1m[rows:0][0m INSERT INTO `pod_capabilities` (`pod_uid`,`namespace`,`capability_id`,`capability_group`,`severity`,`state`,`confidence`,`first_seen_at`,`last_seen_at`,`evidence`,`mitre`,`created_at`,`updated_at`) VALUES ("uid-null-1","default","ID_TOKEN_POD","ID","MEDIUM","detected",0.5,"2026-03-13 06:35:51.55","2026-03-13 06:35:51.55","{""automountServiceAccountToken"":true}","{}","2026-03-13 06:35:51.55","2026-03-13 06:35:51.55") ON CONFLICT (`pod_uid`,`capability_id`) DO UPDATE SET `capability_group`=`excluded`.`capability_group`,`severity`=`excluded`.`severity`,`evidence`=`excluded`.`evidence`,`last_seen_at`=`excluded`.`last_seen_at`,`updated_at`=`excluded`.`updated_at` RETURNING `id`
2026/03/13 06:35:51 [PCE] Failed to initialize capability ID_TOKEN_POD for pod default/p2: SQL logic error: no such table: pod_capabilities (1)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:553 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.457ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-null" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:151 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.596ms] [34;1m[rows:0][0m DELETE FROM `pod_capabilities` WHERE pod_uid = "uid-null-1" AND capability_id NOT IN ("ID_TOKEN_POD")

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:579 [35;1mSQL logic error: no such table: cluster_role_bindings (1)
[0m[33m[0.792ms] [34;1m[rows:0][0m SELECT * FROM `cluster_role_bindings` WHERE (cluster_id = "test-cluster-null" AND deleted_at IS NULL) AND `cluster_role_bindings`.`deleted_at` IS NULL
--- PASS: TestProcessSyncedPods_NullToNonNullSpecHash (0.02s)
=== RUN   TestProcessSyncedPods_UpdatePodDetailFields

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:183 [35;1mSQL logic error: no such table: capability_metadata (1)
[0m[33m[0.325ms] [34;1m[rows:0][0m SELECT * FROM `capability_metadata` WHERE capability_id = "ID_TOKEN_POD" ORDER BY `capability_metadata`.`capability_id` LIMIT 1

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/state_controller.go:215 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.048ms] [34;1m[rows:0][0m INSERT INTO `pod_capabilities` (`pod_uid`,`namespace`,`capability_id`,`capability_group`,`severity`,`state`,`confidence`,`first_seen_at`,`last_seen_at`,`evidence`,`mitre`,`created_at`,`updated_at`) VALUES ("uid-null-1","default","ID_TOKEN_POD","ID","MEDIUM","detected",0.5,"2026-03-13 06:35:51.553","2026-03-13 06:35:51.553","{""automountServiceAccountToken"":true}","{}","2026-03-13 06:35:51.553","2026-03-13 06:35:51.553") ON CONFLICT (`pod_uid`,`capability_id`) DO UPDATE SET `capability_group`=`excluded`.`capability_group`,`severity`=`excluded`.`severity`,`evidence`=`excluded`.`evidence`,`last_seen_at`=`excluded`.`last_seen_at`,`updated_at`=`excluded`.`updated_at` RETURNING `id`
2026/03/13 06:35:51 [PCE] Failed to initialize capability ID_TOKEN_POD for pod default/p2: SQL logic error: no such table: pod_capabilities (1)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:151 [35;1mSQL logic error: no such table: pod_capabilities (1)
[0m[33m[0.022ms] [34;1m[rows:0][0m DELETE FROM `pod_capabilities` WHERE pod_uid = "uid-null-1" AND capability_id NOT IN ("ID_TOKEN_POD")
[AgentService] 2026/03/13 06:35:51.563970 📦 Processing 1 pods
[AgentService] 2026/03/13 06:35:51.564386 📦 Pod phase from agent: default/app phase=Running
[AgentService] 2026/03/13 06:35:51.565110 🔄 Updated Pod default/app (SA: default)

2026/03/13 06:35:51 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.045ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-456" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/13 06:35:51.565908 📡 Backfilled pod_ip=10.0.0.2 for default/app (uid=pod-uid-456)
--- PASS: TestProcessSyncedPods_UpdatePodDetailFields (0.01s)
PASS
ok  	github.com/fortuna/core/internal/service	0.104s
[0;32mAgent service tests: PASS[0m
=== RUN   TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch
2026/03/13 06:35:52 [PCE] Skip stale evaluation for pod default/p (spec_hash changed)
--- PASS: TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch (0.00s)
=== RUN   TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount
--- PASS: TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount (0.00s)
PASS
ok  	github.com/fortuna/core/pkg/capability	0.014s
[0;32mCapability evaluator tests: PASS[0m
=== RUN   TestGetPod_ReturnsPodDetailFields
--- PASS: TestGetPod_ReturnsPodDetailFields (0.01s)
=== RUN   TestGetPodByUID_ReturnsPodDetailFields
--- PASS: TestGetPodByUID_ReturnsPodDetailFields (0.01s)
PASS
ok  	github.com/fortuna/core/internal/api	0.141s
?   	github.com/fortuna/core/internal/api/policy	[no test files]
testing: warning: no tests to run
PASS
ok  	github.com/fortuna/core/internal/api/risk	0.018s [no tests to run]
[0;32mAPI pod handlers tests: PASS[0m

[0;34m========== 2. Database verification ==========[0m

[0;32mUsing Postgres pod: postgres-5484f7745d-998x9[0m
[0;32m  pods.spec_hash: present[0m
[0;32m  pods.last_evaluated_hash: present[0m

[0;34m========== 3. Process / environment info ==========[0m

[0;32mCore pod: fortuna-core-5577f44c8b-hk4fp[0m
[0;32mAgent pod: fortuna-agent-8cxst[0m

[0;34m========== 4. Test suite mapping (testSuite.md) ==========[0m


[0;32mReport written to: /home/k8s/KSAM/docs/test-results/pod-detail-test-suite-report-20260313-063547.md[0m


### API checks
- GET /inventory/pods: pod uid=b59c2f18-6128-4fd8-8a85-b2c5f7a52b23
podIP: None | startTime: None | riskCount: 5
- GET /runtime/pods/b59c2f18-6128-4fd8-8a85-b2c5f7a52b23/metrics: 200
- GET /runtime/pods/b59c2f18-6128-4fd8-8a85-b2c5f7a52b23/processes: 200
- GET /runtime/pods/b59c2f18-6128-4fd8-8a85-b2c5f7a52b23/network: 200
- GET /runtime/pods/b59c2f18-6128-4fd8-8a85-b2c5f7a52b23/events: 200

