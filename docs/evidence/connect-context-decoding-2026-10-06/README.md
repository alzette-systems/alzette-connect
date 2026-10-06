# macOS context decoding failure

Connect 0.3.9 displayed “Your company access has ended” immediately after an invited employee signed in. Read-only server checks confirmed accepted invitation, enabled identity/person/membership, enabled Employees group and an entitled, enabled shared-evaluation DeepSeek V4 Flash route under the `alzette` alias. The effective-model resolver returned that model successfully. No permissions were modified.

The 0.3.9 model response type lacks the server's endpoint-status fields. Its strict JSON decoder rejects the current response with `json: unknown field "status"`. `loadContexts` then returns `ErrAccessRemoved` for any decoding failure, producing the misleading terminal screen. The latest preceding signed macOS release, 0.3.11, has the same model response type.

Current source contains the complete endpoint-status contract. This fix additionally keeps invalid JSON, unsupported schemas and unexpected HTTP responses separate from an explicit 403 access denial. Authentication failures remain sign-in-required; server failures remain service failures. The existing strict response decoder and current identity model are retained.

Regression checks exercise the current server payload on initial context loading and verify that enabled model access is preserved. Error cases cover 401, 403, 503, unexpected HTTP status, malformed JSON, unknown fields and unsupported schema. Existing tests cover entitlement changes, actual access removal and status-only refresh without rotating human credentials.

Local verification: packaging checks, frontend build and 12 tests, all Go tests, race tests and vet passed. The 0.3.9 decoder failure was independently reproduced using the exact response structs from its release tag.

The repair is prepared for signed macOS release 0.3.12. Native package signing, notarization and publication are performed by the existing release workflow; local Debian tests do not establish native macOS login or desktop application acceptance.

## Release attempt

- Fix commit: `80e989b3838364fde88cc9ff7afc6fe459a5e471`.
- [Cross-platform CI](https://github.com/alzette-systems/alzette-connect/actions/runs/37457163820) passed on Linux, Windows and macOS.
- [Signed macOS release attempt](https://github.com/alzette-systems/alzette-connect/actions/runs/37457173423): Apple Silicon source checks, compilation and Developer ID signature verification passed. Apple's notarization service returned HTTP 403 because a required agreement is missing or expired. No notarized 0.3.12 release was published.
- The Apple Developer Account Holder must review the pending agreement before retrying notarization. Signing credentials were removed by the workflow's cleanup step. The production release gate was retained.

The Account Holder subsequently resolved the agreement. [Signed and notarized macOS 0.3.13](../macos-release-0.3.13-2026-10-06/README.md) was published successfully for both architectures, and the portal now points to that release.
