#!/bin/bash

# Script to clean up old Docker images in Minikube
# Removes all dev-* tagged images and keeps only latest

set -e

MINIKUBE_DOCKER_ENV=$(minikube docker-env 2>/dev/null || echo "")

if [ -z "$MINIKUBE_DOCKER_ENV" ]; then
  echo "Error: Minikube is not running or not configured"
  exit 1
fi

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "=========================================="
echo "KSAM Image Cleanup Script"
echo "=========================================="
echo ""

# Setup Minikube Docker environment
eval $(minikube docker-env)

# List of image prefixes to clean
IMAGE_PREFIXES=("ksam/dashboard" "ksam/core" "ksam/agent" "ksam-core")

TOTAL_SIZE=0
DELETED_COUNT=0

for prefix in "${IMAGE_PREFIXES[@]}"; do
  echo -e "${YELLOW}Cleaning images for: ${prefix}${NC}"
  
  # Get all images with dev-* tags
  DEV_IMAGES=$(docker images "${prefix}" --format "{{.Repository}}:{{.Tag}}" | grep -E "dev-|v[0-9]" || true)
  
  if [ -z "$DEV_IMAGES" ]; then
    echo "  No dev-* tagged images found"
    continue
  fi
  
  # Count and calculate size before deletion
  for image in $DEV_IMAGES; do
    SIZE=$(docker images "$image" --format "{{.Size}}" | sed 's/[^0-9.]//g' || echo "0")
    echo "  Found: $image (Size: ${SIZE}MB)"
    DELETED_COUNT=$((DELETED_COUNT + 1))
  done
  
  # Delete dev-* tagged images
  echo "$DEV_IMAGES" | while read -r image; do
    if [ -n "$image" ]; then
      echo "  Deleting: $image"
      docker rmi "$image" 2>/dev/null || echo "    (already deleted or in use)"
    fi
  done
done

# Clean up dangling images
echo ""
echo -e "${YELLOW}Cleaning dangling images...${NC}"
DANGLING_COUNT=$(docker images -f "dangling=true" -q | wc -l | tr -d ' ')
if [ "$DANGLING_COUNT" -gt 0 ]; then
  docker image prune -f
  echo "  Removed $DANGLING_COUNT dangling images"
else
  echo "  No dangling images found"
fi

# Show current disk usage
echo ""
echo -e "${YELLOW}Current image usage:${NC}"
docker system df

# Show remaining KSAM images
echo ""
echo -e "${YELLOW}Remaining KSAM images:${NC}"
for prefix in "${IMAGE_PREFIXES[@]}"; do
  REMAINING=$(docker images "${prefix}" --format "{{.Repository}}:{{.Tag}}" || true)
  if [ -n "$REMAINING" ]; then
    echo "$REMAINING" | while read -r image; do
      if [ -n "$image" ]; then
        SIZE=$(docker images "$image" --format "{{.Size}}")
        echo "  $image ($SIZE)"
      fi
    done
  fi
done

echo ""
echo "=========================================="
echo -e "${GREEN}Cleanup complete!${NC}"
echo "=========================================="
echo ""

