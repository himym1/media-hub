# UI Direction

## Design Read

A private media operations product for a design-conscious technical user, with a cinematic but work-focused interface. It should feel intentional and responsive, not like a generic admin template and not like an experimental marketing page.

The design takes the useful constraints from the referenced Linux.do prompt: one icon system, no emoji, strong visual authorship, and coherent interaction. It deliberately rejects spectacle that harms scanning or readability.

Reference: https://linux.do/t/topic/2642792/1

## Design Dials

```text
Design variance: 6 / 10
Motion intensity: 4 / 10
Visual density: 6 / 10
```

## Visual Language

- Base: neutral graphite, not blue-black.
- Primary text: paper white.
- Secondary text: cool neutral gray with WCAG AA contrast.
- Signal accent: citrus amber used for focus, progress, and primary actions.
- Success, warning, and error keep semantic colors and do not reuse the accent.
- Poster artwork supplies most chromatic variation.
- No decorative gradients, glow orbs, nested cards, or glass on every surface.

## Layout

### Web

- Left navigation rail on desktop; bottom navigation below 768 px.
- Search is the initial route and occupies the primary visual plane.
- Results use a stable poster column plus release facts; controls never resize the row.
- Task detail is an unframed vertical timeline, not a stack of status cards.
- Settings use full-width sections with dividers.

### Android

- Search remains the first screen; result rows form one continuous scanning surface and selected state carries the emphasis.
- The workspace has one typed route. Main destinations show global top/bottom navigation; system and detail routes replace it instead of stacking another toolbar.
- Services and Operations form one system layer with explicit back navigation. Operations shows one of 115 files, local upload, or archive organization at a time.
- Library rows use stable 2:3 posters; movie details expose Media3 as the primary action, and series details require an episode selection.
- Transfer status and subscription editing are independent detail routes with one toolbar and no bottom navigation.
- Secondary subscription commands live behind one overflow control so 320 dp widths and large text retain room for the page title.
- Long-running task state is shown as an unframed vertical timeline with expandable evidence.
- Touch targets are at least 48 dp.
- System back, predictive back, edge-to-edge, dynamic type, and dark mode are supported.

### Launcher Icon

- The canonical geometry and palette are defined once in [`media-hub-icon-mark.svg`](media-hub-icon-mark.svg); it has a transparent canvas and contains the two-color mark only.
- Regenerate Android foreground, themed monochrome, legacy, and round PNGs with `./scripts/generate-android-launcher-icons.sh` on macOS.
- Verify generated resources are current without modifying the worktree with `./scripts/generate-android-launcher-icons.sh --check`.
- Do not edit generated `mipmap-*` PNGs or `ic_launcher_background` by hand. They are derived from the canonical SVG palette and alpha.

## Typography

- Web: system-safe sans stack initially; self-hosted font is introduced only with a licensed asset.
- Android: platform sans typography through Compose, with explicit hierarchy.
- No negative letter spacing.
- Compact panels use compact headings; display-scale text is reserved for empty/search moments.

## Iconography

- One Lucide family across Web and Android where an icon exists.
- No hand-drawn SVG paths.
- Unfamiliar icon-only controls require tooltips on Web and content descriptions on Android.
- Emoji are not used as product icons.

## Project-Owned Components

### Shared Semantics

- Search field
- Source health indicator
- Media type badge
- Release facts
- Workflow status
- Timeline step
- Empty state
- Inline error
- Confirmation sheet

### Android Miuix Boundary

Feature screens may import only project-owned wrappers such as:

```text
MediaHubTheme
MediaHubScaffold
MediaHubSearchField
MediaHubButton
MediaHubNavigationBar
MediaHubBottomSheet
MediaHubStatusBadge
MediaHubPreferenceRow
```

Direct Miuix imports are restricted to `core/designsystem`.

## Interaction States

Every data surface implements:

- loading skeleton matching final geometry;
- useful empty state;
- partial results when one source fails;
- contextual error with retry eligibility;
- content state;
- stale or offline state where relevant.

Motion communicates state transitions only. Respect reduced-motion settings.

## Accessibility Gate

- WCAG AA contrast for text and controls.
- Visible focus ring on all Web controls.
- Keyboard access to search, result selection, menus, and dialogs.
- Android semantics and content descriptions.
- No color-only workflow states.
- Text must not clip at 200% Web zoom or Android large font scale.
