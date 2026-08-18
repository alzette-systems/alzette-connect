# Design System: Alzette Connect — Scoped Control Room

## North Star

Alzette Connect is a pocket connection ledger: a tiny, dependable instrument that answers three questions in order—am I connected, which company models can I use, and are my applications ready? It inherits Alzette's operational paper-and-ink world rather than adopting consumer VPN chrome or a generic settings dashboard.

The physical scene is a quiet workday on an employee laptop in mixed office light. The interface is therefore light, compact, and native-feeling; one night-colored connection field provides depth and a stable status anchor.

## Visual Language

- **Warm paper** `#faf9f6` is the application canvas.
- **Deep paper** `#f2f0ea` separates quiet rows and secondary regions.
- **White surface** `#ffffff` belongs to fields and selected content only.
- **Ink** `#10151a` carries primary reading and actions.
- **Graphite** `#4f5b64` and **faint graphite** `#667179` carry supporting text while meeting contrast requirements.
- **Rule grey** `#d9ddd8` divides ledger rows.
- **Night** `#0d1114`, **night raised** `#151d22`, and **night rule** `#2a363e` form the connection instrument.
- **River green** `#0d9e63` is scarce: verified connection, focus, and affirmative progress only. Use deep green `#087c4e` for readable links.
- **Quiet amber** `#8b5b17` / `#fff4de` means degraded, offline, or needs attention.
- **Quiet red** `#9f3034` / `#fff0ef` means access ended or a destructive action.

Color never carries status alone. Every state has a noun or verb phrase and an explanatory sentence.

## Type

Use the native system sans stack for names, instructions, and actions. Use the native mono stack only for timestamps, system state labels, model identifiers, and diagnostics. Human language is 13–15px with 1.45–1.55 line height; compact verification labels are 10–11px and may be uppercase with `0.08em` tracking. Headlines use the system sans at 600–660 weight with tight but never tighter than `-0.03em` tracking.

## Shape, Depth, and Spacing

The geometry is squared and quiet: 2px fields, 4px buttons and status surfaces, 6px only for the application window shell in browser demos. Use 1px rules and tonal shifts before shadows. The browser demo shell alone may use `0 24px 70px rgba(16, 21, 26, .18)` to represent native window separation.

Spacing follows a 4px compact sub-grid inside the popover and the incumbent 8px system overall. Recurring group gaps are 8, 12, 16, 24, and 32px. Click/tap targets are at least 40px; onboarding targets are at least 44px.

## Surfaces

### Onboarding window

A two-part normal window: a fixed night-colored trust rail records the four setup boundaries, while the paper work area contains exactly one decision per step. On narrow screens the rail becomes a horizontal progress ledger above the content. Browser sign-in and application configuration are described before they happen. No password or token field appears inside Connect.

### Compact status surface

The popover is 380px wide and typically no taller than 620px. Its order is fixed: brand/context header, dark connection instrument, company-model ledger, application ledger, then quiet utility actions. It must remain a useful normal window on Linux and at large text sizes. A compact surface may scroll vertically but never horizontally.

### Connection instrument

The dark field is the only dominant panel. It pairs a written state with company context and freshness. A river-trace line may animate once from connecting to connected; there is no pulsing ambient motion. Offline and attention states use amber; access ended uses quiet red.

### Ledger rows

Models and applications are flat rows separated by rules, not collections of cards. Each row has a plain-language name, compact verifying metadata, and an explicit state/action. “No models” is an entitlement state with guidance, not an error or zero-looking metric.

## Interaction Rules

- The primary action names the outcome: “Continue in browser,” “Connect Jan and Goose,” “Open Jan,” or “Try again.”
- “Sign out on this device” states its scope. If activated, confirmation explains that local applications will disconnect.
- “Quit Alzette Connect” warns that quitting ends the local connection because the confirmed product direction is a persistent companion.
- Connecting has bounded copy and a cancel/retry path; no unbounded spinner communicates progress alone.
- Access ended removes retry and setup actions. It offers portal/help context and device sign-out only.
- Application configuration remains distinct from model availability and from identity state.
- Keyboard focus uses a 3px river-green outline with 3px offset. Hover is supplementary.

## Motion

Use one authored transition: while connecting, the river trace is drawn across the dark connection field and then settles to a static verified line. All content is visible without animation. Under `prefers-reduced-motion: reduce`, transition durations are effectively removed and the trace is immediately complete.

## Responsive and Native Adaptation

- macOS uses menu-bar popover behavior after onboarding.
- Windows uses the same compact tray surface, with Start menu and notification entry points handled by the native shell.
- Linux uses a status item where supported and a normal compact window fallback everywhere else.
- Native shells own title bars, window shadows, menu placement, notifications, and platform font rendering. The web content must not fake traffic lights, title-bar drag regions, or OS menus.
- Below 620px onboarding becomes one column. Below 380px the status surface uses the full viewport width and reduces side padding, never hides labels, and allows vertical scrolling.

## Accessibility

- Use semantic headings, lists, forms, buttons, links, `aria-current`, `aria-live`, and `role="status"` only where they express real structure.
- Restore focus after view/state changes and move focus to the newly active onboarding heading.
- Do not auto-focus on first paint.
- Announce backend state changes through one polite live region; access-ended changes are assertive only when initiated while the app is open.
- Support 200% text zoom, Windows high contrast, VoiceOver, Narrator, and Orca semantics.
- Use written status and distinct icon geometry in addition to green, amber, or red.

## Do / Don't

**Do** keep the company name and last-check time beside connection status. **Do** cap the compact model preview at three rows and provide “View all in portal” when more exist. **Do** make illustrative demo data identifiable in code.

**Don't** expose API keys, local proxy addresses, provider URLs, or token expiry in the ordinary UI. **Don't** use glass, gradients, glow, decorative card grids, or indefinite pulsing. **Don't** say “all systems operational” when only the device connection was checked. **Don't** make the tray icon the only way to recover the app on Windows or Linux.
