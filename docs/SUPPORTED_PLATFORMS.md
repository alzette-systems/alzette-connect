# Supported platform policy

Linux, macOS, and Windows are release-blocking targets for Alzette Connect.
Support is defined by an exact operating-system, architecture, package, desktop
client, and credential-store combination—not by whether the source compiles.

## Initial acceptance targets

| Platform | Architecture | Initial package | Required protected store |
| --- | --- | --- | --- |
| Ubuntu 24.04 LTS | x86-64 | AppImage and `.deb` | freedesktop Secret Service |
| macOS 13 or newer | Universal arm64 + x86-64 | notarized DMG | macOS Keychain |
| Windows 11 | x86-64 | signed NSIS installer | Windows Credential Manager |

These are intended targets, not shipped-support claims. A row becomes supported
only after the exact package digest passes [`QA_ACCEPTANCE.md`](QA_ACCEPTANCE.md)
and is named in release notes.

Linux support must include at least one Wayland session. X11, additional Linux
desktop environments, ARM64, Debian, Fedora, RHEL, Snap, Flatpak, and package-
manager repositories are added only through explicit test evidence. The normal
application window and desktop launcher remain required because a system tray
is not universally available on Linux.

Client compatibility is versioned separately from OS support. The initial
release notes must name the exact tested Jan and Goose versions. Detecting an
installed but untested client version must produce a truthful guided/manual
path, not silently edit its profile.

## Removing support

Raising a minimum OS, architecture, WebView, desktop client, or server protocol
version requires product/security review, release-note notice, and a migration
or continued safe-use path for the last supported release. An auto-update must
not install a build that cannot run on the current machine.
