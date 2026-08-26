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

- Web base: neutral graphite, not blue-black.
- Android base: Material 3 dark surfaces. Large radii, tonal containers, and the official SearchBar / NavigationBar sit on a cool graphite canvas. Android primary is luminous blue (`#8AB4FF`) for contrast on dark surfaces.
- Primary text: paper white.
- Secondary text: cool neutral gray with WCAG AA contrast.
- Signal accent: citrus mint on Web; Android primary is Material 3 blue on dark.
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

- Visual language follows Material 3: theme background, tonal cards, hairline dividers, large titles, SearchBar, labeled NavigationBar, and settings rows as ListItem. Screens do not draw a bordered box around every row.
- Search remains the first screen. Idle discovery uses a compact card strip; result rows sit in one grouped card and selected state carries the emphasis.
- The workspace uses Material 3 Scaffold, TopAppBar, and NavigationBar. Main destinations show global top/bottom navigation; system and detail routes replace the bottom bar instead of stacking another toolbar.
- Services and Operations form one system layer with explicit back navigation. Settings use SmallTitle plus grouped preference rows. Operations shows one of 115 files, local upload, or archive organization at a time.
- Library collections switch with a tab row (电影 / 电视剧), not a row of chips or filled buttons.
- Library search is a docked pill field. The grid appends the next Emby page on scroll instead of website-style prev/next controls.
- Library rows use stable 2:3 posters. Movie details use a TopAppBar, horizontal poster + facts, and one play/subtitle action row. Series details require episode selection. Destructive delete is a text action, not a stacked brick.
- Exclusive in-screen destinations (discovery categories, transfer current/archived, system sections) use tab rows. Filter chips stay compact pills for many additive options.
- Service health opens a `MediaHubDialog`. Close and refresh controls stay at 48 dp; poster chrome never uses text smaller than 12 sp.
- Transfer status and subscription editing are independent detail routes with one toolbar and no bottom navigation.
- Secondary subscription commands live behind a `DropdownMenu` overflow so 320 dp widths and large text retain room for the page title.
- Empty states are unframed icon + title + message, not a nested card.
- Provider settings open directly on the 服务配置 tab; do not hide them behind an extra expand row.
- Long-running task state is shown as an unframed vertical timeline with expandable evidence.
- Touch targets are at least 48 dp.
- The player overlay uses the same Material 3 tokens as other screens. It still owns the Media3 surface and Slider; chrome, drawers, and dialogs do not use the old cyan glass palette.
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

### Android Material 3 Boundary

Feature screens may import only project-owned wrappers such as:

```text
MediaHubTheme
MediaHubScaffold
MediaHubTopAppBar
MediaHubSearchField
MediaHubSearchBar
MediaHubButton
MediaHubCard
MediaHubNavigationBar
MediaHubBottomSheet
MediaHubStatusBadge
MediaHubPreferenceRow
MediaHubSwitchRow
MediaHubCheckboxRow
MediaHubFilterChip
MediaHubSegmentedControl
MediaHubTabRow
MediaHubTextButton
MediaHubOverflowMenu
MediaHubDialog
MediaHubEmptyState
MediaHubSmallTitle
```

Direct Material 3 imports are restricted to `core/designsystem`, except the player overlay which owns the playback surface and Material Slider. Player chrome uses the same graphite surfaces and `#8AB4FF` accent as the rest of the app.

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
