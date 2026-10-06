# Desktop application acceptance

Current Windows priority is ChatGPT and Claude Desktop, following the user’s
5 October scope update. Pi, Jan and Goose are optional follow-up work. macOS
acceptance is handled independently; it is not a prerequisite for this Windows
run. Adding an application to this list does not qualify it for a customer release.

| Application | Current adapter | Windows acceptance remaining |
| --- | --- | --- |
| Pi 0.84.2 | Shared process adapter | Installed 0.84.2 executable and verified native version; launch, model selection, requests and disconnect remain |
| Jan Desktop 0.8.4 | Reversible settings and native client credential store | Installed 0.8.4 with its Windows installer; native keyring, streaming and cleanup remain |
| Goose Desktop 1.46.0 | Reversible provider configuration and native client credential store | Installed 1.46.0 portable desktop package; ASAR qualification, native keyring, tools and cleanup remain |
| ChatGPT unified desktop | Internal macOS/Windows Responses candidate | Native fixture streaming, switching, tools, errors and cleanup passed; live OIDC and gateway inference passed. Windows process-tree shutdown and supervisor crash tests passed; real tool/provider and package acceptance remain; see [Windows ChatGPT evidence](WINDOWS_CHATGPT_PARITY.md) |
| Claude Desktop | Internal Windows third-party gateway adapter | Real DeepSeek Code streaming/PowerShell tools and Cowork Linux Python/file/Excel tools passed, with Pro/Flash switching and final accounting. Signed installer and broader feature acceptance remain; see [Windows Claude evidence](WINDOWS_CLAUDE.md) |

For every row, qualify installation and discovery, browser Casdoor/OIDC sign-in,
OS credential storage, catalogue visibility, model switching, text and streaming,
tool requests and results, protocol errors, supervised exit, disconnect/revocation,
and preservation of personal settings during cleanup and crash recovery. Store
the exact OS, application version, Connect build digest and evidence with each
acceptance run. A compiled executable or passing protocol fixture alone does
not establish application support.

## Windows test lab

The Debian host now has a [dedicated Windows 11 VM](WINDOWS_VM.md), `alzette-windows-qa`, with
Pi 0.84.2, Jan 0.8.4, Goose 1.46.0, unified ChatGPT `26.930.4958.0`, and Claude
Desktop MSIX `2.19675.0.0`. Pi's native `--version` and the Jan/Goose executable
versions match the pinned adapters. The Pi, Jan and Goose downloads matched
their GitHub release asset SHA-256 digests. Installation does not qualify their
Alzette workflows.

See the [installed-client report](evidence/windows-2026-10-05/installed-clients.json),
[Claude package report](evidence/windows-2026-10-05/claude-install-report.json),
and [artifact metadata](evidence/windows-2026-10-05/build-metadata.json).
Windows Virtual Machine Platform was enabled for Cowork and the guest rebooted;
[post-reboot inspection](evidence/windows-2026-10-05/cleanup-evidence.json) confirms
it is enabled and the ChatGPT managed profile and process are absent.
The 5 October run qualified Cowork text only; the standalone readiness-check
download returned HTTP 403. On 6 October the nested Linux VM booted successfully
and Pro/Flash executed Python, wrote/read Windows shared files, and generated
CSV and XLSX outputs. See the [runtime and artifact evidence](evidence/windows-claude-deepseek-2026-10-06/README.md).
Claude opened its first-run UI without signing in to an Anthropic account.

## Claude Desktop integration boundary

Use Anthropic's [third-party desktop deployment mode](https://claude.com/docs/third-party/claude-desktop/overview),
which exposes Chat, Cowork and Code with a configured inference provider. Connect
must use the desktop configuration mechanism rather than assuming CLI environment
variables configure the desktop app.

The [gateway contract](https://claude.com/docs/third-party/claude-desktop/gateway)
requires Anthropic Messages streaming and tools. The Messages adapter now handles
Desktop adaptive thinking, effort, cache hints, reasoning replay and sequential
streamed tool blocks. Connect maps Claude-facing transport IDs to assigned
Alzette routes and keeps actual DeepSeek names in the picker. Windows Code
text/tool round trips, Cowork workspace tools and Pro/Flash switching passed against real
DeepSeek routes on 2026-10-06; see [WINDOWS_CLAUDE.md](WINDOWS_CLAUDE.md). This
does not establish Chat, cloud Cowork, computer/browser use, vision, hosted tools or exact
Anthropic caching/signature semantics.

Use the documented [credential helper](https://claude.com/docs/third-party/claude-desktop/credential-helper)
for temporary connection material. Keep credentials out of persisted application
settings, policy registry entries, helper arguments, UI and diagnostic logs.
Preserve existing managed policy and user profiles; refuse conflicting ownership.
Test credential expiry, helper refresh and revocation as part of the integration.

Windows Cowork additionally needs a compatible MSIX installation, hardware
virtualization and Virtual Machine Platform, as described in the official
[installation requirements](https://claude.com/docs/third-party/claude-desktop/installation).
Use vendor readiness diagnostics and separately prove Cowork VM startup and
workspace tools. Passing Chat or Code must not automatically qualify Cowork.

## Release rule

The active Windows acceptance rows are ChatGPT and Claude Desktop.
Track vendor restrictions and unresolved failures explicitly. Internal candidates
remain labelled and disabled in normal builds until their native acceptance run
passes. Customer Windows distribution additionally requires signed executable
and installer verification and install, upgrade and uninstall acceptance.
