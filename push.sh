#!/usr/bin/env bash
set -euo pipefail

REPO="katasec/dstream-ingester-mssql"

# ---------------------------------------------------------------------------
# Work out the tag to push (last git tag by default)
# Allow override via env var, e.g. TAG=v0.0.25 ./push.sh
# ---------------------------------------------------------------------------
TAG="${TAG:-$(git describe --tags --abbrev=0)}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "🔧 Building cross-platform binaries …"

# ---------------------------------------------------------------------------
# Build matrix                                                                     (CGO disabled for static plugins)
# ---------------------------------------------------------------------------
targets=(
  "linux   amd64"
  "darwin  amd64"
  "darwin  arm64"
  "windows amd64"
)

for entry in "${targets[@]}"; do
  set -- $entry                # $1 = GOOS, $2 = GOARCH
  os="$1" arch="$2"
  outfile="plugin.${os}_${arch}"
  [[ $os == "windows" ]] && outfile+=".exe"

  echo "   • $outfile"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
    go build -o "${TMP_DIR}/${outfile}" ./main.go
done

# ---------------------------------------------------------------------------
# Copy manifest alongside binaries
# ---------------------------------------------------------------------------
cp ./plugin.json "${TMP_DIR}/plugin.json"

# ---------------------------------------------------------------------------
# Push as OCI artifact
# ---------------------------------------------------------------------------
cd "$TMP_DIR"

echo "📦 Pushing to ghcr.io/${REPO}:${TAG}"
oras push "ghcr.io/${REPO}:${TAG}" \
  --artifact-type "application/vnd.dstream.plugin" \
  --annotation "org.opencontainers.image.description=DStream MSSQL ingester plugin" \
  plugin.linux_amd64 \
  plugin.darwin_amd64 \
  plugin.darwin_arm64 \
  plugin.windows_amd64.exe \
  plugin.json

echo "✅ Plugin + manifest pushed: ${TAG}"
