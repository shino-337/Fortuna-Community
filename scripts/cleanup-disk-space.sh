#!/bin/bash

# Script to clean up disk space in Minikube

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Minikube Disk Space Cleanup${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check current disk usage
echo -e "${YELLOW}Current disk usage:${NC}"
minikube ssh -- df -h | grep -E "Filesystem|vda|/dev" || true
echo ""

# Clean up old Docker images
echo -e "${YELLOW}Cleaning up old Docker images...${NC}"
minikube ssh -- docker system prune -af --volumes 2>&1 | tail -5
echo ""

# Clean up old logs
echo -e "${YELLOW}Cleaning up old logs...${NC}"
minikube ssh -- "sudo journalctl --vacuum-time=7d" 2>&1 | tail -3 || true
echo ""

# Clean up temporary files
echo -e "${YELLOW}Cleaning up temporary files...${NC}"
minikube ssh -- "sudo rm -rf /tmp/* /var/tmp/*" 2>&1 || true
echo ""

# Check disk usage after cleanup
echo -e "${YELLOW}Disk usage after cleanup:${NC}"
minikube ssh -- df -h | grep -E "Filesystem|vda|/dev" || true
echo ""

# Check Docker disk usage
echo -e "${YELLOW}Docker disk usage:${NC}"
minikube ssh -- docker system df 2>&1 | tail -5
echo ""

echo -e "${GREEN}✓ Cleanup complete${NC}"

