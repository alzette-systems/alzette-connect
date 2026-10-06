# Windows ChatGPT parity acceptance

Windows acceptance covers the employee workflow:
browser sign-in, assigned model catalogue, explicit launch, streaming and tools,
model switching, disconnect, and restoration of the original application profile.
An unsigned build is sufficient for initial functional testing. Signing and
publisher verification are separate release acceptance steps.

## Local test host

The Debian development host has KVM, QEMU, libvirt, Microsoft-enrolled OVMF UEFI
firmware, and swtpm. The dedicated `alzette-windows-qa` VM uses 4 virtual CPUs,
8 GiB RAM, an 80 GiB sparse disk, TPM 2.0, and a separate NAT network
(`alzette-windows-net`, 192.168.139.0/24). Its VNC console listens on localhost.
The VM does not autostart with the production services.

Installation uses Microsoft's Windows 11 Enterprise 26H2 English x64 evaluation
ISO, downloaded from the official Evaluation Center. Its SHA-256 matched the
published value:

```text
bc3f24086ebadc94489066b5ad78089e2cf5c3491e90e790bb81a2b199c10e38
```

Guest credentials and its SSH key are held outside this repository in a private
host directory. Never attach them to evidence or commit the setup seed ISO.

## Inspect the installed workspace

Install the current unified ChatGPT application from OpenAI's download page.
This integration uses the Codex workspace; ChatGPT Classic alone is not this
adapter’s target. Record the exact Windows package and workspace actually installed.

Run `scripts/inspect-windows-chatgpt.ps1` in the logged-in Windows user session.
The report includes OS and Store package identity, version, publisher, manifest
entry point and activation ID, without reading profiles or credentials.
Discovery verifies the named trusted package, publisher, signature kind,
architecture and manifest, then records its version. Native qualification is
version-specific; a matching display name alone is insufficient.

## Required evidence

| Behaviour | Required result |
| --- | --- |
| Package discovery | Correct Store package, publisher, architecture and version; Classic or unrelated packages cannot be mistaken for the qualified workspace |
| Sign-in and restart | Browser OIDC and Windows Credential Manager work; no plaintext credential fallback |
| Launch | Exact application starts under supervision; an already-running instance cannot silently consume the new profile |
| Private connection | Per-launch capability reaches only the intended app; no credential in arguments, persistent config, UI or diagnostic logs |
| Model catalogue | All assigned aliases appear and model switching uses the selected alias |
| Requests | Buffered text, streaming, function tools and errors work in the installed Windows workspace |
| Disconnect and exit | Proxy closes and human grant is revoked; original profile is restored |
| Crash and restart | Durable recovery restores only Connect-owned settings and preserves user edits |
| Installation and updates | Standard-user install, upgrade and uninstall work; signed-release verification remains pending until signing is configured |

The official unified x64 MSIX inspected during setup identifies `OpenAI.Codex`,
version `26.930.4958.0`, publisher `CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B`,
and full-trust entry point `app/ChatGPT.exe`. Registered-package discovery now
checks this identity and Store signature kind; package inspection is not native
launch acceptance.

## Native results: 5 October 2026

The installed guest reports Windows 11 Enterprise Evaluation, `10.0.26300`,
build `26300` (26H2). The evaluation desktop watermark reports an older build;
use the OS inspection report rather than the watermark. The actual installed
ChatGPT package is `OpenAI.Codex` `26.930.4958.0`, x64, signature kind `Store`,
status `Ok`. Its UI opens the Codex workspace.

The interactive native run used `cmd/windows-chatgpt-qa`, Connect's actual
registered-package discovery, reversible profile adapter, supervised launch,
Windows Credential Manager and Responses-only private proxy. An isolated local
Responses fixture supplied deterministic replies and a harmless `echo` tool.
It used newly generated test credentials, no tenant account and no paid model.

| Check | Result |
| --- | --- |
| Registered package identity and observed version | Passed |
| Native protected-store write, new-store read, delete | Passed |
| Supervised app launch with capability in child environment | Passed |
| Config contains neither loopback capability nor upstream credential | Passed |
| Both aliases appear in the native model picker | Passed |
| Switching to `alzette-windows-b` changes the request model | Passed |
| Streaming text through Connect's private proxy | Passed |
| Native `exec_command` call and returned `ALZETTE_WINDOWS_TOOL_OK` output | Passed |
| Injected upstream HTTP 502 displayed in the native app | Passed |
| New request after the error succeeds | Passed |
| Stopping the fixture closes the app and removes the managed profile | Passed |
| Byte-for-byte original file restoration | Not asserted: the app added personal settings; cleanup preserves those changes |
| Connect native window / WebView2 | Opened successfully in the logged-in desktop; [screenshot](evidence/windows-2026-10-05/connect-native-window-final.png) |
| Native Windows clientconfig, credentialstore, proxy and session tests | Passed; Unix process fixture skipped, replaced by this native launch run |

