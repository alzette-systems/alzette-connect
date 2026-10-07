# Alzette Connect

Alzette Connect is the small desktop launcher that lets an invited employee use company-approved Alzette models in Pi, Jan, and Goose without handling an API key; Windows Claude Desktop supports assigned company models through the Messages API; an internal candidate extends the flow to ChatGPT on macOS/Windows. It signs in through the employee's browser, keeps the refresh session in the operating-system credential store, and creates a fresh short-lived private loopback connection only when a qualified application is launched.

The repository is intentionally separate from the Alzette server. The desktop shell is Wails v3 with a Go security core and a semantic HTML/CSS/vanilla-JavaScript interface.

## Open-source scope

Alzette Connect is open-source software licensed under the
[Apache License 2.0](LICENSE). The license covers this desktop client and its
release automation. It does not cover the separately operated Alzette website,
identity service, inference platform, model infrastructure, or customer
administration services.

See the [privacy notice](docs/PRIVACY.md) for the desktop application's network
and data behavior and the [code signing policy](docs/CODE_SIGNING_POLICY.md) for
the Windows release trust process.

## Employee demo flow

1. Install the current internal macOS demo package from the controlled release channel.
2. Install Pi 0.84.2, Jan 0.8.4, Goose 1.46.0, or—on an internal macOS or Windows candidate build—the ChatGPT app with its Codex workspace. Open configuration-based apps once, then close them so Connect can inspect and update their local profiles safely.
3. Open Alzette Connect and choose **Sign in with Alzette**.
4. Complete browser sign-in with the invited company identity. If more than one workspace is available, choose the safe company/project label in Connect.
5. Review the synchronized company model aliases, their endpoint health, and the status beside each detected application.
6. Double-click an available application, press Enter, or choose **Verify and launch**. Connect creates the private session, checks the installed app, supplies its compatible model catalogue, and supervises it until disconnect.

If an app is open, has an unsupported version, or already contains an Alzette entry Connect does not own, setup stops with a plain-language recovery step and leaves the prior profile intact.

The model drawer shows endpoint status from Alzette every 30 seconds: available,
degraded, unavailable, or unknown. Missing or expired evidence is unknown, and
an endpoint outage leaves its assigned alias visible. Health-only changes do
not replace an active application's credential. Access changes still take
effect at the gateway on every request; health is not a guarantee of admission
or remaining shared token allowance.

## What is working

- system-browser OAuth Authorization Code flow with PKCE S256 and exact loopback callback validation;
- protected refresh storage through macOS Keychain, Windows Credential Manager, or Linux Secret Service, with no plaintext fallback;
- employee context discovery and short-lived `alz_u_` human-credential minting;
- a capability-protected, loopback-only OpenAI-compatible proxy for model discovery, chat completions, and the bounded Responses path exposed by Alzette;
- explicit-launch Pi 0.84.2 qualification and isolated Alzette provider launch;
- provenance-checked configuration adapters for Jan 0.8.4 and Goose 1.46.0, using each app's native protected credential entry;
- a disabled-by-default macOS/Windows ChatGPT candidate with a reversible provider and all-model catalogue using the Responses protocol, with the per-launch local capability injected only into the supervised ChatGPT process;
- Windows Claude Desktop with protected helper credentials, reversible profiles and scoped Messages-only model mapping;
- native window and tray shell with signed-out, context choice, launcher, preparing, running, disconnect, recovery, no-model, and access-ended states;
- in-app demo-channel update checks, SHA-256 verified downloads, and native install handoff;
- deterministic frontend, Go, race, vet, security-boundary, and packaging-source checks;
- unsigned native build smoke on the host platform.

The current Windows priorities are ChatGPT and Claude Desktop; Pi, Jan and Goose are optional follow-up work, and macOS is tested independently. ChatGPT has completed live Windows OIDC/gateway inference. Claude’s Windows adapter now maps assigned DeepSeek routes into its third-party profile and has passed real Code text/tools, Cowork Linux workspace tools and Excel generation, model switching and cleanup against an isolated Alzette gateway. Its Windows build gate is enabled; signed installer and release qualification remain separate. See the [desktop acceptance matrix](docs/DESKTOP_PARITY.md), [Windows ChatGPT evidence](docs/WINDOWS_CHATGPT_PARITY.md) and [Windows Claude evidence](docs/WINDOWS_CLAUDE.md).

Signed/notarized installers, publisher continuity, real Casdoor acceptance, and release qualification on clean macOS, Windows, and Ubuntu machines remain release gates. The app must not be distributed to employees as a production build until those gates pass.

## Downloads and updates

