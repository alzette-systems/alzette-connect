# Packaging contract

Alzette Connect is released for Linux, macOS, and Windows. Linux is a first-
class release target, not an untested by-product of the macOS/Windows builds.
Every published version must pass the platform-specific acceptance matrix in
[`docs/QA_ACCEPTANCE.md`](../docs/QA_ACCEPTANCE.md).

The packaging tree contains source metadata and non-secret release inputs.
Generated installers, update bundles, signatures, certificates, notarization
credentials, and publishing tokens do not belong in Git.

Package license metadata must follow the repository owner's explicit licensing
decision. No open-source license is inferred from the framework or generated
packaging defaults.

## Target artifacts

| Platform | Initial artifacts | Release host |
| --- | --- | --- |
| Linux | x86-64 AppImage and `.deb` for Ubuntu 24.04 LTS | Ubuntu 24.04 runner |
| macOS | Universal `.app` inside a notarized DMG | macOS runner |
| Windows | x86-64 NSIS installer; evaluate MSIX after the pilot | Windows runner |

The current CI workflow builds application smoke artifacts only. It does not
sign, notarize, staple, publish, or assert that an artifact is production-
installable. Those operations remain disabled until the release environment
has protected credentials and the signing paths have passed the acceptance
matrix.

## Common preparation

1. Install the Go version declared by `go.mod` and Node.js 22 or newer.
2. Install the platform dependencies described in
   [`docs/BUILDING.md`](../docs/BUILDING.md).
3. Run `scripts/verify.sh`.
4. Run `scripts/install-wails.sh`; it installs the exact Wails module version
   selected by `go.mod`, never `@latest`.
5. Run `scripts/build-smoke.sh` for a local application build.
6. Run `scripts/package-current.sh` for a clearly marked unsigned host archive.
   It does not replace the native release packages in the table above.

## Asset source

`icons/alzette-connect.svg` is the vector source for the application mark and
Linux desktop icon. Release PNG, ICNS, and ICO files are generated from this
source and reviewed at native sizes before packaging. Do not substitute a
downloaded or remotely loaded icon at build time.

## Security rules

- The application binary and installer are separate signing targets.
- Update metadata and update artifacts require their own verified signature.
- Release jobs must pin actions and tools, use least-privilege permissions,
  and produce checksums and a provenance record.
- A package must stop and revoke the local agent session before installing an
  update.
- No signing key, OAuth token, employee credential, proxy capability, or
  notarization password may appear in an artifact, log, command argument, or
  checked-in configuration.
- Unsigned CI artifacts must be labelled `UNSIGNED-CI` and must never be
  promoted to a customer release.
