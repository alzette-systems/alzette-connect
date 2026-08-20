#!/usr/bin/env bash

# Source this helper before invoking Go. Older Go releases reject the modern
# go directive before they can offer a useful toolchain error, so prefer an
# already-installed Go 1.25 binary and otherwise fail with one clear action.
alzette_connect_go_supported() {
  local binary="$1" version major minor
  version="$($binary version 2>/dev/null)" || return 1
  version="${version#* go}"
  version="${version%% *}"
  version="${version#go}"
  major="${version%%.*}"
  minor="${version#*.}"
  minor="${minor%%.*}"
  [[ "$major" =~ ^[0-9]+$ && "$minor" =~ ^[0-9]+$ ]] || return 1
  (( major > 1 || major == 1 && minor >= 25 ))
}

alzette_connect_go="$(command -v go 2>/dev/null || true)"
if [[ -z "$alzette_connect_go" ]] || ! alzette_connect_go_supported "$alzette_connect_go"; then
  for alzette_connect_candidate in /usr/local/go/bin/go /opt/homebrew/bin/go /usr/local/bin/go; do
    if [[ -x "$alzette_connect_candidate" ]] && alzette_connect_go_supported "$alzette_connect_candidate"; then
      export PATH="$(dirname "$alzette_connect_candidate"):$PATH"
      alzette_connect_go="$alzette_connect_candidate"
      break
    fi
  done
fi
if [[ -z "$alzette_connect_go" ]] || ! alzette_connect_go_supported "$alzette_connect_go"; then
  echo "Alzette Connect requires Go 1.25 or newer; install it or put it first on PATH." >&2
  return 1 2>/dev/null || exit 1
fi
unset alzette_connect_go alzette_connect_candidate
