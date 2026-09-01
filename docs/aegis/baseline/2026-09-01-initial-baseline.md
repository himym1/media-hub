# Media Hub Initial Baseline

Date: `2026-09-01`
Status: `initial dual-baseline snapshot`

## 1. Purpose

Give later Aegis alignment checks a dual baseline: product non-goals versus runtime owners. This snapshot records current authority, not a new product decision.

## 2. Workspace Structure

- `api/`: shared OpenAPI contract
- `backend/`: Go modular monolith, SQLite, provider adapters, playback descriptors
- `web/`: React, TypeScript, Vite, TanStack Query
- `android/`: Kotlin, Jetpack Compose, Media3 player
- `docs/architecture/`: ADRs and system design

## 3. Current Authority Surfaces

- `AGENTS.md`, `docs/product/vision.md`, `docs/development/roadmap.md`
- `docs/architecture/adr/0001` through `0006`
- `docs/architecture/system-design.md`
- Gap: no accepted ADR for Web library playback (deferred on the roadmap)

## 4. Product / Requirement Baseline

### 4.1 Current Truth

- Single-user NAS control plane: search, transfer, STRM, Emby index, WeCom
- Web is the desktop client; Android is the phone client
- Playback readiness is verified without proxying media bytes

### 4.2 Non-negotiables

1. Media payloads never transit Media Hub; playback is 115 CDN to the player.
2. Web and Android share OpenAPI semantics, not UI code.
3. Provider secrets stay server-side and encrypted at rest.
4. Android only for native apps: no iOS, desktop, Flutter, or KMP source sets.

### 4.3 Product Non-goals

- Windows / Tauri / Electron client
- Web built-in player was deferred until this workstream
- PT / MoviePilot, public registration, multi-tenant accounts

## 5. Architecture / Runtime Boundary Baseline

### 5.1 Current Truth

- Playback descriptors mint a temporary HTTPS `streamUrl` plus the User-Agent used to mint it
- Android Media3 plays with fixed `PlayerUserAgent`
- Emby owns library identity, resume, watched state, and progress APIs
- Web library shows metadata and Emby maintenance actions, not a player

### 5.2 Architecture Non-negotiables

1. Typed `Drive115Target` and `EmbyItemTarget` resolvers; adapters do not depend on each other.
2. A series is not a playable item; episodes are.
3. Cookie + CSRF for Web; bearer for Android.

### 5.3 Architecture Non-goals

- Transcoding or Range proxying inside Media Hub
- Compose Web / Canvas

## 6. Ownership / Contract Snapshot

- Descriptor minting: `backend/internal/playback`
- 115 download URL: `backend/internal/drive115`
- Emby library + progress: `backend/internal/emby`
- Android player: `android/.../playback` and `feature/player`
- Web library: `web/src/features/library`

## 7. Current State and Risks

- API `0.20.18`; Android library playback is experimental pending real probe
- Descriptor mint always uses `PlayerUserAgent`, which browsers cannot attach to `<video>`

## 8. Alignment Use

- Product baseline: whether a Windows client or in-page player is in scope
- Architecture baseline: who mints URLs, who plays bytes, what must not be proxied

## 9. Compatibility Boundary

- Existing Android descriptor requests with no `playbackUserAgent` must keep minting `PlayerUserAgent`
- Web must not place `streamUrl`, credentials, or media paths in the URL, logs, or screenshots
