# Native Windows acceptance — 7 October 2026

**The functional Windows acceptance checks passed after three fixes:** occupied callback-port guidance, Windows executable version resources, and a native explanation when WebView2 is missing. Actual ChatGPT and Claude Desktop Cowork tasks both returned `ALZETTE_CONNECT_SMOKE_OK` through the Alzette gateway. This establishes the tested inference path, not every client feature or tool.

## Test environment and build boundaries

- VM: `alzette-windows-qa`, Windows 11 Enterprise Evaluation, AMD64, four vCPUs, 8 GiB RAM. Interactive desktop: `alzette-test`, session 1. Installer permissions tested separately as `alzette-qa-standard`, whose token was **not an administrator**.
- Functional candidate: Connect main `e752298` plus the fixes in this change, version `0.3.14-windows-qa`, built with `ALZETTE_CONNECT_CHATGPT_CANDIDATE=true`. The original functional candidate is recorded in [native-candidate.json](native-candidate.json); the final native build's hash and Windows version properties are in [resource-check.json](resource-check.json). The prerequisite fix was verified on the final build; the earlier client smoke and lifecycle checks used the same candidate before that startup-only fix and resource addition.
- Native tools: Go 1.25.0, Node 22.23.3, Portable Git 2.56.0.2, GCC 16.2.0, repository-pinned Wails v3 alpha2.114. Verification ran in Git Bash on Windows, not through cross-compilation.
- Installed clients: ChatGPT `26.930.4958.0`; Claude Desktop `2.19675.0.0`. Claude's smoke used its **Cowork** view.
- Live authentication: a disposable Casdoor employee identity, never the user's account. Control and gateway used an isolated copy of the application database and existing DeepSeek shared endpoint; quota changes were confined to that copy. Recovery cases used a separate disposable OIDC fixture with explicit browser approval and controlled credential rejection.
- Published updater pair: Windows `0.3.12-demo.1` → `0.3.12-demo.2`. These are the available published Windows packages; the Mac-only `0.3.14` release is not a Windows update. The source candidate was not published as a release by this acceptance run.

## 1. Source verification

**PASS.** Both commands completed with exit code zero in the Windows VM:

```bash
ALZETTE_CONNECT_CHATGPT_CANDIDATE=true scripts/verify.sh
ALZETTE_CONNECT_CHATGPT_CANDIDATE=true scripts/build-smoke.sh
```

This ran shell and packaging checks, generated bindings, the 12 frontend tests, all Go tests, race tests, vet, and the production native build. Verification was repeated after the final startup fix. See [acceptance-verification.log](acceptance-verification.log) and [acceptance-verification.exit](acceptance-verification.exit). The source changes also passed all Go tests on Debian with Wails' GTK3 tag.

## 2. Login and protected credential persistence

| Check | Result |
| --- | --- |
| First launch without saved credentials stays signed out | PASS |
| Sign in opens the system browser and completes OAuth | PASS, real Casdoor/OIDC |
| Assigned workspace and model catalogue load | PASS, two QA model aliases |
| Refresh credential saved in Windows Credential Manager | PASS, inspected in the interactive user session without printing the value |
| Restart restores access without browser login | PASS |
| Sign out clears the test profile's protected credential | PASS |
| Restart after sign-out stays signed out | PASS |
| Account menu and diagnostics usable | PASS |

Evidence: [workspace](01-workspace.png), [diagnostics](02-diagnostics.png), [restored workspace](03-restored-after-restart.png), [signed-out restart](04-signed-out-after-restart.png), [credential presence](credential-present.json), [credential absence](credential-absent.json). All identity flows remain OIDC; no local human-password authentication was introduced.

## 3. Callback and application lifecycle

| Check | Result |
| --- | --- |
| Callback binds only loopback, normally `127.0.0.1:43127` | PASS |
| Invalid callback returns HTTP 400 without authenticating | PASS |
| Cancel closes the callback listener | PASS |
| Sign in works after cancellation | PASS |
| Occupied callback port produces recoverable failure | PASS after fix; releasing the port and retrying opens browser authentication |
| Second opening leaves one primary instance | PASS; final count one, original primary PID retained |
| Explicit Quit exits Connect | PASS; process count zero |

