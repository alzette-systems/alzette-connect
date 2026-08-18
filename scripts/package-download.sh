#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

version="${ALZETTE_CONNECT_VERSION:-0.1.0-demo}"
version="${version#connect-v}"
if [[ ! "$version" =~ ^[0-9][0-9A-Za-z._+-]*$ ]]; then
  echo "invalid package version: $version" >&2
  exit 2
fi

case "$(uname -s)" in
  Linux*) target="linux" ;;
  Darwin*) target="macos" ;;
  MINGW*|MSYS*|CYGWIN*) target="windows" ;;
  *) echo "unsupported packaging host: $(uname -s)" >&2; exit 1 ;;
esac

case "$#:${1:-}" in
  0:) scripts/build-smoke.sh ;;
  1:--from-build) ;;
  *) echo "usage: scripts/package-download.sh [--from-build]" >&2; exit 2 ;;
esac

arch="$(go env GOARCH)"
case "$arch" in
  amd64) package_arch="x64"; deb_arch="amd64" ;;
  arm64) package_arch="arm64"; deb_arch="arm64" ;;
  *) echo "unsupported package architecture: $arch" >&2; exit 1 ;;
esac

binary="bin/alzette-connect"
if [[ "$target" == "windows" ]]; then binary="${binary}.exe"; fi
if [[ ! -f "$binary" ]]; then
  echo "host binary is missing after build: $binary" >&2
  exit 1
fi

mkdir -p dist
stage="$(mktemp -d "${TMPDIR:-/tmp}/alzette-connect-download.XXXXXX")"
trap 'rm -rf -- "$stage"' EXIT

notice="$stage/UNSIGNED-DEMO.txt"
cat > "$notice" <<EOF
Alzette Connect $version — unsigned demo build

Source commit: ${GITHUB_SHA:-$(git rev-parse HEAD)}
Target: $target/$package_arch

This build is intended for internal demonstration and acceptance testing.
It has not completed Alzette's production signing, notarization, clean-machine,
or customer release gates. Do not redistribute it as a production release.
EOF

generate_icons() {
  command -v wails3 >/dev/null 2>&1 || {
    echo "wails3 is required; run scripts/install-wails.sh first" >&2
    exit 1
  }
  wails3 generate icons -input build/appicon.png \
    -windowsfilename "$stage/alzette-connect.ico" \
    -macfilename "$stage/icons.icns"
}

package_macos() {
  generate_icons
  app="$stage/Alzette Connect.app"
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
  cp "$binary" "$app/Contents/MacOS/alzette-connect"
  chmod 0755 "$app/Contents/MacOS/alzette-connect"
  cp build/darwin/Info.plist "$app/Contents/Info.plist"
  cp "$stage/icons.icns" "$app/Contents/Resources/icons.icns"
  cp "$notice" "$app/Contents/Resources/UNSIGNED-DEMO.txt"
  marketing_version="${version%%-*}"
  marketing_version="${marketing_version%%+*}"
  bundle_version="${GITHUB_RUN_NUMBER:-$marketing_version}"
  /usr/libexec/PlistBuddy -c "Set :CFBundleVersion $bundle_version" "$app/Contents/Info.plist"
  /usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $marketing_version" "$app/Contents/Info.plist"
  codesign --force --deep --sign - "$app"
  output="dist/Alzette-Connect-$version-macOS-$package_arch-unsigned-demo.zip"
  ditto -c -k --sequesterRsrc --keepParent "$app" "$output"
}

package_windows() {
  generate_icons
  makensis_bin="$(command -v makensis || command -v makensis.exe || true)"
  if [[ -z "$makensis_bin" ]]; then
    echo "NSIS makensis is required to create the Windows installer" >&2
    exit 1
  fi
  output="$repo_root/dist/Alzette-Connect-$version-windows-$package_arch-unsigned-demo.exe"
  "$makensis_bin" \
    -DAPP_VERSION="$version" \
    -DAPP_BINARY="$(cygpath -w "$repo_root/$binary")" \
    -DAPP_ICON="$(cygpath -w "$stage/alzette-connect.ico")" \
    -DNOTICE_FILE="$(cygpath -w "$notice")" \
    -DOUTPUT_FILE="$(cygpath -w "$output")" \
    packaging/windows/installer.nsi
}

package_linux() {
  command -v dpkg-deb >/dev/null 2>&1 || {
    echo "dpkg-deb is required to create the Linux package" >&2
    exit 1
  }
  deb_version="${version//+/.}"
  deb_version="${deb_version//_/.}"
  root="$stage/deb"
  mkdir -p "$root/DEBIAN" "$root/usr/bin" \
    "$root/usr/share/applications" \
    "$root/usr/share/icons/hicolor/1024x1024/apps" \
    "$root/usr/share/doc/alzette-connect"
  install -m 0755 "$binary" "$root/usr/bin/alzette-connect"
  install -m 0644 packaging/linux/systems.alzette.Connect.desktop \
    "$root/usr/share/applications/systems.alzette.Connect.desktop"
  install -m 0644 build/appicon.png \
    "$root/usr/share/icons/hicolor/1024x1024/apps/systems.alzette.Connect.png"
  install -m 0644 "$notice" "$root/usr/share/doc/alzette-connect/UNSIGNED-DEMO.txt"
  cat > "$root/DEBIAN/control" <<EOF
Package: alzette-connect
Version: $deb_version
Section: net
Priority: optional
Architecture: $deb_arch
Maintainer: Alzette Systems <engineering@alzette.systems>
Depends: libgtk-4-1, libwebkitgtk-6.0-4, libsecret-tools
Homepage: https://alzette.systems
Description: Connect desktop AI clients to company-authorised Alzette models
 Alzette Connect signs employees in through their browser and configures
 supported desktop clients without exposing a permanent API key.
EOF
  output="dist/Alzette-Connect-$version-linux-$package_arch-unsigned-demo.deb"
  dpkg-deb --root-owner-group --build "$root" "$output"
}

case "$target" in
  macos) package_macos ;;
  windows) package_windows ;;
  linux) package_linux ;;
esac

sha_file="${output}.sha256"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$(dirname "$output")" && sha256sum "$(basename "$output")" > "$(basename "$sha_file")")
else
  (cd "$(dirname "$output")" && shasum -a 256 "$(basename "$output")" > "$(basename "$sha_file")")
fi
echo "Created $output"
echo "Created $sha_file"
