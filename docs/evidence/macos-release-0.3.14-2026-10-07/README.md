# Signed macOS release and in-app update 0.3.14

[Release](https://github.com/alzette-systems/alzette-connect/releases/tag/connect-v0.3.14)
and [successful native release workflow](https://github.com/alzette-systems/alzette-connect/actions/runs/37615398391).
Source: `c74d167084f4ec1f4afa4e253e13cd3280d606a9`.

Apple Silicon and Intel passed source verification, tests, race checks, vet,
native compilation, Developer ID signing, Apple notarization, ticket stapling
and Gatekeeper assessment. Source CI also passed macOS, Windows and Linux.

| Package | SHA-256 |
| --- | --- |
| macOS arm64 | `e9e859c8c006e02c0b10a9236ab8e13903eaa7da38183f6240d81777976fd6ef` |
| macOS x64 | `0d983fb2a2d241e2c789e3a5ca84135e1c22f9fb18d2cc68f4437c585b08bfe7` |

## Native in-app update acceptance

Test machine: macOS 26.4.1 (25E253), Apple Silicon. Starting app:
`/Applications/Alzette Connect.app`, signed release 0.3.13.

1. Used Diagnostics and updates to check the live GitHub release channel.
2. Connect offered 0.3.14 with the Download update button.
3. Pressed Download update in the running 0.3.13 app. Connect downloaded and
   verified the published archive, quit, replaced its installed bundle and
   automatically reopened. No manual installation or relaunch was used.
4. The installed bundle and native diagnostics reported 0.3.14. The installed
   bundle's file contents matched the independently unpacked release archive.
5. The app resumed its saved company profile and workspace. Before update,
   0.3.13 displayed a sign-in-required state; after relaunch, 0.3.14 was ready
   without manual sign-out or browser login. This observation does not isolate
   the cause of the old app's sign-in-required state.
6. A fresh update check reported Connect is current. Installed signature,
   Gatekeeper and stapled-ticket verification all passed.

The updater retained its prior-bundle backup. This acceptance covers this
Apple Silicon machine and update path; the Intel package passed CI packaging
checks but was not launched on this machine.

[Before update](before-update.png), [update offered](update-available.png),
[after automatic relaunch](update-success.png).

The reconnect fix also has nine regression tests and native fixture acceptance
for browser reauthentication after a rejected refresh credential. This release
does not resolve the separate ChatGPT compatible-function-tool-limit failure
or the misleading callback-port-conflict guidance.
