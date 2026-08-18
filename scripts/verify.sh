#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go_test_args=()
if [[ "$(uname -s)" == "Linux" ]] && command -v pkg-config >/dev/null 2>&1 && \
  ! pkg-config --exists gtk4 webkitgtk-6.0 && \
  pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
  echo "Using Wails' GTK3 compatibility tag for Go verification on this host."
  go_test_args=(-tags gtk3)
fi

for script in scripts/*.sh; do
  bash -n "$script"
done
scripts/check-packaging.sh

if [[ -f frontend/package.json ]]; then
  if [[ ! -f frontend/package-lock.json ]]; then
    echo "frontend/package-lock.json is required for reproducible verification" >&2
    exit 1
  fi
  npm --prefix frontend ci
  if node -e 'const p=require("./frontend/package.json"); process.exit(p.scripts && p.scripts.test ? 0 : 1)'; then
    npm --prefix frontend test
  fi
fi

go test "${go_test_args[@]}" ./...
go test -race "${go_test_args[@]}" ./...
go vet "${go_test_args[@]}" ./...
