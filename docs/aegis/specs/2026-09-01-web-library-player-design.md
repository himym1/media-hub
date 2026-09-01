# Web Library Player Design Spec

Date: `2026-09-01`
Status: `approved in conversation 2026-09-01; recorded here`
Owner: Media Hub playback service + Web library feature

## Goal

PC users play an already-indexed movie or episode from the Web library in the browser. Media Hub still only mints a temporary HTTPS descriptor and forwards opaque progress. Video bytes go 115 CDN → `<video>`.

## Approved decisions

1. v1 entry is library movies and series episodes, not 115 operations diagnostic play.
2. Descriptor requests gain optional `playbackUserAgent`. Web sends `navigator.userAgent`. Android omits the field and keeps `PlayerUserAgent`.
3. Decode or resolve failure stays on the in-page player with a manual “在 Emby 打开” action using that item’s `externalUrl`.
4. Player chrome is native `<video>` plus project-owned controls. `hls.js` loads only when `streamUrl` is HLS.

## Current → target

| Surface | Current | Target |
|---|---|---|
| `POST /playback/descriptors/*` | Always mint with `PlayerUserAgent` | Optional body `playbackUserAgent`; empty/omitted → `PlayerUserAgent` |
| Web library movie detail | Refresh, subtitles, delete | Same, plus 播放 / 继续播放 / 重新播放 |
| Web library series detail | No episode list | Episode list from existing Emby episodes API; play per episode |
| Web player | None | Overlay `<video>` for `play=<itemId>` |
| Android | Media3 + fixed player UA | Unchanged request body and playback UA |

## Behavior

### Playable targets

- Movie detail: playing the movie item id.
- Series detail: never create a descriptor for the series id. Load `GET /api/v1/integrations/emby/items/{id}/episodes` and play a selected episode id.
- 115 file browser play is out of v1.

### Labels

Match Android:

- `played` → `重新播放`
- `playbackPositionMs >= 30000` → `继续播放`
- otherwise → `播放`

Episode control accessible name: `{label} {episode label}` (for example `继续播放 第 2 集`).

### URL state

- Keep `view=library`, `library`, `media`.
- Add `play=<embyItemId>` while the overlay is open. That id is the movie or episode being played, which may differ from `media` on a series page.
- Back, Escape, and the overlay close control clear `play`, report `stopped` if a session exists, and destroy the media element.
- Never put `streamUrl`, User-Agent, pickcode, or credentials in the URL.

### Descriptor flow

1. `POST /api/v1/playback/descriptors/emby` with `{ itemId, playbackUserAgent: navigator.userAgent }` and CSRF.
2. On 201, attach `streamUrl` to `<video src>` for progressive HTTPS media.
3. If the URL path or query indicates HLS (`m3u8`), attach via `hls.js` instead of raw `src`.
4. Seek to `startPositionMs` after the media can seek.
5. If `sessionId` is present, report `started` when playback is ready, `progress` every 15s and on pause/play changes, `stopped` on close/ended. Positions are milliseconds. Failures to report are swallowed (same as Android).
6. Do not log `streamUrl` or `userAgent`.

### `playbackUserAgent` rules

- Optional string, max 512 characters after trim.
- Omitted, empty, or whitespace → `PlayerUserAgent`.
- Reject (400 `invalid_playback_request`) if longer than 512 or if it contains ASCII control characters (U+0000–U+001F, U+007F).
- The resolved value is passed to 115/Emby resolvers and copied to `PlaybackDescriptor.userAgent`.
- Apply the same field on `Drive115PlaybackTarget` for contract symmetry. Web v1 does not call that route.

### Player chrome

- Native `<video>` with project-owned play/pause, seek bar, time, and fullscreen.
- Keyboard: overlay is a modal dialog (`role="dialog"`, `aria-modal="true"`). Escape closes. Play/pause control is a 44px-min button.
- Autoplay after a successful descriptor. If the browser blocks autoplay, show the paused player with controls.
- Do not introduce ArtPlayer, Vidstack, media-chrome, or a second visual language.

### Failure

Show the overlay error state when:

- descriptor request fails
- `hls.js` or `video.error` fires
- the element reaches a non-playable error after load

Copy is the API problem title when present, otherwise `当前浏览器无法直接播放`. Primary recovery is a link-button **在 Emby 打开** that opens `externalUrl` in a new tab (`rel="noreferrer"`). Do not add a standing “打开 Emby 网页” control on library detail (existing e2e forbids it). Do not auto-redirect.

### Subtitles

v1 does not select or render subtitle tracks in the Web player. Existing “搜中文字幕” on detail remains an Emby maintenance action.

## Owners

- Contract: `api/openapi.yaml` (bump patch version).
- Minting: `backend/internal/playback.Service`; HTTP decode in `backend/internal/httpapi`.
- Web API: `web/src/shared/api/mediaHub.ts`.
- Web UI: `web/src/features/library/` — extract player overlay and episode list rather than growing `LibraryView.tsx` in place.
- Android: no required source change.

## Compatibility

- Android clients that omit `playbackUserAgent` keep today’s mint and descriptor UA.
- Additive JSON field; `additionalProperties: false` schemas gain one optional property.
- Playback session event schema unchanged.
- Four primary Web destinations unchanged.

## Non-goals

- Windows, Tauri, Electron, KMP
- 115 operations diagnostic player on Web
- Transcode or media byte proxy
- In-player subtitle picker
- Web replacement for Emby as the HEVC/MKV fallback

## Acceptance

1. Movie detail exposes a play action; overlay plays a fixture HTTPS stream without putting the stream URL in the location bar.
2. Series detail lists episodes; series itself has no play action; an episode play opens the overlay for that episode id.
3. Descriptor POST from Web includes `playbackUserAgent`; backend tests prove omitted UA still mints `PlayerUserAgent` and a supplied UA is forwarded to resolvers and the descriptor.
4. Overlay error state offers “在 Emby 打开” using the item `externalUrl`; library detail still has no standing Emby web link.
5. `pnpm --dir web test`, `lint`, `quality`, `build`, and Playwright `desktop` + `mobile` pass. `go -C backend test ./internal/playback ./internal/httpapi` pass.
6. Android debug unit tests still build against unchanged request bodies.

## ADR signal

Revive deferred “Web built-in player” as a follow-up ADR after implementation: Web library playback uses the same descriptor contract as Android, with client-supplied playback User-Agent, native `<video>`, and Emby `externalUrl` as decode fallback. Alternatives rejected: wrapping Web in Tauri/Electron; Compose Multiplatform; ArtPlayer; always using request `User-Agent` header (breaks Android Media3 vs API client UA split).

## Drafts that shaped this spec

```text
TaskIntentDraft:
- Outcome: PC plays library movies/episodes in Web
- Non-goals: desktop shell, 115 diagnostic play, proxy/transcode, in-player subtitles
- Success: direct play when the browser can decode; manual Emby fallback otherwise

ImpactStatementDraft:
- api + playback service + Web library
- Android request compatibility preserved
- Invariant: no media bytes through Media Hub
```
