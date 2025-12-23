#!/bin/bash
#
# migrate-imports.sh
# Updates all import paths from KSAM to Fortuna naming
#

set -e

PROJECT_ROOT="/Users/tuatnh/Desktop/Learn/K8s Service Account Management Platform/KSAM"
cd "$PROJECT_ROOT"

echo "🔄 Starting import migration..."
echo "================================"

# Step 1: Replace example.com/fortunaproto → github.com/fortuna/api/proto/agent
echo "📦 Step 1: Updating proto imports..."
find . -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" -not -path "*/api/proto/agent/*.pb.go" -type f -exec sed -i '' \
  -e 's|import pb "example.com/fortunaproto"|import pb "github.com/fortuna/api/proto/agent"|g' \
  -e 's|import fortuna "example.com/fortunaproto"|import fortuna "github.com/fortuna/api/proto/agent"|g' \
  -e 's|import agentpb "example.com/fortunaproto"|import agentpb "github.com/fortuna/api/proto/agent"|g' \
  -e 's|"example.com/fortunaproto"|"github.com/fortuna/api/proto/agent"|g' \
  {} \;

echo "✅ Proto imports updated"

# Step 2: Replace github.com/ksam/agent → github.com/fortuna/agent
echo "📦 Step 2: Updating agent imports..."
find . -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" -type f -exec sed -i '' \
  's|"github.com/ksam/agent|"github.com/fortuna/agent|g' \
  {} \;

echo "✅ Agent imports updated"

# Step 3: Replace github.com/ksam/core → github.com/fortuna/core
echo "📦 Step 3: Updating core imports..."
find . -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" -type f -exec sed -i '' \
  's|"github.com/ksam/core|"github.com/fortuna/core|g' \
  {} \;

echo "✅ Core imports updated"

echo "================================"
echo "🎉 Import migration complete!"
echo ""
echo "📊 Summary:"
echo "  - Proto imports: example.com/fortunaproto → github.com/fortuna/api/proto/agent"
echo "  - Agent imports: github.com/ksam/agent → github.com/fortuna/agent"
echo "  - Core imports: github.com/ksam/core → github.com/fortuna/core"
echo ""
echo "⚠️  Next steps:"
echo "  1. Run: cd agent && go mod tidy"
echo "  2. Run: cd core && go mod tidy"
echo "  3. Run: cd api && go mod tidy"
echo "  4. Verify: go build ./..."
