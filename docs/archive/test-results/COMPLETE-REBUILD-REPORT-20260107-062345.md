# Complete Rebuild & Deployment Report

**Date**: $(date '+%Y-%m-%d %H:%M:%S')
**Action**: Complete Clean, Rebuild & Redeploy

---

## Network Issue Analysis & Solution

### Problem Identified
- Core pod (master node, 10.244.0.x) cannot connect to PostgreSQL (worker node, 10.244.1.190)
- Network routing issue between master pod network (10.244.0.0/24) and worker pod network (10.244.1.0/24)
- Cannot ping worker pod IP from master host

### Root Cause
- Flannel network routing not working correctly between master and worker nodes
- Cross-node pod-to-pod communication blocked

### Solution Applied
**Schedule PostgreSQL on master node (same as Core)**
- Modified `deploy/infrastructure/postgresql.yaml` to add `nodeSelector: node-role.kubernetes.io/control-plane: ""`
- Deleted and recreated PostgreSQL PVC to allow scheduling on master node
- Result: PostgreSQL and Core now on same node → local network communication

---

## Cleanup & Rebuild Process

### Step 1: Complete Cleanup
- ✅ All deployments deleted (Core, Agent, NATS, PostgreSQL)
- ✅ All pods deleted
- ✅ Images cleaned (fortuna-core, fortuna/agent)

### Step 2: Rebuild Images
- ✅ Core image rebuilt using `scripts/build-and-load-containerd.sh`
- ✅ Agent image rebuilt with correct context from root directory

### Step 3: Redeploy Infrastructure
- ✅ PostgreSQL: Deployed with nodeSelector for master node
- ✅ NATS: Deployed (3 replicas, distributed across nodes)
- ✅ Core: Deployed (master node only)
- ✅ Agent: Deployed (DaemonSet on all nodes)

---

## Deployment Results

### Pod Status
NAME                            READY   STATUS    RESTARTS       AGE    IP             NODE           NOMINATED NODE   READINESS GATES
fortuna-agent-75z5v             1/1     Running   0              43s    10.244.0.181   k8s-master     <none>           <none>
fortuna-agent-8nddw             1/1     Running   0              44s    10.244.1.219   k8s-worker01   <none>           <none>
fortuna-core-5845698f8c-mw8pm   1/1     Running   3 (110m ago)   118m   10.244.0.173   k8s-master     <none>           <none>
nats-0                          1/1     Running   0              118m   10.244.1.210   k8s-worker01   <none>           <none>
nats-1                          1/1     Running   0              118m   10.244.0.172   k8s-master     <none>           <none>
nats-2                          1/1     Running   0              118m   10.244.1.211   k8s-worker01   <none>           <none>
postgres-77cb8f5995-rl6x7       1/1     Running   0              118m   10.244.0.176   k8s-master     <none>           <none>

### Services
NAME           TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                      AGE
fortuna-core   ClusterIP   10.100.74.197    <none>        8080/TCP,9090/TCP            27h
nats           ClusterIP   None             <none>        4222/TCP,8222/TCP,6222/TCP   27h
nats-client    ClusterIP   10.100.224.111   <none>        4222/TCP,8222/TCP            27h
postgres       ClusterIP   10.102.67.249    <none>        5432/TCP                     27h

### Core Database Connection
{"status":"ready","timestamp":"2026-01-07T06:23:46.645042904Z","checks":{"database":"ok"}}
### Network Solution
PostgreSQL Pod IP: 10.244.0.176
Core Pod IP: 10.244.0.173
Both pods on: k8s-master
