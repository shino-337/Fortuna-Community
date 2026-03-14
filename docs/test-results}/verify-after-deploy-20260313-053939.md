# Verify after clean/rebuild/redeploy
**Time:** 2026-03-13T05:39:39+00:00

NAME                               READY   STATUS    RESTARTS      AGE   IP             NODE           NOMINATED NODE   READINESS GATES
fortuna-agent-2q9z5                1/1     Running   0             30s   10.244.1.216   k8s-worker01   <none>           <none>
fortuna-agent-nwj77                1/1     Running   0             28s   10.244.0.250   k8s-master     <none>           <none>
fortuna-core-774dc47859-5z5kr      1/1     Running   0             64s   10.244.0.248   k8s-master     <none>           <none>
fortuna-dashboard-8667f9b5-2wm9t   1/1     Running   0             48s   10.244.0.249   k8s-master     <none>           <none>
nats-0                             1/1     Running   1 (26h ago)   35h   10.244.1.206   k8s-worker01   <none>           <none>
nats-1                             1/1     Running   1 (26h ago)   35h   10.244.0.234   k8s-master     <none>           <none>
nats-2                             1/1     Running   1 (26h ago)   35h   10.244.1.204   k8s-worker01   <none>           <none>
postgres-5484f7745d-998x9          1/1     Running   1 (26h ago)   35h   10.244.1.205   k8s-worker01   <none>           <none>

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

  pods: 44
  pod_runtime_metrics: 1549
  pod_processes: 1448
  pod_network_connections: 166627
  k8s_events: 608
  runtime_events: 0
  runtime_signals: 0
  pod_capabilities: 139

Sample pods (id, name, uid, spec_hash):
 17 | kube-proxy-nr8lb                        | 5bbc734d-f668-4d2a-a3b6-cb2ed2998244 | 65b8aa627790
 19 | kube-scheduler-k8s-master               | c88e5f07-fd1d-4688-8d22-411e62228024 | 8d47f1c24969
 15 | kube-proxy-kt6sf                        | bd71cabf-2360-48af-8c60-164a7bd7728e | 441b2d6064aa
 20 | local-path-provisioner-844bd8758f-4njgq | 749db13b-b936-427c-bcd1-73be5ff552be | dc1b4f41fccf
 14 | kube-controller-manager-k8s-master      | e86b05b2-a2fa-48cf-9a3e-4a2d190696f5 | 43847f98e1fd


