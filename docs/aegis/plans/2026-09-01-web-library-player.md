# Web Library Player Implementation Plan

Date: `2026-09-01`
Status: executable
Spec: `docs/aegis/specs/2026-09-01-web-library-player-design.md`

## Goal

Let the Web library play movies and episodes through native `<video>` using existing playback descriptors, with optional `playbackUserAgent` so 115 URLs match the browser.

## Architecture

Playback service resolves the User-Agent used to mint 115/Emby URLs. Web library overlay consumes `POST /playback/descriptors/emby`. Android keeps omitting the new field.

## Tech Stack

Go 1.x backend tests, OpenAPI 3.1, React 19, Vite, Vitest, Playwright, optional `hls.js` for `m3u8` only.

## Baseline/Authority Refs

- Spec above
- `docs/architecture/adr/0004-react-web-native-dom.md`
- `docs/architecture/adr/0006-emby-managed-media3-playback.md`
- `AGENTS.md` media-byte rule
- `docs/aegis/baseline/2026-09-01-initial-baseline.md`

## Compatibility Boundary

- Omit/empty `playbackUserAgent` → `playback.PlayerUserAgent`
- No `streamUrl` in URL/logs/screenshots
- Series is not playable
- Android request JSON unchanged
- Library detail still has no standing “打开 Emby 网页” link

## TDD Route

```text
TDD Route:
- Mode: off
- Decision: skipped
- Strict authority: not applicable
- Strict signals: contract, behavior, producer/consumer (recorded, not used to force TDD)
- Light eligibility: no
- TDD-fit exception: tdd_mode=off
- Test posture: post-change regression
- Reason: user-local Aegis tdd_mode is off; Media Hub verification is focused Go/Vitest/Playwright after each slice
- Verification: go test playback/httpapi; pnpm web test/lint/quality/build; playwright desktop+mobile
```

## Verification

```bash
go -C backend test ./internal/playback ./internal/httpapi
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web quality
pnpm --dir web build
pnpm --dir web test:e2e
```

Android source is not required to change. If OpenAPI comments in Android models exist, leave them; hand-written JSON stays valid.

```text
Aegis Visibility: Planning is required because this slice changes the public playback contract and adds a user-visible Web player without breaking Android minting.

Plan Basis: approved design spec 2026-09-01-web-library-player-design.md

BaselineUsageDraft:
- Required: ADR 0004, ADR 0006, AGENTS.md, OpenAPI PlaybackDescriptor, vision/roadmap deferred player
- Cited in plan: those plus the design spec
- Decision: continue

Requirement Ready Check:
- Decision: ready
- Source: design spec approved 2026-09-01

Change Necessity:
- User-visible need: play from Web library
- No-change option: Emby Web only — rejected by approved spec
- Minimum boundary: optional UA on descriptors + Web library overlay
- Decision: code-change

Existence Check:
- New UI owner files under web/src/features/library (player overlay, episode list)
- Reuse playback descriptor and session APIs
- Decision: add-with-proof for UI files; reuse-existing for playback APIs

Architecture Integrity Lens:
- Canonical owner: playback.Service for UA+URL minting; Web library for presentation
- Verdict: do not mint inside the browser or proxy bytes

Plan-Time Complexity Check:
- LibraryView.tsx ~400 lines — extract overlay and episode list
- Recommendation: add owner files, wiring-only in LibraryView

Execution Readiness View:
- Intent Lock: Web library movie/episode direct play
- Scope Fence: no 115 diagnostic player, no Android edits, no desktop shell
- Baseline Lock: no media proxy; optional UA additive
- Compatibility Boundary: default PlayerUserAgent
- Retirement Boundary: none
- Task Batches: contract → web API → library UI → e2e
- Test Obligations: commands in Verification
- Evidence Required Before Completion: passing commands above
- Advisory Boundary: method-pack only
```

## Files

Create:

- `web/src/features/library/LibraryPlayer.tsx`
- `web/src/features/library/LibraryPlayer.css`
- `web/src/features/library/LibraryEpisodes.tsx`
- `web/src/features/library/libraryPlaybackSession.ts`
- `web/src/features/library/isHlsStream.ts`
- tests beside those files

Modify:

- `api/openapi.yaml`
- `backend/internal/playback/service.go`
- `backend/internal/playback/service_test.go`
- `backend/internal/httpapi/playback.go`
- `backend/internal/httpapi/router.go` (interface signatures)
- `backend/internal/httpapi/playback_test.go`
- `web/src/shared/api/mediaHub.ts`
- `web/src/features/library/libraryPlayback.ts`
- `web/src/features/library/libraryPlayback.test.ts`
- `web/src/features/library/LibraryView.tsx`
- `web/e2e/apiFixtures.ts`
- `web/e2e/workspace.spec.ts`
- `web/package.json` (add `hls.js`)
- `docs/development/roadmap.md` (move Web built-in player out of Deferred for this slice — only the checkbox/note, not a product rewrite)

## Tasks

### Task 1 — Optional playback User-Agent in playback service

Files: `backend/internal/playback/service.go`, `backend/internal/playback/service_test.go`

Why: browsers cannot set `<video>` UA; mint must use the client-supplied UA.

Change Necessity: code-change; playback service is the canonical mint owner.

Impact: internal function signatures gain a UA string; HTTP layer in Task 2.

Steps:

