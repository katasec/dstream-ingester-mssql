#!/bin/bash
# Build script for MSSQL CDC Provider
# Compiles cross-platform Go binaries for all platforms
# Binaries are named as plugin.* for DStream OCI artifact compatibility

set -e

BINARY_NAME="plugin"
TMP_DIR="${1:-.build}"
PLATFORMS="linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 windows_amd64"

echo "🔧 Building cross-platform Go binaries…"
mkdir -p "$TMP_DIR"

for platform in $PLATFORMS; do
    os=$(echo "$platform" | cut -d_ -f1)
    arch=$(echo "$platform" | cut -d_ -f2)
    
    outfile="$TMP_DIR/${BINARY_NAME}.${platform}"
    if [ "$os" = "windows" ]; then
        outfile="${outfile}.exe"
    fi
    
    echo "   • $outfile"
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -o "$outfile" -ldflags="-s -w"
done

echo "✅ Binaries built in $TMP_DIR"
