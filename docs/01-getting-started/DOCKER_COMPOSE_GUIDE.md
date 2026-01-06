# Docker Compose Guide

**Date**: 2025-12-29  
**Status**: ✅ Updated

---

## Overview

Docker Compose configuration for local development and testing of Fortuna.

---

## Quick Start

### Basic Usage

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

### With Dashboard

```bash
# Start with dashboard
docker-compose --profile dashboard up -d
```

---

## Services

### 1. PostgreSQL

- **Image**: `postgres:15-alpine`
- **Port**: `5432`
- **Database**: `ksam`
- **User**: `postgres`
- **Password**: `postgres`
- **Volume**: `postgres_data`

### 2. NATS

- **Image**: `nats:2.10-alpine`
- **Ports**: 
  - `4222` (client)
  - `8222` (monitoring)
- **JetStream**: Enabled
- **Volume**: `nats_data`

### 3. Fortuna Core

- **Image**: `fortuna-core:dev` (built from `core/Dockerfile`)
- **Ports**:
  - `8080` (HTTP API)
  - `9090` (gRPC)
- **Depends on**: PostgreSQL, NATS
- **Environment**:
  - `DATABASE_URL`: PostgreSQL connection
  - `NATS_ENDPOINT`: NATS connection
  - `TLS_ENABLED`: `false` (local dev)

### 4. Fortuna Agent

- **Image**: `fortuna-agent:dev` (built from `agent/Dockerfile`)
- **Depends on**: Core
- **Volumes**:
  - Docker socket: `/var/run/docker.sock` (for container inspection)
  - Or containerd socket: `/run/containerd/containerd.sock`
- **Environment**:
  - `CORE_GRPC_ENDPOINT`: `core:9090`
  - `TLS_ENABLED`: `false` (local dev)

### 5. Dashboard (Optional)

- **Image**: `fortuna-dashboard:dev`
- **Port**: `3000`
- **Profile**: `dashboard` (only starts with `--profile dashboard`)
- **Depends on**: Core

---

## Configuration

### Environment Variables

#### Core

```yaml
DATABASE_URL: postgres://postgres:postgres@postgres:5432/ksam?sslmode=disable
NATS_ENDPOINT: nats://nats:4222
HTTP_PORT: 8080
GRPC_PORT: 9090
TLS_ENABLED: false
LOG_LEVEL: debug
AUTH_ENABLED: false
```

#### Agent

```yaml
CORE_GRPC_ENDPOINT: core:9090
TLS_ENABLED: false
NODE_NAME: local-dev
LOG_LEVEL: debug
BATCH_SIZE: 50
BATCH_TIMEOUT_MS: 5000
```

---

## Development Workflow

### 1. Start Services

```bash
# Start infrastructure only
docker-compose up -d postgres nats

# Start Core
docker-compose up -d core

# Start Agent
docker-compose up -d agent

# Start Dashboard (optional)
docker-compose --profile dashboard up -d dashboard
```

### 2. Hot Reload (Optional)

Create `docker-compose.override.yml`:

```yaml
version: '3.8'
services:
  core:
    volumes:
      - ./core:/app/core:ro
      - ./api:/app/api:ro
  agent:
    volumes:
      - ./agent:/app/agent:ro
      - ./api:/app/api:ro
```

### 3. View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f core
docker-compose logs -f agent
docker-compose logs -f postgres
docker-compose logs -f nats
```

### 4. Rebuild Images

```bash
# Rebuild all
docker-compose build

# Rebuild specific service
docker-compose build core
docker-compose build agent

# Rebuild and restart
docker-compose up -d --build core
```

### 5. Access Services

- **Core API**: http://localhost:8080
- **Core gRPC**: localhost:9090
- **Dashboard**: http://localhost:3000
- **NATS Monitoring**: http://localhost:8222
- **PostgreSQL**: localhost:5432

---

## Production Configuration

### Using Production Compose

```bash
# Use production configuration
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### Production Settings

- Use production images (from registry)
- Enable TLS
- Set `LOG_LEVEL: info`
- Use proper resource limits
- Enable health checks
- Configure backups

---

## Troubleshooting

### Services Not Starting

```bash
# Check service status
docker-compose ps

# Check logs
docker-compose logs

# Restart service
docker-compose restart <service>
```

### Database Connection Issues

```bash
# Check PostgreSQL is ready
docker-compose exec postgres pg_isready -U postgres

# Connect to database
docker-compose exec postgres psql -U postgres -d ksam
```

### NATS Connection Issues

```bash
# Check NATS is ready
curl http://localhost:8222/healthz

# Check NATS monitoring
curl http://localhost:8222/varz
```

### Agent Cannot Connect to Core

```bash
# Check Core is healthy
curl http://localhost:8080/health

# Check Core logs
docker-compose logs core

# Verify network
docker-compose exec agent ping core
```

### Port Conflicts

If ports are already in use:

```yaml
# Edit docker-compose.yml and change ports
services:
  core:
    ports:
      - "18080:8080"  # Use different host port
      - "19090:9090"
```

---

## Volumes

### Persistent Data

- `postgres_data`: PostgreSQL data
- `nats_data`: NATS JetStream data

### Backup

```bash
# Backup PostgreSQL
docker-compose exec postgres pg_dump -U postgres ksam > backup.sql

# Restore PostgreSQL
docker-compose exec -T postgres psql -U postgres ksam < backup.sql
```

---

## Network

All services are on `fortuna-network` bridge network.

Services can communicate using service names:
- `postgres:5432`
- `nats:4222`
- `core:8080` and `core:9090`
- `agent` (connects to `core:9090`)

---

## Cleanup

### Stop and Remove

```bash
# Stop services
docker-compose down

# Stop and remove volumes (⚠️ deletes data)
docker-compose down -v

# Remove images
docker-compose down --rmi all
```

### Remove Everything

```bash
# Stop, remove containers, volumes, and images
docker-compose down -v --rmi all
```

---

## Differences from Kubernetes

| Feature | Docker Compose | Kubernetes |
|---------|---------------|------------|
| **Agent** | Single container | DaemonSet (one per node) |
| **TLS** | Disabled by default | Enabled with mTLS |
| **Service Discovery** | Docker DNS | Kubernetes DNS |
| **Volumes** | Named volumes | PVCs |
| **Networking** | Bridge network | Cluster network |
| **Scaling** | Manual | Horizontal scaling |

---

## Best Practices

1. **Use override files** for local development
2. **Enable hot-reload** during development
3. **Use profiles** for optional services (dashboard)
4. **Set proper resource limits** in production
5. **Enable health checks** for all services
6. **Use production images** in production
7. **Enable TLS** in production
8. **Configure backups** for persistent data

---

## Examples

### Development with Hot Reload

```bash
# Create override file
cp docker-compose.override.yml.example docker-compose.override.yml

# Start services
docker-compose up -d
```

### Production-like Setup

```bash
# Build production images
docker-compose -f docker-compose.yml -f docker-compose.prod.yml build

# Start with production config
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### Testing Specific Component

```bash
# Test Core only
docker-compose up -d postgres nats core

# Test Agent connection
docker-compose up -d core agent
```

---

**Last Updated**: 2025-12-29

