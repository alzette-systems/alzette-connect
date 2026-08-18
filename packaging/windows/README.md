# Windows packaging

The initial Windows artifact is an x86-64 per-user NSIS installer. MSIX is an
evaluation target after the first pilot because its identity, install/update,
and enterprise deployment behavior must be tested separately.

## Release sequence

1. Build on a Windows runner with the required WebView2 tooling available.
2. Generate and review ICO assets from the committed vector source.
3. Embed stable company, product, file-version, and application-manifest
   metadata using the identity `systems.alzette.Connect`.
4. Build the application executable and NSIS installer.
5. Sign and timestamp both the executable and installer with the approved code
   signing service or certificate.
6. Verify signatures with `signtool`, then install, upgrade, repair, and remove
   the package on clean Windows 11 machines.
7. Produce SHA-256 checksums and retain signing-service evidence.

The installer must not require administrator rights for the default per-user
installation. It must not create an OAuth login, start the proxy, modify
Jan/Goose configuration, or enable startup without an explicit in-application
choice. Uninstall removes program files and registered shortcuts but preserves
user configuration unless the user separately requests its deletion.

The repository contains no PFX, certificate password, Azure signing token, or
timestamping credential. CI therefore performs an unsigned build/package smoke
only and makes no SmartScreen or publisher-reputation claim.
