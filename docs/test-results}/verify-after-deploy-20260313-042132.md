# Verify after clean/rebuild/redeploy
**Time:** 2026-03-13T04:21:32+00:00

NAME                               READY   STATUS    RESTARTS      AGE    IP             NODE           NOMINATED NODE   READINESS GATES
fortuna-agent-2995b                1/1     Running   0             105s   10.244.1.211   k8s-worker01   <none>           <none>
fortuna-agent-5tmsr                1/1     Running   0             102s   10.244.0.245   k8s-master     <none>           <none>
fortuna-core-57dcf965b5-mzkkp      1/1     Running   0             106s   10.244.0.244   k8s-master     <none>           <none>
fortuna-dashboard-c4d76b6f-cwq7r   1/1     Running   0             106s   10.244.0.243   k8s-master     <none>           <none>
nats-0                             1/1     Running   1 (25h ago)   33h    10.244.1.206   k8s-worker01   <none>           <none>
nats-1                             1/1     Running   1 (25h ago)   33h    10.244.0.234   k8s-master     <none>           <none>
nats-2                             1/1     Running   1 (25h ago)   33h    10.244.1.204   k8s-worker01   <none>           <none>
postgres-5484f7745d-998x9          1/1     Running   1 (25h ago)   33h    10.244.1.205   k8s-worker01   <none>           <none>

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

  pods: 35
  pod_runtime_metrics: 1126
  pod_processes: 1023
  pod_network_connections: 110624
  k8s_events: 511
  runtime_events: 0
  runtime_signals: 0
  pod_capabilities: 110

Sample pods (id, name, uid, spec_hash):
 17 | kube-proxy-nr8lb                        | 5bbc734d-f668-4d2a-a3b6-cb2ed2998244 | 65b8aa627790
 15 | kube-proxy-kt6sf                        | bd71cabf-2360-48af-8c60-164a7bd7728e | 441b2d6064aa
 19 | kube-scheduler-k8s-master               | c88e5f07-fd1d-4688-8d22-411e62228024 | 8d47f1c24969
 20 | local-path-provisioner-844bd8758f-4njgq | 749db13b-b936-427c-bcd1-73be5ff552be | dc1b4f41fccf
 14 | kube-controller-manager-k8s-master      | e86b05b2-a2fa-48cf-9a3e-4a2d190696f5 | 43847f98e1fd


### Core last 30 lines
2026/03/13 04:21:12.134376 [SBOM] Published SBOM_CREATED event for sbom_id=13 (pod_uid=c88e5f07-fd1d-4688-8d22-411e62228024, reused=true)
2026/03/13 04:21:12.134655 [SBOM] Successfully stored SBOM id=13 with 0 components
2026/03/13 04:21:14.412490 [SBOM] Received SBOM from agent=k8s-master-agent, pod=local-path-provisioner-844bd8758f-4njgq, image=sha256:b7dea5221f06f6feed7788db0ad6b024a433c8f55533bd6cc792dc2079ff9ad2

