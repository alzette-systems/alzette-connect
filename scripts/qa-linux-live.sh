#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
source scripts/use-go-toolchain.sh

if [[ "${ALZETTE_CONNECT_LIVE_QA:-}" != "1" ]]; then
  echo "Set ALZETTE_CONNECT_LIVE_QA=1 to approve real Casdoor login and paid model requests." >&2
  exit 2
fi
for required in \
  ALZETTE_CONNECT_LIVE_QA_USERNAME \
  ALZETTE_CONNECT_LIVE_QA_PASSWORD \
  ALZETTE_CONNECT_LIVE_QA_MODELS; do
  if [[ -z "${!required:-}" ]]; then
    echo "$required is required." >&2
    exit 2
  fi
done
for command in dbus-run-session gnome-keyring-daemon secret-tool; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "$command is required for isolated Linux Secret Service acceptance." >&2
    exit 1
  fi
done

qa_state="$(mktemp -d /tmp/alzette-connect-linux-qa.XXXXXX)"
chmod 0700 "$qa_state"
cleanup() {
  rm -rf -- "$qa_state"
}
trap cleanup EXIT

export ALZETTE_CONNECT_LIVE_QA_CONTROL_URL="${ALZETTE_CONNECT_LIVE_QA_CONTROL_URL:-https://app.alzette.systems}"
export XDG_DATA_HOME="$qa_state/data"
export XDG_CONFIG_HOME="$qa_state/config"
export XDG_CACHE_HOME="$qa_state/cache"
export XDG_RUNTIME_DIR="$qa_state/runtime"
mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME" "$XDG_RUNTIME_DIR"
chmod 0700 "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME" "$XDG_RUNTIME_DIR"

source_state="clean"
if ! git diff --quiet || ! git diff --cached --quiet || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
  source_state="dirty"
fi
printf 'Alzette Connect live QA: commit=%s version=%s source=%s platform=%s/%s\n' \
  "$(git rev-parse HEAD)" "$(git describe --tags --always)" "$source_state" "$(uname -s)" "$(uname -m)"

env -u ALZETTE_CONNECT_LIVE_QA scripts/verify.sh
dbus-run-session -- bash -c '
  set -euo pipefail
  keyring_password="$(openssl rand -base64 32)"
  eval "$(printf "%s" "$keyring_password" | gnome-keyring-daemon --unlock --components=secrets)"
  keyring_password=""
  go test ./internal/clientconfig -run "^TestLiveLinuxConnectAcceptance$" -count=1 -v
'