1. Add `ResolvePlaybackUserAgent(value string) (string, error)` in `service.go`: trim; empty → `PlayerUserAgent`; length > 512 or ASCII controls → `ErrInvalidRequest`.
2. Change `CreateDrive115` and `CreateEmbyItem` to take `playbackUserAgent string`, run it through the resolver, pass that string into `ResolveDrive115` / `ResolveEmbyItem` / `ResolvePickCode`, and put it on the descriptor instead of hardcoding `PlayerUserAgent` inside `descriptor()`.
3. Update every `CreateDrive115` / `CreateEmbyItem` call in `service_test.go` to pass `""` for default-UA cases.
4. Add tests:
   - empty UA keeps `PlayerUserAgent` on resolver and descriptor
   - `"Mozilla/5.0 test"` is forwarded
   - 513-byte and `"bad\nua"` return `ErrInvalidRequest`

Verification:

```bash
go -C backend test ./internal/playback
```

Expect PASS.

### Task 2 — HTTP + OpenAPI wire-up

Files: `api/openapi.yaml`, `backend/internal/httpapi/playback.go`, `backend/internal/httpapi/router.go`, `backend/internal/httpapi/playback_test.go`

Why: Web must send the field; Android must keep working without it.

Steps:

1. Bump `info.version` patch (0.20.18 → 0.20.19).
2. Add optional `playbackUserAgent` (`maxLength: 512`) to `Drive115PlaybackTarget` and `EmbyPlaybackTarget`.
3. Decode the field on both create handlers; pass it into the service. Update `PlaybackService` interface.
4. HTTP tests: body without the field still 201; body with a UA still 201 (stub records it if the interface is updated to accept the string). Prefer threading UA through the stub so a test can assert `"Mozilla/5.0 Web"` is passed.

Verification:

```bash
go -C backend test ./internal/httpapi ./internal/playback
```

### Task 3 — Web API helpers

Files: `web/src/shared/api/mediaHub.ts` plus a focused test if request helpers are currently untested; otherwise cover via library unit tests in Task 4.

Add types `PlaybackDescriptor`, `EmbyEpisode`, and functions:

- `getEmbyEpisodes(id: string)`
- `createEmbyPlaybackDescriptor(itemId: string, playbackUserAgent: string)`
- `reportPlaybackSessionEvent(sessionId: string, event: 'started' | 'progress' | 'stopped', positionMs: number, paused: boolean)` → 204

POSTs use `writeHeaders()`. Do not include `playbackUserAgent` when the string is empty.

### Task 4 — Library labels, episodes, overlay wiring

Files: `libraryPlayback.ts`, `LibraryEpisodes.tsx`, `LibraryPlayer.tsx`, `LibraryPlayer.css`, `isHlsStream.ts`, `libraryPlaybackSession.ts`, `LibraryView.tsx`, matching `*.test.ts`

Behavior to implement exactly as spec.

`isHlsStream(url: string)`: parse URL; true when pathname ends with `.m3u8` (case-insensitive).

`playbackActionLabel` as Android.

`LibraryEpisodes`: list episodes with play buttons; no series-level play.

`LibraryPlayer`: dialog overlay; `<video>` or hls.js; error + `在 Emby 打开`; Escape/close clears play.

URL: `commitUrl({ play: itemId })` on start; `commitUrl({ play: null })` on close. Restore `play` from search params on load only when `media` is present.

Add `hls.js` dependency. Dynamic-import it from `LibraryPlayer` when `isHlsStream` is true so progressive playback does not need it at startup if the bundler allows; static import is acceptable if simpler.

Controls: play/pause button `min-height: 44px`, range input for seek, current/duration text ≥12px, fullscreen button.

Session helper mirrors Android 15s progress.

Vitest:

- `playbackActionLabel`
- `isHlsStream`
- episode list does not call play with the series id (component test via role queries if lightweight; otherwise unit the click handler mapping)

Do not grow `LibraryView.tsx` with player internals.

Verification:

```bash
pnpm --dir web test
pnpm --dir web lint
```

### Task 5 — Fixtures, e2e, quality, roadmap note

Files: `web/e2e/apiFixtures.ts`, `web/e2e/workspace.spec.ts`, `docs/development/roadmap.md`

Fixtures:

- `POST /playback/descriptors/emby` → 201 `{ streamUrl: 'https://cdn.example/acceptance.mp4', userAgent: 'test-ua', title: '验收影片' }` (no real media needed if e2e asserts overlay + request body rather than decoding)
- `GET .../items/series-1/episodes` → one episode `episode-1` with `externalUrl`

Because Playwright cannot decode a fake mp4 reliably, e2e should:

1. Click 播放 on 验收影片
2. Intercept descriptor POST and assert JSON body contains `playbackUserAgent` (non-empty)
3. Assert overlay dialog is visible and location has `play=item-1` without the cdn host
4. Simulate video error via `page.evaluate` on the video element or fixture a 502 descriptor on a second button path — simpler path: add a fixture movie `item-fail` OR stub descriptor 201 then dispatch `error` on `video`. Prefer: keep 201 and dispatch error, then assert `在 Emby 打开` and that it has `https://emby.example/...`
5. Series: add library item type Series in fixtures if missing; assert no series play button; episode play uses episode id

Keep existing assertion: library detail has zero links named `打开 Emby 网页`.

Roadmap: under Deferred, note Web library player is in progress / implemented for movies and episodes (do not claim production 115 probe).

Verification:

```bash
pnpm --dir web quality
pnpm --dir web build
pnpm --dir web test:e2e
```

## Risks

- 115 still 403 if `navigator.userAgent` reduced in some browsers — then Emby fallback; do not add a UA forge.
- HEVC/MKV will hit fallback often; that is accepted.
- `hls.js` CORS may fail on 115 HLS; same fallback.
- `LibraryView` URL restore must not reopen a player after navigating to Tasks (existing test clears `media`).

## Retirement

None. No old Web player to delete. Android player remains.

## Execution Route

```text
Execution Route:
- Decision: inline
- Evidence: sequential contract then UI then e2e; shared OpenAPI/library files
- Fallback: none
- User confirmation required: no
```