2026/03/13 04:21:14 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:84
[0m[33m[1.335ms] [34;1m[rows:1][0m SELECT * FROM "sboms" WHERE (pod_uid = '749db13b-b936-427c-bcd1-73be5ff552be' AND deleted_at IS NULL) AND "sboms"."deleted_at" IS NULL ORDER BY "sboms"."id" LIMIT 1

2026/03/13 04:21:14 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:91
[0m[33m[2.945ms] [34;1m[rows:1][0m UPDATE "sboms" SET "container_name"='local-path-provisioner',"generated_at"='2026-03-13 04:21:14.411',"image_digest"='sha256:b7dea5221f06f6feed7788db0ad6b024a433c8f55533bd6cc792dc2079ff9ad2',"image_name"='rancher/local-path-provisioner',"image_tag"='v0.0.24',"last_used_at"='2026-03-13 04:21:14.424',"namespace"='local-path-storage',"package_count"=15,"pod_name"='local-path-provisioner-844bd8758f-4njgq',"use_count"=3,"updated_at"='2026-03-13 04:21:14.424' WHERE "sboms"."deleted_at" IS NULL AND "id" = 12
2026/03/13 04:21:14.427689 [SBOM] Updated existing SBOM id=12 for pod_uid=749db13b-b936-427c-bcd1-73be5ff552be

2026/03/13 04:21:14 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:110
[0m[33m[1.804ms] [34;1m[rows:0][0m UPDATE "sbom_components" SET "deleted_at"='2026-03-13 04:21:14.427' WHERE sbom_id = 12 AND "sbom_components"."deleted_at" IS NULL

2026/03/13 04:21:14 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:157
[0m[33m[6.029ms] [34;1m[rows:0][0m INSERT INTO "sbom_components" ("sbom_id","component_type","component_name","component_version","purl","licenses","source","description","homepage","maintainer","created_at","deleted_at") VALUES (12,'os-package','alpine-baselayout-data','3.4.0-r0','pkg:PACKAGE_TYPE_APK/alpine-baselayout-data@3.4.0-r0','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','musl','1.2.3-r4','pkg:PACKAGE_TYPE_APK/musl@1.2.3-r4','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','busybox','1.35.0-r29','pkg:PACKAGE_TYPE_APK/busybox@1.35.0-r29','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','busybox-binsh','1.35.0-r29','pkg:PACKAGE_TYPE_APK/busybox-binsh@1.35.0-r29','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','alpine-baselayout','3.4.0-r0','pkg:PACKAGE_TYPE_APK/alpine-baselayout@3.4.0-r0','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','alpine-keys','2.4-r1','pkg:PACKAGE_TYPE_APK/alpine-keys@2.4-r1','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','ca-certificates-bundle','20220614-r4','pkg:PACKAGE_TYPE_APK/ca-certificates-bundle@20220614-r4','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','libcrypto3','3.0.8-r0','pkg:PACKAGE_TYPE_APK/libcrypto3@3.0.8-r0','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','libssl3','3.0.8-r0','pkg:PACKAGE_TYPE_APK/libssl3@3.0.8-r0','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','ssl_client','1.35.0-r29','pkg:PACKAGE_TYPE_APK/ssl_client@1.35.0-r29','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','zlib','1.2.13-r0','pkg:PACKAGE_TYPE_APK/zlib@1.2.13-r0','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','apk-tools','2.12.10-r1','pkg:PACKAGE_TYPE_APK/apk-tools@2.12.10-r1','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','scanelf','1.3.5-r1','pkg:PACKAGE_TYPE_APK/scanelf@1.3.5-r1','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','musl-utils','1.2.3-r4','pkg:PACKAGE_TYPE_APK/musl-utils@1.2.3-r4','null','','','','','2026-03-13 04:21:14.429',NULL),(12,'os-package','libc-utils','0.7.2-r3','pkg:PACKAGE_TYPE_APK/libc-utils@0.7.2-r3','null','','','','','2026-03-13 04:21:14.429',NULL) ON CONFLICT ("sbom_id","purl") DO NOTHING RETURNING "id"

2026/03/13 04:21:14 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:186
[0m[33m[2.444ms] [34;1m[rows:1][0m SELECT * FROM "pods" WHERE (uid = '749db13b-b936-427c-bcd1-73be5ff552be' AND deleted_at IS NULL) AND "pods"."deleted_at" IS NULL ORDER BY "pods"."id" LIMIT 1
2026/03/13 04:21:14.445953 [SBOM] Published SBOM_CREATED event for sbom_id=12 (pod_uid=749db13b-b936-427c-bcd1-73be5ff552be, reused=true)
2026/03/13 04:21:14.446042 [SBOM] Successfully stored SBOM id=12 with 15 components
[GIN] 2026/03/13 - 04:21:17 | 200 |     3.64569ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/13 - 04:21:17 | 200 |    2.030858ms |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/13 - 04:21:22 | 200 |    3.279709ms |      10.244.0.1 | GET      "/ready"

2026/03/13 04:21:22 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:289
[0m[33m[2.208ms] [34;1m[rows:1][0m UPDATE agents SET last_seen_at = '2026-03-13 04:21:22.425', node_name = 'k8s-master', status = 'ready', updated_at = '2026-03-13 04:21:22.425', deleted_at = NULL WHERE agent_id = 'k8s-master-agent'
[GIN] 2026/03/13 - 04:21:27 | 200 |      53.841µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/03/13 - 04:21:27 | 200 |    7.206954ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/03/13 - 04:21:32 | 200 |    2.702201ms |      10.244.0.1 | GET      "/ready"

### Agent last 25 lines
[SBOMExtractor] 2026/03/13 04:20:58 Detected OS: debian 12.9
[SBOMExtractor] 2026/03/13 04:20:58    Selected parsers for OS 'debian': [dpkg npm pip gomod]
[SBOMExtractor] 2026/03/13 04:20:58    Parser dpkg: not applicable (OS: debian)
2026/03/13 04:20:58.765553 [SBOMExtractor] npm fallback: found 0 package-lock.json, 0 node_modules/.../package.json
[SBOMExtractor] 2026/03/13 04:20:58    Parser npm: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 04:20:58    Parser pip: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 04:20:58    Parser gomod: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 04:20:58 ✅ Extracted 0 unique packages in 6.45370282s
[SBOMProcessor] 2026/03/13 04:20:58 ✅ Extracted 0 packages from registry.k8s.io/kube-proxy:v1.29.15
[gRPCClient] 2026/03/13 04:20:58 Sending SBOM: pod=kube-system/kube-proxy-kt6sf image=sha256:f6074f465fb3700456dccc5915340df4b59a7960c591693182fbd297cbe72b53
[SBOMProcessor] 2026/03/13 04:20:58 ✅ SBOM sent to Core: sbom_id=17 message=SBOM received and stored
[SBOMQueue] 2026/03/13 04:20:58 [Worker 1] ✅ Completed pod kube-system/kube-proxy-kt6sf in 6.631826201s
[SBOMExtractor] 2026/03/13 04:20:59    Virtual FS: 720 files total, 0 paths containing package.json
[SBOMExtractor] 2026/03/13 04:20:59 Detected OS: alpine 3.22.3
[SBOMExtractor] 2026/03/13 04:20:59    Selected parsers for OS 'alpine': [apk npm pip gomod]
[SBOMExtractor] 2026/03/13 04:20:59 ✅ Parser apk found 51 packages
2026/03/13 04:20:59.047805 [SBOMExtractor] npm fallback: found 0 package-lock.json, 0 node_modules/.../package.json
[SBOMExtractor] 2026/03/13 04:20:59    Parser npm: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 04:20:59    Parser pip: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 04:20:59    Parser gomod: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 04:20:59 ✅ Extracted 51 unique packages in 8.180066711s
[SBOMProcessor] 2026/03/13 04:20:59 ✅ Extracted 51 packages from ghcr.io/flannel-io/flannel:v0.28.1
[gRPCClient] 2026/03/13 04:20:59 Sending SBOM: pod=kube-flannel/kube-flannel-ds-sk5wz image=sha256:e105bc4081a2eb86102db73192df83065e08bf042fcaaab4e7dd25d1facc8196
[SBOMProcessor] 2026/03/13 04:20:59 ✅ SBOM sent to Core: sbom_id=34 message=SBOM received and stored
[SBOMQueue] 2026/03/13 04:20:59 [Worker 0] ✅ Completed pod kube-flannel/kube-flannel-ds-sk5wz in 8.207226423s


[0;34m========== 1. Unit tests (Spec Hash, PCE, Pod Detail) ==========[0m

=== RUN   TestEqualTimePtr
--- PASS: TestEqualTimePtr (0.00s)
=== RUN   TestProcessSyncedPods_PodDetailFields
[AgentService] 2026/03/13 04:21:40.275984 📦 Processing 1 pods
[AgentService] 2026/03/13 04:21:40.276051 📦 Pod phase from agent: default/nginx phase=Running

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1062 [35;1mrecord not found
[0m[33m[0.124ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-1" AND uid = "pod-uid-123") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1157 [35;1mrecord not found
[0m[33m[0.233ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-1" AND uid = "pod-uid-123" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.277123 ✨ Created Pod default/nginx (SA: default)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.142ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-123" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
--- PASS: TestProcessSyncedPods_PodDetailFields (0.01s)
=== RUN   TestProcessSyncedPods_PodDetailFields_SnakeCaseKeys

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[3.233ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-1" AND uid = "pod-uid-123") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.282033 ⚠️  PCE skipped: pod not found (cluster=test-cluster-1 uid=pod-uid-123): SQL logic error: no such table: pods (1)
[AgentService] 2026/03/13 04:21:40.301995 📦 Processing 1 pods
[AgentService] 2026/03/13 04:21:40.302036 📦 Pod phase from agent: default/p2 phase=Running

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1062 [35;1mrecord not found
[0m[33m[2.956ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-snake" AND uid = "pod-uid-snake") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1157 [35;1mrecord not found
[0m[33m[0.121ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-snake" AND uid = "pod-uid-snake" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.306042 ✨ Created Pod default/p2 (SA: default)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.061ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-snake" ORDER BY `pod_instances`.`pod_uid` LIMIT 1

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.266ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-snake" AND uid = "pod-uid-snake") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.311282 ⚠️  PCE skipped: pod not found (cluster=test-cluster-snake uid=pod-uid-snake): SQL logic error: no such table: pods (1)
--- PASS: TestProcessSyncedPods_PodDetailFields_SnakeCaseKeys (0.04s)
=== RUN   TestProcessSyncedPods_SpecHashStoredAndConditionalPCE
[AgentService] 2026/03/13 04:21:40.326848 📦 Processing 1 pods
[AgentService] 2026/03/13 04:21:40.326877 📦 Pod phase from agent: default/p1 phase=Running

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1062 [35;1mrecord not found
[0m[33m[0.113ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1157 [35;1mrecord not found
[0m[33m[0.622ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-spec" AND uid = "uid-spec-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.329187 ✨ Created Pod default/p1 (SA: default)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.120ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-spec-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/13 04:21:40.330239 📦 Processing 1 pods
[AgentService] 2026/03/13 04:21:40.330263 📦 Pod phase from agent: default/p1 phase=Pending

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.789ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.330991 ⚠️  PCE skipped: pod not found (cluster=test-cluster-spec uid=uid-spec-1): SQL logic error: no such table: pods (1)
[AgentService] 2026/03/13 04:21:40.331646 🔄 Updated Pod default/p1 (SA: default)
[AgentService] 2026/03/13 04:21:40.334658 📦 Processing 1 pods
[AgentService] 2026/03/13 04:21:40.334683 📦 Pod phase from agent: default/p1 phase=Running
[AgentService] 2026/03/13 04:21:40.335074 🔄 Updated Pod default/p1 (SA: default)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.041ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.335777 ⚠️  PCE skipped: pod not found (cluster=test-cluster-spec uid=uid-spec-1): SQL logic error: no such table: pods (1)
--- PASS: TestProcessSyncedPods_SpecHashStoredAndConditionalPCE (0.02s)
=== RUN   TestProcessSyncedPods_NullToNonNullSpecHash
[AgentService] 2026/03/13 04:21:40.347157 📦 Processing 1 pods
[AgentService] 2026/03/13 04:21:40.347313 📦 Pod phase from agent: default/p2 phase=Running

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1062 [35;1mrecord not found
[0m[33m[0.304ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1157 [35;1mrecord not found
[0m[33m[0.232ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-null" AND uid = "uid-null-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.349535 ✨ Created Pod default/p2 (SA: default)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.897ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-null-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/13 04:21:40.353140 📦 Processing 1 pods
[AgentService] 2026/03/13 04:21:40.353224 📦 Pod phase from agent: default/p2 phase=Running
[AgentService] 2026/03/13 04:21:40.353871 🔄 Updated Pod default/p2 (SA: default)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[1.584ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.354351 ⚠️  PCE skipped: pod not found (cluster=test-cluster-null uid=uid-null-1): SQL logic error: no such table: pods (1)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.634ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.354870 ⚠️  PCE skipped: pod not found (cluster=test-cluster-null uid=uid-null-1): SQL logic error: no such table: pods (1)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1240 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.029ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY created_at DESC

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1256 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.018ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1" AND deleted_at IS NULL) AND `pods`.`deleted_at` IS NULL ORDER BY created_at DESC

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service_pod_detail_test.go:279 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.088ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL AND `pods`.`id` = 1 ORDER BY `pods`.`id` LIMIT 1
    agent_service_pod_detail_test.go:280: find pod again: SQL logic error: no such table: pods (1)
--- FAIL: TestProcessSyncedPods_NullToNonNullSpecHash (0.02s)
=== RUN   TestProcessSyncedPods_UpdatePodDetailFields
[AgentService] 2026/03/13 04:21:40.358802 📦 Processing 1 pods
[AgentService] 2026/03/13 04:21:40.358832 📦 Pod phase from agent: default/app phase=Running
[AgentService] 2026/03/13 04:21:40.361212 🔄 Updated Pod default/app (SA: default)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.050ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-456" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/13 04:21:40.362049 📡 Backfilled pod_ip=10.0.0.2 for default/app (uid=pod-uid-456)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.453ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-2" AND uid = "pod-uid-456") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 04:21:40.362548 ⚠️  PCE skipped: pod not found (cluster=test-cluster-2 uid=pod-uid-456): SQL logic error: no such table: pods (1)

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1226 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.017ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-2" AND uid NOT IN ("pod-uid-456")) AND `pods`.`deleted_at` IS NULL

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1240 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.021ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-2" AND uid = "pod-uid-456") AND `pods`.`deleted_at` IS NULL ORDER BY created_at DESC

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1256 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.130ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-2" AND uid = "pod-uid-456" AND deleted_at IS NULL) AND `pods`.`deleted_at` IS NULL ORDER BY created_at DESC

2026/03/13 04:21:40 [31;1m/home/k8s/KSAM/core/internal/service/agent_service_pod_detail_test.go:341 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.020ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-2" AND uid = "pod-uid-456") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
    agent_service_pod_detail_test.go:342: find pod: SQL logic error: no such table: pods (1)
--- FAIL: TestProcessSyncedPods_UpdatePodDetailFields (0.01s)
FAIL
FAIL	github.com/fortuna/core/internal/service	0.125s
FAIL
[0;31mAgent service tests: FAIL[0m
=== RUN   TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch
2026/03/13 04:21:41 [PCE] Skip stale evaluation for pod default/p (spec_hash changed)
--- PASS: TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch (0.00s)
=== RUN   TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount
--- PASS: TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount (0.01s)
PASS
ok  	github.com/fortuna/core/pkg/capability	0.020s
[0;32mCapability evaluator tests: PASS[0m
=== RUN   TestGetPod_ReturnsPodDetailFields
--- PASS: TestGetPod_ReturnsPodDetailFields (0.01s)
=== RUN   TestGetPodByUID_ReturnsPodDetailFields
--- PASS: TestGetPodByUID_ReturnsPodDetailFields (0.00s)
PASS
ok  	github.com/fortuna/core/internal/api	0.115s
?   	github.com/fortuna/core/internal/api/policy	[no test files]
testing: warning: no tests to run
PASS
ok  	github.com/fortuna/core/internal/api/risk	0.013s [no tests to run]
[0;32mAPI pod handlers tests: PASS[0m

[0;34m========== 2. Database verification ==========[0m

[0;32mUsing Postgres pod: postgres-5484f7745d-998x9[0m
[0;32m  pods.spec_hash: present[0m
[0;32m  pods.last_evaluated_hash: present[0m
[0;32m  EXPLAIN uses index (or seq scan on small table).[0m

[0;34m========== 3. Process / environment info ==========[0m

[0;32mCore pod: fortuna-core-57dcf965b5-mzkkp[0m
[0;32mAgent pod: fortuna-agent-2995b[0m

[0;34m========== 4. Test suite mapping (testSuite.md) ==========[0m


[0;32mReport written to: /home/k8s/KSAM/docs/test-results/pod-detail-test-suite-report-20260313-042135.md[0m


### API checks
- GET /inventory/pods: pod uid=c7caa8ab-58ed-41f5-8882-b59246af1b52
podIP: None | startTime: 2026-03-13T04:19:50Z | riskCount: 5
- GET /runtime/pods/c7caa8ab-58ed-41f5-8882-b59246af1b52/metrics: 200
- GET /runtime/pods/c7caa8ab-58ed-41f5-8882-b59246af1b52/processes: 200
- GET /runtime/pods/c7caa8ab-58ed-41f5-8882-b59246af1b52/network: 200
- GET /runtime/pods/c7caa8ab-58ed-41f5-8882-b59246af1b52/events: 200

