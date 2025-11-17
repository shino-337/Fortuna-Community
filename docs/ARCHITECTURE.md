# KSAM Architecture

## Overview

KSAM (Kubernetes Service Account Manager) là một hệ thống quản lý tập trung cho ServiceAccounts trên nhiều Kubernetes clusters.

## Components

### 1. KSAM Agent
- **Type**: DaemonSet
- **Language**: Go
- **Chức năng**: Thu thập và theo dõi ServiceAccounts, RoleBindings, ClusterRoleBindings
- **Communication**: gRPC với Core Controller

### 2. KSAM Core Controller
- **Type**: Deployment
- **Language**: Go
- **Chức năng**: 
  - Nhận và lưu trữ dữ liệu từ Agents
  - Cung cấp REST/gRPC API
  - Xử lý graph data
  - Audit logging

### 3. KSAM Dashboard
- **Type**: Deployment
- **Language**: React + TypeScript
- **Chức năng**: Web UI với graph visualization

## Data Flow

```
K8s Cluster → Agent → Core Controller → Database
                                    ↓
                              Dashboard
```

## Storage

- **PostgreSQL**: Metadata, audit logs
- **Redis**: Caching, sync states (optional)
- **Neo4j**: Advanced graph queries (optional)

## Security

- mTLS giữa Agent ↔ Core
- RBAC cho API access
- Tokens lưu trong Vault/sealed-secrets