### Core last 30 lines
[0m[33m[11.822ms] [34;1m[rows:1][0m INSERT INTO "sboms" ("image_name","image_tag","image_digest","pod_uid","pod_name","namespace","container_name","os_name","os_version","os_architecture","package_count","sbom_format","sbom_content","generated_at","agent_id","node_id","labels","annotations","last_used_at","use_count","created_at","updated_at","deleted_at") VALUES ('fortuna-agent','latest','sha256:9bde18116fc739ba081d4bdb867d3781f656319034a2acc1df52cc5bfe388ba8','543432aa-8481-4516-bfdd-cc35dd61fb94','fortuna-agent-nwj77','fortuna','agent','','','',106,'fortuna-agent','{}','2026-03-13 05:39:36.897','k8s-master-agent','k8s-master','{}','{}','2026-03-13 05:39:36.899',1,'2026-03-13 05:39:36.905','2026-03-13 05:39:36.905',NULL) RETURNING "id"
2026/03/13 05:39:36.917186 [SBOM] Created new SBOM id=40 for pod_uid=543432aa-8481-4516-bfdd-cc35dd61fb94

2026/03/13 05:39:36 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:157
[0m[33m[10.362ms] [34;1m[rows:106][0m INSERT INTO "sbom_components" ("sbom_id","component_type","component_name","component_version","purl","licenses","source","description","homepage","maintainer","created_at","deleted_at") VALUES (40,'os-package','adduser','3.134','pkg:PACKAGE_TYPE_DEB/adduser@3.134','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','apt','2.6.1','pkg:PACKAGE_TYPE_DEB/apt@2.6.1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','base-files','12.4+deb12u13','pkg:PACKAGE_TYPE_DEB/base-files@12.4+deb12u13','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','base-passwd','3.6.1','pkg:PACKAGE_TYPE_DEB/base-passwd@3.6.1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','bash','5.2.15-2+b10','pkg:PACKAGE_TYPE_DEB/bash@5.2.15-2+b10','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','bsdutils','1:2.38.1-5+deb12u3','pkg:PACKAGE_TYPE_DEB/bsdutils@1:2.38.1-5+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','ca-certificates','20230311+deb12u1','pkg:PACKAGE_TYPE_DEB/ca-certificates@20230311+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','containerd','1.6.20~ds1-1+deb12u2','pkg:PACKAGE_TYPE_DEB/containerd@1.6.20~ds1-1+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','coreutils','9.1-1','pkg:PACKAGE_TYPE_DEB/coreutils@9.1-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','dash','0.5.12-2','pkg:PACKAGE_TYPE_DEB/dash@0.5.12-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','debconf','1.5.82','pkg:PACKAGE_TYPE_DEB/debconf@1.5.82','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','debian-archive-keyring','2023.3+deb12u2','pkg:PACKAGE_TYPE_DEB/debian-archive-keyring@2023.3+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','debianutils','5.7-0.5~deb12u1','pkg:PACKAGE_TYPE_DEB/debianutils@5.7-0.5~deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','diffutils','1:3.8-4','pkg:PACKAGE_TYPE_DEB/diffutils@1:3.8-4','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','dmsetup','2:1.02.185-2','pkg:PACKAGE_TYPE_DEB/dmsetup@2:1.02.185-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','docker.io','20.10.24+dfsg1-1+deb12u1+b3','pkg:PACKAGE_TYPE_DEB/docker.io@20.10.24+dfsg1-1+deb12u1+b3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','dpkg','1.21.22','pkg:PACKAGE_TYPE_DEB/dpkg@1.21.22','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','e2fsprogs','1.47.0-2+b2','pkg:PACKAGE_TYPE_DEB/e2fsprogs@1.47.0-2+b2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','findutils','4.9.0-4','pkg:PACKAGE_TYPE_DEB/findutils@4.9.0-4','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','gcc-12-base','12.2.0-14+deb12u1','pkg:PACKAGE_TYPE_DEB/gcc-12-base@12.2.0-14+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','gpgv','2.2.40-1.1+deb12u2','pkg:PACKAGE_TYPE_DEB/gpgv@2.2.40-1.1+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','grep','3.8-5','pkg:PACKAGE_TYPE_DEB/grep@3.8-5','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','gzip','1.12-1','pkg:PACKAGE_TYPE_DEB/gzip@1.12-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','hostname','3.23+nmu1','pkg:PACKAGE_TYPE_DEB/hostname@3.23+nmu1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','init-system-helpers','1.65.2+deb12u1','pkg:PACKAGE_TYPE_DEB/init-system-helpers@1.65.2+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','iptables','1.8.9-2','pkg:PACKAGE_TYPE_DEB/iptables@1.8.9-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libacl1','2.3.1-3','pkg:PACKAGE_TYPE_DEB/libacl1@2.3.1-3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libapt-pkg6.0','2.6.1','pkg:PACKAGE_TYPE_DEB/libapt-pkg6.0@2.6.1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libattr1','1:2.5.1-4','pkg:PACKAGE_TYPE_DEB/libattr1@1:2.5.1-4','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libaudit-common','1:3.0.9-1','pkg:PACKAGE_TYPE_DEB/libaudit-common@1:3.0.9-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libaudit1','1:3.0.9-1','pkg:PACKAGE_TYPE_DEB/libaudit1@1:3.0.9-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libblkid1','2.38.1-5+deb12u3','pkg:PACKAGE_TYPE_DEB/libblkid1@2.38.1-5+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libbz2-1.0','1.0.8-5+b1','pkg:PACKAGE_TYPE_DEB/libbz2-1.0@1.0.8-5+b1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libc-bin','2.36-9+deb12u13','pkg:PACKAGE_TYPE_DEB/libc-bin@2.36-9+deb12u13','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libc6','2.36-9+deb12u13','pkg:PACKAGE_TYPE_DEB/libc6@2.36-9+deb12u13','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libcap-ng0','0.8.3-1+b3','pkg:PACKAGE_TYPE_DEB/libcap-ng0@0.8.3-1+b3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libcap2','1:2.66-4+deb12u2+b2','pkg:PACKAGE_TYPE_DEB/libcap2@1:2.66-4+deb12u2+b2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libcom-err2','1.47.0-2+b2','pkg:PACKAGE_TYPE_DEB/libcom-err2@1.47.0-2+b2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libcrypt1','1:4.4.33-2','pkg:PACKAGE_TYPE_DEB/libcrypt1@1:4.4.33-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libdb5.3','5.3.28+dfsg2-1','pkg:PACKAGE_TYPE_DEB/libdb5.3@5.3.28+dfsg2-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libdebconfclient0','0.270','pkg:PACKAGE_TYPE_DEB/libdebconfclient0@0.270','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libdevmapper1.02.1','2:1.02.185-2','pkg:PACKAGE_TYPE_DEB/libdevmapper1.02.1@2:1.02.185-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libext2fs2','1.47.0-2+b2','pkg:PACKAGE_TYPE_DEB/libext2fs2@1.47.0-2+b2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libffi8','3.4.4-1','pkg:PACKAGE_TYPE_DEB/libffi8@3.4.4-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libgcc-s1','12.2.0-14+deb12u1','pkg:PACKAGE_TYPE_DEB/libgcc-s1@12.2.0-14+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libgcrypt20','1.10.1-3','pkg:PACKAGE_TYPE_DEB/libgcrypt20@1.10.1-3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libgmp10','2:6.2.1+dfsg1-1.1','pkg:PACKAGE_TYPE_DEB/libgmp10@2:6.2.1+dfsg1-1.1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libgnutls30','3.7.9-2+deb12u6','pkg:PACKAGE_TYPE_DEB/libgnutls30@3.7.9-2+deb12u6','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libgpg-error0','1.46-1','pkg:PACKAGE_TYPE_DEB/libgpg-error0@1.46-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libhogweed6','3.8.1-2','pkg:PACKAGE_TYPE_DEB/libhogweed6@3.8.1-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libidn2-0','2.3.3-1+b1','pkg:PACKAGE_TYPE_DEB/libidn2-0@2.3.3-1+b1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libip4tc2','1.8.9-2','pkg:PACKAGE_TYPE_DEB/libip4tc2@1.8.9-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libip6tc2','1.8.9-2','pkg:PACKAGE_TYPE_DEB/libip6tc2@1.8.9-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','liblz4-1','1.9.4-1','pkg:PACKAGE_TYPE_DEB/liblz4-1@1.9.4-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','liblzma5','5.4.1-1','pkg:PACKAGE_TYPE_DEB/liblzma5@5.4.1-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libmd0','1.0.4-2','pkg:PACKAGE_TYPE_DEB/libmd0@1.0.4-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libmnl0','1.0.4-3','pkg:PACKAGE_TYPE_DEB/libmnl0@1.0.4-3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libmount1','2.38.1-5+deb12u3','pkg:PACKAGE_TYPE_DEB/libmount1@2.38.1-5+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libnetfilter-conntrack3','1.0.9-3','pkg:PACKAGE_TYPE_DEB/libnetfilter-conntrack3@1.0.9-3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libnettle8','3.8.1-2','pkg:PACKAGE_TYPE_DEB/libnettle8@3.8.1-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libnfnetlink0','1.0.2-2','pkg:PACKAGE_TYPE_DEB/libnfnetlink0@1.0.2-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libnftnl11','1.2.4-2','pkg:PACKAGE_TYPE_DEB/libnftnl11@1.2.4-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libp11-kit0','0.24.1-2','pkg:PACKAGE_TYPE_DEB/libp11-kit0@0.24.1-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libpam-modules','1.5.2-6+deb12u2','pkg:PACKAGE_TYPE_DEB/libpam-modules@1.5.2-6+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libpam-modules-bin','1.5.2-6+deb12u2','pkg:PACKAGE_TYPE_DEB/libpam-modules-bin@1.5.2-6+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libpam-runtime','1.5.2-6+deb12u2','pkg:PACKAGE_TYPE_DEB/libpam-runtime@1.5.2-6+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libpam0g','1.5.2-6+deb12u2','pkg:PACKAGE_TYPE_DEB/libpam0g@1.5.2-6+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libpcre2-8-0','10.42-1','pkg:PACKAGE_TYPE_DEB/libpcre2-8-0@10.42-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libseccomp2','2.5.4-1+deb12u1','pkg:PACKAGE_TYPE_DEB/libseccomp2@2.5.4-1+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libselinux1','3.4-1+b6','pkg:PACKAGE_TYPE_DEB/libselinux1@3.4-1+b6','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libsemanage-common','3.4-1','pkg:PACKAGE_TYPE_DEB/libsemanage-common@3.4-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libsemanage2','3.4-1+b5','pkg:PACKAGE_TYPE_DEB/libsemanage2@3.4-1+b5','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libsepol2','3.4-2.1','pkg:PACKAGE_TYPE_DEB/libsepol2@3.4-2.1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libsmartcols1','2.38.1-5+deb12u3','pkg:PACKAGE_TYPE_DEB/libsmartcols1@2.38.1-5+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libss2','1.47.0-2+b2','pkg:PACKAGE_TYPE_DEB/libss2@1.47.0-2+b2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libssl3','3.0.18-1~deb12u2','pkg:PACKAGE_TYPE_DEB/libssl3@3.0.18-1~deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libstdc++6','12.2.0-14+deb12u1','pkg:PACKAGE_TYPE_DEB/libstdc++6@12.2.0-14+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libsystemd0','252.39-1~deb12u1','pkg:PACKAGE_TYPE_DEB/libsystemd0@252.39-1~deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libtasn1-6','4.19.0-2+deb12u1','pkg:PACKAGE_TYPE_DEB/libtasn1-6@4.19.0-2+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libtinfo6','6.4-4','pkg:PACKAGE_TYPE_DEB/libtinfo6@6.4-4','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libudev1','252.39-1~deb12u1','pkg:PACKAGE_TYPE_DEB/libudev1@252.39-1~deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libunistring2','1.0-2','pkg:PACKAGE_TYPE_DEB/libunistring2@1.0-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libuuid1','2.38.1-5+deb12u3','pkg:PACKAGE_TYPE_DEB/libuuid1@2.38.1-5+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libxtables12','1.8.9-2','pkg:PACKAGE_TYPE_DEB/libxtables12@1.8.9-2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libxxhash0','0.8.1-1','pkg:PACKAGE_TYPE_DEB/libxxhash0@0.8.1-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','libzstd1','1.5.4+dfsg2-5','pkg:PACKAGE_TYPE_DEB/libzstd1@1.5.4+dfsg2-5','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','login','1:4.13+dfsg1-1+deb12u2','pkg:PACKAGE_TYPE_DEB/login@1:4.13+dfsg1-1+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','logsave','1.47.0-2+b2','pkg:PACKAGE_TYPE_DEB/logsave@1.47.0-2+b2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','mawk','1.3.4.20200120-3.1','pkg:PACKAGE_TYPE_DEB/mawk@1.3.4.20200120-3.1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','mount','2.38.1-5+deb12u3','pkg:PACKAGE_TYPE_DEB/mount@2.38.1-5+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','ncurses-base','6.4-4','pkg:PACKAGE_TYPE_DEB/ncurses-base@6.4-4','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','ncurses-bin','6.4-4','pkg:PACKAGE_TYPE_DEB/ncurses-bin@6.4-4','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','netbase','6.4','pkg:PACKAGE_TYPE_DEB/netbase@6.4','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','openssl','3.0.18-1~deb12u2','pkg:PACKAGE_TYPE_DEB/openssl@3.0.18-1~deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','passwd','1:4.13+dfsg1-1+deb12u2','pkg:PACKAGE_TYPE_DEB/passwd@1:4.13+dfsg1-1+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','perl-base','5.36.0-7+deb12u3','pkg:PACKAGE_TYPE_DEB/perl-base@5.36.0-7+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','runc','1.1.5+ds1-1+deb12u1','pkg:PACKAGE_TYPE_DEB/runc@1.1.5+ds1-1+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','sed','4.9-1','pkg:PACKAGE_TYPE_DEB/sed@4.9-1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','sysvinit-utils','3.06-4','pkg:PACKAGE_TYPE_DEB/sysvinit-utils@3.06-4','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','tar','1.34+dfsg-1.2+deb12u1','pkg:PACKAGE_TYPE_DEB/tar@1.34+dfsg-1.2+deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','tini','0.19.0-1+b3','pkg:PACKAGE_TYPE_DEB/tini@0.19.0-1+b3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','tzdata','2025b-0+deb12u2','pkg:PACKAGE_TYPE_DEB/tzdata@2025b-0+deb12u2','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','usr-is-merged','37~deb12u1','pkg:PACKAGE_TYPE_DEB/usr-is-merged@37~deb12u1','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','util-linux','2.38.1-5+deb12u3','pkg:PACKAGE_TYPE_DEB/util-linux@2.38.1-5+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','util-linux-extra','2.38.1-5+deb12u3','pkg:PACKAGE_TYPE_DEB/util-linux-extra@2.38.1-5+deb12u3','null','','','','','2026-03-13 05:39:36.917',NULL),(40,'os-package','zlib1g','1:1.2.13.dfsg-1','pkg:PACKAGE_TYPE_DEB/zlib1g@1:1.2.13.dfsg-1','null','','','','','2026-03-13 05:39:36.917',NULL) ON CONFLICT ("sbom_id","purl") DO NOTHING RETURNING "id"

2026/03/13 05:39:36 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:186
[0m[33m[1.904ms] [34;1m[rows:1][0m SELECT * FROM "pods" WHERE (uid = '543432aa-8481-4516-bfdd-cc35dd61fb94' AND deleted_at IS NULL) AND "pods"."deleted_at" IS NULL ORDER BY "pods"."id" LIMIT 1
2026/03/13 05:39:36.938722 [SBOM] Published SBOM_CREATED event for sbom_id=40 (pod_uid=543432aa-8481-4516-bfdd-cc35dd61fb94, reused=false)
2026/03/13 05:39:36.938819 [SBOM] Successfully stored SBOM id=40 with 106 components
2026/03/13 05:39:38.881077 [SBOM] Received SBOM from agent=k8s-master-agent, pod=nats-1, image=sha256:7e2f895a12d5bc4586191452f5dd20968e8c800d53622073a2730c38d6b0eccb

2026/03/13 05:39:38 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:84
[0m[33m[1.251ms] [34;1m[rows:1][0m SELECT * FROM "sboms" WHERE (pod_uid = 'eb24d3ce-b5cc-43cb-b766-7ad7ec4a5af4' AND deleted_at IS NULL) AND "sboms"."deleted_at" IS NULL ORDER BY "sboms"."id" LIMIT 1

2026/03/13 05:39:38 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:91
[0m[33m[1.014ms] [34;1m[rows:1][0m UPDATE "sboms" SET "container_name"='nats',"generated_at"='2026-03-13 05:39:38.88',"image_digest"='sha256:7e2f895a12d5bc4586191452f5dd20968e8c800d53622073a2730c38d6b0eccb',"image_name"='nats',"image_tag"='2.10-alpine',"last_used_at"='2026-03-13 05:39:38.885',"namespace"='fortuna',"package_count"=18,"pod_name"='nats-1',"use_count"=4,"updated_at"='2026-03-13 05:39:38.885' WHERE "sboms"."deleted_at" IS NULL AND "id" = 1
2026/03/13 05:39:38.886281 [SBOM] Updated existing SBOM id=1 for pod_uid=eb24d3ce-b5cc-43cb-b766-7ad7ec4a5af4

2026/03/13 05:39:38 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:110
[0m[33m[1.382ms] [34;1m[rows:0][0m UPDATE "sbom_components" SET "deleted_at"='2026-03-13 05:39:38.886' WHERE sbom_id = 1 AND "sbom_components"."deleted_at" IS NULL

2026/03/13 05:39:38 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:157
[0m[33m[3.162ms] [34;1m[rows:0][0m INSERT INTO "sbom_components" ("sbom_id","component_type","component_name","component_version","purl","licenses","source","description","homepage","maintainer","created_at","deleted_at") VALUES (1,'os-package','alpine-baselayout','3.7.0-r0','pkg:PACKAGE_TYPE_APK/alpine-baselayout@3.7.0-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','alpine-baselayout-data','3.7.0-r0','pkg:PACKAGE_TYPE_APK/alpine-baselayout-data@3.7.0-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','alpine-keys','2.5-r0','pkg:PACKAGE_TYPE_APK/alpine-keys@2.5-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','alpine-release','3.22.3-r0','pkg:PACKAGE_TYPE_APK/alpine-release@3.22.3-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','apk-tools','2.14.9-r3','pkg:PACKAGE_TYPE_APK/apk-tools@2.14.9-r3','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','busybox','1.37.0-r20','pkg:PACKAGE_TYPE_APK/busybox@1.37.0-r20','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','busybox-binsh','1.37.0-r20','pkg:PACKAGE_TYPE_APK/busybox-binsh@1.37.0-r20','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','ca-certificates','20250911-r0','pkg:PACKAGE_TYPE_APK/ca-certificates@20250911-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','ca-certificates-bundle','20250911-r0','pkg:PACKAGE_TYPE_APK/ca-certificates-bundle@20250911-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','libapk2','2.14.9-r3','pkg:PACKAGE_TYPE_APK/libapk2@2.14.9-r3','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','libcrypto3','3.5.5-r0','pkg:PACKAGE_TYPE_APK/libcrypto3@3.5.5-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','libssl3','3.5.5-r0','pkg:PACKAGE_TYPE_APK/libssl3@3.5.5-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','musl','1.2.5-r10','pkg:PACKAGE_TYPE_APK/musl@1.2.5-r10','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','musl-utils','1.2.5-r10','pkg:PACKAGE_TYPE_APK/musl-utils@1.2.5-r10','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','scanelf','1.3.8-r1','pkg:PACKAGE_TYPE_APK/scanelf@1.3.8-r1','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','ssl_client','1.37.0-r20','pkg:PACKAGE_TYPE_APK/ssl_client@1.37.0-r20','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','tzdata','2025c-r0','pkg:PACKAGE_TYPE_APK/tzdata@2025c-r0','null','','','','','2026-03-13 05:39:38.887',NULL),(1,'os-package','zlib','1.3.1-r2','pkg:PACKAGE_TYPE_APK/zlib@1.3.1-r2','null','','','','','2026-03-13 05:39:38.887',NULL) ON CONFLICT ("sbom_id","purl") DO NOTHING RETURNING "id"

2026/03/13 05:39:38 [32mgithub.com/fortuna/core/internal/grpc/handler_sbom.go:186
[0m[33m[1.145ms] [34;1m[rows:1][0m SELECT * FROM "pods" WHERE (uid = 'eb24d3ce-b5cc-43cb-b766-7ad7ec4a5af4' AND deleted_at IS NULL) AND "pods"."deleted_at" IS NULL ORDER BY "pods"."id" LIMIT 1
2026/03/13 05:39:38.896016 [SBOM] Published SBOM_CREATED event for sbom_id=1 (pod_uid=eb24d3ce-b5cc-43cb-b766-7ad7ec4a5af4, reused=true)
2026/03/13 05:39:38.896219 [SBOM] Successfully stored SBOM id=1 with 18 components
[GIN] 2026/03/13 - 05:39:41 | 200 |    2.432306ms |      10.244.0.1 | GET      "/ready"

### Agent last 25 lines
[SBOMExtractor] 2026/03/13 05:39:19 ✅ Extracted digest from image manifest: sha256:7e2f895a12d5bc4586191452f5dd20968e8c800d53622073a2730c38d6b0eccb
[SBOMExtractor] 2026/03/13 05:39:23    Virtual FS: 586 files total, 0 paths containing package.json
[SBOMExtractor] 2026/03/13 05:39:23 Detected OS: alpine 3.22.3
[SBOMExtractor] 2026/03/13 05:39:23    Selected parsers for OS 'alpine': [apk npm pip gomod]
[SBOMExtractor] 2026/03/13 05:39:23 ✅ Parser apk found 18 packages
2026/03/13 05:39:23.370974 [SBOMExtractor] npm fallback: found 0 package-lock.json, 0 node_modules/.../package.json
[SBOMExtractor] 2026/03/13 05:39:23    Parser npm: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 05:39:23    Parser pip: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 05:39:23    Parser gomod: 0 packages (no matching files in image)
[SBOMExtractor] 2026/03/13 05:39:23 ✅ Extracted 18 unique packages in 5.111116786s
[SBOMProcessor] 2026/03/13 05:39:23 ✅ Extracted 18 packages from nats:2.10-alpine
[gRPCClient] 2026/03/13 05:39:23 Sending SBOM: pod=fortuna/nats-2 image=sha256:7e2f895a12d5bc4586191452f5dd20968e8c800d53622073a2730c38d6b0eccb
[SBOMProcessor] 2026/03/13 05:39:23 ✅ SBOM sent to Core: sbom_id=4 message=SBOM received and stored
[SBOMQueue] 2026/03/13 05:39:23 [Worker 1] ✅ Completed pod fortuna/nats-2 in 5.709199918s
[SBOMQueue] 2026/03/13 05:39:23 [Worker 1] Processing pod fortuna/postgres-5484f7745d-998x9
[SBOMProcessor] 2026/03/13 05:39:23 Processing pod fortuna/postgres-5484f7745d-998x9 on node k8s-worker01
[SBOMProcessor] 2026/03/13 05:39:23 🔍 Extracting SBOM: pod=fortuna/postgres-5484f7745d-998x9 container=postgres image=postgres:15-alpine
[SBOMExtractor] 2026/03/13 05:39:23 Extracting SBOM from image: postgres:15-alpine
[SBOMExtractor] 2026/03/13 05:39:24 ⚠️  Containerd fetch failed (image/layer may be missing on this node): containerd export error: content digest sha256:f0f0e2b18c9028792161db49359e64a801eefa9634e67bd09eb39fd0b375db95: not found
[SBOMExtractor] 2026/03/13 05:39:24 🔍 Falling back to remote registry: index.docker.io/library/postgres:15-alpine
[SBOMExtractor] 2026/03/13 05:39:24 Fetching image from remote registry: index.docker.io/library/postgres:15-alpine
[SBOMExtractor] 2026/03/13 05:39:25 ✅ Fetched image from remote registry
[SBOMExtractor] 2026/03/13 05:39:25 🔍 Attempting to get digest from image manifest...
[SBOMExtractor] 2026/03/13 05:39:25 ✅ Extracted digest from image manifest: sha256:79e73c0029e3a99b462253bba85352c6e748b93b9ffc5818ac0326cdbed62e76
2026/03/13 05:39:26.059435 ✅ Heartbeat OK (interval=15s)


[0;34m========== 1. Unit tests (Spec Hash, PCE, Pod Detail) ==========[0m

=== RUN   TestEqualTimePtr
--- PASS: TestEqualTimePtr (0.00s)
=== RUN   TestProcessSyncedPods_PodDetailFields
[AgentService] 2026/03/13 05:39:49.272475 📦 Processing 1 pods
[AgentService] 2026/03/13 05:39:49.272496 📦 Pod phase from agent: default/nginx phase=Running

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1062 [35;1mrecord not found
[0m[33m[0.268ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-1" AND uid = "pod-uid-123") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1157 [35;1mrecord not found
[0m[33m[0.122ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-1" AND uid = "pod-uid-123" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 05:39:49.273385 ✨ Created Pod default/nginx (SA: default)

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.053ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-123" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
--- PASS: TestProcessSyncedPods_PodDetailFields (0.01s)
=== RUN   TestProcessSyncedPods_PodDetailFields_SnakeCaseKeys

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:543 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.035ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-1" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL
[AgentService] 2026/03/13 05:39:49.282502 ❌ PCE evaluation failed for pod default/nginx: SQL logic error: no such table: role_bindings (1)
[AgentService] 2026/03/13 05:39:49.286558 📦 Processing 1 pods
[AgentService] 2026/03/13 05:39:49.287155 📦 Pod phase from agent: default/p2 phase=Running

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1062 [35;1mrecord not found
[0m[33m[0.168ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-snake" AND uid = "pod-uid-snake") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1157 [35;1mrecord not found
[0m[33m[0.174ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-snake" AND uid = "pod-uid-snake" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 05:39:49.288025 ✨ Created Pod default/p2 (SA: default)

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.046ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-snake" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
--- PASS: TestProcessSyncedPods_PodDetailFields_SnakeCaseKeys (0.01s)
=== RUN   TestProcessSyncedPods_SpecHashStoredAndConditionalPCE
[AgentService] 2026/03/13 05:39:49.292101 📦 Processing 1 pods
[AgentService] 2026/03/13 05:39:49.294053 📦 Pod phase from agent: default/p1 phase=Running

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.775ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-snake" AND uid = "pod-uid-snake") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 05:39:49.294397 ⚠️  PCE skipped: pod not found (cluster=test-cluster-snake uid=pod-uid-snake): SQL logic error: no such table: pods (1)

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1062 [35;1mrecord not found
[0m[33m[0.355ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1157 [35;1mrecord not found
[0m[33m[0.097ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-spec" AND uid = "uid-spec-1" ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 05:39:49.295203 ✨ Created Pod default/p1 (SA: default)

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.051ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-spec-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.239ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-spec" AND uid = "uid-spec-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 05:39:49.296685 ⚠️  PCE skipped: pod not found (cluster=test-cluster-spec uid=uid-spec-1): SQL logic error: no such table: pods (1)
[AgentService] 2026/03/13 05:39:49.297276 📦 Processing 1 pods
[AgentService] 2026/03/13 05:39:49.298016 📦 Pod phase from agent: default/p1 phase=Pending
[AgentService] 2026/03/13 05:39:49.298427 🔄 Updated Pod default/p1 (SA: default)
[AgentService] 2026/03/13 05:39:49.299280 📦 Processing 1 pods
[AgentService] 2026/03/13 05:39:49.299368 📦 Pod phase from agent: default/p1 phase=Running
[AgentService] 2026/03/13 05:39:49.299672 🔄 Updated Pod default/p1 (SA: default)
--- PASS: TestProcessSyncedPods_SpecHashStoredAndConditionalPCE (0.01s)
=== RUN   TestProcessSyncedPods_NullToNonNullSpecHash
[AgentService] 2026/03/13 05:39:49.303589 📦 Processing 1 pods
[AgentService] 2026/03/13 05:39:49.305214 📦 Pod phase from agent: default/p2 phase=Running

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1062 [35;1mrecord not found
[0m[33m[0.239ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1157 [35;1mrecord not found
[0m[33m[0.212ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE cluster_id = "test-cluster-null" AND uid = "uid-null-1" ORDER BY `pods`.`id` LIMIT 1

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/pkg/capability/evaluator.go:543 [35;1mSQL logic error: no such table: role_bindings (1)
[0m[33m[0.017ms] [34;1m[rows:0][0m SELECT * FROM `role_bindings` WHERE (cluster_id = "test-cluster-spec" AND namespace = "default" AND deleted_at IS NULL) AND `role_bindings`.`deleted_at` IS NULL
[AgentService] 2026/03/13 05:39:49.310272 ❌ PCE evaluation failed for pod default/p1: SQL logic error: no such table: role_bindings (1)
[AgentService] 2026/03/13 05:39:49.310532 ✨ Created Pod default/p2 (SA: default)

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.095ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "uid-null-1" ORDER BY `pod_instances`.`pod_uid` LIMIT 1

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.278ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 05:39:49.312078 ⚠️  PCE skipped: pod not found (cluster=test-cluster-null uid=uid-null-1): SQL logic error: no such table: pods (1)
[AgentService] 2026/03/13 05:39:49.313060 📦 Processing 1 pods
[AgentService] 2026/03/13 05:39:49.313980 📦 Pod phase from agent: default/p2 phase=Running
[AgentService] 2026/03/13 05:39:49.314557 🔄 Updated Pod default/p2 (SA: default)

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.037ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-null" AND uid = "uid-null-1") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 05:39:49.315338 ⚠️  PCE skipped: pod not found (cluster=test-cluster-null uid=uid-null-1): SQL logic error: no such table: pods (1)
--- PASS: TestProcessSyncedPods_NullToNonNullSpecHash (0.02s)
=== RUN   TestProcessSyncedPods_UpdatePodDetailFields
[AgentService] 2026/03/13 05:39:49.322565 📦 Processing 1 pods
[AgentService] 2026/03/13 05:39:49.322781 📦 Pod phase from agent: default/app phase=Running
[AgentService] 2026/03/13 05:39:49.323998 🔄 Updated Pod default/app (SA: default)

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/pkg/lifecycle/pod_instance_manager.go:24 [35;1mrecord not found
[0m[33m[0.052ms] [34;1m[rows:0][0m SELECT * FROM `pod_instances` WHERE pod_uid = "pod-uid-456" ORDER BY `pod_instances`.`pod_uid` LIMIT 1
[AgentService] 2026/03/13 05:39:49.326434 📡 Backfilled pod_ip=10.0.0.2 for default/app (uid=pod-uid-456)

2026/03/13 05:39:49 [31;1m/home/k8s/KSAM/core/internal/service/agent_service.go:1276 [35;1mSQL logic error: no such table: pods (1)
[0m[33m[0.192ms] [34;1m[rows:0][0m SELECT * FROM `pods` WHERE (cluster_id = "test-cluster-2" AND uid = "pod-uid-456") AND `pods`.`deleted_at` IS NULL ORDER BY `pods`.`id` LIMIT 1
[AgentService] 2026/03/13 05:39:49.327132 ⚠️  PCE skipped: pod not found (cluster=test-cluster-2 uid=pod-uid-456): SQL logic error: no such table: pods (1)
--- PASS: TestProcessSyncedPods_UpdatePodDetailFields (0.01s)
PASS
ok  	github.com/fortuna/core/internal/service	0.098s
[0;32mAgent service tests: PASS[0m
=== RUN   TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch
2026/03/13 05:39:51 [PCE] Skip stale evaluation for pod default/p (spec_hash changed)
--- PASS: TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch (0.01s)
=== RUN   TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount
--- PASS: TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount (0.01s)
PASS
ok  	github.com/fortuna/core/pkg/capability	0.051s
[0;32mCapability evaluator tests: PASS[0m
=== RUN   TestGetPod_ReturnsPodDetailFields
--- PASS: TestGetPod_ReturnsPodDetailFields (0.01s)
=== RUN   TestGetPodByUID_ReturnsPodDetailFields
--- PASS: TestGetPodByUID_ReturnsPodDetailFields (0.01s)
PASS
ok  	github.com/fortuna/core/internal/api	0.143s
?   	github.com/fortuna/core/internal/api/policy	[no test files]
testing: warning: no tests to run
PASS
ok  	github.com/fortuna/core/internal/api/risk	0.016s [no tests to run]
[0;32mAPI pod handlers tests: PASS[0m

[0;34m========== 2. Database verification ==========[0m

[0;32mUsing Postgres pod: postgres-5484f7745d-998x9[0m
[0;32m  pods.spec_hash: present[0m
[0;32m  pods.last_evaluated_hash: present[0m
[0;32m  EXPLAIN uses index (or seq scan on small table).[0m

[0;34m========== 3. Process / environment info ==========[0m

[0;32mCore pod: fortuna-core-774dc47859-5z5kr[0m
[0;32mAgent pod: fortuna-agent-2q9z5[0m

[0;34m========== 4. Test suite mapping (testSuite.md) ==========[0m


[0;32mReport written to: /home/k8s/KSAM/docs/test-results/pod-detail-test-suite-report-20260313-053943.md[0m


### API checks
- GET /inventory/pods: pod uid=543432aa-8481-4516-bfdd-cc35dd61fb94
podIP: None | startTime: 2026-03-13T05:39:11Z | riskCount: 5
- GET /runtime/pods/543432aa-8481-4516-bfdd-cc35dd61fb94/metrics: 200
- GET /runtime/pods/543432aa-8481-4516-bfdd-cc35dd61fb94/processes: 200
- GET /runtime/pods/543432aa-8481-4516-bfdd-cc35dd61fb94/network: 200
- GET /runtime/pods/543432aa-8481-4516-bfdd-cc35dd61fb94/events: 200

