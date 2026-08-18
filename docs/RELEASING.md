# Release process

Alzette Connect has an independent release cadence from the Alzette server.
The server/client HTTP contract is versioned; a server change must remain
compatible with supported Connect releases or explicitly raise the advertised
minimum client version through the reviewed protocol process.

## Release stages

1. **Prepare** — choose a version, freeze the server-contract fixtures and
   supported Jan/Goose versions, and confirm the platform matrix.
2. **Verify** — run unit, contract, security, browser, native accessibility,
   client-integration, suspend/resume, and clean-VM package tests.
3. **Build** — build each artifact on its target operating system from the same
   reviewed commit.
4. **Sign** — sign executables, bundles, installers, and update artifacts with
   protected release identities. CI without credentials skips this stage.
5. **Platform validate** — notarize and staple macOS; verify Authenticode and
   timestamping on Windows; verify Linux package metadata, checksums, and any
   configured package signature.
6. **Pilot** — publish to an internal channel, exercise update and rollback on
   clean machines, and inspect safe telemetry/logging.
7. **Publish** — promote the exact tested digests. Never rebuild between pilot
   acceptance and publication.

## Required artifacts

- platform installer/package
- `SHA256SUMS`
- versioned release notes with supported OS and Jan/Goose versions
- source commit and dependency lock evidence
- platform signing/notarization evidence where applicable
- completed QA acceptance record
- software-bill-of-materials and provenance attestations once the release
  pipeline is enabled

## Credentials and workflow permissions

Release credentials live only in the approved CI environment or platform
signing service. Use environment protection and human approval. The publishing
job receives `contents: write` only after all build/test/sign gates pass; pull-
request workflows remain `contents: read`. Never expose credentials to forked
pull requests or third-party build steps.

## Updates

Automatic update remains disabled until all three supported platform paths can
verify signed metadata and artifacts, refuse tampered content, drain the local
proxy, revoke the current grant, install, relaunch, and roll back. TLS alone is
not update authenticity.

Linux package-manager updates and in-app AppImage updates are distinct paths
and require separate evidence. macOS updates require a signed/notarized target.
Windows updates must preserve installer identity and publisher continuity.

## Rollback

Keep the prior supported installer available. Rollback is allowed only to an
artifact whose protocol version remains accepted by the server. A revoked or
security-unsafe build is not a rollback target. Document whether user settings
are forward-compatible before changing their on-disk schema.

## No-current-claim boundary

Repository scaffolding proves none of the following by itself: valid publisher
identity, notarization acceptance, SmartScreen reputation, Linux desktop
compatibility, working updater, protected refresh persistence, or Jan/Goose
automatic provisioning. Claims begin only when the evidence rows in the QA
matrix are complete for the exact release digest.
