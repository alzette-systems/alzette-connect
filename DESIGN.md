# Design System: Alzette Connect — Scoped Control Room

## North Star

Alzette Connect is an app-first launch ledger: a compact, dependable instrument that answers three questions in order—which company models can I use, which installed applications are qualified, and which application session is active? It inherits Alzette's operational paper-and-ink world rather than adopting consumer VPN chrome or a generic settings dashboard.

The physical scene is a quiet workday on an employee laptop in mixed office light. The interface is therefore light, compact, and native-feeling; one night-colored launch or session plane provides depth and a stable action anchor.

## Visual Language

- **Warm paper** `#faf9f6` is the application canvas.
- **Deep paper** `#f2f0ea` separates quiet rows and secondary regions.
- **White surface** `#ffffff` belongs to fields and selected content only.
- **Ink** `#10151a` carries primary reading and actions.
- **Graphite** `#4f5b64` and **faint graphite** `#667179` carry supporting text while meeting contrast requirements.
- **Rule grey** `#d9ddd8` divides ledger rows.
- **Night** `#0d1114`, **night raised** `#151d22`, and **night rule** `#2a363e` form the launch and session plane.
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

### Signed-out window

A two-part normal window: a night-colored employee-connection statement and a paper work area that records the three custody boundaries before one browser sign-in action. No password, provider URL, API key, or token field appears inside Connect.

### Application launcher

The primary window is 720×640 and remains usable at approximately 520×480. Its order is fixed: brand/context header, synchronized company-catalogue strip, application ledger, then one night-colored launch plane. The application ledger may scroll vertically but the window never scrolls horizontally.

### Launch and session plane

The dark field is the only dominant panel. Before launch it names the selected application and catalogue behavior; while running it names the supervised application and offers tray/disconnect actions. Preparing becomes a full-window progress ledger. Recovery uses quiet amber and never claims revocation or restoration that was not confirmed.

### Ledger rows

Models and applications are flat rows separated by rules, not collections of cards. Each application row has a plain-language name, exact observed version when qualified, delivery mode, compatible-model count, and explicit support state. “No models” is an entitlement state with guidance, not an error or zero-looking metric.

## Interaction Rules

- The primary action names the outcome: “Sign in with Alzette,” “Verify and launch ChatGPT,” “Launch Jan Desktop,” or “Disconnect.”
- “Sign out on this device” states its scope. If activated, confirmation explains that local applications will disconnect.
- “Quit Alzette Connect” warns that quitting ends the local connection because the confirmed product direction is a persistent companion.
- Connecting has bounded copy and a cancel/retry path; no unbounded spinner communicates progress alone.
- Access ended removes retry and setup actions. It offers portal/help context and device sign-out only.
- Sign-in, application qualification/configuration, model availability, and an active inference session remain distinct.
- Keyboard focus uses a 3px river-green outline with 3px offset. Hover is supplementary.

## Motion

Use one authored transition: while preparing, the current progress-ledger marker breathes gently and then settles when the application starts. There is no ambient pulsing elsewhere. All content is visible without animation. Under `prefers-reduced-motion: reduce`, transition durations are effectively removed.

## Responsive and Native Adaptation

- macOS keeps the launcher available from the menu bar while supervising an active application session.
- Windows keeps the same launcher available from the tray, with Start menu and notification entry points handled by the native shell.
- Linux uses a status item where supported and the normal launcher window everywhere else.
- Native shells own title bars, window shadows, menu placement, notifications, and platform font rendering. The web content must not fake traffic lights, title-bar drag regions, or OS menus.
- Below 600px signed-out content becomes one column and application rows reflow without hiding their written status. The application ledger scrolls vertically while the launch/session plane remains visible.

## Accessibility

- Use semantic headings, lists, forms, buttons, links, `aria-current`, `aria-live`, and `role="status"` only where they express real structure.
- Restore focus after view/state changes and move focus to the newly active onboarding heading.
- Do not auto-focus on first paint.
- Announce backend state changes through one polite live region; access-ended changes are assertive only when initiated while the app is open.
- Support 200% text zoom, Windows high contrast, VoiceOver, Narrator, and Orca semantics.
- Use written status and distinct icon geometry in addition to green, amber, or red.

## Do / Don't

**Do** keep company/workspace context in the header. **Do** expose the complete current alias catalogue in the drawer. **Do** distinguish detected, qualified, configured, and running states.

**Don't** expose API keys, local proxy addresses, provider URLs, or token expiry in the ordinary UI. **Don't** use glass, gradients, glow, decorative card grids, or indefinite pulsing. **Don't** say “all systems operational” when only the device connection was checked. **Don't** make the tray icon the only way to recover the app on Windows or Linux.
