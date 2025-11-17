#!/bin/bash

set -e

echo "Building KSAM components..."

# Build Agent
echo "Building Agent..."
cd agent
go mod download
go build -o ../bin/ksam-agent ./cmd
cd ..

# Build Core
echo "Building Core..."
cd core
go mod download
go build -o ../bin/ksam-core ./cmd
cd ..

# Build Dashboard
echo "Building Dashboard..."
cd dashboard
npm install
npm run build
cd ..

echo "Build complete! Binaries are in ./bin"

