#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
VERSION=${VERSION:-dev}
[[ "$VERSION" == dev || "$VERSION" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { echo '版本号无效。' >&2; exit 1; }
mkdir -p dist
for ARCH in amd64 arm64; do
  DEST="dist/prismflow-linux-$ARCH"
  mkdir -p "$DEST/scripts" "$DEST/deploy" "$DEST/third_party/mihomo" "$DEST/docs/screenshots" "$DEST/web/public"
  CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -trimpath -ldflags="-s -w -X main.version=${VERSION#v}" -o "$DEST/prismflow" ./cmd/prismflow
  chmod 0755 "$DEST/prismflow"
  cp scripts/install.sh scripts/uninstall.sh scripts/update.sh scripts/release-common.sh "$DEST/scripts/"
  cp deploy/prismflow.service deploy/prismflow-update.service deploy/prismflow-update.path deploy/prismflow-update.timer deploy/Caddyfile.example "$DEST/deploy/"
  cp web/public/logo.svg "$DEST/web/public/"
  cp README.md LICENSE THIRD_PARTY.md "$DEST/"
  printf '%s\n' "$VERSION" > "$DEST/VERSION"
  cp third_party/dependencies.txt "$DEST/third_party/"
  cp third_party/mihomo/LICENSE "$DEST/third_party/mihomo/"
  cp docs/PROTOCOLS.md docs/VERIFICATION.md docs/USAGE.md docs/UPDATES.md "$DEST/docs/"
  cp docs/screenshots/*.png "$DEST/docs/screenshots/"
  tar -C dist -czf "$DEST.tar.gz" "prismflow-linux-$ARCH"
done
(cd dist && sha256sum *.tar.gz > SHA256SUMS)