Windows socket errors use `WSAEADDRINUSE`, so the fix recognizes the native Windows error and reports `sign_in_port_in_use`. The sign-in page now explains that another application is using the local sign-in port. It no longer tells the user to check their internet connection for this case. The regression test occupies an actual local listener, checks that no browser authentication starts, then releases it and verifies a successful retry.

Evidence: [invalid callback](callback-invalid.json), [cancelled callback](callback-cancelled.json), [port-busy guidance](07-local-port-busy.png), [retry](08-port-released-retry.png), [single instance](single-instance-final.json), [Quit](explicit-quit.json).

## 4. Reconnect regressions and native recovery

**PASS.** The existing nine reconnect tests ran in native Windows Go tests and race checks:

- Rejected refresh on an existing session permits reconnect without logout.
- Explicit sign-in falls back to browser authentication after refresh rejection.
- Rejected refresh during startup waits for explicit sign-in.
- Launch failure during an outage permits retry.
- Temporary refresh failure preserves the saved login.
- Reconnect cannot replace an active client/proxy session.
- Status polling recovers when the service returns.
- Concurrent sign-in/launch attempts are blocked; cancellation stays retryable; polling does not overwrite sign-in.
- Rejected browser authorization code does not restart authentication automatically.

The native UI sequence also passed: sign in → expire the fixture access credential → reject refresh → click **Sign in** → approve the new browser authorization → launcher becomes ready **without signing out**. [Rejected session](05-expired-refresh-rejected.png), [ready after recovery](06-reconnected-without-signout.png), and [fixture authorization counts](reconnect-fixture-counts.json) document the sequence. This fixture exercises recovery behavior; the separate login tests exercise real Casdoor.

## 5. Native client launch, actual inference, and cleanup

| Check | Result |
| --- | --- |
| Missing application has disabled launch control | PASS; temporarily hid Jan's executable, verified its disabled row, then restored it |
| Installed clients and assigned catalogue detected | PASS, ChatGPT and Claude |
| Connect launch starts supervised client and loopback proxy | PASS, `127.0.0.1:43128` |
| ChatGPT actual model response | PASS, exact smoke reply |
| Claude Desktop Cowork actual model response | PASS, exact smoke reply |
| Disconnect stops supervised client and closes proxy | PASS, both clients |
| Temporary provider/catalogue configuration removed | PASS, both clients |
| Configuration restored | PASS for Connect-owned and stable client configuration; raw ChatGPT TOML has the client-owned runtime difference described below |
| Repeat launch/disconnect cleanup | PASS, both clients |

The actual prompt was:

```text
Reply with exactly ALZETTE_CONNECT_SMOKE_OK. Do not run tools, access files, or change anything.
```

See the [ChatGPT reply](10-chatgpt-smoke-reply.png) and [Claude Cowork reply](11-claude-cowork-smoke-reply.png). Both were launched with Connect's native controls; a “Running” indicator or a protocol-only harness was not counted as inference success.

[Live accounting](live-inference-accounting.json) records three successful HTTP 200 requests: the two main replies and a ChatGPT helper request. Each has final usage, a settled lease, and exactly one meter event. Charged totals are 21,361, 488, and 35,524 tokens respectively. The original “too many compatible function tools” error did not recur against the gateway's deployed tool-catalogue fix.

ChatGPT's raw semantic baseline changed at **one client-owned field**, `mcp_servers.node_repl.env.SKY_CUA_NATIVE_PIPE_DIRECTORY`, because ChatGPT creates a new internal pipe directory on launch. Removing only that ephemeral field yields identical semantic hashes; all Connect provider entries were removed. This difference is explicitly recorded in [chatgpt-semantic-cleanup.json](chatgpt-semantic-cleanup.json), rather than claiming raw TOML equality. Claude's `_meta.json` hash matched its baseline exactly after both cleanup cycles.

Cleanup evidence: [ChatGPT first disconnect](chatgpt-disconnected.json), [ChatGPT repeated disconnect](chatgpt-repeat-disconnected.json), [Claude first disconnect](claude-disconnected.json), [Claude repeated disconnect](claude-repeat-disconnected.json), and [missing-app state](09-missing-application-disabled.png).

## 6. Release integrity and installed in-app update

