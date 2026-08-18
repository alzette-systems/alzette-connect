# Product

<!-- impeccable:product-schema 1 -->

## Platform

adaptive

## Users

Alzette Connect is for non-technical employees invited by their company owner to use company-approved private AI models in tools they already know, initially Jan and Goose. They should not need to understand API keys, local proxies, endpoint configuration, or token refresh.

## Product Purpose

Alzette Connect keeps an employee's approved Alzette model access quietly available on their computer. Success means an invited employee can sign in once, let Connect configure a supported AI client, and later understand at a glance whether their connection, applications, and company models are ready.

## Positioning

Connect is the device-side custody boundary between a human company identity and local AI applications. It maintains the authenticated local connection and configures supported applications without exposing a reusable Alzette credential to the employee.

## Operating Context

- The employee downloads Connect from the invitation portal.
- First run uses a normal onboarding window and browser-based sign-in.
- After onboarding, Connect starts at OS login and stays quietly connected.
- The primary recurring surface is a macOS menu-bar or Windows tray popover.
- Linux is a first-class build platform; when no reliable status-item host exists, the same compact status surface opens as a normal window.
- Jan and Goose are the first supported applications.
- Employees may have zero or more company models through group grants managed elsewhere in Alzette.

## Capabilities and Constraints

- Required states: connected, signed out, connecting, offline, no models, setup needs attention, and access ended.
- Connection and model availability are distinct: a signed-in employee can have no callable company models.
- Offboarding is terminal for the current company access and must not resemble a transient network failure.
- Signing out is device-scoped and removes the saved human sign-in; it does not alter application access managed by the company owner.
- The companion must never render, log, or persist plaintext service credentials in its web UI.
- This repository includes a Wails native-shell and Go runtime foundation. The frontend is semantic HTML/CSS/vanilla JavaScript with a narrow, safe event boundary for that runtime.
- Jan 0.8.4 and Goose 1.46.0 adapters are implemented with version, ownership, protected-store, backup, atomic-write, and rollback checks. Native client automation remains release-gated until the signed package passes the exact macOS, Windows, and Linux compatibility matrix.
- The internal demo channel can check a pinned GitHub release, select the exact OS/architecture package, verify GitHub's SHA-256 asset digest, and hand off installation. Production signing, notarization, publisher continuity, rollback evidence, and the production model-route discovery contract are not yet proven.

## Brand Commitments

The product is named **Alzette Connect** and extends Alzette's “Scoped Control Room” identity: quiet, precise, provenance-first, and operational. The incumbent Alzette river mark, paper/ink palette, scarce river-green signal, squared geometry, and plain-language/mono-verification split remain binding.

## Evidence on Hand

- Incumbent design authority: `/root/code/alzette/DESIGN.md`
- Incumbent portal implementation: `/root/code/alzette/portal.css`
- Incumbent brand asset: `/root/code/alzette/alzette-mark.svg`
- The Wails shell, Go runtime, and pinned Jan/Goose adapters pass deterministic source-level checks. They are not proof of signed production packaging or native compatibility on an employee machine. No signed-in production user payload is present yet. Demo names, company, timestamps, and model entries in the browser prototype are illustrative and must not be presented as production facts.

## Product Principles

- Explain human outcomes; keep infrastructure vocabulary behind diagnostics.
- Connection, application setup, and model entitlement are separate truths.
- Keep credentials in the native boundary, never in the interface.
- Stay quiet while healthy; become specific and actionable when attention is needed.
- Preserve native platform expectations while keeping state meaning consistent across operating systems.

## Accessibility & Inclusion

The recurring surface must work entirely by keyboard, expose written state independent of color, support screen readers, OS text scaling, high-contrast modes, and reduced motion. Onboarding and repair tasks use a normal window so they are not constrained by transient menu-bar or tray behavior.