Evidence: [native report](evidence/windows-2026-10-05/native-chatgpt-report.json),
[model picker](evidence/windows-2026-10-05/chatgpt-model-picker.png),
[tool round trip](evidence/windows-2026-10-05/chatgpt-tool-round-trip.png),
[error](evidence/windows-2026-10-05/chatgpt-upstream-error.png),
[recovery](evidence/windows-2026-10-05/chatgpt-error-recovery.png), and
[artifact digests](evidence/windows-2026-10-05/build-metadata.json). Native OS
and WebView2 details are in the [platform report](evidence/windows-2026-10-05/windows-platform.json).

This run found and fixed two real Windows problems: version observation relied
on an invalid PowerShell positional argument, and configuring a fresh profile
failed because its directory did not yet exist when the lock was acquired.
The configuration tests also needed valid TOML escaping for Windows paths.

## Repeat the isolated native run

Cross-compile on the development host:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o windows-chatgpt-qa.exe ./cmd/windows-chatgpt-qa
```

Install the official unified ChatGPT package, close it, then run the fixture
from the logged-in Windows desktop (an SSH service session cannot substitute):

```powershell
.\windows-chatgpt-qa.exe --dir C:\AlzetteQA --duration 20m
```

Choose a disposable workspace. Send a message with each model, send
`ALZETTE_QA_TOOL` in a new thread, then `ALZETTE_QA_ERROR` in another thread.
Send a normal message in a new thread to verify recovery. Close the app or create
`C:\AlzetteQA\stop-chatgpt-qa.txt`, then inspect the final report. Remove that
stop marker before another run. Do not publish fixture executables or
use this local deterministic run as evidence of real model compatibility.

The [final-build smoke](evidence/windows-2026-10-05/final-build-smoke.json) repeated
native launch, streaming and the tool round trip after adding the x64 identity
check and strict teardown failure reporting.

Post-reboot [cleanup inspection](evidence/windows-2026-10-05/cleanup-evidence.json)
also confirms no ChatGPT process or managed profile/catalogue remains.

## Remaining release acceptance

The normal-build candidate gate remains disabled. Windows internal candidate
builds can now discover and launch the registered workspace; enabling a customer
release still requires actual provider tool compatibility, structured-output
handling, buffered-response evidence, concurrent launch and full application
crash/restart recovery, and signed installer install/upgrade/uninstall acceptance.
Live OIDC/gateway and Windows process supervision results are recorded below. The software-rendered VM's ChatGPT GPU process consumed substantial
CPU; the run establishes functional behaviour, not desktop performance.

The VM is dedicated to continued parity work. See
[the complete desktop matrix](DESKTOP_PARITY.md) for Pi, Jan, Goose and the new
Claude Desktop integration target.

## Live gateway and Windows supervision follow-up

The native OIDC QA driver completed Casdoor authorization-code/PKCE using a
dedicated disposable employee in the existing QA company. It stored the refresh
credential in Windows Credential Manager and made bounded Responses requests
through Connect’s real private proxy to both `qa-shared-flash` and
`qa-shared-pro`; both completed with HTTP 200 in the gateway ledger. The driver
exercises real Casdoor/OIDC and the loopback callback, but automates the Casdoor
page interaction; this is not a claim of manual browser-UI acceptance.

The actual installed ChatGPT application then sent a streaming request with
`qa-shared-flash`. The ledger records `succeeded`, HTTP 200 and 21,341 tokens.
A later native run correctly encountered HTTP 429 `usage_limit_exceeded`: its
88,836-byte input required a conservative 92,932-token reservation, exceeding
the QA account’s remaining allowance after the first run. No allowance or
accounting record was reset to bypass that limit. Both native runs revoked the
grant, deleted the protected refresh credential and restored the managed profile.

The native application also makes a request containing `text` that returns HTTP
400 before its ordinary Responses request. Structured-output compatibility
remains an explicit release gap. Native fixture tool success does not establish
real-provider tool success.

Two Windows defects were fixed: long Casdoor refresh credentials exceeded a
single Credential Manager record, and stopping the root application left
descendants running. Refresh rotation now uses protected chunks with an atomic
manifest, integrity checks and cleanup; it has no plaintext fallback. Windows
launches now enter a Job Object before execution, so Disconnect and Connect
crashes terminate the owned process tree. Native descendant shutdown and
supervisor-crash tests passed. Actual Claude and ChatGPT shutdown restored their
profiles with this supervision enabled.

Evidence: [live ledger](evidence/windows-2026-10-05/windows-live-ledger.json),
[successful reply in the native UI](evidence/windows-2026-10-05/chatgpt-live-reply.png),
[later native run](evidence/windows-2026-10-05/native-chatgpt-live-quota.json),
[usage-limit UI](evidence/windows-2026-10-05/chatgpt-live-usage-limit.png),
[native Job Object tests](evidence/windows-2026-10-05/native-job-tests.txt), and
[native protected-store tests](evidence/windows-2026-10-05/native-vault-tests.txt).

The [final supervised fixture run](evidence/windows-2026-10-05/native-chatgpt-job-report.json)
repeated streaming, model switching and a shell tool round trip with the new
process-tree supervision, then restored the profile. All native Windows
clientconfig, proxy, credentialstore and session suites passed against that
source; [suite exit codes](evidence/windows-2026-10-05/final-native-tests.json)
record each result.
