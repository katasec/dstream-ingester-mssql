#!/bin/bash
# Push script for MSSQL CDC Provider to GHCR using ORAS

set -e

TAG="${1:-$(git describe --tags --abbrev=0 2>/dev/null || echo 'v0.1.0')}"
TMP_DIR="${2:-.build}"
REPO="writeameer/dstream-ingester-mssql"
GHCR_REPO="ghcr.io/$REPO"
BINARY_NAME="dstream-ingester-mssql"

echo "📦 Pushing provider to $GHCR_REPO:$TAG…"

# Authenticate if GITHUB_TOKEN is set
if [ -n "$GITHUB_TOKEN" ]; then
    echo "🔐 Authenticating with GitHub Container Registry…"
    echo "$GITHUB_TOKEN" | /usr/local/bin/oras login ghcr.io --username writeameer --password-stdin
else
    echo "⚠️  GITHUB_TOKEN not set. Attempting unauthenticated push."
fi

# Push OCI artifact with ORAS
/usr/local/bin/oras push "$GHCR_REPO:$TAG" \
    --artifact-type "application/vnd.dstream.provider" \
    --annotation "org.opencontainers.image.description=DStream SQL Server CDC provider" \
    --annotation "org.opencontainers.image.source=https://github.com/katasec/dstream-ingester-mssql" \
    --annotation "org.opencontainers.image.version=$TAG" \
    "$TMP_DIR/${BINARY_NAME}.linux_amd64" \
    "$TMP_DIR/${BINARY_NAME}.linux_arm64" \
    "$TMP_DIR/${BINARY_NAME}.darwin_amd64" \
    "$TMP_DIR/${BINARY_NAME}.darwin_arm64" \
    "$TMP_DIR/${BINARY_NAME}.windows_amd64.exe" \
    "$TMP_DIR/provider.json"

echo "✅ Provider pushed: $TAG"
echo "📍 Available at: $GHCR_REPO:$TAG"
