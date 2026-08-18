# Linux packaging

Linux is an explicit release target. The first supported Linux combination is
Ubuntu 24.04 LTS on x86-64, packaged as both AppImage and `.deb`. Other
distributions and architectures are not claimed until the same install,
credential-store, tray/window, client-integration, and update checks pass.

## Runtime contract

- Build against Wails' GTK4/WebKitGTK 6.0 path on Ubuntu 24.04.
- Install the desktop file as
  `/usr/share/applications/systems.alzette.Connect.desktop`.
- Install the SVG or reviewed raster derivatives with the icon name
  `systems.alzette.Connect` under the appropriate hicolor icon directories.
- Install the executable as `alzette-connect` in the package's executable
  path.
- Do not make the tray the only way to reopen, disconnect, or quit. Some Linux
  desktop environments do not expose a notification area. The application
  must remain reachable through the desktop launcher and ordinary window.
- Autostart is opt-in and is not part of the initial package. If later added,
  create an XDG autostart entry only after explicit user consent.
- Protected refresh storage requires a working freedesktop Secret Service.
  Absence or lock failure is a visible fail-closed state; there is no plaintext
  fallback.

## AppImage plan

Build on the oldest supported distribution, inspect linked libraries, and run
the AppImage in a clean Ubuntu 24.04 VM. Verify launch from a path containing
spaces, read-only media, install/update handoff, desktop integration, and full
removal. AppImage publication requires a checksum; signed update publication
remains disabled until its signature path is tested end to end.

## Debian package plan

The `.deb` owns only its installed binary, icon, and desktop entry. Maintainer
scripts must not create a login, start a session, modify Jan/Goose profiles, or
delete user configuration on uninstall. Validate metadata with `dpkg-deb` and
install/remove in a clean VM before release.

Linux release evidence must include the desktop environment, display server
(Wayland or X11), Secret Service provider, installed Jan/Goose versions, and
the exact artifact digest.
