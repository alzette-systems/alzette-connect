#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

required=(
  LICENSE
  NOTICE
  THIRD_PARTY_NOTICES.md
  packaging/README.md
  packaging/icons/alzette-connect.svg
  packaging/linux/systems.alzette.Connect.desktop
  packaging/linux/README.md
  packaging/macos/README.md
  packaging/macos/entitlements.plist
  packaging/windows/README.md
  packaging/windows/installer.nsi
  .signpath/artifact-configuration.xml
  scripts/package-download.sh
  scripts/package-macos-release.sh
  scripts/package-windows-release.sh
  .github/workflows/desktop-downloads.yml
  .github/workflows/macos-release.yml
  .github/workflows/windows-signing.yml
  docs/BUILDING.md
  docs/CODE_SIGNING_POLICY.md
  docs/PRIVACY.md
  docs/RELEASING.md
  docs/SUPPORTED_PLATFORMS.md
  docs/QA_ACCEPTANCE.md
)

for path in "${required[@]}"; do
  if [[ ! -s "$path" ]]; then
    echo "required packaging source is missing or empty: $path" >&2
    exit 1
  fi
done

for packaging_source in scripts/package-current.sh scripts/package-download.sh scripts/package-macos-release.sh scripts/package-windows-release.sh packaging/windows/installer.nsi; do
  if ! grep -Fq 'LICENSE' "$packaging_source" || ! grep -Fq 'NOTICE' "$packaging_source"; then
    echo "project license and attribution notice are not included by: $packaging_source" >&2
    exit 1
  fi
done

for packaging_source in scripts/package-current.sh scripts/package-download.sh scripts/package-macos-release.sh scripts/package-windows-release.sh packaging/windows/installer.nsi; do
  if ! grep -Fq 'THIRD_PARTY_NOTICES' "$packaging_source"; then
    echo "third-party notices are not included by: $packaging_source" >&2
    exit 1
  fi
done

for executable in scripts/package-download.sh scripts/package-macos-release.sh scripts/package-windows-release.sh; do
  if [[ ! -x "$executable" ]]; then
    echo "packaging script is not executable: $executable" >&2
    exit 1
  fi
done

bash -n scripts/package-download.sh
bash -n scripts/package-macos-release.sh
bash -n scripts/package-windows-release.sh

if ! grep -Fq "ALZETTE_CONNECT_CHATGPT_CANDIDATE: 'true'" .github/workflows/macos-release.yml; then
  echo "signed macOS acceptance releases must enable the ChatGPT adapter" >&2
  exit 1
fi

desktop="packaging/linux/systems.alzette.Connect.desktop"
for entry in \
  'Type=Application' \
  'Name=Alzette Connect' \
  'TryExec=alzette-connect' \
  'Exec=alzette-connect' \
  'Icon=systems.alzette.Connect' \
  'Terminal=false'; do
  if ! grep -Fqx "$entry" "$desktop"; then
    echo "desktop entry is missing: $entry" >&2
    exit 1
  fi
done

if grep -RIE --exclude='*.md' --exclude='check-packaging.sh' \
  '(BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|APPLE_ID_PASSWORD=|TAURI_SIGNING_PRIVATE_KEY=|PFX_PASSWORD=)' \
  packaging scripts .github 2>/dev/null; then
  echo "possible signing secret found in packaging sources" >&2
  exit 1
fi

echo "Packaging source checks passed."
