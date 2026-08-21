# macOS packaging

The initial macOS artifacts are native arm64 and x86-64 application bundles,
each distributed in a stapled ZIP. Packaging, signing, notarization, stapling,
and Gatekeeper verification run on their matching macOS GitHub runners. A
cross-compiled `.app` is a build input, not a release.

## Release sequence

1. Build the arm64 and x86-64 application bundles on their matching runners.
2. Generate and review ICNS assets from the committed vector source.
3. Set the bundle identifier to `systems.alzette.Connect` and apply the minimum
   supported macOS version from the release matrix.
4. Sign nested code and the application with a Developer ID Application
   identity, hardened runtime, timestamping, and the reviewed entitlements.
5. Verify the signature locally with `codesign` and Gatekeeper with `spctl`.
6. Submit a ZIP of the signed app with `notarytool`, wait for acceptance, staple the
   ticket, and verify again on a clean machine.
7. Package the stapled app, produce a SHA-256 checksum, and record the notarization request identifier in
   the release evidence.

`entitlements.plist` is deliberately empty. Add an entitlement only after a
reviewed feature demonstrates that it is required. Never add broad network,
JIT, library-validation, automation, or keychain-sharing exceptions merely to
make a failing package run.

## CI credentials

The `Signed macOS Release` workflow reads the following secrets only from the
protected `macos-release` GitHub environment:

- `MACOS_CERTIFICATE_P12_BASE64`
- `MACOS_CERTIFICATE_PASSWORD`
- `APP_STORE_CONNECT_API_KEY_P8_BASE64`
- `APP_STORE_CONNECT_KEY_ID`
- `APP_STORE_CONNECT_ISSUER_ID`
- `APPLE_TEAM_ID`

The workflow imports credentials into runner-temporary files and a temporary
Keychain, then removes them in an `always()` cleanup step. Stable
`connect-vX.Y.Z` tags publish immutable signed and notarized archives. Demo
tags continue through the explicitly unsigned `Desktop Downloads` workflow.
