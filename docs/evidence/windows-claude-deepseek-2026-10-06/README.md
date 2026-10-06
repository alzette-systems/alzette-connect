# Windows Claude / DeepSeek qualification — 6 October 2026

The official native Windows apps ran against an isolated build of Alzette's real
gateway and control service. Casdoor OIDC, Windows Credential Manager, scoped
human grants and the existing routing/metering pipeline were exercised. Actual
inference used the QA company's assigned DeepSeek V4 Flash and Pro routes.
Public gateway/control containers and download artifacts were not changed.

## Native observations

- [Cowork Pro text](claude-cowork-live-pro.png): `ALZETTE_CLAUDE_LIVE_OK`.
- [Code Pro tool and model picker](claude-code-live-tool-and-models.png): actual
  PowerShell `Write-Output ALZETTE_CLAUDE_TOOL_OK`, stdout and final reply.
- [Code Flash in the same conversation](claude-code-live-flash.png):
  `ALZETTE_CLAUDE_FLASH_OK`; previous tool history accepted.
- [Code Pro High effort](claude-code-live-high-effort.png):
  `ALZETTE_CLAUDE_HIGH_OK`.
- [Native ChatGPT Responses regression](chatgpt-openai-regression.png):
  `ALZETTE_OPENAI_REGRESSION_OK` through Flash, HTTP 200 streaming.

Final native reports include payload field names, mapped model aliases, request
IDs and status codes, without prompts, output bodies, authentication tokens or
provider credentials:
[Code/Flash](native-live-claude-code-final.json),
[High effort](native-live-claude-high-final.json), and
[ChatGPT](native-live-openai-final.json).
All three report successful process stop, profile restoration, grant revocation
and refresh credential deletion. A separate [inspection](cleanup.json) found no
Claude/Codex processes, Claude recovery journal or private input files.

[Provider accounting](provider-accounting.json) and
[meter settlement](meter-settlement.json) show final provider usage and exact
settlement. Every successful inference request in the three native final reports
has exactly one final meter event, charged input plus output, and a settled
lease. The remaining 103 reserved tokens in the copied account are an older
5 October `overrun_pending` fixture lease, not one created by these tests.
The initial copied 100k QA allowance blocked large native tool prompts; only the
disposable QA database's company allowance was increased to 1m. The initial Code
400 exposed the 8 KiB tool-description limit; the fixed adapter permits 64 KiB
per tool within the unchanged total-body bound, and the retry/tool turn passed.
ChatGPT's separate `text.format` helper request retains its existing explicit
400 while its main native reply succeeds.

## Automated checks

Passed with Go 1.26.5:

```sh
# Gateway repository
/usr/local/go/bin/go test ./...
/usr/local/go/bin/go test -race ./internal/gateway ./internal/inference
/usr/local/go/bin/go vet ./...
# Real PostgreSQL integration, separate random test schemas
ALZETTE_TEST_DATABASE_URL=... /usr/local/go/bin/go test ./internal/store/postgres -run 'TestPostgres.*(Gateway|SharedUsage|Meter|Overrun)' -count=1

# Connect repository
/usr/local/go/bin/go test -tags gtk3 ./...
/usr/local/go/bin/go test -race ./internal/proxy ./internal/clientconfig ./internal/appstate
GOOS=windows GOARCH=amd64 /usr/local/go/bin/go vet ./...
```

Native Windows proxy and runtime suites passed. Native client suite passed when
excluding `TestQualifyPiRequiresTheNamedRelease`, an optional Pi launch check
whose initial unsigned-binary invocation exceeded its 5-second timeout. See
[first result](native-tests-first.json), [final result](native-tests.json) and
[client](clientconfig-claude-mapping-tests.txt),
[proxy](proxy-claude-mapping-tests.txt),
[runtime](appstate-claude-mapping-tests.txt) logs. The complete Linux suite passes
without that exclusion. Tests cover route mapping/revocation, isolated protocol
paths, malformed requests, thinking replay/streaming, cache accounting, tool
interleaving, and unchanged OpenAI protocol behaviour.

## Limits and artifacts

See [supported scope](../../WINDOWS_CLAUDE.md). This is not a claim that every
Anthropic feature works with every model. Cowork local VM Python/file tools and
spreadsheet creation are now qualified below. Vision, document ingestion, cloud
Cowork, browser/computer use, hosted tools, encrypted thinking signatures and
exact cache TTLs remain outside this qualification. Only two model choices were tested natively.

[Build metadata](build-metadata.json) records source heads, dirty working-tree
qualification, native app versions and binary hashes. The final Windows GUI
candidate is unsigned at the recorded host path. It was rebuilt after the final
message-text update; the native OIDC harness exercised the same adapter/proxy
implementation. No public installer was published. The isolated containers and
copied database were removed after evidence collection; the VM remains available.

## Cowork VM tools follow-up

The Windows VM booted Cowork's nested Linux workspace with Virtual Machine
Platform and host nested virtualization enabled. Actual DeepSeek Pro ran Python,
created `cowork-proof.txt` reporting `Linux` and `ALZETTE_COWORK_VM_OK`, created
`totals.csv`, then read both files and computed 385. It subsequently generated
`totals.xlsx` with square formulas and `SUM(B2:B11)`. The workbook was reopened
in Cowork and its saved XML formulas were independently verified on the host.
Excel cached calculation values are not claimed. DeepSeek Flash continued the
same task, read the CSV in Python, wrote `flash-proof.txt`, and read it back
(`Linux`, `ALZETTE_COWORK_FLASH_OK`, `385`).

- Native screenshots: [Pro files](cowork-pro-linux-files.png),
  [Pro workbook](cowork-pro-xlsx.png), [Flash files](cowork-flash-linux-files.png).
- Downloaded [proof](cowork-files/cowork-proof.txt),
  [CSV](cowork-files/totals.csv), [XLSX](cowork-files/totals.xlsx),
  [Flash proof](cowork-files/flash-proof.txt).
- [Independent artifact verification](cowork-artifact-verification.json),
  [VM startup observations](cowork-vm-startup.txt),
  [qualification metadata](cowork-qualification.json).
- [Final native report](native-cowork-vm-final.json) proves process stop, profile
  restoration, grant revocation and refresh credential deletion.
- [Provider accounting](cowork-provider-accounting.json): all 11 successful
  inference requests were HTTP 200 with final provider usage, exactly one meter
  event, charge equal to input plus output, and a settled lease.

Initial requests failed because the disposable copied fixture's increased
policy limit differed from the endpoint allowance. Only the isolated fixture
was corrected to consistent 1m account/endpoint limits. Native retry then passed.
This did not alter public policy, authentication, gateway configuration or
OpenAI compatibility. No product-code changes were necessary for these Cowork
tool tests. The temporary QA containers and copied database were removed after
evidence collection; the Windows VM and test output files remain available.
