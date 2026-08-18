# Alzette Connect

Alzette Connect is the small desktop companion that lets an invited employee use company-approved Alzette models in Jan and Goose without handling an API key. It signs in through the employee's browser, keeps the refresh session in the operating-system credential store, and gives supported local clients a short-lived private connection on loopback only.

The repository is intentionally separate from the Alzette server. The desktop shell is Wails v3 with a Go security core and a semantic HTML/CSS/vanilla-JavaScript interface.

## Employee demo flow

1. Open the Alzette invitation and choose **Download Alzette Connect**.
2. Install Jan 0.8.4 and Goose 1.46.0. Open each once, then close it so Connect can verify its local profile safely.
3. Open Alzette Connect and choose **Start setup**.
4. Choose **Continue in browser**, use the invited company identity, and return to Connect.
5. Choose **Connect Jan and Goose**. Connect verifies the exact app versions and adds the company connection without showing an API key.
6. Open Jan or Goose from Connect and select one of the company model names shown there.

If an app is open, has an unsupported version, or already contains an Alzette entry Connect does not own, setup stops with a plain-language recovery step and leaves the prior profile intact.

## What is working

- system-browser OAuth Authorization Code flow with PKCE S256 and exact loopback callback validation;
- protected refresh storage through macOS Keychain, Windows Credential Manager, or Linux Secret Service, with no plaintext fallback;
- employee context discovery and short-lived `alz_u_` human-credential minting;
- a capability-protected, loopback-only OpenAI-compatible proxy for model discovery and chat completions;
- provenance-checked configuration adapters for Jan 0.8.4 and Goose 1.46.0, using each app's native protected credential entry;
- native window and tray shell, onboarding, status, offline, no-model, repair, sign-out, and access-ended states;
- deterministic frontend, Go, race, vet, security-boundary, and packaging-source checks;
- unsigned native build smoke on the host platform.

Signed installers, auto-updates, real Casdoor/TLS acceptance, and release qualification on clean macOS, Windows, and Ubuntu machines remain release gates. The app must not be distributed to employees as a production build until those gates pass.

## Download an internal demo build

The **Desktop Downloads** GitHub Actions workflow builds a native-shaped download for macOS (Apple Silicon and Intel), Windows x64, and Ubuntu x64. Open the repository's **Actions** tab, choose **Desktop Downloads**, select **Run workflow**, and enter a demo version such as `0.1.0-demo.2`. When the run is green, download the artifact for the employee's operating system from the run summary.

The current downloads are intentionally named `unsigned-demo`: macOS receives an ad-hoc-signed `.app.zip`, Windows receives a per-user `.exe` installer, and Ubuntu receives a `.deb`. They are suitable for the controlled demo and acceptance work, but they are not production releases. macOS notarization, Windows Authenticode signing, platform clean-machine QA, and protected release publication remain mandatory before self-service employee distribution.

## Developer quick start

Use Go 1.25 or newer and Node.js 22. Install the native packages for your host from [the build guide](docs/BUILDING.md), then run:

```sh
scripts/install-wails.sh
scripts/verify.sh
scripts/build-smoke.sh
```

To assemble the native-shaped demo download produced by CI on the current operating system:

```sh
ALZETTE_CONNECT_VERSION=0.1.0-demo scripts/package-download.sh
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

Connect listens for its local client endpoint at `127.0.0.1:43128` and uses `127.0.0.1:43127/callback` only during sign-in. The local capability and remote human credential are intentionally absent from the UI, logs, command line, environment, and application state.

## Repository map

- `internal/session`: OAuth, protected refresh, contexts, and human credential lifecycle
- `internal/proxy`: strict loopback OpenAI-compatible boundary
- `internal/clientconfig`: version-pinned Jan/Goose configuration, protected client secrets, rollback, and launch
- `internal/credentialstore`: native operating-system protected storage
- `internal/appstate`: credential-free runtime state shared with the desktop UI
- `frontend`: onboarding and recurring connection surface
- `packaging`, `build`, `.github`: unsigned build scaffolding and release gates
- `docs/QA_ACCEPTANCE.md`: evidence required before a platform/client combination is supported

See [PRODUCT.md](PRODUCT.md), [DESIGN.md](DESIGN.md), and [docs/SUPPORTED_PLATFORMS.md](docs/SUPPORTED_PLATFORMS.md) for the product, interface, and platform contracts.