| Check | Result |
| --- | --- |
| Published Windows asset SHA-256 | PASS, matches both published digest and checksum |
| Build-provenance attestation | PASS, `gh attestation verify` exit zero for the repository |
| Authenticode signatures and timestamps | N/A: published demo installers and executables are unsigned (`NotSigned`), not certified Windows releases |
| Start older installed Connect | PASS, `0.3.12-demo.1` |
| Correct newer Windows package offered | PASS, `0.3.12-demo.2` |
| Download through Connect UI; verify, exit, install, relaunch | PASS |
| Diagnostics reports new version | PASS |
| Installed executable version properties | Published binaries lacked these properties; fixed and verified in the fresh source candidate |
| Saved login/workspace restored | PASS |
| Explicitly check again and report current | PASS |
| No updater failure or lingering helper | PASS |

The older package was installed to prepare the test. The **upgrade itself** was initiated by **Download update inside Connect**. Manual execution of the newer installer was not counted as the updater test. Connect relaunched with a new process ID, retained the saved QA login/workspace, and its subsequent **Check for updates** reported current.

The SHA-256 of `Alzette-Connect-0.3.12-demo.2-windows-x64-unsigned-demo.exe` is:

```text
ca62ead4e3f544e9e3ada4687b946e89c2a000b4e10f84b92c1baa0d296205c7
```

The public demo executables had blank Windows `FileVersion` and `ProductVersion`, although app diagnostics correctly showed their versions. The build now generates and embeds the icon, manifest, numeric Windows versions, and full product/file version strings. [resource-check.json](resource-check.json) verifies the final native candidate's `0.3.14-windows-qa` strings and `0.3.14.0` numeric properties. This fix does not alter the already published demo binaries.

Evidence: [digests](published-asset-digests.json), [verified provenance](release-provenance.json), [signature and runtime metadata](release-metadata.json), [older installed app](12-installed-older-version.png), [offered update](13-update-offered.png), [update transition](14-update-download.png), [restored workspace](15-updated-workspace-restored.png), [current status](16-updater-current.png), [post-update process/registry state](updater-after.json).

## 7. Additional Windows acceptance

| Check | Result |
| --- | --- |
| Standard-user installation without administrator rights | PASS, actual non-admin Windows token and per-user NSIS installation |
| Production startup without debug console | PASS, current published demo2 and source candidate use GUI subsystem; old demo1 did display its old debug console |
| Upgrade, reinstall, uninstall | PASS under the standard user |
| External client profiles and user data preserved | PASS, fixture ChatGPT/Claude profiles and sentinel data remained after uninstall; real interactive client cleanup also checked above |
| WebView2 available | PASS, normal app UI launches with installed runtime |
| WebView2 unavailable | PASS after startup fix; native prerequisite dialog explains what to install; dismissing it exits cleanly |

[standard-user-package.json](standard-user-package.json) records the child process's Windows identity, non-admin token, installs, removal of application/registration, preserved external fixture data, and GUI PE subsystem. The package lifecycle used silent installer switches under that standard-user token. A scheduled-task preparation attempt did not run and was not counted as a pass.

For the missing-runtime test, only WebView2 discovery registration was temporarily hidden in this isolated VM; binaries were not uninstalled. Before the fix, Connect exited with a WebView2 stack trace and no useful user explanation. The corrected build shows a [native Windows prerequisite dialog](17-webview-required.png) before creating its web interface. Runtime registration was restored afterwards and [normal startup](18-webview-restored-startup.png) and [final-build diagnostics](19-final-native-build.png) checked again. The prerequisite message does not silently install software. [Dialog process metadata](webview-prerequisite-dialog.json) confirms it ran in interactive session 1.

## Test cleanup

All three test profiles' protected credentials were removed and absence confirmed without printing values. The native app and supervised clients were stopped, Jan's executable and WebView2 registration restored, private input files removed, and the standard-user test account disabled. The isolated control/gateway containers, copied database, OIDC recovery fixture, and test tunnels were removed or stopped. The disposable Casdoor fixture password was rotated. The installed published demo2 remains available in the VM; the final unsigned source candidate is also available at `C:\\AlzetteQA\\acceptance-source\\bin\\alzette-connect.exe`.

See [VM cleanup](acceptance-cleanup.json) and [protected credential cleanup](credentials-cleanup.json). External client profiles and user data were retained.

## Limits

These checks establish Windows sign-in, recovery, basic actual inference in both requested clients, cleanup, package lifecycle, and updater behavior for the versions above. They do **not** certify all Cowork VM tools, every ChatGPT capability, Authenticode signing, or a newer public Windows release. Windows release signing remains a separate release requirement.
