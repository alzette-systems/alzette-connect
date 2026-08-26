# Product

<!-- impeccable:product-schema 1 -->

## Platform

adaptive

## Users

Alzette Connect is for non-technical employees invited by their company owner to use company-approved private AI models in tools they already know, initially Pi, Jan, Goose, and ChatGPT. They should not need to understand API keys, local proxies, endpoint configuration, or token refresh.

## Product Purpose

Alzette Connect is the employee's application launcher for approved AI work. Success means an invited employee can sign in once, see the complete current company model catalogue, launch a qualified installed application, and disconnect safely without seeing or copying a reusable remote credential.

## Positioning

Connect is the device-side custody boundary between a human company identity and local AI applications. It maintains the authenticated local connection and configures supported applications without exposing a reusable Alzette credential to the employee.

## Operating Context

- The employee downloads Connect from the invitation portal.
- First run uses a normal onboarding window and browser-based sign-in.
- Sign-in establishes protected reusable identity but does not create inference authority or a local proxy.
- The primary recurring surface is a compact 720×640 application launcher; Connect may remain in the macOS menu bar, Windows tray, or Linux status area while an application session is active.
- Linux is a first-class build platform; when no reliable status-item host exists, the same launcher remains available as a normal window.
- Pi, Jan, and Goose are the accepted adapter set. ChatGPT is an internal macOS adapter candidate, disabled in normal builds until a named desktop version passes the real Responses compatibility run. Windows Store integration remains future work. Claude Code is not in the current product scope.
- Employees may have zero or more company models through group grants managed elsewhere in Alzette.
- Connect presents company model aliases to employees, not the replaceable provider route behind an alias. Generated client instructions identify the active alias, state that the underlying provider is not exposed in that client, and must never misrepresent the assistant as ChatGPT, Codex, or an Alzette-built foundation model.

## Capabilities and Constraints

- Required states: signed out, context choice, launcher, preparing, running, disconnecting, recovery, offline, no models, and access ended.
- Connection and model availability are distinct: a signed-in employee can have no callable company models.
- Offboarding is terminal for the current company access and must not resemble a transient network failure.
- Signing out is device-scoped and removes the saved human sign-in; it does not alter application access managed by the company owner.
- The companion must never render, log, or persist plaintext service credentials in its web UI.
- This repository includes a Wails native-shell and Go runtime foundation. The frontend is semantic HTML/CSS/vanilla JavaScript with a narrow, safe event boundary for that runtime.
- Pi 0.84.2 qualification and isolated-provider launch, Jan 0.8.4 and Goose 1.46.0 configuration adapters, and a reversible macOS ChatGPT Responses candidate are implemented with ownership, protected-secret, backup, atomic-write, supervision, and rollback boundaries. ChatGPT receives a short-lived per-launch local capability through its child-process environment; neither its profile nor model catalogue contains that capability. Native automation remains release-gated until each named app and operating-system combination passes its compatibility matrix.
- The internal demo channel can check a pinned GitHub release, select the exact OS/architecture package, verify GitHub's SHA-256 asset digest, and hand off installation. Production signing, notarization, publisher continuity, rollback evidence, and the production model-route discovery contract are not yet proven.

## Brand Commitments

The product is named **Alzette Connect** and extends Alzette's “Scoped Control Room” identity: quiet, precise, provenance-first, and operational. The incumbent Alzette river mark, paper/ink palette, scarce river-green signal, squared geometry, and plain-language/mono-verification split remain binding.

## Evidence on Hand

- Incumbent design authority: `/root/code/alzette/DESIGN.md`
- Incumbent portal implementation: `/root/code/alzette/portal.css`
- Incumbent brand asset: `/root/code/alzette/alzette-mark.svg`
- The Wails shell, Go runtime, and Pi/Jan/Goose/ChatGPT adapter boundaries pass deterministic source-level checks. The launcher renders only native snapshots; no demo identity, company, version, or model fixture ships in the normal UI. This is not proof of signed production packaging, a complete ChatGPT Responses dialect, named-client behavior, or native compatibility on an employee machine.

## Product Principles

- Explain human outcomes; keep infrastructure vocabulary behind diagnostics.
- Identity, model entitlement, application qualification, and active inference sessions are separate truths.
- Keep credentials in the native boundary, never in the interface.
- Stay quiet while healthy; become specific and actionable when attention is needed.
- Preserve native platform expectations while keeping state meaning consistent across operating systems.

## Accessibility & Inclusion

The recurring surface must work entirely by keyboard, expose written state independent of color, support screen readers, OS text scaling, high-contrast modes, and reduced motion. Onboarding and repair tasks use a normal window so they are not constrained by transient menu-bar or tray behavior.
