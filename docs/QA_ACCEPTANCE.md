# Desktop release acceptance matrix

Complete this matrix for each release candidate and retain links to logs,
screenshots, package digests, and test records. `PASS` means the exact signed
candidate passed on a clean machine. A compile or unsigned CI smoke is not a
substitute.

| Gate | Linux | macOS | Windows | Required evidence |
| --- | --- | --- | --- | --- |
| Reproducible source and dependency locks | Required | Required | Required | Commit, tool versions, dependency diff, artifact digest |
| Unit, race, vet, and server-contract tests | Required | Required | Required | CI run for exact commit |
| Package installs without developer tools | Required | Required | Required | Clean-machine install log and screenshot |
| Package upgrade preserves safe settings | Required | Required | Required | Previous-to-current upgrade record |
| Uninstall removes owned files only | Required | Required | Required | Before/after filesystem and launcher evidence |
| Publisher/package verification | Checksum/package signature | codesign, Gatekeeper, notarization and staple | Authenticode and timestamp | Native verification output |
| Normal window launches without tray | Required | Required | Required | Native smoke test |
| Tray/menu actions and explicit Quit | Required where tray exists | Required | Required | Click and keyboard path; Linux DE named |
| Keyboard-only operation and focus order | Required | Required | Required | Recorded manual path |
| Screen-reader labels and status changes | Orca | VoiceOver | Narrator | Manual transcript with no blocker |
| 200% text/OS scaling and high contrast | Required | Required | Required | Screenshots and task completion |
| Protected refresh store write/read/delete | Secret Service | Keychain | Credential Manager | Native integration test; no secret output |
| Missing/locked protected store fails closed | Required | Required | Required | Visible recovery state; no file fallback |
| Browser PKCE success/cancel/timeout/port collision | Required | Required | Required | Fake IdP plus configured real-IdP evidence |
| Loopback listener binds IP literal only | Required | Required | Required | Socket inspection and hostile-request suite |
| Proxy capability never reaches DOM/log/argv/disk | Required | Required | Required | Leak scan and process inspection |
| Single-instance and concurrent-login handling | Required | Required | Required | Two-launch test |
| Sleep, wake, network loss, clock change | Required | Required | Required | State/recovery test |
| Disconnect/logout revokes and clears local state | Required | Required | Required | Server grant count and credential-store check |
| Crash/forced-kill recovery is safe | Required | Required | Required | Restart test and bounded stale-grant evidence |
| Pi pinned-version workflow | Required | Required | Required | Native installation, model switching, request/tool round trip and clean exit |
| Jan pinned-version guided/adapter flow | Required | Required | Required | First request and clean exit on named version |
| Goose pinned-version adapter flow | Required | Required | Required | First request and clean exit on named version |
| Unknown Jan/Goose version is not modified | Required | Required | Required | Profile digest unchanged |
| ChatGPT candidate is absent from normal builds | N/A | Required | Required | Build-gate test and native UI evidence |
| ChatGPT Responses candidate flow | N/A | Required before enabling | Required before enabling | Exact bundle/version, buffered/streaming text, function tools, model switching, protocol errors, profile restore, and server ledger evidence |
| ChatGPT process capability is Responses-only | N/A | Required before enabling | Required before enabling | Chat Completions and every unsupported path fail with the same per-launch capability |
| Claude Desktop third-party adapter | N/A | Required before adding support | Required before adding support | Exact package; Chat, Cowork and Code; Messages streaming/tools; helper refresh; alias selection; profile preservation |
| Employee removed from group loses next-request access | Required | Required | Required | End-to-end authorization test |
| Owner sees all active company endpoints | Required | Required | Required | End-to-end context/model test |
| Endpoint outage keeps assigned aliases visible; recovery refreshes in place | Required | Required | Required | Server health/entitlement test plus Connect refresh test |
| Missing, stale, or failed-refresh endpoint evidence shows unknown | Required | Required | Required | Status expiry test, including native Date bindings |
| Tampered update is rejected | Required | Required | Required | Invalid metadata/artifact signature test |
| Signed update drains, installs, relaunches | Required | Required | Required | Staged update record |
| Rollback to prior supported build | Required | Required | Required | Staged rollback record |

## Linux-specific evidence

Record Ubuntu point release, kernel, Wayland/X11, desktop environment, WebKitGTK
version, Secret Service implementation, and whether the tray was available.
Test both AppImage and `.deb`; passing one does not qualify the other.

The explicitly gated Linux core harness covers the reusable Connect boundary
against the configured real Casdoor and inference deployment:

```bash
ALZETTE_CONNECT_LIVE_QA=1 \
ALZETTE_CONNECT_LIVE_QA_USERNAME='<invited employee>' \
ALZETTE_CONNECT_LIVE_QA_PASSWORD='<from the QA secret store>' \
ALZETTE_CONNECT_LIVE_QA_ORGANISATION='<expected company name>' \
ALZETTE_CONNECT_LIVE_QA_MODELS='deepseek-v4-flash,deepseek-v4-pro' \
ALZETTE_CONNECT_LIVE_QA_PI="$(command -v pi)" \
scripts/qa-linux-live.sh
```

It uses an isolated Secret Service collection, performs the real Casdoor
authorization-code/PKCE callback, rotates the protected refresh credential on
resume, verifies exact model discovery through the private loopback proxy,
sends bounded requests to every explicitly allowed real model, optionally runs
the pinned Pi client, scans its temporary state for Connect credentials, then
disconnects and proves the revoked grant cannot be reused. The harness never
prints credentials or model output. It does not replace package, tray,
accessibility, WebView, or platform-signing evidence.

## macOS-specific evidence

Record hardware architecture, macOS version, bundle/Team identifiers,
notarization request identifier, stapling result, Gatekeeper result, Keychain
prompt behavior, menu-bar icon in light/dark mode, and Universal binary slices.
For the internal ChatGPT candidate, also record the exact ChatGPT bundle path,
`com.openai.codex` identifier, observed version, config/catalogue before/after
digests, and forced-kill recovery result.

## Windows-specific evidence

Record Windows edition/build, WebView2 version, installer scope, publisher and
timestamp chain, Credential Manager behavior, notification-area persistence,
repair/upgrade/uninstall behavior, and standard-user installation.

The initial unsigned VM findings are recorded in
[`WINDOWS_CHATGPT_PARITY.md`](WINDOWS_CHATGPT_PARITY.md). They establish native
functional progress and do not count as a signed release `PASS`. Every macOS
desktop adapter needs its corresponding native Windows acceptance row; see
[`DESKTOP_PARITY.md`](DESKTOP_PARITY.md).

## Release decision

Any failure involving authentication, authorization, tenant isolation,
credential persistence, loopback exposure, signed updates, inaccessible
disconnect/logout, or package integrity blocks release. A missing Linux tray
does not block release if the documented launcher/window fallback works and the
tested desktop environment is named truthfully.
