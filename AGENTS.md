# Media Hub Agent Rules

## Product Boundary

- Media Hub is a single-user, single-NAS media automation control plane.
- The repository contains one backend, one Web client, one Android client, and may add a Windows desktop client when native decode is required.
- Media payloads must never be proxied through Media Hub. Playback remains 115 CDN to player.
- Media Hub owns STRM generation and `/115/url/` 302 playback. Emby owns library management and playback APIs.
- MoviePilot and local NAS upload workflows are outside the MVP.

## Architecture

- Keep the backend a Go modular monolith until measured pressure justifies a split.
- Keep one OpenAPI contract under `api/`; Web and Android clients consume the same semantics.
- Persist workflow state before executing external side effects.
- Every external operation must be timeout-bounded, retryable when safe, and idempotent.
- Never delete cloud or Emby media as a side effect of a failed workflow.

## Android

- Phone client is Android. Do not add iOS, Flutter, or Kotlin Multiplatform source sets.
- Use Kotlin, Jetpack Compose, and pinned stable dependencies.
- Material 3 is the presentation system. Business screens use project-owned `MediaHub*` wrappers instead of importing Material 3 components directly.
- Use unidirectional data flow, ViewModel, StateFlow, repositories, and explicit UI states.

## Desktop

- Windows desktop is allowed when the browser cannot decode the library stream (DTS / TrueHD / similar).
- Prefer Tauri wrapping the existing Web UI. Use native playback (mpv / FFmpeg or OS decoders) only for the player surface.
- Do not add Electron, or a second desktop UI stack, unless Tauri is proven insufficient.
- Desktop still consumes the same OpenAPI contract. Do not proxy media bytes through Media Hub.

## Web

- The first screen is the usable search experience, not a landing page.
- Use React, TypeScript, Vite, TanStack Query, and Lucide icons.
- Use native DOM controls and preserve keyboard and screen-reader support.
- Avoid generic admin dashboards, nested cards, decorative gradients, and feature-tour copy.

## Security

- Never commit secrets, cookies, tokens, passwords, private media names, or NAS exports.
- User-entered provider credentials stay server-side and encrypted at rest before real integrations ship.
- Logs must redact query credentials, share codes, direct URLs, and media-provider cookies.

## Verification

- Backend logic requires focused Go tests from `backend/`.
- Web changes require `pnpm build` and relevant tests from `web/`.
- Android changes require `./gradlew :app:assembleDebug` and focused unit/UI tests from `android/`.
- API contract changes require both client models to remain buildable.
- Desktop Rust changes require `cargo test --manifest-path desktop/src-tauri/Cargo.toml` from the repository root.
