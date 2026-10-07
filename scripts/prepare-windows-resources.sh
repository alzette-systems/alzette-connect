#!/usr/bin/env bash
set -euo pipefail

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) ;;
  *) exit 0 ;;
esac

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
source scripts/use-go-toolchain.sh

version="${1:-${ALZETTE_CONNECT_VERSION:-0.2.0-demo.1}}"
version="${version#connect-v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid Windows application version: $version" >&2
  exit 2
fi

stage="packaging/generated/windows"
mkdir -p "$stage"
node --input-type=module - "$version" "$stage" <<'NODE'
import fs from "node:fs";
import path from "node:path";
const [version, stage] = process.argv.slice(2);
const numeric = `${version.split("-")[0]}.0`;
const info = JSON.parse(fs.readFileSync("build/windows/info.json", "utf8"));
info.fixed.file_version = numeric;
info.fixed.product_version = numeric;
info.info["0000"].FileVersion = version;
info.info["0000"].ProductVersion = version;
info.info["0000"].OriginalFilename = "alzette-connect.exe";
fs.writeFileSync(path.join(stage, "info.json"), JSON.stringify(info, null, 2));
const manifest = fs.readFileSync("build/windows/wails.exe.manifest", "utf8")
  .replace(/(<assemblyIdentity\b[^>]*\bname="systems\.alzette\.connect"[^>]*\bversion=")[^"]+"/, `$1${numeric}"`);
fs.writeFileSync(path.join(stage, "alzette-connect.manifest"), manifest);
NODE

wails3 generate icons -input build/appicon.png -windowsfilename "$stage/alzette-connect.ico"
arch="$(go env GOARCH)"
wails3 generate syso -arch "$arch" -icon "$stage/alzette-connect.ico" \
  -manifest "$stage/alzette-connect.manifest" -info "$stage/info.json" \
  -out "rsrc_windows_${arch}.syso"
