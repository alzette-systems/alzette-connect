# Supported platform policy

Linux, macOS, and Windows are release-blocking targets for Alzette Connect.
Support is defined by an exact operating-system, architecture, package, desktop
client, and credential-store combination—not by whether the source compiles.

## Initial acceptance targets

| Platform | Architecture | Initial package | Required protected store |
| --- | --- | --- | --- |
| Ubuntu 24.04 LTS | x86-64 | AppImage and `.deb` | freedesktop Secret Service |
| macOS 13 or newer | Universal arm64 + x86-64 | notarized DMG | macOS Keychain |
| Windows 11 | x86-64 | signed NSIS installer | Windows Credential Manager |

These are intended targets, not shipped-support claims. A row becomes supported
only after the exact package digest passes [`QA_ACCEPTANCE.md`](QA_ACCEPTANCE.md)
and is named in release notes.

Linux support must include at least one Wayland session. X11, additional Linux
desktop environments, ARM64, Debian, Fedora, RHEL, Snap, Flatpak, and package-
manager repositories are added only through explicit test evidence. The normal
application window and desktop launcher remain required because a system tray
is not universally available on Linux.

Client compatibility is versioned separately from OS support. The initial
release notes must name the exact tested Jan and Goose versions. Detecting an
installed but untested client version must produce a truthful guided/manual
path, not silently edit its profile.

Windows acceptance currently prioritizes ChatGPT and Claude Desktop. Pi, Jan
and Goose are optional follow-up work; macOS is tested independently. Track
results in [`DESKTOP_PARITY.md`](DESKTOP_PARITY.md). Claude’s Windows
adapter uses official third-party configuration, a protected credential
helper and assigned-route model mapping. Real DeepSeek Code/Cowork inference
has passed; the adapter is enabled in source builds. Signed installer acceptance
remains separate; see [`WINDOWS_CLAUDE.md`](WINDOWS_CLAUDE.md).

ChatGPT is currently a disabled-by-default macOS/Windows acceptance candidate.
The Windows candidate discovers the registered, Store-signed `OpenAI.Codex`
package and verifies its publisher and manifest before supervised launch.
Native Windows streaming, model switching, a function-tool round trip, errors,
Credential Manager and profile cleanup passed against an isolated fixture; see
[`WINDOWS_CHATGPT_PARITY.md`](WINDOWS_CHATGPT_PARITY.md). Live Casdoor/OIDC and gateway inference have now passed; actual provider tool
compatibility and signed-package release acceptance remain required. Linux has no ChatGPT Desktop
adapter; Linux Connect uses its separately qualified clients.

## Removing support

Raising a minimum OS, architecture, WebView, desktop client, or server protocol
version requires product/security review, release-note notice, and a migration
or continued safe-use path for the last supported release. An auto-update must
not install a build that cannot run on the current machine.

## Microsoft Copilot integration research

The consumer Windows package `Microsoft.Copilot` version `1.25121.84.0` was
installed from the Microsoft Store and inspected on 6 October 2026. Its internal
endpoint selector chooses production, staging or a fixed local development
service. The selector UI is gated on Microsoft-internal enrollment, and chat
uses Copilot-specific WebSocket messages. No supported consumer custom-model
configuration or working Alzette integration was established. A local service
emulator would require a separate protocol and client-routing research project;
adding OpenAI/Anthropic compatibility to the gateway alone does not provide it.

Microsoft 365's [custom engine agents](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/overview-custom-engine-agent)
officially support bringing our own models and orchestration into Copilot Chat
and Teams. An Alzette-backed custom agent is the recommended supported route.
It needs a hosted agent adapter, Microsoft tenant/app registration and user/company
identity linking with the existing Alzette authorization model. It provides an
Alzette agent inside Microsoft 365; it does not automatically replace the consumer
assistant or inherit Microsoft's built-in Cowork/Office tools. Agent tools must
be implemented and authorized separately. No Microsoft 365 tenant integration
was live-qualified during this run. See [native research evidence](evidence/windows-copilot-research-2026-10-06/findings.json).
