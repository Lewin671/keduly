#!/bin/sh
# Builds the release archives into dist/: one keduly-OS-ARCH.tar.gz per platform
# and checksums.txt. The release workflow uploads them; install.sh downloads them.
set -eu
cd "$(dirname "$0")/.."

rm -rf dist
mkdir -p dist
skip_web=
for target in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do
  stage=dist/keduly-$target
  GOOS=${target%-*} GOARCH=${target#*-} OUT=$stage/keduly SKIP_WEB=$skip_web scripts/build.sh
  skip_web=1
  cp LICENSE "$stage/"
  tar -czf "$stage.tar.gz" -C "$stage" keduly LICENSE
  rm -r "$stage"
done

cd dist
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum keduly-*.tar.gz > checksums.txt
else
  shasum -a 256 keduly-*.tar.gz > checksums.txt
fi
cat checksums.txt
