#!/bin/bash

set -e

echo "🚀 Setting up KSAM on Minikube..."

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"
command -v minikube >/dev/null 2>&1 || { echo -e "${RED}minikube is required but not installed.${NC}" >&2; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo -e "${RED}kubectl is required but not installed.${NC}" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo -e "${RED}docker is required but not installed.${NC}" >&2; exit 1; }
command -v helm >/dev/null 2>&1 || { echo -e "${YELLOW}helm is not installed (optional)${NC}"; }

# Start Minikube
echo -e "${GREEN}Starting Minikube...${NC}"
if ! minikube status >/dev/null 2>&1; then
    minikube start --memory=4096 --cpus=2
else
    echo "Minikube is already running"
fi

# Set Docker environment
echo -e "${GREEN}Setting up Docker environment...${NC}"
eval $(minikube docker-env)

# Create namespace
echo -e "${GREEN}Creating namespace...${NC}"
kubectl create namespace ksam --dry-run=client -o yaml | kubectl apply -f -

# Deploy PostgreSQL
echo -e "${GREEN}Deploying PostgreSQL...${NC}"
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: postgres-secret
  namespace: ksam
type: Opaque
stringData:
  postgres-password: postgres
  postgres-user: postgres
  postgres-db: ksam
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
  namespace: ksam
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: ksam
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        env:
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: postgres-user
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: postgres-password
        - name: POSTGRES_DB
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: postgres-db
        ports:
        - containerPort: 5432
        volumeMounts:
        - name: postgres-storage
          mountPath: /var/lib/postgresql/data
      volumes:
      - name: postgres-storage
        persistentVolumeClaim:
          claimName: postgres-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: ksam
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
EOF

# Wait for PostgreSQL
echo -e "${YELLOW}Waiting for PostgreSQL to be ready...${NC}"
kubectl wait --for=condition=ready pod -l app=postgres -n ksam --timeout=120s

# Build Docker images
echo -e "${GREEN}Building Docker images...${NC}"

# Ensure go.sum files exist
echo "Preparing Go modules..."
cd agent && go mod tidy && cd ..
cd core && go mod tidy && cd ..

echo "Building Agent image..."
cd agent
docker build -t ksam/agent:latest . || { echo -e "${RED}Failed to build agent image${NC}"; exit 1; }
cd ..

echo "Building Core image..."
cd core
docker build -t ksam/core:latest . || { echo -e "${RED}Failed to build core image${NC}"; exit 1; }
cd ..

echo "Building Dashboard image..."
cd dashboard
docker build -t ksam/dashboard:latest . || { echo -e "${RED}Failed to build dashboard image${NC}"; exit 1; }
cd ..

# Create secrets
echo -e "${GREEN}Creating secrets...${NC}"
kubectl create secret generic ksam-secrets -n ksam \
  --from-literal=database-url="postgres://postgres:postgres@postgres:5432/ksam?sslmode=disable" \
  --from-literal=jwt-secret="minikube-test-secret-change-in-production" \
  --dry-run=client -o yaml | kubectl apply -f -

# Deploy Core Controller
echo -e "${GREEN}Deploying Core Controller...${NC}"
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ksam-core
  namespace: ksam
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ksam-core
  template:
    metadata:
      labels:
        app: ksam-core
    spec:
      containers:
      - name: core
        image: ksam/core:latest
        imagePullPolicy: Never
        ports:
        - name: http
          containerPort: 8080
        - name: grpc
          containerPort: 9090
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: ksam-secrets
              key: database-url
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: ksam-secrets
              key: jwt-secret
        - name: HTTP_PORT
          value: "8080"
        - name: GRPC_PORT
          value: "9090"
        - name: AUTH_ENABLED
          value: "true"
        - name: KSAM_ADMIN_USERNAME
          value: "admin"
        - name: KSAM_ADMIN_PASSWORD
          value: "admin123"
        - name: KSAM_ADMIN_EMAIL
          value: "admin@ksam.local"
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /live
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: http
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: ksam-core
  namespace: ksam
spec:
  type: ClusterIP
  selector:
    app: ksam-core
  ports:
  - name: http
    port: 8080
    targetPort: http
  - name: grpc
    port: 9090
    targetPort: grpc
EOF

# Wait for Core
echo -e "${YELLOW}Waiting for Core Controller to be ready...${NC}"
kubectl wait --for=condition=ready pod -l app=ksam-core -n ksam --timeout=120s

# Deploy Agent
echo -e "${GREEN}Deploying Agent...${NC}"
kubectl apply -f agent/deploy/rbac.yaml

kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ksam-agent
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: ksam-agent
  template:
    metadata:
      labels:
        app: ksam-agent
    spec:
      serviceAccountName: ksam-agent
      containers:
      - name: agent
        image: ksam/agent:latest
        imagePullPolicy: Never
        env:
        - name: KSAM_CORE_ENDPOINT
          value: "ksam-core.ksam.svc.cluster.local:9090"
        - name: KSAM_CLUSTER_ID
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: KSAM_SYNC_INTERVAL
          value: "30s"
        resources:
          requests:
            memory: "50Mi"
            cpu: "50m"
          limits:
            memory: "100Mi"
            cpu: "100m"
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
EOF

# Wait for Agent
echo -e "${YELLOW}Waiting for Agent to be ready...${NC}"
kubectl wait --for=condition=ready pod -l app=ksam-agent -n kube-system --timeout=120s

# Deploy Dashboard
echo -e "${GREEN}Deploying Dashboard...${NC}"
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ksam-dashboard
  namespace: ksam
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ksam-dashboard
  template:
    metadata:
      labels:
        app: ksam-dashboard
    spec:
      containers:
      - name: dashboard
        image: ksam/dashboard:latest
        imagePullPolicy: Never
        ports:
        - name: http
          containerPort: 80
        resources:
          requests:
            memory: "128Mi"
            cpu: "50m"
          limits:
            memory: "256Mi"
            cpu: "200m"
---
apiVersion: v1
kind: Service
metadata:
  name: ksam-dashboard
  namespace: ksam
spec:
  type: NodePort
  selector:
    app: ksam-dashboard
  ports:
  - port: 80
    targetPort: http
    nodePort: 30080
EOF

# Wait for Dashboard
echo -e "${YELLOW}Waiting for Dashboard to be ready...${NC}"
kubectl wait --for=condition=ready pod -l app=ksam-dashboard -n ksam --timeout=120s

# Get access information
echo -e "${GREEN}✅ Setup complete!${NC}"
echo ""
echo "📊 Access Information:"
echo "======================"
MINIKUBE_IP=$(minikube ip)
echo "Dashboard URL: http://$MINIKUBE_IP:30080"
echo ""
echo "🔐 Default Credentials:"
echo "Username: admin"
echo "Password: admin123"
echo ""
echo "🔧 Useful Commands:"
echo "  kubectl port-forward -n ksam svc/ksam-dashboard 3000:80"
echo "  kubectl port-forward -n ksam svc/ksam-core 8080:8080"
echo "  kubectl logs -l app=ksam-core -n ksam -f"
echo "  kubectl logs -l app=ksam-agent -n kube-system -f"
echo ""
echo "📈 Check Status:"
echo "  kubectl get pods -n ksam"
echo "  kubectl get pods -n kube-system | grep ksam"

