# PersistentVolumeClaim Troubleshooting

## Problem: Pod has unbound immediate PersistentVolumeClaims

### Symptoms

```
Warning  FailedScheduling  default-scheduler  
0/2 nodes are available: pod has unbound immediate PersistentVolumeClaims. 
preemption: 0/2 nodes are available: 2 Preemption is not helpful for scheduling.
```

### Root Cause

The PersistentVolumeClaim (PVC) cannot find a PersistentVolume (PV) to bind to. This typically happens when:
1. No StorageClass is configured in the cluster
2. No default StorageClass exists
3. No PVs are available that match the PVC requirements

---

## Solutions

### Solution 1: Check and Configure StorageClass

#### Step 1: Check existing StorageClasses

```bash
kubectl get storageclass
```

#### Step 2: If no StorageClass exists, install local-path-provisioner

For kubeadm clusters, you can use `local-path-provisioner`:

```bash
# Install local-path-provisioner
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml

# Set as default
kubectl patch storageclass local-path -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
```

#### Step 3: Update PVC to use StorageClass

In `deploy/infrastructure/postgresql-with-age.yaml`, the PVC already has:
```yaml
storageClassName: ""  # Uses default storage class
```

Or specify explicitly:
```yaml
storageClassName: local-path
```

---

### Solution 2: Use Local PersistentVolume (Manual)

For environments without StorageClass, use manual PV creation:

#### Step 1: Create directory on node

```bash
# On the node where PostgreSQL will run
sudo mkdir -p /mnt/postgres-data
sudo chmod 777 /mnt/postgres-data
```

#### Step 2: Use local PV manifest

```bash
kubectl apply -f deploy/infrastructure/postgresql-local-pv.yaml
```

This creates:
- A PersistentVolume pointing to `/mnt/postgres-data`
- A PersistentVolumeClaim that binds to it

---

### Solution 3: Use emptyDir (Development Only)

For development/testing, you can use `emptyDir` instead of PVC:

```yaml
volumes:
- name: postgres-storage
  emptyDir: {}
```

**Warning**: Data will be lost when pod is deleted!

---

## Verification

### Check PVC Status

```bash
kubectl get pvc -n fortuna
```

Should show:
```
NAME           STATUS   VOLUME       CAPACITY   ACCESS MODES   STORAGECLASS   AGE
postgres-pvc   Bound    postgres-pv  20Gi       RWO            local-storage  5m
```

### Check PV Status

```bash
kubectl get pv
```

Should show:
```
NAME          CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                STORAGECLASS   AGE
postgres-pv   20Gi       RWO            Retain           Bound    fortuna/postgres-pvc   local-storage  5m
```

### Check Pod Status

```bash
kubectl get pods -n fortuna -l app=postgres
```

Should show:
```
NAME                        READY   STATUS    RESTARTS   AGE
postgres-xxxxxxxxxx-xxxxx   1/1     Running   0          2m
```

---

## Common Issues

### Issue 1: PVC stuck in Pending

**Cause**: No StorageClass or PV available

**Solution**: 
- Install StorageClass (Solution 1)
- Or create manual PV (Solution 2)

### Issue 2: PV not binding to PVC

**Cause**: StorageClass mismatch or access mode mismatch

**Solution**:
- Ensure PVC and PV have same `storageClassName`
- Ensure access modes match (ReadWriteOnce, ReadWriteMany, etc.)

### Issue 3: Permission denied on hostPath

**Cause**: Directory permissions on node

**Solution**:
```bash
sudo chmod 777 /mnt/postgres-data
# Or use specific user/group
sudo chown -R 999:999 /mnt/postgres-data  # PostgreSQL UID
```

---

## Quick Fix for Development

If you just want to get started quickly:

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Install local-path-provisioner
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml
kubectl patch storageclass local-path -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'

# 3. Apply PostgreSQL
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml

# 4. Verify
kubectl get pvc -n fortuna
kubectl get pods -n fortuna -l app=postgres
```

---

## Production Recommendations

For production environments:

1. **Use proper StorageClass**: Install a production-ready storage provisioner (e.g., NFS, Ceph, AWS EBS, etc.)
2. **Use StatefulSet**: For databases, consider using StatefulSet instead of Deployment
3. **Backup Strategy**: Implement regular backups of PV data
4. **Monitoring**: Monitor PV/PVC usage and capacity

---

## References

- [Kubernetes PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
- [Local Path Provisioner](https://github.com/rancher/local-path-provisioner)
- [Storage Classes](https://kubernetes.io/docs/concepts/storage/storage-classes/)

