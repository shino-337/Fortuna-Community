.PHONY: build build-agent build-core build-dashboard clean test docker-build docker-push deploy minikube-setup minikube-test minikube-cleanup rebuild rebuild-dashboard rebuild-core rebuild-agent

# Build all components
build: build-agent build-core build-dashboard

# Build Agent
build-agent:
	@echo "Building KSAM Agent..."
	cd agent && go mod download && go build -o ../bin/ksam-agent ./cmd

# Build Core
build-core:
	@echo "Building KSAM Core..."
	cd core && go mod download && go build -o ../bin/ksam-core ./cmd

# Build Dashboard
build-dashboard:
	@echo "Building KSAM Dashboard..."
	cd dashboard && npm install && npm run build

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf dashboard/dist/
	rm -rf dashboard/node_modules/

# Run tests
test:
	cd agent && go test ./...
	cd core && go test ./...

# Docker build
docker-build:
	docker build -t ksam/agent:latest ./agent
	docker build -t ksam/core:latest ./core
	docker build -t ksam/dashboard:latest ./dashboard

# Docker build in Minikube
docker-build-minikube:
	@echo "Setting up Minikube Docker environment..."
	eval $$(minikube docker-env) && \
	docker build -t ksam/agent:latest ./agent && \
	docker build -t ksam/core:latest ./core && \
	docker build -t ksam/dashboard:latest ./dashboard

# Docker compose
docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

# Helm deploy
deploy:
	helm install ksam ./helm/ksam

undeploy:
	helm uninstall ksam

# Minikube setup
minikube-setup:
	@echo "Setting up KSAM on Minikube..."
	./scripts/setup-minikube.sh

# Minikube test
minikube-test:
	@echo "Creating test data..."
	./scripts/create-test-data.sh
	@echo "Testing API..."
	./scripts/test-api.sh

# Minikube cleanup
minikube-cleanup:
	@echo "Cleaning up KSAM resources..."
	./scripts/cleanup.sh

# Quick start (setup + test)
quickstart: minikube-setup minikube-test
	@echo "✅ Quick start complete!"
	@echo "Access Dashboard: kubectl port-forward -n ksam svc/ksam-dashboard 3000:80"
	@echo "Login: admin / admin123"

# Rebuild and redeploy (for Minikube)
rebuild:
	@echo "Rebuilding all components..."
	./scripts/rebuild.sh all

rebuild-dashboard:
	@echo "Rebuilding Dashboard..."
	./scripts/rebuild.sh dashboard

rebuild-core:
	@echo "Rebuilding Core Controller..."
	./scripts/rebuild.sh core

rebuild-agent:
	@echo "Rebuilding Agent..."
	./scripts/rebuild.sh agent

