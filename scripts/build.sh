#!/bin/sh
# Builds the release binary bin/keduly with the web app embedded.
# GOOS and GOARCH are honoured, e.g. GOOS=linux GOARCH=amd64 scripts/build.sh
# OUT names another output file; SKIP_WEB=1 reuses the web app already built.
set -eu
cd "$(dirname "$0")/.."

version=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
out=${OUT:-bin/keduly}

if [ -f web/package.json ] && [ -z "${SKIP_WEB:-}" ]; then
  pnpm -C web install --frozen-lockfile
  pnpm -C web run build
  # Copy the build over unless the web app already writes into the embed directory.
  if [ -d web/dist ]; then
    find internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
    cp -R web/dist/. internal/webui/dist/
  fi
fi

mkdir -p "$(dirname "$out")"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "$out" ./cmd/keduly
echo "built $out ${version}"