Tagged builds are published on the repository's **Releases** page for macOS (Apple Silicon and Intel), Windows x64, and Ubuntu x64. The **Desktop Downloads** workflow also keeps the same files as short-lived Actions artifacts. Releases carry GitHub build-provenance attestations and GitHub's asset SHA-256 digest.

Stable macOS releases provide Developer ID signed, Apple-notarized and stapled application ZIPs for Apple Silicon and Intel. The separate internal demo downloads are named `unsigned-demo`: macOS receives an ad-hoc-signed `.app.zip`, Windows receives a per-user `.exe` installer, and Ubuntu receives a `.deb`. Windows and Linux demo artifacts remain intended for controlled acceptance work.

The unsigned internal workflow and signed macOS 0.3 acceptance channel enable the ChatGPT adapter candidate so it can complete named native acceptance. Normal local builds keep that adapter disabled unless `ALZETTE_CONNECT_CHATGPT_CANDIDATE=true` is supplied at build time. Even in a candidate build, the row remains **Verify at launch**; a process start never turns unproven native compatibility into **Ready**.

Use **Diagnostics and updates** or **Check for Updates…** in the tray menu. Connect accepts only a newer matching package from the pinned `alzette-systems/alzette-connect` repository and verifies its size and SHA-256 digest before installation. Stable clients skip prereleases; preview clients can also receive newer previews. macOS and Windows close, replace/install, and reopen Connect; Linux opens the verified `.deb` in the system package installer. On macOS, run Connect from a writable Applications folder. The installed Apple Silicon app passed the complete in-app update from [0.3.13 to 0.3.14](docs/evidence/macos-release-0.3.14-2026-10-07/README.md).

### Code signing policy

Free code signing provided by SignPath.io, certificate by SignPath Foundation.
The maintainer roles, origin-verification rules, signed-artifact boundary, and
privacy disclosure are defined in the project's
[code signing policy](docs/CODE_SIGNING_POLICY.md).

## Developer quick start

Use Go 1.25 or newer and Node.js 22. Install the native packages for your host from [the build guide](docs/BUILDING.md), then run:

```sh
scripts/install-wails.sh
scripts/verify.sh
scripts/build-smoke.sh
```

To assemble the native-shaped demo download produced by CI on the current operating system:

```sh
ALZETTE_CONNECT_VERSION=0.2.0-demo.1 scripts/package-download.sh
```

On Debian 12 the build script detects the installed GTK3/WebKitGTK 4.1 compatibility stack. Release Linux builds target Ubuntu 24.04 with GTK4/WebKitGTK 6.0.

To inspect only the interface in a browser:

```sh
npm --prefix frontend ci
npm --prefix frontend run dev
```

The browser build is a visual prototype. It cannot receive or persist a credential.

## Connect to an Alzette development server

The native app discovers its OAuth and gateway configuration from the Alzette control origin. For a loopback-only development server:

```sh
ALZETTE_CONTROL_URL=http://127.0.0.1:8080 \
ALZETTE_CONNECT_ALLOW_INSECURE=1 \
ALZETTE_CONNECT_MEMBERSHIP_ID=<employee-membership-id> \
./bin/alzette-connect
```

`ALZETTE_CONNECT_ALLOW_INSECURE=1` is restricted to loopback development. A remotely reachable demo must use the canonical HTTPS control origin and exact registered OAuth callback.

Connect listens for its local client endpoint at `127.0.0.1:43128` and uses `127.0.0.1:43127/callback` only during sign-in. The local capability is absent from the UI, logs, command line, persistent state, and profile files. It reaches the supervised application through its per-launch environment or, for Windows Claude, a temporary protected credential helper. The remote human credential never enters the application process or configuration.

## Repository map

- `internal/session`: OAuth, protected refresh, contexts, and human credential lifecycle
- `internal/proxy`: strict loopback OpenAI-compatible boundary
- `internal/clientconfig`: Pi/Jan/Goose qualification plus ChatGPT and Windows Claude candidates, reversible configuration, protected client secrets, rollback, and supervised launch
- `internal/credentialstore`: native operating-system protected storage
- `internal/updater`: pinned release discovery, integrity verification, and native install handoff
- `internal/appstate`: credential-free runtime state shared with the desktop UI
- `frontend`: approved application-launcher lifecycle and recovery surface
- `packaging`, `build`, `.github`: unsigned build scaffolding and release gates
- `docs/QA_ACCEPTANCE.md`: evidence required before a platform/client combination is supported

See [PRODUCT.md](PRODUCT.md), [DESIGN.md](DESIGN.md), and [docs/SUPPORTED_PLATFORMS.md](docs/SUPPORTED_PLATFORMS.md) for the product, interface, and platform contracts.
