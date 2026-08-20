#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
source scripts/use-go-toolchain.sh

case "$(uname -s)" in
  Linux*) target="linux" ;;
  Darwin*) target="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) target="windows" ;;
  *) echo "unsupported packaging host: $(uname -s)" >&2; exit 1 ;;
esac

case "$#:${1:-}" in
  0:) scripts/build-smoke.sh ;;
  1:--from-task) ;;
  *) echo "usage: scripts/package-current.sh [--from-task]" >&2; exit 2 ;;
esac

binary="bin/alzette-connect"
if [[ "$target" == "windows" ]]; then
  binary="${binary}.exe"
fi
if [[ ! -f "$binary" ]]; then
  echo "host binary is missing after build: $binary" >&2
  exit 1
fi

arch="$(go env GOARCH)"
stage="$(mktemp -d "${TMPDIR:-/tmp}/alzette-connect-package.XXXXXX")"
trap 'rm -rf -- "$stage"' EXIT
payload="$stage/alzette-connect-$target-$arch"
mkdir -p "$payload/bin"
cp "$binary" "$payload/bin/"
cp THIRD_PARTY_NOTICES.md "$payload/THIRD_PARTY_NOTICES.md"

if [[ "$target" == "linux" ]]; then
  mkdir -p "$payload/share/applications" "$payload/share/icons/hicolor/scalable/apps"
  cp packaging/linux/systems.alzette.Connect.desktop "$payload/share/applications/"
  cp packaging/icons/alzette-connect.svg \
    "$payload/share/icons/hicolor/scalable/apps/systems.alzette.Connect.svg"
fi

cat > "$payload/UNSIGNED-CI.txt" <<'EOF'
UNSIGNED development archive

This archive is for packaging smoke tests only. It is not signed, notarized,
stapled, an OS-native installer, or approved for customer distribution.
EOF

mkdir -p dist
archive="dist/alzette-connect-$target-$arch-unsigned-ci.tar.gz"
tar -C "$stage" -czf "$archive" "$(basename "$payload")"
echo "Created $archive"
