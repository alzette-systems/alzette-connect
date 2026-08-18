# Build and package Alzette Connect

This guide covers developer and unsigned CI builds. Customer releases require
the additional platform gates in [`RELEASING.md`](RELEASING.md).

## Prerequisites

- Go at the version declared by `go.mod`
- Node.js 22 or newer
- Git
- platform-native build tools

The scripts install the Wails CLI at the exact module version selected by
`go.mod`. They deliberately do not use `@latest`.

### Linux

The supported build host is Ubuntu 24.04 LTS:

```sh
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev libsecret-tools
```

Debian 12 is accepted as a development host using Wails' GTK3 compatibility
tag, but it is not the Linux release host or a shipped-support claim:

```sh
sudo apt-get install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libsecret-tools
WAILS_BUILD_TAGS=gtk3 scripts/build-smoke.sh
```

The build script detects this compatibility stack when GTK4 is not available.
Release CI deliberately uses Ubuntu 24.04 and the default GTK4 path. Native
Linux release packages are produced and accepted only on that release host.

### macOS

Install Xcode Command Line Tools:

```sh
xcode-select --install
```

DMG creation, signing, and notarization require a macOS host. Development and
unsigned package smoke do not require release credentials.

### Windows

Use Windows 11 with the Go toolchain, Node.js, WebView2 Runtime, and NSIS for
installer work. Git for Windows supplies the Bash used by repository scripts.

## Verify and build

From the repository root:

```sh
scripts/verify.sh
scripts/install-wails.sh
scripts/build-smoke.sh
```

`verify.sh` runs shell/package checks, Go tests (including the race detector),
and `go vet`. If the frontend has a package manifest it also runs its declared
`test` script when present.

To build an unsigned packaging-smoke archive on the current operating system:

```sh
scripts/package-current.sh
```

The command refuses to cross-package and emits a `.tar.gz` containing the host
binary, an explicit `UNSIGNED-CI` marker, and Linux desktop metadata where
applicable. It is not an OS-native installer. Final package assembly and
signing must run on the target operating system because a successful build or
archive does not prove native packaging, installation, keychain, tray, or
notarization.

## Outputs

Wails writes intermediate build/package files under its configured build tree.
Generated deliverables belong in `dist/` or `packaging/generated/`; both are
ignored by Git. Never rename an unsigned CI artifact so that it resembles a
customer release.

## Troubleshooting

- Run `wails3 doctor` on the failing operating system.
- On Linux, confirm GTK4 and WebKitGTK 6.0 rather than silently switching to an
  older compatibility stack.
- On Windows, verify WebView2 before changing application code.
- On macOS, distinguish build failure from signing/notarization failure and
  retain the full notarization log as restricted release evidence.
- If the OS credential store is unavailable, fix the store. Do not enable a
  plaintext credential fallback to make a test pass.
