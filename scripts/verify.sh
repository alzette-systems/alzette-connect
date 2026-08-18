#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go_build_tags=""
if [[ "$(uname -s)" == "Linux" ]] && command -v pkg-config >/dev/null 2>&1 && \
  ! pkg-config --exists gtk4 webkitgtk-6.0 && \
  pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
  echo "Using Wails' GTK3 compatibility tag for Go verification on this host."
  go_build_tags="gtk3"
fi

for script in scripts/*.sh; do
  bash -n "$script"
done
scripts/check-packaging.sh

if [[ -f frontend/package.json ]]; then
  if ! command -v wails3 >/dev/null 2>&1; then
    echo "wails3 is required to generate frontend bindings; run scripts/install-wails.sh" >&2
    exit 1
  fi
  if [[ -n "$go_build_tags" ]]; then
    wails3 generate bindings -f "-tags $go_build_tags"
  else
    wails3 generate bindings
  fi
  if [[ ! -f frontend/package-lock.json ]]; then
    echo "frontend/package-lock.json is required for reproducible verification" >&2
    exit 1
  fi
  npm --prefix frontend ci
  if node -e 'const p=require("./frontend/package.json"); process.exit(p.scripts && p.scripts.test ? 0 : 1)'; then
    npm --prefix frontend test
  fi
fi

if [[ -n "$go_build_tags" ]]; then
  go test -tags "$go_build_tags" ./...
  go test -race -tags "$go_build_tags" ./...
  go vet -tags "$go_build_tags" ./...
else
  go test ./...
  go test -race ./...
  go vet ./...
fi
