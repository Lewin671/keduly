#!/bin/sh
# Runs every check. It must pass before a commit.
set -eu
cd "$(dirname "$0")/.."

unformatted=$(gofmt -l cmd internal skills)
if [ -n "$unformatted" ]; then
  echo "gofmt needed on:" >&2
  echo "$unformatted" >&2
  exit 1
fi

go vet ./...
go test ./...

if [ -f web/package.json ] && command -v pnpm >/dev/null 2>&1; then
  pnpm -C web install --frozen-lockfile
  pnpm -C web run check
fi

echo "all checks passed"
