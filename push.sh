#!/usr/bin/env bash
set -euo pipefail

REPO="katasec/dstream-ingester-mssql"
ARCHIVE_TAG=$(git describe --tags --abbrev=0)

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "🔧 Building cross-platform binaries..."

# Linux
GOOS=linux GOARCH=amd64 go build -o "$TMP_DIR/plugin.linux_amd64" ./main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o "$TMP_DIR/plugin.windows_amd64.exe" ./main.go

# Manifest
cp ./plugin.json "$TMP_DIR/plugin.json"

# Switch to relative paths
cd "$TMP_DIR"

echo "📦 Pushing to ghcr.io/$REPO:$ARCHIVE_TAG"
oras push "ghcr.io/$REPO:$ARCHIVE_TAG" \
  --artifact-type application/vnd.dstream.plugin \
  --annotation "org.opencontainers.image.description=Time ingester plugin" \
  plugin.linux_amd64 \
  plugin.windows_amd64.exe \
  plugin.json

echo "✅ Plugin + manifest pushed: $ARCHIVE_TAG"
