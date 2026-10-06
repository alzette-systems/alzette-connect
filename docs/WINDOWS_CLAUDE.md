# Windows Claude Desktop acceptance

Windows ChatGPT and Claude Desktop are the active priorities. macOS is tested
separately; Pi, Jan and Goose are optional follow-up work.

## Implemented connection

Connect discovers the registered x64 Claude MSIX, verifies its publisher,
signature kind, status and FullTrust entry point, and records its installed
version. The real-provider qualification used Claude **2.19675.0.0** on Windows
11 Enterprise Evaluation, OS version **10.0.26300** (queried through CIM;
the desktop evaluation watermark still shows an older base-build string).

The adapter follows Anthropic's documented
[configuration](https://claude.com/docs/third-party/claude-desktop/configuration),
[gateway](https://claude.com/docs/third-party/claude-desktop/gateway) and
[credential-helper](https://claude.com/docs/third-party/claude-desktop/credential-helper)
contracts. A separate profile under `%LOCALAPPDATA%\Claude-3p\configLibrary`
contains company model labels, a loopback gateway, and the helper's public launch
ID. It contains no credential. Conflicting enterprise inference policies are
preserved and reported.

Claude's SDK requires recognised Claude transport model IDs. Connect maps these
privately to the actual assigned Alzette aliases and shows the real model labels:
DeepSeek V4 Flash and DeepSeek V4 Pro in this test. These IDs do not change the
underlying model. Both proxy startup and each request check assignment; unmapped
or removed models cannot be forwarded. Mapping is restricted to the dedicated
Messages proxy and cannot affect the OpenAI proxy.

The helper reads a launch capability from Windows Credential Manager and checks
the loopback connection. Only the scoped human grant reaches Alzette. Casdoor
OIDC, gateway routing, allowance reservations and accounting remain authoritative.
The Messages adapter translates adaptive/enabled thinking and effort, provider
reasoning, text, function tools and tool results through the existing pipeline.
It validates cache hints and reports only provider-observed cache usage, without
counting cached input twice. Parallel tool fragments become sequential Anthropic
content blocks. The native SDK's exact `?beta=true` marker is accepted.

Disconnect removes the launch credential, stops the owned process tree, removes
the owned profile and restores the previous selection. A durable recovery
journal preserves unrelated profiles and edited settings. Windows Job Objects
own descendants; unrelated processes are not terminated by name.

## Real-provider acceptance: 6 October 2026

The official Windows applications used real Casdoor OIDC and scoped grants,
Connect's protected helper, an isolated build of the Alzette gateway, and actual
DeepSeek inference. No Anthropic model credential was needed. The isolated
database used a larger disposable QA allowance for Code's substantial tool
prompt; public limits and deployments were untouched.

| Behaviour | Result |
| --- | --- |
| Cowork text with DeepSeek Pro | Passed |
| Cowork Linux VM Python execution, Windows shared-file write/read, Pro and Flash | Passed |
| Cowork CSV and Excel workbook creation; independent formula verification | Passed |
| Code PowerShell tool call and result replay with Pro | Passed |
| Switch to Flash in the same Code conversation | Passed |
| Low and High effort in Code | Passed |
| Native model picker shows actual DeepSeek labels | Passed |
| Provider usage recorded with final accounting | Passed |
| Native ChatGPT streaming Responses regression with Flash | Passed |
| Disconnect, profile restoration, grant revocation and refresh deletion | Passed |
| Windows proxy, runtime and client tests | Passed, excluding optional Pi release qualification |
| Full native app crash followed by journal recovery | Not qualified; journal recovery and owned-process crash tests pass separately |

Evidence and test details: [qualification record](evidence/windows-claude-deepseek-2026-10-06/README.md).
Earlier fixture evidence covers controlled error/recovery and owned-process
termination: [5 October report](evidence/windows-2026-10-05/native-claude-job-report.json).

## Supported scope and limits

This qualifies the tested **Code text/tools and local Cowork workspace tools**
paths. Cowork booted its nested Linux VM, ran Python through Pro and Flash,
wrote/read files in a Windows shared folder, and generated CSV and XLSX files.
The downloaded workbook passed independent formula verification. The installed
third-party UI has no Chat tab. Vision, document ingestion, computer/browser use,
cloud Cowork, Anthropic-hosted tools and arbitrary structured output are not
qualified. Workbook formula presence is verified; cached Excel calculation
results are not claimed.
Encrypted Anthropic thinking signatures and exact thinking budgets/cache TTLs
are not promised by a translation to another provider. Unsupported payloads fail
explicitly. Only the two assigned model choices were tested natively.

The native ChatGPT reply succeeds; its separate helper request for unsupported
Responses `text.format` still returns the existing explicit 400. This change
adds no structured-output capability to the OpenAI adapter.

Claude is enabled by default in Windows source builds. The tested candidate is
unsigned; the public installer and public gateway have not been updated by this
qualification. Signing and install/upgrade/uninstall qualification remain
separate distribution work.

## Repeat the native fixture

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags '-H windowsgui -X main.chatGPTCandidateEnabled=true' -o Alzette-Connect-windows-candidate.exe .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o windows-claude-qa.exe ./cmd/windows-claude-qa
```

Run from the logged-in Windows desktop, because SSH uses a different logon
context and does not substitute for the interactive credential vault or UI:

```powershell
.\windows-claude-qa.exe --dir C:\AlzetteQA\claude-native --duration 20m --helper C:\AlzetteQA\Alzette-Connect-windows-candidate.exe
```

Use a disposable Code folder. Send `ALZETTE_QA_TOOL`, approve the harmless echo,
select both models, and test `ALZETTE_QA_ERROR` followed by a new normal request.
Test Cowork separately. Create `stop-claude-qa.txt` in the evidence directory to
exercise Disconnect and inspect the final report. This deterministic fixture
checks client behaviour; the OIDC QA harness additionally checks real routing
and provider accounting against an isolated deployment.
