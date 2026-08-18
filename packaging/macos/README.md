# macOS packaging

The initial macOS artifact is a Universal application bundle distributed in a
DMG. Packaging, signing, notarization, stapling, and Gatekeeper verification
must run on macOS. A cross-compiled `.app` is a build input, not a release.

## Release sequence

1. Build the arm64 and x86-64 binaries and combine them into one Universal
   application bundle.
2. Generate and review ICNS assets from the committed vector source.
3. Set the bundle identifier to `systems.alzette.Connect` and apply the minimum
   supported macOS version from the release matrix.
4. Sign nested code and the application with a Developer ID Application
   identity, hardened runtime, timestamping, and the reviewed entitlements.
5. Verify the signature locally with `codesign` and Gatekeeper with `spctl`.
6. Build and sign the DMG.
7. Submit the final artifact with `notarytool`, wait for acceptance, staple the
   ticket, and verify again on a clean machine.
8. Produce SHA-256 checksums and record the notarization request identifier in
   the release evidence.

`entitlements.plist` is deliberately empty. Add an entitlement only after a
reviewed feature demonstrates that it is required. Never add broad network,
JIT, library-validation, automation, or keychain-sharing exceptions merely to
make a failing package run.

The repository contains no Apple certificate, private key, App Store Connect
key, Apple ID password, or stored `notarytool` profile. CI therefore performs
an unsigned package smoke only and makes no notarization claim.
