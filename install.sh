#!/bin/sh
# Installs the keduly command from a GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/Lewin671/keduly/main/install.sh | sh
#
# KEDULY_VERSION      release to install, e.g. v0.1.0 (default: the latest)
# KEDULY_BASE_URL     where the archives are, for a mirror of the release files
# KEDULY_INSTALL_DIR  where the command goes (default: ~/.local/bin)
#
# The agent skill is a separate step, because where it belongs is yours to
# choose: your home directory or one project, .agents/skills or Claude Code's.
set -eu

repo=Lewin671/keduly

fail() {
  echo "install.sh: $*" >&2
  exit 1
}

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    fail "curl or wget is required"
  fi
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  else
    fail "sha256sum or shasum is required"
  fi
}

# Everything runs from main, called on the last line, so that a download cut
# short cannot run half a script.
main() {
  case $(uname -s) in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) fail "unsupported system $(uname -s); build from source instead" ;;
  esac
  case $(uname -m) in
    arm64 | aarch64) arch=arm64 ;;
    x86_64 | amd64) arch=amd64 ;;
    *) fail "unsupported processor $(uname -m); build from source instead" ;;
  esac

  archive=keduly-$os-$arch.tar.gz
  version=${KEDULY_VERSION:-}
  if [ -n "${KEDULY_BASE_URL:-}" ]; then
    base=${KEDULY_BASE_URL%/}
  elif [ -n "$version" ]; then
    base=https://github.com/$repo/releases/download/$version
  else
    base=https://github.com/$repo/releases/latest/download
  fi
  dir=${KEDULY_INSTALL_DIR:-$HOME/.local/bin}

  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT

  echo "Downloading $base/$archive"
  fetch "$base/$archive" "$tmp/$archive" || fail "cannot download $base/$archive"
  fetch "$base/checksums.txt" "$tmp/checksums.txt" || fail "cannot download $base/checksums.txt"

  want=$(awk -v name="$archive" '$2 == name || $2 == "*" name { print $1 }' "$tmp/checksums.txt")
  [ -n "$want" ] || fail "checksums.txt does not list $archive"
  got=$(sha256 "$tmp/$archive")
  [ "$want" = "$got" ] || fail "checksum mismatch for $archive: expected $want, got $got"

  tar -xzf "$tmp/$archive" -C "$tmp" keduly
  mkdir -p "$dir"
  # Moving into place, rather than writing over, keeps a running keduly intact.
  mv -f "$tmp/keduly" "$dir/keduly.new"
  chmod 755 "$dir/keduly.new"
  mv -f "$dir/keduly.new" "$dir/keduly"
  echo "Installed $("$dir/keduly" version) to $dir/keduly"

  case ":$PATH:" in
    *":$dir:"*) ;;
    *) echo "$dir is not on your PATH. Add it, for example: export PATH=\"$dir:\$PATH\"" ;;
  esac
  cat <<'NEXT'

Next:
  keduly login --server https://your-keduly-server    sign in with a token from Settings
  keduly skill install                                the agent skill, into ~/.agents/skills
  keduly skill install --claude                       or into ~/.claude/skills, where Claude Code looks
                                                      add --project to install into the current project
NEXT
}

main "$@"
