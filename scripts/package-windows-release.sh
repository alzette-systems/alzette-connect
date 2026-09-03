#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
source scripts/use-go-toolchain.sh

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) ;;
  *) echo "Windows release packaging must run on Windows" >&2; exit 1 ;;
esac

if [[ $# -ne 1 || "$1" != "--from-signed-build" ]]; then
  echo "usage: scripts/package-windows-release.sh --from-signed-build" >&2
  exit 2
fi

version="${ALZETTE_CONNECT_VERSION:-}"
version="${version#connect-v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "a stable semantic version is required: ALZETTE_CONNECT_VERSION=$version" >&2
  exit 2
fi

binary="bin/alzette-connect.exe"
if [[ ! -f "$binary" ]]; then
  echo "signed Windows application binary is missing: $binary" >&2
  exit 1
fi

signed_status="$(powershell.exe -NoProfile -NonInteractive -Command \
  "(Get-AuthenticodeSignature -LiteralPath '$(cygpath -w "$repo_root/$binary")').Status")"
if [[ "${signed_status//$'\r'/}" != "Valid" ]]; then
  echo "Windows application binary does not have a valid Authenticode signature" >&2
  exit 1
fi

arch="$(go env GOARCH)"
case "$arch" in
  amd64) package_arch="x64" ;;
  *) echo "unsupported Windows release architecture: $arch" >&2; exit 1 ;;
esac

command -v wails3 >/dev/null 2>&1 || {
  echo "wails3 is required; run scripts/install-wails.sh first" >&2
  exit 1
}

makensis_bin="$(command -v makensis || command -v makensis.exe || true)"
if [[ -z "$makensis_bin" ]]; then
  for candidate in \
    "/c/Program Files (x86)/NSIS/makensis.exe" \
    "/c/Program Files/NSIS/makensis.exe"; do
    if [[ -x "$candidate" ]]; then
      makensis_bin="$candidate"
      break
    fi
  done
fi
if [[ -z "$makensis_bin" ]]; then
  echo "NSIS makensis is required to create the Windows installer" >&2
  exit 1
fi

stage="$(mktemp -d "${TMPDIR:-/tmp}/alzette-connect-windows-release.XXXXXX")"
trap 'rm -rf -- "$stage"' EXIT
mkdir -p dist

wails3 generate icons -input build/appicon.png \
  -windowsfilename "$stage/alzette-connect.ico"

output="$repo_root/dist/Alzette-Connect-$version-windows-$package_arch.exe"
"$makensis_bin" \
  -DAPP_VERSION="$version" \
  -DAPP_BINARY="$(cygpath -w "$repo_root/$binary")" \
  -DAPP_ICON="$(cygpath -w "$stage/alzette-connect.ico")" \
  -DLICENSE_FILE="$(cygpath -w "$repo_root/LICENSE")" \
  -DATTRIBUTION_NOTICE_FILE="$(cygpath -w "$repo_root/NOTICE")" \
  -DTHIRD_PARTY_NOTICES_FILE="$(cygpath -w "$repo_root/THIRD_PARTY_NOTICES.md")" \
  -DOUTPUT_FILE="$(cygpath -w "$output")" \
  packaging/windows/installer.nsi

echo "Created unsigned signing input $output"
