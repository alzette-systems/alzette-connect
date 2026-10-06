# Connect visual refinement

Ported the existing **Alzette Connect — Visual refinement** screens from OpenDesign Cloud on 6 October 2026. The production frontend retains its original DOM and native bridge. Changes cover presentation, application artwork and its build-time imports.

The workspace header, catalogue, application rows, launch/session planes, sign-in, context selection, preparing, recovery and dialogs follow the supplied design. Two rendered-fit corrections keep the signed-out account menu on the right and switch the header to two rows based on available width relative to text size. DESIGN.md records the current layout rules.

Application artwork keeps the original supplied bytes. Vite imports package both emitted and inline assets for offline rendering. No review fixtures, preview controls or simulated native bridge are included in the application bundle. Authentication, adapters, permissions, launch handling and cleanup behavior are unchanged.

## Verification

- Production Vite build and all 12 frontend tests passed.
- Chromium against the built frontend: 72 layout cases passed.
- WebKit against the source frontend: the same 72 cases passed.
- Cases cover all 12 review states at 720×640, 520×480 and 380×560, each at normal and 200% text. Enlarged-text cases include long company, workspace and application names. Screen/header horizontal fit and scrolling each primary action into view were checked.
- Application artwork decoded successfully in both engines, including the production bundle's emitted and inline assets.
- Selection, Enter launch, catalogue/menu Escape with focus restoration, running sign-out confirmation, disconnect, disabled application behavior and unknown-ID icon fallback passed in both engines. Forced colours and reduced motion received a rendering smoke check. No browser errors were reported.
- Windows x64 production GUI cross-compilation succeeded with the rebuilt embedded frontend.

Browser checks use synthetic snapshots and stubbed native actions. They validate presentation and action dispatch, not live sign-in, inference, application launch or native OS accessibility. ChatGPT and Claude rows in screenshots receive verification-required fixture states; release qualification continues to come from the real runtime. No Windows installer was published as part of this visual change.

## Captures and reports

- [Launcher](launcher.png)
- [Signed out](signed-out.png)
- [Compact launcher at 200% text, scrolled to its action](launcher-compact-200.png)
- [Chromium production report](chromium-production.json)
- [WebKit source report](webkit-source.json)
