# Fortuna Proto Build Makefile

.PHONY: all generate clean verify install-tools

# Default target
all: generate

# Install protoc plugins
install-tools:
	@echo "Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✅ Tools installed"

# Generate Go code from proto files
generate:
	@echo "Generating Go code from proto files..."
	protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		sbom.proto service.proto
	@echo "✅ Go code generated"
	@ls -lh *.pb.go

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	rm -f *.pb.go
	@echo "✅ Cleaned"

# Verify proto files
verify:
	@echo "Verifying proto files..."
	protoc --descriptor_set_out=/dev/null sbom.proto service.proto
	@echo "✅ Proto files valid"

# Help
help:
	@echo "Fortuna Proto Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all           - Generate Go code (default)"
	@echo "  generate      - Generate Go code from proto files"
	@echo "  clean         - Remove generated .pb.go files"
	@echo "  verify        - Verify proto files are valid"
	@echo "  install-tools - Install protoc plugins"
	@echo "  help          - Show this help"

