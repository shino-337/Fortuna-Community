# Containerd Build Troubleshooting

**Date**: 2025-12-29

---

## Common Issues

### Issue 1: Multiple Core Images (latest and dirty)

**Symptoms:**
- Two Core images appear: `fortuna-core:latest` and `fortuna-core:dirty`
- No Agent image built

**Root Cause:**
- `git describe --tags --always --dirty` returns version with `-dirty` suffix
- Script creates tags for both `${VERSION}` (which may be "dirty") and `latest`

**Solution:**
✅ **Fixed in script** - Version is now sanitized:
```bash
# Old (problematic)
VERSION=$(git describe --tags --always --dirty)

# New (fixed)
GIT_VERSION=$(git describe --tags --always --dirty)
VERSION=$(echo "$GIT_VERSION" | sed 's/-dirty$//' | sed 's/[^a-zA-Z0-9._-]/-/g')
```

**Manual Fix:**
```bash
# Set clean version
VERSION=v1.0.0 ./scripts/build-with-containerd.sh

# Or use dev
VERSION=dev ./scripts/build-with-containerd.sh
```

---

### Issue 2: Agent Image Not Built

**Symptoms:**
- Only Core image appears
- Agent build fails silently or script exits early

**Root Cause:**
- Build error not properly caught
- Script continues even if Agent build fails

**Solution:**
✅ **Fixed in script** - Better error handling:
- Build function returns error code
- Script stops if any build fails
- Clear error messages

**Manual Check:**
```bash
# Check if Agent Dockerfile exists
ls -la agent/Dockerfile

# Try building Agent manually
cd /path/to/KSAM
nerdctl build -f agent/Dockerfile -t fortuna-agent:test --namespace k8s.io .

# Check build errors
nerdctl build -f agent/Dockerfile -t fortuna-agent:test --namespace k8s.io . 2>&1 | tee /tmp/agent-build.log
```

---

### Issue 3: Build Fails with "dirty" Tag

**Symptoms:**
- Build creates image with tag containing "dirty"
- Image tag invalid for Kubernetes

**Solution:**
```bash
# Clean git working directory
git status
git add .
git commit -m "WIP"  # or stash changes

# Or set explicit version
VERSION=v1.0.0 ./scripts/build-with-containerd.sh
```

---

## Verification Commands

### Check Built Images

```bash
# List all Fortuna images
ctr -n k8s.io images ls | grep fortuna

# Should see:
# fortuna-core:latest
# fortuna-core:<version>
# fortuna-agent:latest
# fortuna-agent:<version>
```

### Check Build Logs

```bash
# Run build with verbose output
./scripts/build-with-containerd.sh 2>&1 | tee /tmp/build.log

# Check for errors
grep -i error /tmp/build.log
grep -i fail /tmp/build.log
```

### Verify Both Components

```bash
# Check Core
ctr -n k8s.io images ls | grep "fortuna-core"

# Check Agent
ctr -n k8s.io images ls | grep "fortuna-agent"

# Both should show latest and version tags
```

---

## Clean Build Workflow

### Step 1: Clean Old Images

```bash
# Remove old images
./scripts/clean-containerd-images.sh
```

### Step 2: Clean Git State (Optional)

```bash
# Commit or stash changes to avoid "dirty" tag
git status
git add .
git commit -m "WIP: before build"
```

### Step 3: Build with Clean Version

```bash
# Option 1: Use explicit version
VERSION=v1.0.0 ./scripts/build-with-containerd.sh

# Option 2: Use dev (always clean)
VERSION=dev ./scripts/build-with-containerd.sh

# Option 3: Let script sanitize (recommended)
./scripts/build-with-containerd.sh
```

### Step 4: Verify

```bash
# Check images
ctr -n k8s.io images ls | grep fortuna

# Should see:
# fortuna-core:latest
# fortuna-core:<sanitized-version>
# fortuna-agent:latest
# fortuna-agent:<sanitized-version>
```

---

## Debugging Build Failures

### Check Dockerfile Paths

```bash
# Verify Dockerfiles exist
ls -la core/Dockerfile
ls -la agent/Dockerfile

# Check build context
cd /path/to/KSAM
pwd  # Should be repository root
```

### Test Individual Builds

```bash
# Test Core build
cd /path/to/KSAM
nerdctl build -f core/Dockerfile -t fortuna-core:test --namespace k8s.io .

# Test Agent build
nerdctl build -f agent/Dockerfile -t fortuna-agent:test --namespace k8s.io .
```

### Check Build Context

```bash
# Verify required files exist
ls -la api/
ls -la core/go.mod
ls -la agent/go.mod
```

### Check Containerd

```bash
# Verify containerd is running
sudo systemctl status containerd

# Check socket
ls -l /run/containerd/containerd.sock
ls -l /var/run/containerd/containerd.sock

# Test nerdctl
nerdctl --namespace k8s.io images ls
```

---

## Expected Output

### Successful Build

```
==========================================
Fortuna Build with Containerd (nerdctl)
==========================================

Configuration:
  Image Prefix: fortuna
  Version:     v1.0.0  (or dev, not dirty)
  Commit:      abc1234
  Build Time:  2025-12-29T10:00:00Z
  Namespace:   k8s.io

✅ nerdctl found
✅ ctr found
✅ containerd.sock found

==========================================
Building Core
==========================================
Building core with nerdctl...
  Image: fortuna-core:v1.0.0
  Image (latest): fortuna-core:latest
  Dockerfile: core/Dockerfile

[build output...]

✅ core built successfully
  fortuna-core:v1.0.0
  fortuna-core:latest

✅ Image fortuna-core:v1.0.0 found in containerd

==========================================
Building Agent
==========================================
Building agent with nerdctl...
  Image: fortuna-agent:v1.0.0
  Image (latest): fortuna-agent:latest
  Dockerfile: agent/Dockerfile

[build output...]

✅ agent built successfully
  fortuna-agent:v1.0.0
  fortuna-agent:latest

✅ Image fortuna-agent:v1.0.0 found in containerd

==========================================
✅ Build Complete!
==========================================
```

---

## Quick Fixes

### Fix "dirty" Tag Issue

```bash
# Clean git state
git add .
git commit -m "WIP"

# Rebuild
./scripts/build-with-containerd.sh
```

### Fix Missing Agent Image

```bash
# Check Agent Dockerfile
ls -la agent/Dockerfile

# Build Agent manually
cd /path/to/KSAM
nerdctl build -f agent/Dockerfile -t fortuna-agent:latest --namespace k8s.io .

# Verify
ctr -n k8s.io images ls | grep fortuna-agent
```

### Clean and Rebuild

```bash
# Clean old images
./scripts/clean-containerd-images.sh

# Rebuild
./scripts/build-with-containerd.sh
```

---

**Last Updated**: 2025-12-29

