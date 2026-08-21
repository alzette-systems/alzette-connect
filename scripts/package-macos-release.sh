#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
source scripts/use-go-toolchain.sh

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "signed macOS packaging must run on macOS" >&2
  exit 1
fi

version="${ALZETTE_CONNECT_VERSION:-}"
version="${version#connect-v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "a stable semantic version is required: ALZETTE_CONNECT_VERSION=$version" >&2
  exit 2
fi

: "${MACOS_SIGNING_IDENTITY:?MACOS_SIGNING_IDENTITY is required}"
: "${APP_STORE_CONNECT_API_KEY_PATH:?APP_STORE_CONNECT_API_KEY_PATH is required}"
: "${APP_STORE_CONNECT_KEY_ID:?APP_STORE_CONNECT_KEY_ID is required}"
: "${APP_STORE_CONNECT_ISSUER_ID:?APP_STORE_CONNECT_ISSUER_ID is required}"

if [[ $# -ne 1 || "$1" != "--from-build" ]]; then
  echo "usage: scripts/package-macos-release.sh --from-build" >&2
  exit 2
fi

binary="bin/alzette-connect"
if [[ ! -x "$binary" ]]; then
  echo "macOS application binary is missing after build: $binary" >&2
  exit 1
fi

arch="$(go env GOARCH)"
case "$arch" in
  amd64) package_arch="x64" ;;
  arm64) package_arch="arm64" ;;
  *) echo "unsupported macOS architecture: $arch" >&2; exit 1 ;;
esac

stage="$(mktemp -d "${TMPDIR:-/tmp}/alzette-connect-release.XXXXXX")"
trap 'rm -rf -- "$stage"' EXIT
mkdir -p dist

wails3 generate icons -input build/appicon.png \
  -macfilename "$stage/icons.icns"

app="$stage/Alzette Connect.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
install -m 0755 "$binary" "$app/Contents/MacOS/alzette-connect"
install -m 0644 build/darwin/Info.plist "$app/Contents/Info.plist"
install -m 0644 "$stage/icons.icns" "$app/Contents/Resources/icons.icns"
install -m 0644 THIRD_PARTY_NOTICES.md \
  "$app/Contents/Resources/THIRD_PARTY_NOTICES.md"

bundle_version="${GITHUB_RUN_NUMBER:-1}"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $bundle_version" \
  "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $version" \
  "$app/Contents/Info.plist"

codesign \
  --force \
  --options runtime \
  --timestamp \
  --entitlements packaging/macos/entitlements.plist \
  --sign "$MACOS_SIGNING_IDENTITY" \
  "$app"

codesign --verify --deep --strict --verbose=2 "$app"
codesign -dvv "$app"

notarization_zip="$stage/Alzette-Connect-notarization.zip"
ditto -c -k --sequesterRsrc --keepParent "$app" "$notarization_zip"

notarization_result="dist/Alzette-Connect-$version-macOS-$package_arch.notarization.json"
xcrun notarytool submit "$notarization_zip" \
  --key "$APP_STORE_CONNECT_API_KEY_PATH" \
  --key-id "$APP_STORE_CONNECT_KEY_ID" \
  --issuer "$APP_STORE_CONNECT_ISSUER_ID" \
  --wait \
  --output-format json | tee "$notarization_result"

notarization_status="$(plutil -extract status raw "$notarization_result")"
if [[ "$notarization_status" != "Accepted" ]]; then
  echo "Apple notarization was not accepted: $notarization_status" >&2
  exit 1
fi

xcrun stapler staple "$app"
xcrun stapler validate "$app"
spctl --assess --type execute --verbose=4 "$app"

output="dist/Alzette-Connect-$version-macOS-$package_arch.zip"
ditto -c -k --sequesterRsrc --keepParent "$app" "$output"
(cd dist && shasum -a 256 "$(basename "$output")" > "$(basename "$output").sha256")

echo "Created $output"
echo "Created ${output}.sha256"
echo "Created $notarization_result"
