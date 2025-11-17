# Database Disk Space Management

## Vấn đề

Database có thể bị đầy do:
- Docker images tích lũy
- Build cache
- Logs
- Temporary files

Khi database đầy, sẽ xuất hiện lỗi:
```
PANIC: could not write to file "pg_logical/replorigin_checkpoint.tmp": No space left on device
```

## Giải pháp

### 1. Cleanup Disk Space

Sử dụng script cleanup:

```bash
./scripts/cleanup-disk-space.sh
```

Script này sẽ:
- Xóa old Docker images
- Xóa build cache
- Xóa old logs
- Xóa temporary files

### 2. Cleanup Docker Images

```bash
./scripts/cleanup-images.sh
```

### 3. Manual Cleanup

```bash
# Clean up Docker system
minikube ssh -- docker system prune -af --volumes

# Clean up old logs
minikube ssh -- "sudo journalctl --vacuum-time=7d"

# Clean up temporary files
minikube ssh -- "sudo rm -rf /tmp/* /var/tmp/*"
```

## Kiểm tra Disk Space

```bash
# Check disk usage
minikube ssh -- df -h

# Check Docker disk usage
minikube ssh -- docker system df
```

## Phòng ngừa

1. **Regular Cleanup**: Chạy cleanup scripts định kỳ
2. **Monitor Disk Usage**: Kiểm tra disk usage thường xuyên
3. **Increase Disk Size**: Nếu cần, tăng disk size của Minikube:
   ```bash
   minikube stop
   minikube delete
   minikube start --disk-size=100g
   ```

## Recovery

Nếu database bị crash do đầy disk:

1. Cleanup disk space
2. Restart postgres pod:
   ```bash
   kubectl delete pod -n ksam -l app=postgres
   ```
3. Wait for postgres to recover
4. Restart core pod để chạy migrations:
   ```bash
   kubectl delete pod -n ksam -l app=ksam-core
   ```

## Monitoring

Check database status:

```bash
# Check postgres pod
kubectl get pods -n ksam -l app=postgres

# Check postgres logs
kubectl logs -n ksam -l app=postgres --tail=50

# Check disk usage
minikube ssh -- df -h
```

