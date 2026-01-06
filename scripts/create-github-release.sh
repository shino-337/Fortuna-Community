#!/bin/bash
# Script to create GitHub release for v1.0.0

set -e

TAG="v1.0.0"
REPO="shino-337/KSAM"
NOTES_FILE="RELEASE_NOTES_v1.0.0.md"

echo "🚀 Creating GitHub Release for $TAG..."

if command -v gh &> /dev/null; then
    if gh auth status &> /dev/null; then
        echo "✅ GitHub CLI authenticated - creating release..."
        gh release create "$TAG" \
            --title "Fortuna Platform $TAG" \
            --notes-file "$NOTES_FILE"
        echo "✅ Release created successfully!"
        echo ""
        echo "View release at: https://github.com/$REPO/releases/tag/$TAG"
    else
        echo "⚠️  GitHub CLI not authenticated"
        echo "Run: gh auth login"
    fi
else
    echo "⚠️  GitHub CLI not found"
    echo ""
    echo "Please create release manually:"
    echo "1. Go to: https://github.com/$REPO/releases/new"
    echo "2. Select tag: $TAG"
    echo "3. Title: Fortuna Platform $TAG"
    echo "4. Copy content from: $NOTES_FILE"
    echo "5. Publish"
fi
