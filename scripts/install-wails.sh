#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

module="github.com/wailsapp/wails/v3"
version="$(go list -m -f '{{.Version}}' "$module")"
if [[ -z "$version" || "$version" == "<no value>" ]]; then
  echo "Wails v3 is not selected in go.mod" >&2
  exit 1
fi

echo "Installing Wails CLI from $module@$version"
go_install_args=()
if [[ "$(uname -s)" == "Linux" ]] && command -v pkg-config >/dev/null 2>&1 && \
  ! pkg-config --exists gtk4 webkitgtk-6.0 && \
  pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
  echo "Using Wails' GTK3 compatibility tag on this development host."
  go_install_args=(-tags gtk3)
fi
go install "${go_install_args[@]}" "$module/cmd/wails3@$version"

wails_bin_dir="$(go env GOPATH)/bin"
if [[ -n "${GITHUB_PATH:-}" ]]; then
  printf '%s\n' "$wails_bin_dir" >> "$GITHUB_PATH"
fi
echo "Installed wails3 in $wails_bin_dir"
