# Signed macOS release 0.3.13

[Release and downloads](https://github.com/alzette-systems/alzette-connect/releases/tag/connect-v0.3.13)

[Successful native release workflow](https://github.com/alzette-systems/alzette-connect/actions/runs/37460570659), source `7832b4b739df4bce2458b153d578e6caae0165ea`.

Apple Silicon and Intel passed source verification, tests, race checks, native compilation, Developer ID signature verification, Apple notarization, ticket stapling/validation and Gatekeeper assessment. Published packages carry build-provenance attestations. Both downloaded archives were independently checked for version 0.3.13, their accompanying SHA-256 digest and accepted notarization records.

| Package | SHA-256 |
| --- | --- |
| macOS arm64 | `7519cf91530d3dacf6d4190ee90a2918894dc1a039538751f53f18f617546402` |
| macOS x64 | `f6607963c8e402471db67db2c4e10e215aa553689a3586a01d761947781e0e87` |

This release includes the current endpoint-status response contract, context-error classification fix and OpenDesign visual refinement. It fixes the older client's rejection of status fields that produced a false company-access-ended screen. It does not alter employee permissions.

The previous 0.3.12 attempt was blocked on Apple Silicon by a missing/expired Apple agreement; its Intel job subsequently succeeded. The Account Holder resolved the agreement. GitHub rejected rerunning the failed job with the available account, so a new immutable 0.3.13 tag triggered both native builds. No certificate was renewed and the notarization gate remained in place.

The live portal's Connect download version was changed from 0.3.7 to 0.3.13 after publication. The running control image and existing runtime environment were preserved. Container health and the public agent-configuration endpoint passed after the configuration update.

Native signing and packaging acceptance are established here. The employee's actual login and desktop application session on their Mac remain a separate interactive acceptance check.
