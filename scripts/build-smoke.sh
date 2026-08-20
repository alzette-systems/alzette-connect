#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
source scripts/use-go-toolchain.sh
export PATH="$(go env GOPATH)/bin:$PATH"

connect_version="${ALZETTE_CONNECT_VERSION:-0.2.0-demo.1}"
connect_version="${connect_version#connect-v}"
if [[ ! "$connect_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid application version: $connect_version" >&2
  exit 2
fi
export ALZETTE_CONNECT_VERSION="$connect_version"

find_wails() {
  if command -v wails3 >/dev/null 2>&1; then
    command -v wails3
    return
  fi
  local suffix=""
  if [[ "${OS:-}" == "Windows_NT" ]]; then
    suffix=".exe"
  fi
  local candidate
  candidate="$(go env GOPATH)/bin/wails3${suffix}"
  if [[ -x "$candidate" ]]; then
    printf '%s\n' "$candidate"
    return
  fi
  echo "wails3 was not found; run scripts/install-wails.sh" >&2
  exit 1
}

wails_bin="$(find_wails)"
arguments=(build)
if [[ "$(uname -s)" == "Linux" ]]; then
  if [[ -n "${WAILS_BUILD_TAGS:-}" ]]; then
    arguments+=("-tags" "$WAILS_BUILD_TAGS")
  elif command -v pkg-config >/dev/null 2>&1 && \
    ! pkg-config --exists gtk4 webkitgtk-6.0 && \
    pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
    echo "Using Wails' GTK3 compatibility build on this development host."
    arguments+=("-tags" "gtk3")
  fi
fi
"$wails_bin" "${arguments[@]}"
